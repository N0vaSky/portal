package websocket

import (
	"net/http"
	"time"

	"github.com/N0vaSky/portal/internal/api/middleware"
	ws "github.com/N0vaSky/portal/internal/services/websocket"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// Handler handles WebSocket connections
type Handler struct {
	service *ws.Service
	upgrader websocket.Upgrader
}

// NewHandler creates a new instance of Handler
func NewHandler(service *ws.Service) *Handler {
	return &Handler{
		service: service,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// In production, this should be more restrictive
				return true
			},
			HandshakeTimeout: 10 * time.Second,
		},
	}
}

// HandleAgentConnection handles WebSocket connections from agents
func (h *Handler) HandleAgentConnection(w http.ResponseWriter, r *http.Request) {
	// Get API key ID from context
	apiKeyID, ok := middleware.GetAPIKeyIDFromContext(r.Context())
	if !ok {
		http.Error(w, "API key not found in context", http.StatusInternalServerError)
		return
	}

	// Get hostname from query parameters
	hostname := r.URL.Query().Get("hostname")
	if hostname == "" {
		http.Error(w, "Hostname is required", http.StatusBadRequest)
		return
	}

	logrus.WithFields(logrus.Fields{
		"hostname":  hostname,
		"api_key_id": apiKeyID,
		"remote_addr": r.RemoteAddr,
	}).Info("Agent requesting WebSocket upgrade")

	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.WithError(err).WithField("hostname", hostname).Error("Failed to upgrade connection")
		return
	}

	// Register connection with service
	if err := h.service.RegisterConnection(hostname, conn); err != nil {
		logrus.WithError(err).WithField("hostname", hostname).Error("Failed to register connection")
		conn.Close()
		return
	}

	// Connection is now being handled by the service
}

// GetStats returns statistics about WebSocket connections
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context to verify permissions
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		return
	}

	// Get user role from context
	role, ok := middleware.GetUserRoleFromContext(r.Context())
	if !ok || (role != "admin" && role != "analyst") {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	// Get connection stats
	stats := h.service.GetConnectionStats()

	// Return response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		logrus.WithError(err).Error("Failed to encode response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// SendCommand sends a command to an agent
func (h *Handler) SendCommand(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		return
	}

	// Parse request
	var req struct {
		Command     string                 `json:"command"`
		Priority    string                 `json:"priority"`
		Details     map[string]interface{} `json:"details"`
		TargetNode  string                 `json:"target_node"`
		TargetGroup string                 `json:"target_group"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Command == "" {
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}
	if req.TargetNode == "" && req.TargetGroup == "" {
		http.Error(w, "Either target_node or target_group is required", http.StatusBadRequest)
		return
	}

	// Create command
	cmd := ws.Command{
		CommandID:   fmt.Sprintf("cmd-%d", time.Now().UnixNano()),
		Command:     req.Command,
		Priority:    req.Priority,
		Details:     req.Details,
		TargetNode:  req.TargetNode,
		TargetGroup: req.TargetGroup,
	}

	// Send command
	if err := h.service.SendCommand(cmd); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"command":     req.Command,
			"target_node": req.TargetNode,
			"user_id":     userID,
		}).Error("Failed to send command")
		http.Error(w, "Failed to send command", http.StatusInternalServerError)
		return
	}

	// Record command in database
	// TODO: Implement command recording

	// Return response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"command_id": cmd.CommandID,
		"status":     "queued",
	}); err != nil {
		logrus.WithError(err).Error("Failed to encode response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// IsolateNode sends an isolation command to a node
func (h *Handler) IsolateNode(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		return
	}

	// Get node ID from URL
	vars := mux.Vars(r)
	hostname := vars["hostname"]
	if hostname == "" {
		http.Error(w, "Hostname is required", http.StatusBadRequest)
		return
	}

	// Parse request
	var req struct {
		Reason string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default reason if not provided
		req.Reason = "Manual isolation by admin"
	}

	// Send isolation command
	err := h.service.SendIsolationCommand(hostname, req.Reason)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"hostname": hostname,
			"user_id":  userID,
		}).Error("Failed to send isolation command")
		
		http.Error(w, fmt.Sprintf("Failed to send isolation command: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "isolation_initiated",
		"message": "Isolation command sent to agent",
	}); err != nil {
		logrus.WithError(err).Error("Failed to encode response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}