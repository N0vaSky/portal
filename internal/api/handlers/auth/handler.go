package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/fibratus/portal/internal/config"
	"github.com/fibratus/portal/internal/services/auth/jwt"
	"github.com/fibratus/portal/internal/services/auth/mfa"
	"github.com/fibratus/portal/internal/services/users"
	"github.com/gorilla/sessions"
	"github.com/sirupsen/logrus"
)

// Handler handles authentication-related HTTP requests
type Handler struct {
	userService *users.Service
	jwtService  *jwt.Service
	mfaService  *mfa.Service
	store       *sessions.CookieStore
	config      *config.Config
}

// NewHandler creates a new instance of Handler
func NewHandler(userService *users.Service, jwtService *jwt.Service, mfaService *mfa.Service, cfg *config.Config) *Handler {
	// Create session store
	store := sessions.NewCookieStore([]byte(cfg.Auth.SessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   cfg.Auth.CookieSecure,
	}

	return &Handler{
		userService: userService,
		jwtService:  jwtService,
		mfaService:  mfaService,
		store:       store,
		config:      cfg,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	OTP      string `json:"otp,omitempty"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token       string      `json:"token"`
	User        users.User  `json:"user"`
	MFARequired bool        `json:"mfa_required"`
	MFASetup    *MFASetupData `json:"mfa_setup,omitempty"`
}

// MFASetupData represents MFA setup data
type MFASetupData struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
}

// Login handles user login requests
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Authenticate user
	user, err := h.userService.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		logrus.WithError(err).WithField("username", req.Username).Info("Login failed")
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// Check if MFA is enabled for the user
	if user.MFAEnabled {
		// If MFA is enabled, check for OTP
		if req.OTP == "" {
			// No OTP provided, require MFA
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(LoginResponse{
				MFARequired: true,
			})
			return
		}

		// Validate OTP
		secret, err := h.userService.GetMFASecret(user.ID)
		if err != nil {
			logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to get MFA secret")
			http.Error(w, "MFA verification failed", http.StatusInternalServerError)
			return
		}

		if !h.mfaService.ValidateCode(secret, req.OTP) {
			logrus.WithField("user_id", user.ID).Info("Invalid MFA code")
			http.Error(w, "Invalid MFA code", http.StatusUnauthorized)
			return
		}
	} else if h.config.Auth.MFAEnabled {
		// MFA is required by server config but not set up for user
		// Check if user has a secret already
		secret, err := h.userService.GetMFASecret(user.ID)
		if err != nil || secret == "" {
			// Generate a new MFA secret
			key, err := h.mfaService.GenerateSecret(user.Username)
			if err != nil {
				logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to generate MFA secret")
				http.Error(w, "Failed to generate MFA secret", http.StatusInternalServerError)
				return
			}

			// Save the secret
			if err := h.userService.SetupMFA(user.ID, key.Secret()); err != nil {
				logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to save MFA secret")
				http.Error(w, "Failed to save MFA secret", http.StatusInternalServerError)
				return
			}

			// Return the setup data
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(LoginResponse{
				MFARequired: true,
				MFASetup: &MFASetupData{
					Secret: key.Secret(),
					URL:    key.URL(),
				},
			})
			return
		}

		// Secret exists but not enabled, require OTP
		if req.OTP == "" {
			// No OTP provided, require MFA
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(LoginResponse{
				MFARequired: true,
			})
			return
		}

		// Validate OTP
		if !h.mfaService.ValidateCode(secret, req.OTP) {
			logrus.WithField("user_id", user.ID).Info("Invalid MFA code")
			http.Error(w, "Invalid MFA code", http.StatusUnauthorized)
			return
		}

		// Enable MFA for the user
		if err := h.userService.EnableMFA(user.ID); err != nil {
			logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to enable MFA")
			http.Error(w, "Failed to enable MFA", http.StatusInternalServerError)
			return
		}
	}

	// Generate JWT token
	expiration := time.Now().Add(24 * time.Hour)
	token, err := h.jwtService.GenerateToken(user.ID, user.Username, user.Role, expiration)
	if err != nil {
		logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to generate token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token: token,
		User:  *user,
	})
}

// SetupMFA handles MFA setup requests
func (h *Handler) SetupMFA(w http.ResponseWriter, r *http.Request) {
	// Get user ID from token
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Remove Bearer prefix
	tokenString = tokenString[7:]

	// Verify token
	claims, err := h.jwtService.VerifyToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Check if MFA is already enabled
	mfaEnabled, err := h.userService.IsMFAEnabled(claims.UserID)
	if err != nil {
		logrus.WithError(err).WithField("user_id", claims.UserID).Error("Failed to check MFA status")
		http.Error(w, "Failed to check MFA status", http.StatusInternalServerError)
		return
	}

	if mfaEnabled {
		http.Error(w, "MFA is already enabled", http.StatusBadRequest)
		return
	}

	// Generate a new MFA secret
	key, err := h.mfaService.GenerateSecret(claims.Username)
	if err != nil {
		logrus.WithError(err).WithField("user_id", claims.UserID).Error("Failed to generate MFA secret")
		http.Error(w, "Failed to generate MFA secret", http.StatusInternalServerError)
		return
	}

	// Save the secret
	if err := h.userService.SetupMFA(claims.UserID, key.Secret()); err != nil {
		logrus.WithError(err).WithField("user_id", claims.UserID).Error("Failed to save MFA secret")
		http.Error(w, "Failed to save MFA secret", http.StatusInternalServerError)
		return
	}

	// Return the setup data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MFASetupData{
		Secret: key.Secret(),
		URL:    key.URL(),
	})
}

// VerifyMFARequest represents an MFA verification request
type VerifyMFARequest struct {
	OTP string `json:"otp"`
}

// VerifyMFA handles MFA verification requests
func (h *Handler) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	// Get user ID from token
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Remove Bearer prefix
	tokenString = tokenString[7:]

	// Verify token
	claims, err := h.jwtService.VerifyToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Parse request
	var req VerifyMFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.OTP == "" {
		http.Error(w, "OTP is required", http.StatusBadRequest)
		return
	}

	// Get the MFA secret
	secret, err := h.userService.GetMFASecret(claims.UserID)
	if err != nil {
		logrus.WithError(err).WithField("user_id", claims.UserID).Error("Failed to get MFA secret")
		http.Error(w, "Failed to get MFA secret", http.StatusInternalServerError)
		return
	}

	// Validate the OTP
	if !h.mfaService.ValidateCode(secret, req.OTP) {
		http.Error(w, "Invalid OTP", http.StatusBadRequest)
		return
	}

	// Enable MFA for the user
	if err := h.userService.EnableMFA(claims.UserID); err != nil {
		logrus.WithError(err).WithField("user_id", claims.UserID).Error("Failed to enable MFA")
		http.Error(w, "Failed to enable MFA", http.StatusInternalServerError)
		return
	}

	// Generate a new token with MFA enabled
	user, err := h.userService.GetUserByID(claims.UserID)
	if err != nil {
		logrus.WithError(err).WithField("user_id", claims.UserID).Error("Failed to get user")
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	// Generate JWT token
	expiration := time.Now().Add(24 * time.Hour)
	newToken, err := h.jwtService.GenerateToken(user.ID, user.Username, user.Role, expiration)
	if err != nil {
		logrus.WithError(err).WithField("user_id", user.ID).Error("Failed to generate token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token: newToken,
		User:  *user,
	})
}