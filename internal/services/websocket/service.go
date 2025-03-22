package websocket

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/N0vaSky/portal/internal/services/nodes"
	"github.com/gorilla/websocket"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

// Service handles WebSocket connections and real-time communication
type Service struct {
	db           *sqlx.DB
	nodeService  *nodes.Service
	connections  map[string]*AgentConnection
	commandChan  chan Command
	mutex        sync.RWMutex
}

// AgentConnection represents a connected agent
type AgentConnection struct {
	NodeID       int
	Hostname     string
	Conn         *websocket.Conn
	Connected    time.Time
	LastActivity time.Time
}

// Command represents a real-time command to be sent to an agent
type Command struct {
	CommandID   string                 `json:"command_id"`
	Command     string                 `json:"command"`
	Priority    string                 `json:"priority,omitempty"`
	Details     map[string]interface{} `json:"details"`
	TargetNode  string                 `json:"target_node,omitempty"`
	TargetGroup string                 `json:"target_group,omitempty"`
}

// CommandResponse represents a response from an agent
type CommandResponse struct {
	CommandID  string                 `json:"command_id"`
	Status     string                 `json:"status"`
	Success    bool                   `json:"success,omitempty"`
	Message    string                 `json:"message,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Timestamp  string                 `json:"timestamp"`
}

// NewService creates a new instance of WebSocket service
func NewService(db *sqlx.DB, nodeService *nodes.Service) *Service {
	service := &Service{
		db:          db,
		nodeService: nodeService,
		connections: make(map[string]*AgentConnection),
		commandChan: make(chan Command, 1000),
	}

	// Start command dispatcher
	go service.commandDispatcher()

	// Start connection health checker
	go service.connectionHealthChecker()

	return service
}

// RegisterConnection registers a new WebSocket connection
func (s *Service) RegisterConnection(hostname string, conn *websocket.Conn) error {
	// Find the node by hostname
	node, err := s.nodeService.GetNodeByHostname(hostname)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Create agent connection
	agent := &AgentConnection{
		NodeID:       node.ID,
		Hostname:     hostname,
		Conn:         conn,
		Connected:    time.Now(),
		LastActivity: time.Now(),
	}

	// Register connection
	s.mutex.Lock()
	s.connections[hostname] = agent
	s.mutex.Unlock()

	logrus.WithFields(logrus.Fields{
		"hostname": hostname,
		"node_id":  node.ID,
	}).Info("Agent WebSocket connection established")

	// Update node status to online
	node.Status = "online"
	node.LastHeartbeat = time.Now()
	if err := s.nodeService.UpdateNode(node); err != nil {
		logrus.WithError(err).WithField("hostname", hostname).Warn("Failed to update node status")
	}

	// Start message listener for this connection
	go s.listenForMessages(agent)

	return nil
}

// SendCommand queues a command to be sent to an agent
func (s *Service) SendCommand(cmd Command) error {
	// Validate command
	if cmd.CommandID == "" {
		return fmt.Errorf("command ID is required")
	}
	if cmd.Command == "" {
		return fmt.Errorf("command type is required")
	}
	if cmd.TargetNode == "" && cmd.TargetGroup == "" {
		return fmt.Errorf("either target node or target group is required")
	}

	// Queue command
	s.commandChan <- cmd

	return nil
}

// SendIsolationCommand sends an immediate isolation command to an agent
func (s *Service) SendIsolationCommand(hostname string, reason string) error {
	// Create isolation command
	cmd := Command{
		CommandID: fmt.Sprintf("isolate-%d", time.Now().UnixNano()),
		Command:   "isolate-host",
		Priority:  "critical",
		Details: map[string]interface{}{
			"reason": reason,
			"allow_portal_communication": true,
			"allow_dns": true,
		},
		TargetNode: hostname,
	}

	// Get agent connection
	s.mutex.RLock()
	agent, exists := s.connections[hostname]
	s.mutex.RUnlock()

	if !exists {
		// Queue command for when agent connects
		s.commandChan <- cmd
		return fmt.Errorf("agent not currently connected, command queued")
	}

	// Send command immediately
	return s.sendCommandToAgent(agent, cmd)
}

// commandDispatcher processes and dispatches queued commands
func (s *Service) commandDispatcher() {
	for cmd := range s.commandChan {
		// Process by target type
		if cmd.TargetNode != "" {
			// Single node target
			s.dispatchToNode(cmd)
		} else if cmd.TargetGroup != "" {
			// Group target
			s.dispatchToGroup(cmd)
		}
	}
}

// dispatchToNode sends a command to a specific node
func (s *Service) dispatchToNode(cmd Command) {
	s.mutex.RLock()
	agent, exists := s.connections[cmd.TargetNode]
	s.mutex.RUnlock()

	if !exists {
		logrus.WithFields(logrus.Fields{
			"command_id": cmd.CommandID,
			"hostname":   cmd.TargetNode,
		}).Warn("Target node not connected, command will be queued for later delivery")
		
		// Store command in database for later delivery
		// TODO: Implement command persistence
		return
	}

	if err := s.sendCommandToAgent(agent, cmd); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"command_id": cmd.CommandID,
			"hostname":   cmd.TargetNode,
		}).Error("Failed to send command to agent")
	}
}

// dispatchToGroup sends a command to all nodes in a group
func (s *Service) dispatchToGroup(cmd Command) {
	// TODO: Implement group-based dispatch by querying nodes in the group
	logrus.WithFields(logrus.Fields{
		"command_id": cmd.CommandID,
		"group":      cmd.TargetGroup,
	}).Info("Group-based command dispatch not yet implemented")
}

// sendCommandToAgent sends a command to a specific agent
func (s *Service) sendCommandToAgent(agent *AgentConnection, cmd Command) error {
	// Serialize command
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	// Send command over WebSocket
	if err := agent.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	// Log command dispatch
	logrus.WithFields(logrus.Fields{
		"command_id": cmd.CommandID,
		"command":    cmd.Command,
		"hostname":   agent.Hostname,
		"priority":   cmd.Priority,
	}).Info("Command dispatched to agent")

	return nil
}

// listenForMessages handles incoming messages from agents
func (s *Service) listenForMessages(agent *AgentConnection) {
	defer func() {
		s.unregisterConnection(agent.Hostname)
	}()

	// Configure read deadline
	agent.Conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	agent.Conn.SetPongHandler(func(string) error {
		agent.Conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		agent.LastActivity = time.Now()
		return nil
	})

	// Message handling loop
	for {
		_, message, err := agent.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.WithError(err).WithField("hostname", agent.Hostname).Error("WebSocket read error")
			}
			break
		}

		// Update last activity
		agent.LastActivity = time.Now()

		// Process message
		if err := s.processAgentMessage(agent, message); err != nil {
			logrus.WithError(err).WithField("hostname", agent.Hostname).Error("Failed to process agent message")
		}
	}
}

// processAgentMessage processes a message received from an agent
func (s *Service) processAgentMessage(agent *AgentConnection, message []byte) error {
	// Try to parse as command response
	var resp CommandResponse
	if err := json.Unmarshal(message, &resp); err == nil && resp.CommandID != "" {
		// This is a command response
		return s.handleCommandResponse(agent, &resp)
	}

	// TODO: Handle other message types (heartbeat, alert, etc.)
	logrus.WithField("hostname", agent.Hostname).Debug("Received unrecognized message from agent")
	return nil
}

// handleCommandResponse processes a command response from an agent
func (s *Service) handleCommandResponse(agent *AgentConnection, resp *CommandResponse) error {
	logrus.WithFields(logrus.Fields{
		"hostname":   agent.Hostname,
		"command_id": resp.CommandID,
		"status":     resp.Status,
		"success":    resp.Success,
	}).Info("Received command response from agent")

	// TODO: Update command status in database
	// TODO: Trigger appropriate actions based on command response

	return nil
}

// unregisterConnection removes a WebSocket connection
func (s *Service) unregisterConnection(hostname string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if agent, exists := s.connections[hostname]; exists {
		// Close connection
		agent.Conn.Close()
		
		// Remove from connections map
		delete(s.connections, hostname)

		logrus.WithField("hostname", hostname).Info("Agent WebSocket connection closed")

		// Update node status to offline after a grace period
		// In production this should be done after confirming the agent hasn't reconnected
		go func() {
			time.Sleep(30 * time.Second)
			
			// Check if the agent has reconnected
			s.mutex.RLock()
			_, reconnected := s.connections[hostname]
			s.mutex.RUnlock()
			
			if !reconnected {
				// Update node status to offline
				node, err := s.nodeService.GetNodeByHostname(hostname)
				if err != nil {
					logrus.WithError(err).WithField("hostname", hostname).Error("Failed to get node for status update")
					return
				}
				
				node.Status = "offline"
				if err := s.nodeService.UpdateNode(node); err != nil {
					logrus.WithError(err).WithField("hostname", hostname).Error("Failed to update node status")
				}
			}
		}()
	}
}

// connectionHealthChecker periodically checks the health of WebSocket connections
func (s *Service) connectionHealthChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.checkConnections()
	}
}

// checkConnections verifies all connections are healthy
func (s *Service) checkConnections() {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	now := time.Now()
	for hostname, agent := range s.connections {
		// Check if the connection is stale (no activity for 2 minutes)
		if now.Sub(agent.LastActivity) > 2*time.Minute {
			logrus.WithField("hostname", hostname).Warn("Stale WebSocket connection, sending ping")
			
			// Send ping
			if err := agent.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				logrus.WithError(err).WithField("hostname", hostname).Error("Failed to send ping, closing connection")
				
				// Close and unregister in a separate goroutine to avoid deadlock
				go s.unregisterConnection(hostname)
			}
		}

		// Send periodic pings to keep connection alive
		if now.Sub(agent.LastActivity) > 30*time.Second {
			// Send ping
			if err := agent.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				logrus.WithError(err).WithField("hostname", hostname).Error("Failed to send ping, closing connection")
				
				// Close and unregister in a separate goroutine to avoid deadlock
				go s.unregisterConnection(hostname)
			}
		}
	}
}

// GetConnectionStats returns statistics about current WebSocket connections
func (s *Service) GetConnectionStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_connections": len(s.connections),
		"nodes": make([]map[string]interface{}, 0, len(s.connections)),
	}

	for hostname, agent := range s.connections {
		nodeStats := map[string]interface{}{
			"hostname":       hostname,
			"node_id":        agent.NodeID,
			"connected_at":   agent.Connected,
			"last_activity":  agent.LastActivity,
			"connection_age": time.Since(agent.Connected).String(),
		}
		stats["nodes"] = append(stats["nodes"].([]map[string]interface{}), nodeStats)
	}

	return stats
}