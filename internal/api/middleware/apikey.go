package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// APIKey context keys
const (
	APIKeyIDKey ContextKey = "api_key_id"
)

// APIKeyAuth middleware for validating API keys
func APIKeyAuth(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Check if the header starts with "ApiKey "
			if !strings.HasPrefix(authHeader, "ApiKey ") {
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			// Extract the API key
			apiKey := strings.TrimPrefix(authHeader, "ApiKey ")

			// Validate the API key
			var (
				keyID     int
				keyHash   string
				expiresAt sql.NullTime
			)

			err := db.QueryRow(
				"SELECT id, key_hash, expires_at FROM api_keys WHERE id = ?",
				apiKey,
			).Scan(&keyID, &keyHash, &expiresAt)

			if err != nil {
				logrus.WithError(err).Warn("Failed to find API key")
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Check if the API key has expired
			if expiresAt.Valid && expiresAt.Time.Before(r.Context().Value("now").(string)) {
				logrus.WithField("key_id", keyID).Warn("Expired API key")
				http.Error(w, "API key has expired", http.StatusUnauthorized)
				return
			}

			// Verify the API key hash
			if err := bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(apiKey)); err != nil {
				logrus.WithError(err).Warn("Invalid API key hash")
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Add API key information to request context
			ctx := context.WithValue(r.Context(), APIKeyIDKey, keyID)

			// Call the next handler with the updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAPIKeyIDFromContext extracts the API key ID from the request context
func GetAPIKeyIDFromContext(ctx context.Context) (int, bool) {
	keyID, ok := ctx.Value(APIKeyIDKey).(int)
	return keyID, ok
}