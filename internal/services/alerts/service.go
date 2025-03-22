package alerts

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

// Service handles alert-related operations
type Service struct {
	db             *sqlx.DB
	alertsJsonPath string
	mu             sync.Mutex // Mutex for file operations
}

// NewService creates a new instance of Service
func NewService(db *sqlx.DB, alertsJsonPath string) *Service {
	return &Service{
		db:             db,
		alertsJsonPath: alertsJsonPath,
	}
}

// Alert represents a security alert from Fibratus
type Alert struct {
	ID             int             `db:"id" json:"id"`
	NodeID         *int            `db:"node_id" json:"node_id,omitempty"`
	RuleID         *int            `db:"rule_id" json:"rule_id,omitempty"`
	AlertType      string          `db:"alert_type" json:"alert_type"`
	Severity       string          `db:"severity" json:"severity"`
	Title          string          `db:"title" json:"title"`
	Description    string          `db:"description" json:"description"`
	Metadata       json.RawMessage `db:"metadata" json:"metadata"`
	Timestamp      time.Time       `db:"timestamp" json:"timestamp"`
	Acknowledged   bool            `db:"acknowledged" json:"acknowledged"`
	AcknowledgedBy *int            `db:"acknowledged_by" json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time      `db:"acknowledged_at" json:"acknowledged_at,omitempty"`
}

// AlertSubmission represents an alert submitted by a Fibratus agent
type AlertSubmission struct {
	Hostname    string          `json:"hostname"`
	RuleName    string          `json:"rule_name,omitempty"`
	AlertType   string          `json:"alert_type"`
	Severity    string          `json:"severity"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Metadata    json.RawMessage `json:"metadata"`
	Timestamp   time.Time       `json:"timestamp"`
}

// AlertFilter defines filters for listing alerts
type AlertFilter struct {
	NodeID      int
	RuleID      int
	AlertType   string
	Severity    string
	Search      string
	StartTime   time.Time
	EndTime     time.Time
	Acknowledged *bool
	Limit       int
	Offset      int
}

// SubmitAlert processes an alert submission from a Fibratus agent
func (s *Service) SubmitAlert(submission *AlertSubmission) (*Alert, error) {
	// Start a transaction
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Find the node by hostname
	var nodeID *int
	var node struct {
		ID int `db:"id"`
	}
	err = tx.Get(&node, "SELECT id FROM nodes WHERE hostname = $1", submission.Hostname)
	if err == nil {
		nodeID = &node.ID
	} else {
		// If node not found, we'll still create the alert but without nodeID
		logrus.WithError(err).WithField("hostname", submission.Hostname).Warn("Node not found for alert")
	}

	// Find the rule by name if provided
	var ruleID *int
	if submission.RuleName != "" {
		var rule struct {
			ID int `db:"id"`
		}
		err = tx.Get(&rule, "SELECT id FROM rules WHERE name = $1", submission.RuleName)
		if err == nil {
			ruleID = &rule.ID
		} else {
			// If rule not found, we'll still create the alert but without ruleID
			logrus.WithError(err).WithField("rule_name", submission.RuleName).Warn("Rule not found for alert")
		}
	}

	// Create the alert
	alert := &Alert{
		NodeID:       nodeID,
		RuleID:       ruleID,
		AlertType:    submission.AlertType,
		Severity:     submission.Severity,
		Title:        submission.Title,
		Description:  submission.Description,
		Metadata:     submission.Metadata,
		Timestamp:    submission.Timestamp,
		Acknowledged: false,
	}

	// Insert the alert into the database
	query := `
		INSERT INTO alerts (
			node_id, rule_id, alert_type, severity, title, description, metadata, timestamp, acknowledged
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		) RETURNING id
	`

	row := tx.QueryRow(
		query,
		alert.NodeID, alert.RuleID, alert.AlertType, alert.Severity, alert.Title,
		alert.Description, alert.Metadata, alert.Timestamp, alert.Acknowledged,
	)

	if err := row.Scan(&alert.ID); err != nil {
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Update the alerts.json file
	if err := s.updateAlertsJson(); err != nil {
		logrus.WithError(err).Warn("Failed to update alerts.json file")
		// We don't return an error here because the alert was successfully created in the database
	}

	return alert, nil
}

// updateAlertsJson updates the alerts.json file with all alerts
func (s *Service) updateAlertsJson() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get all alerts
	alerts, err := s.ListAlerts(AlertFilter{Limit: 1000})
	if err != nil {
		return err
	}

	// Create the directory if it doesn't exist
	dir := filepath.Dir(s.alertsJsonPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal alerts to JSON
	data, err := json.MarshalIndent(alerts, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return ioutil.WriteFile(s.alertsJsonPath, data, 0644)
}

// GetAlertByID retrieves an alert by its ID
func (s *Service) GetAlertByID(id int) (*Alert, error) {
	var alert Alert
	err := s.db.Get(&alert, "SELECT * FROM alerts WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

// ListAlerts retrieves a list of alerts based on filters
func (s *Service) ListAlerts(filter AlertFilter) ([]Alert, error) {
	query := "SELECT * FROM alerts WHERE 1=1"
	var args []interface{}
	var argCount int

	if filter.NodeID > 0 {
		argCount++
		query += fmt.Sprintf(" AND node_id = $%d", argCount)
		args = append(args, filter.NodeID)
	}

	if filter.RuleID > 0 {
		argCount++
		query += fmt.Sprintf(" AND rule_id = $%d", argCount)
		args = append(args, filter.RuleID)
	}

	if filter.AlertType != "" {
		argCount++
		query += fmt.Sprintf(" AND alert_type = $%d", argCount)
		args = append(args, filter.AlertType)
	}

	if filter.Severity != "" {
		argCount++
		query += fmt.Sprintf(" AND severity = $%d", argCount)
		args = append(args, filter.Severity)
	}

	if filter.Search != "" {
		argCount++
		searchTerm := "%" + filter.Search + "%"
		query += fmt.Sprintf(" AND (title LIKE $%d OR description LIKE $%d)", argCount, argCount)
		args = append(args, searchTerm)
	}

	if !filter.StartTime.IsZero() {
		argCount++
		query += fmt.Sprintf(" AND timestamp >= $%d", argCount)
		args = append(args, filter.StartTime)
	}

	if !filter.EndTime.IsZero() {
		argCount++
		query += fmt.Sprintf(" AND timestamp <= $%d", argCount)
		args = append(args, filter.EndTime)
	}

	if filter.Acknowledged != nil {
		argCount++
		query += fmt.Sprintf(" AND acknowledged = $%d", argCount)
		args = append(args, *filter.Acknowledged)
	}

	// Add ordering
	query += " ORDER BY timestamp DESC"

	// Add limit and offset
	if filter.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)

		if filter.Offset > 0 {
			argCount++
			query += fmt.Sprintf(" OFFSET $%d", argCount)
			args = append(args, filter.Offset)
		}
	}

	var alerts []Alert
	err := s.db.Select(&alerts, query, args...)
	return alerts, err
}

// AcknowledgeAlert acknowledges an alert
func (s *Service) AcknowledgeAlert(id, userID int) error {
	// Start a transaction
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update the alert
	_, err = tx.Exec(
		"UPDATE alerts SET acknowledged = TRUE, acknowledged_by = $1, acknowledged_at = NOW() WHERE id = $2",
		userID, id,
	)
	if err != nil {
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return err
	}

	// Update the alerts.json file
	if err := s.updateAlertsJson(); err != nil {
		logrus.WithError(err).Warn("Failed to update alerts.json file")
		// We don't return an error here because the alert was successfully updated in the database
	}

	return nil
}

// GetAlertCounts retrieves alert counts by severity
func (s *Service) GetAlertCounts() (map[string]int, error) {
	type Count struct {
		Severity string `db:"severity"`
		Count    int    `db:"count"`
	}

	var counts []Count
	err := s.db.Select(&counts, "SELECT severity, COUNT(*) as count FROM alerts GROUP BY severity")
	if err != nil {
		return nil, err
	}

	result := make(map[string]int)
	for _, count := range counts {
		result[strings.ToLower(count.Severity)] = count.Count
	}

	return result, nil
}

// GetRecentAlerts retrieves recent alerts for a dashboard
func (s *Service) GetRecentAlerts(limit int) ([]Alert, error) {
	var alerts []Alert
	err := s.db.Select(
		&alerts,
		"SELECT * FROM alerts ORDER BY timestamp DESC LIMIT $1",
		limit,
	)
	return alerts, err
}

// DeleteOldAlerts deletes alerts older than a specified date
func (s *Service) DeleteOldAlerts(olderThan time.Time) (int, error) {
	result, err := s.db.Exec(
		"DELETE FROM alerts WHERE timestamp < $1",
		olderThan,
	)
	if err != nil {
		return 0, err
	}

	// Get the number of deleted rows
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	// Update the alerts.json file
	if err := s.updateAlertsJson(); err != nil {
		logrus.WithError(err).Warn("Failed to update alerts.json file")
		// We don't return an error here because the alerts were successfully deleted from the database
	}

	return int(affected), nil
}

// GetAlertTypes retrieves all distinct alert types
func (s *Service) GetAlertTypes() ([]string, error) {
	var types []string
	err := s.db.Select(&types, "SELECT DISTINCT alert_type FROM alerts ORDER BY alert_type")
	return types, err
}

// GetAlertSeverities retrieves all distinct alert severities
func (s *Service) GetAlertSeverities() ([]string, error) {
	var severities []string
	err := s.db.Select(&severities, "SELECT DISTINCT severity FROM alerts ORDER BY severity")
	return severities, err
}

// DeleteAlert deletes an alert
func (s *Service) DeleteAlert(id int) error {
	_, err := s.db.Exec("DELETE FROM alerts WHERE id = $1", id)
	if err != nil {
		return err
	}

	// Update the alerts.json file
	if err := s.updateAlertsJson(); err != nil {
		logrus.WithError(err).Warn("Failed to update alerts.json file")
		// We don't return an error here because the alert was successfully deleted from the database
	}

	return nil
}