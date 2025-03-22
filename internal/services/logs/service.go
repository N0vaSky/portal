package command

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fibratus/portal/internal/api/middleware"
	"github.com/fibratus/portal/internal/services/remediation"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Handler handles command-related HTTP requests
type Handler struct {
	service *remediation.Service
}

// NewHandler creates a new instance of Handler
func NewHandler(service *remediation.Service) *Handler {
	return &Handler{
		service: service,
	}
}

// ExecuteCommandRequest represents a request to execute a command
type ExecuteCommandRequest struct {
	NodeID         int             `json:"node_id"`
	CommandType    string          `json:"command_type"`
	CommandDetails json.RawMessage `json:"command_details"`
}

// ExecuteCommand handles requests to execute a command
func (h *Handler) ExecuteCommand(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req ExecuteCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.NodeID == 0 {
		http.Error(w, "Node ID is required", http.StatusBadRequest)
		return
	}
	if req.CommandType == "" {
		http.Error(w, "Command type is required", http.StatusBadRequest)
		return
	}

	// Validate command type
	cmdType := remediation.CommandType(req.CommandType)
	if !remediation.ValidateCommandType(cmdType) {
		http.Error(w, "Invalid command type", http.StatusBadRequest)
		return
	}

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusInternalServerError)
		return
	}

	// Create command
	cmd := &remediation.Command{
		NodeID:         req.NodeID,
		UserID:         userID,
		CommandType:    cmdType,
		CommandDetails: req.CommandDetails,
	}

	// Execute command
	if err := h.service.ExecuteCommand(cmd); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"node_id":      req.NodeID,
			"command_type": req.CommandType,
			"user_id":      userID,
		}).Error("Failed to execute command")
		http.Error(w, "Failed to execute command", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     cmd.ID,
		"status": "pending",
	})
}

// GetCommandDetails handles requests to get command details
func (h *Handler) GetCommandDetails(w http.ResponseWriter, r *http.Request) {
	// Extract command ID from path
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid command ID", http.StatusBadRequest)
		return
	}

	// Get command
	cmd, err := h.service.GetCommand(id)
	if err != nil {
		logrus.WithError(err).WithField("command_id", id).Error("Failed to get command")
		http.Error(w, "Command not found", http.StatusNotFound)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cmd)
}

// ListCommandHistory handles requests to list command history
func (h *Handler) ListCommandHistory(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	q := r.URL.Query()

	// Parse node ID
	var nodeID int
	if q.Get("node_id") != "" {
		var err error
		nodeID, err = strconv.Atoi(q.Get("node_id"))
		if err != nil {
			http.Error(w, "Invalid node ID", http.StatusBadRequest)
			return
		}
	}

	// Parse limit and offset
	var limit, offset int
	if q.Get("limit") != "" {
		var err error
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil {
			http.Error(w, "Invalid limit", http.StatusBadRequest)
			return
		}
	} else {
		limit = 10 // Default limit
	}

	if q.Get("offset") != "" {
		var err error
		offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil {
			http.Error(w, "Invalid offset", http.StatusBadRequest)
			return
		}
	}

	// Get commands
	commands, err := h.service.ListCommands(nodeID, limit, offset)
	if err != nil {
		logrus.WithError(err).Error("Failed to list commands")
		http.Error(w, "Failed to list commands", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"commands": commands,
	})
}

// CancelCommand handles requests to cancel a command
func (h *Handler) CancelCommand(w http.ResponseWriter, r *http.Request) {
	// Extract command ID from path
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid command ID", http.StatusBadRequest)
		return
	}

	// Cancel command
	if err := h.service.CancelCommand(id); err != nil {
		logrus.WithError(err).WithField("command_id", id).Error("Failed to cancel command")
		http.Error(w, "Failed to cancel command", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "canceled",
	})
}