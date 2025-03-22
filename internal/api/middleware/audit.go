package middleware

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

// auditableRoutes defines routes that should be audited
var auditableRoutes = map[string]map[string]string{
	"POST /api/v1/nodes/{id}/actions/isolate":   {"resource_type": "node", "action": "isolate"},
	"POST /api/v1/nodes/{id}/actions/unisolate": {"resource_type": "node", "action": "unisolate"},
	"POST /api/v1/rules":                        {"resource_type": "rule", "action": "create"},
	"PUT /api/v1/rules/{id}":                    {"resource_type": "rule", "action": "update"},
	"DELETE /api/v1/rules/{id}":                 {"resource_type": "rule", "action": "delete"},
	"POST /api/v1/alerts/{id}/acknowledge":      {"resource_type": "alert", "action": "acknowledge"},
	"POST /api/v1/commands/execute":             {"resource_type": "command", "action": "execute"},
	"POST /api/v1/configs":                      {"resource_type": "config", "action": "create"},
	"PUT /api/v1/configs/{id}":                  {"resource_type": "config", "action": "update"},
	"DELETE /api/v1/configs/{id}":               {"resource_type": "config", "action": "delete"},
	"POST /api/v1/users":                        {"resource_type": "user", "action": "create"},
	"PUT /api/v1/users/{id}":                    {"resource_type": "user", "action": "update"},
	"DELETE /api/v1/users/{id}":                 {"resource_type": "user", "action": "delete"},
	"POST /api/v1/api-keys":                     {"resource_type": "api_key", "action": "create"},
	"DELETE /api/v1/api-keys/{id}":              {"resource_type": "api_key", "action": "delete"},
}

// routeRegex is used to match parameterized routes
var routeRegex = regexp.MustCompile(`\{[^/]+\}`)

// Audit middleware logs auditable actions
func Audit(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Process the request first
			next.ServeHTTP(w, r)

			// Check if this route should be audited
			route := strings.TrimSuffix(mux.CurrentRoute(r).GetPathTemplate(), "/")
			method := r.Method
			key := method + " " + route

			// Check if this is an auditable route
			auditInfo, shouldAudit := auditableRoutes[key]
			if !shouldAudit {
				return
			}

			// Get user ID from context
			userID, ok := GetUserIDFromContext(r.Context())
			if !ok {
				logrus.Warn("Failed to get user ID for audit log")
				return
			}

			// Extract resource ID from path variables
			vars := mux.Vars(r)
			resourceID := vars["id"]

			// Extract client IP
			clientIP := r.RemoteAddr
			if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
				clientIP = forwardedFor
			}

			// Prepare details based on request body
			var details map[string]interface{}
			if r.Body != nil {
				if err := json.NewDecoder(r.Body).Decode(&details); err != nil {
					logrus.WithError(err).Warn("Failed to parse request body for audit log")
					details = make(map[string]interface{})
				}
			} else {
				details = make(map[string]interface{})
			}

			// Add the audit log entry
			_, err := db.Exec(
				"INSERT INTO audit_log (user_id, action, resource_type, resource_id, details, ip_address) VALUES (?, ?, ?, ?, ?, ?)",
				userID, auditInfo["action"], auditInfo["resource_type"], resourceID, details, clientIP,
			)
			if err != nil {
				logrus.WithError(err).Error("Failed to insert audit log entry")
			}
		})
	}
}