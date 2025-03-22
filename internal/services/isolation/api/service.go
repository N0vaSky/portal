package isolation

import (
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

// Service handles network isolation operations
type Service struct {
	db *sqlx.DB
}

// NewService creates a new instance of Service
func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

// IsolationAction represents the isolation actions that can be performed
type IsolationAction string

const (
	// Isolate represents the action to isolate a node
	Isolate IsolationAction = "isolate"
	// Unisolate represents the action to remove isolation from a node
	Unisolate IsolationAction = "unisolate"
)

// IsolationCommand represents a command to change a node's isolation status
type IsolationCommand struct {
	NodeID    int             `json:"node_id"`
	Action    IsolationAction `json:"action"`
	UserID    int             `json:"user_id"`
	Timestamp time.Time       `json:"timestamp"`
}

// IsolationResult represents the result of an isolation command
type IsolationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// IsolateNode isolates a node from the network
func (s *Service) IsolateNode(nodeID, userID int) (*IsolationResult, error) {
	// Start a transaction
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check if the node exists and is not already isolated
	var (
		exists   int
		isolated bool
	)
	
	err = tx.Get(&exists, "SELECT COUNT(*) FROM nodes WHERE id = $1", nodeID)
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, errors.New("node not found")
	}
	
	err = tx.Get(&isolated, "SELECT isolated FROM nodes WHERE id = $1", nodeID)
	if err != nil {
		return nil, err
	}
	if isolated {
		return &IsolationResult{
			Success: false,
			Message: "Node is already isolated",
		}, nil
	}

	// Record the isolation command
	_, err = tx.Exec(
		`INSERT INTO command_history (
			node_id, user_id, command_type, command_details, status, executed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)`,
		nodeID, userID, "isolate", 
		map[string]interface{}{"action": "isolate"}, 
		"pending", time.Now(),
	)
	if err != nil {
		return nil, err
	}

	// Update the node's isolation status
	_, err = tx.Exec(
		"UPDATE nodes SET isolated = TRUE, updated_at = NOW() WHERE id = $1",
		nodeID,
	)
	if err != nil {
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"node_id": nodeID,
		"user_id": userID,
		"action":  "isolate",
	}).Info("Node isolated successfully")

	return &IsolationResult{
		Success: true,
		Message: "Node isolated successfully",
	}, nil
}

// UnisolateNode removes network isolation from a node
func (s *Service) UnisolateNode(nodeID, userID int) (*IsolationResult, error) {
	// Start a transaction
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check if the node exists and is isolated
	var (
		exists   int
		isolated bool
	)
	
	err = tx.Get(&exists, "SELECT COUNT(*) FROM nodes WHERE id = $1", nodeID)
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, errors.New("node not found")
	}
	
	err = tx.Get(&isolated, "SELECT isolated FROM nodes WHERE id = $1", nodeID)
	if err != nil {
		return nil, err
	}
	if !isolated {
		return &IsolationResult{
			Success: false,
			Message: "Node is not currently isolated",
		}, nil
	}

	// Record the unisolation command
	_, err = tx.Exec(
		`INSERT INTO command_history (
			node_id, user_id, command_type, command_details, status, executed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)`,
		nodeID, userID, "unisolate", 
		map[string]interface{}{"action": "unisolate"}, 
		"pending", time.Now(),
	)
	if err != nil {
		return nil, err
	}

	// Update the node's isolation status
	_, err = tx.Exec(
		"UPDATE nodes SET isolated = FALSE, updated_at = NOW() WHERE id = $1",
		nodeID,
	)
	if err != nil {
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"node_id": nodeID,
		"user_id": userID,
		"action":  "unisolate",
	}).Info("Node unisolated successfully")

	return &IsolationResult{
		Success: true,
		Message: "Node unisolated successfully",
	}, nil
}

// GetIsolatedNodes retrieves all isolated nodes
func (s *Service) GetIsolatedNodes() ([]int, error) {
	var nodeIDs []int
	err := s.db.Select(&nodeIDs, "SELECT id FROM nodes WHERE isolated = TRUE")
	return nodeIDs, err
}

// GetIsolationHistory retrieves the isolation history for a node
func (s *Service) GetIsolationHistory(nodeID int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			ch.id, 
			ch.node_id, 
			ch.user_id, 
			u.username AS username,
			ch.command_type, 
			ch.command_details, 
			ch.status, 
			ch.executed_at 
		FROM command_history ch
		LEFT JOIN users u ON ch.user_id = u.id
		WHERE ch.node_id = $1 AND (ch.command_type = 'isolate' OR ch.command_type = 'unisolate')
		ORDER BY ch.executed_at DESC
	`
	
	var history []map[string]interface{}
	err := s.db.Select(&history, query, nodeID)
	return history, err
}