package agent

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fibratus/portal/internal/services/alerts"
	"github.com/fibratus/portal/internal/services/configs"
	"github.com/fibratus/portal/internal/services/nodes"
	"github.com/fibratus/portal/internal/services/remediation"
	"github.com/fibratus/portal/internal/services/rules"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Handler handles agent-related HTTP requests
type Handler struct {
	nodeService        *nodes.Service
	ruleService        *rules.Service
	alertService       *alerts.Service
	remediationService *remediation.Service
	configService      *configs.Service
}

// NewHandler creates a new instance of Handler
func NewHandler(
	nodeService *nodes.Service,
	ruleService *rules.Service,
	alertService *alerts.Service,
	remediationService *remediation.Service,
	configService *configs.Service,
) *Handler {
	return &Handler{
		nodeService:        nodeService,
		ruleService:        ruleService,
		alertService:       alertService,
		remediationService: remediationService,
		configService:      configService,
	}
}

// Heartbeat handles heartbeat requests from agents
func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req nodes.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Hostname == "" {
		http.Error(w, "Hostname is required", http.StatusBadRequest)
		return
	}

	// Process heartbeat
	node, err := h.nodeService.ProcessHeartbeat(req)
	if err != nil {
		logrus.WithError(err).WithField("hostname", req.Hostname).Error("Failed to process heartbeat")
		http.Error(w, "Failed to process heartbeat", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"node_id":  node.ID,
		"isolated": node.Isolated,
	})
}

// GetRules handles requests for rules from agents
func (h *Handler) GetRules(w http.ResponseWriter, r *http.Request) {
	// Get hostname from query
	hostname := r.URL.Query().Get("hostname")
	if hostname == "" {
		http.Error(w, "Hostname is required", http.StatusBadRequest)
		return
	}

	// Get rules for the node
	rules, err := h.ruleService.GetRulesForNodeByHostname(hostname)
	if err != nil {
		logrus.WithError(err).WithField("hostname", hostname).Error("Failed to get rules for node")
		http.Error(w, "Failed to get rules for node", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": rules,
	})
}

// GetConfig handles requests for configuration from agents
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	// Get hostname from query
	hostname := r.URL.Query().Get("hostname")
	if hostname == "" {
		http.Error(w, "Hostname is required", http.StatusBadRequest)
		return
	}

	// Get config for the node (this is a placeholder, implement the actual config service)
	config, err := h.configService.GetConfigForNodeByHostname(hostname)
	if err != nil {
		logrus.WithError(err).WithField("hostname", hostname).Error("Failed to get config for node")
		http.Error(w, "Failed to get config for node", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// SubmitAlert handles alert submissions from agents
func (h *Handler) SubmitAlert(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req alerts.AlertSubmission
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Hostname == "" {
		http.Error(w, "Hostname is required", http.StatusBadRequest)
		return
	}
	if req.AlertType == "" {
		http.Error(w, "Alert type is required", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	// Process alert
	alert, err := h.alertService.SubmitAlert(&req)
	if err != nil {
		logrus.WithError(err).WithField("hostname", req.Hostname).Error("Failed to process alert")
		http.Error(w, "Failed to process alert", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"alert_id": alert.ID,
	})
}

// GetPendingCommands handles requests for pending commands from agents
func (h *Handler) GetPendingCommands(w http.ResponseWriter, r *http.Request) {
	// Get hostname from query
	hostname := r.URL.Query().Get("hostname")
	if hostname == "" {
		http.Error(w, "Hostname is required", http.StatusBadRequest)
		return
	}

	// Get pending commands for the node
	commands, err := h.remediationService.GetPendingCommandsByHostname(hostname)
	if err != nil {
		logrus.WithError(err).WithField("hostname", hostname).Error("Failed to get pending commands")
		http.Error(w, "Failed to get pending commands", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"commands": commands,
	})
}

// SubmitCommandResult handles command result submissions from agents
func (h *Handler) SubmitCommandResult(w http.ResponseWriter, r *http.Request) {
	// Get command ID from path
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid command ID", http.StatusBadRequest)
		return
	}

	// Parse request
	var result remediation.CommandResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update command result
	if err := h.remediationService.UpdateCommandResult(id, &result); err != nil {
		logrus.WithError(err).WithField("command_id", id).Error("Failed to update command result")
		http.Error(w, "Failed to update command result", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}