package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/N0vaSky/portal/internal/api/handlers"
	"github.com/N0vaSky/portal/internal/api/handlers/websocket"
	"github.com/N0vaSky/portal/internal/api/middleware"
	"github.com/N0vaSky/portal/internal/config"
	"github.com/N0vaSky/portal/internal/services/websocket"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

// NewRouter creates a new HTTP router for the API
func NewRouter(cfg *config.Config, db *sqlx.DB) http.Handler {
	router := mux.NewRouter()

	// Create handler dependencies
	deps := handlers.NewHandlerDependencies(cfg, db)

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}).Methods("GET")

	// API versioning
	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	// Public routes
	apiRouter.HandleFunc("/login", deps.AuthHandler.Login).Methods("POST")
	apiRouter.HandleFunc("/mfa/setup", deps.AuthHandler.SetupMFA).Methods("GET")
	apiRouter.HandleFunc("/mfa/verify", deps.AuthHandler.VerifyMFA).Methods("POST")

	// Protected routes
	protectedRouter := apiRouter.NewRoute().Subrouter()
	protectedRouter.Use(middleware.JWTAuth(cfg.Auth.JWTSecret))
	protectedRouter.Use(middleware.Logging)
	protectedRouter.Use(middleware.Audit(db))

	// Node management
	protectedRouter.HandleFunc("/nodes", deps.NodeHandler.ListNodes).Methods("GET")
	protectedRouter.HandleFunc("/nodes/{id}", deps.NodeHandler.GetNode).Methods("GET")
	protectedRouter.HandleFunc("/nodes/{id}/actions/isolate", deps.NodeHandler.IsolateNode).Methods("POST")
	protectedRouter.HandleFunc("/nodes/{id}/actions/unisolate", deps.NodeHandler.UnisolateNode).Methods("POST")
	protectedRouter.HandleFunc("/nodes/groups", deps.NodeHandler.ListGroups).Methods("GET")
	protectedRouter.HandleFunc("/nodes/groups", deps.NodeHandler.CreateGroup).Methods("POST")
	protectedRouter.HandleFunc("/nodes/groups/{id}", deps.NodeHandler.GetGroup).Methods("GET")
	protectedRouter.HandleFunc("/nodes/groups/{id}", deps.NodeHandler.UpdateGroup).Methods("PUT")
	protectedRouter.HandleFunc("/nodes/groups/{id}", deps.NodeHandler.DeleteGroup).Methods("DELETE")

	// Rule management
	protectedRouter.HandleFunc("/rules", deps.RuleHandler.ListRules).Methods("GET")
	protectedRouter.HandleFunc("/rules", deps.RuleHandler.CreateRule).Methods("POST")
	protectedRouter.HandleFunc("/rules/{id}", deps.RuleHandler.GetRule).Methods("GET")
	protectedRouter.HandleFunc("/rules/{id}", deps.RuleHandler.UpdateRule).Methods("PUT")
	protectedRouter.HandleFunc("/rules/{id}", deps.RuleHandler.DeleteRule).Methods("DELETE")
	protectedRouter.HandleFunc("/rules/{id}/versions", deps.RuleHandler.ListRuleVersions).Methods("GET")
	protectedRouter.HandleFunc("/rules/{id}/versions/{version}", deps.RuleHandler.GetRuleVersion).Methods("GET")
	protectedRouter.HandleFunc("/rules/assignments", deps.RuleHandler.ListRuleAssignments).Methods("GET")
	protectedRouter.HandleFunc("/rules/assignments", deps.RuleHandler.CreateRuleAssignment).Methods("POST")
	protectedRouter.HandleFunc("/rules/assignments/{id}", deps.RuleHandler.DeleteRuleAssignment).Methods("DELETE")

	// Alert management
	protectedRouter.HandleFunc("/alerts", deps.AlertHandler.ListAlerts).Methods("GET")
	protectedRouter.HandleFunc("/alerts/{id}", deps.AlertHandler.GetAlert).Methods("GET")
	protectedRouter.HandleFunc("/alerts/{id}/acknowledge", deps.AlertHandler.AcknowledgeAlert).Methods("POST")

	// Log management
	protectedRouter.HandleFunc("/logs", deps.LogHandler.ListLogs).Methods("GET")
	protectedRouter.HandleFunc("/logs/collect", deps.LogHandler.CollectLogs).Methods("POST")

	// Command execution
	protectedRouter.HandleFunc("/commands", deps.CommandHandler.ListCommandHistory).Methods("GET")
	protectedRouter.HandleFunc("/commands/execute", deps.CommandHandler.ExecuteCommand).Methods("POST")
	protectedRouter.HandleFunc("/commands/{id}", deps.CommandHandler.GetCommandDetails).Methods("GET")

	// Configuration management
	protectedRouter.HandleFunc("/configs", deps.ConfigHandler.ListConfigs).Methods("GET")
	protectedRouter.HandleFunc("/configs", deps.ConfigHandler.CreateConfig).Methods("POST")
	protectedRouter.HandleFunc("/configs/{id}", deps.ConfigHandler.GetConfig).Methods("GET")
	protectedRouter.HandleFunc("/configs/{id}", deps.ConfigHandler.UpdateConfig).Methods("PUT")
	protectedRouter.HandleFunc("/configs/{id}", deps.ConfigHandler.DeleteConfig).Methods("DELETE")
	protectedRouter.HandleFunc("/configs/{id}/versions", deps.ConfigHandler.ListConfigVersions).Methods("GET")
	protectedRouter.HandleFunc("/configs/assignments", deps.ConfigHandler.ListConfigAssignments).Methods("GET")
	protectedRouter.HandleFunc("/configs/assignments", deps.ConfigHandler.CreateConfigAssignment).Methods("POST")
	protectedRouter.HandleFunc("/configs/assignments/{id}", deps.ConfigHandler.DeleteConfigAssignment).Methods("DELETE")

	// User management
	protectedRouter.HandleFunc("/users", deps.UserHandler.ListUsers).Methods("GET")
	protectedRouter.HandleFunc("/users", deps.UserHandler.CreateUser).Methods("POST")
	protectedRouter.HandleFunc("/users/{id}", deps.UserHandler.GetUser).Methods("GET")
	protectedRouter.HandleFunc("/users/{id}", deps.UserHandler.UpdateUser).Methods("PUT")
	protectedRouter.HandleFunc("/users/{id}", deps.UserHandler.DeleteUser).Methods("DELETE")
	protectedRouter.HandleFunc("/users/me", deps.UserHandler.GetCurrentUser).Methods("GET")
	protectedRouter.HandleFunc("/users/me/password", deps.UserHandler.ChangePassword).Methods("POST")

	// API Key management
	protectedRouter.HandleFunc("/api-keys", deps.APIKeyHandler.ListAPIKeys).Methods("GET")
	protectedRouter.HandleFunc("/api-keys", deps.APIKeyHandler.CreateAPIKey).Methods("POST")
	protectedRouter.HandleFunc("/api-keys/{id}", deps.APIKeyHandler.DeleteAPIKey).Methods("DELETE")

	// Audit log
	protectedRouter.HandleFunc("/audit-log", deps.AuditHandler.ListAuditLog).Methods("GET")

	// WebSocket management
	protectedRouter.HandleFunc("/websocket/stats", deps.WebSocketHandler.GetStats).Methods("GET")
	protectedRouter.HandleFunc("/websocket/command", deps.WebSocketHandler.SendCommand).Methods("POST")
	protectedRouter.HandleFunc("/websocket/isolate/{hostname}", deps.WebSocketHandler.IsolateNode).Methods("POST")

	// Agent API (used by the Fibratus agents)
	agentRouter := router.PathPrefix("/agent").Subrouter()
	agentRouter.Use(middleware.APIKeyAuth(db))

	agentRouter.HandleFunc("/heartbeat", deps.AgentHandler.Heartbeat).Methods("POST")
	agentRouter.HandleFunc("/rules", deps.AgentHandler.GetRules).Methods("GET")
	agentRouter.HandleFunc("/config", deps.AgentHandler.GetConfig).Methods("GET")
	agentRouter.HandleFunc("/alerts", deps.AgentHandler.SubmitAlert).Methods("POST")
	agentRouter.HandleFunc("/commands", deps.AgentHandler.GetPendingCommands).Methods("GET")
	agentRouter.HandleFunc("/commands/{id}/result", deps.AgentHandler.SubmitCommandResult).Methods("POST")
	
	// WebSocket endpoint for agents
	agentRouter.HandleFunc("/ws", deps.WebSocketHandler.HandleAgentConnection)

	// Serve static files for web interface
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("/usr/share/fibratus/web")))

	// Set up CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	})

	logrus.Info("Router initialized successfully")
	return c.Handler(router)
}