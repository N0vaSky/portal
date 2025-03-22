package jwt

import (
	"errors"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// Service handles JWT token operations
type Service struct {
	secret []byte
}

// NewService creates a new instance of Service
func NewService(secret string) *Service {
	return &Service{
		secret: []byte(secret),
	}
}

// Claims represents the JWT claims
type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.StandardClaims
}

// GenerateToken generates a JWT token for a user
func (s *Service) GenerateToken(userID int, username, role string, expiration time.Time) (string, error) {
	// Create the claims
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expiration.Unix(),
			IssuedAt:  time.Now().Unix(),
		},
	}

	// Create the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret
	return token.SignedString(s.secret)
}

// VerifyToken verifies a JWT token and returns the claims
func (s *Service) VerifyToken(tokenString string) (*Claims, error) {
	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})

	if err != nil {
		return nil, err
	}

	// Check if the token is valid
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Extract the claims
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims")
	}

	return claims, nil
}

// RefreshToken refreshes a JWT token
func (s *Service) RefreshToken(tokenString string, newExpiration time.Time) (string, error) {
	// Verify the token
	claims, err := s.VerifyToken(tokenString)
	if err != nil {
		return "", err
	}

	// Create a new token with the same claims but a new expiration
	return s.GenerateToken(claims.UserID, claims.Username, claims.Role, newExpiration)
}