package rules

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

// Service handles rule management operations
type Service struct {
	db               *sqlx.DB
	defaultRulesPath string
}

// NewService creates a new instance of Service
func NewService(db *sqlx.DB, defaultRulesPath string) *Service {
	return &Service{
		db:               db,
		defaultRulesPath: defaultRulesPath,
	}
}

// Rule represents a Fibratus rule
type Rule struct {
	ID          int       `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Content     string    `db:"content" json:"content"`
	Version     int       `db:"version" json:"version"`
	Active      bool      `db:"active" json:"active"`
	CreatedBy   int       `db:"created_by" json:"created_by"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// RuleVersion represents a historical version of a rule
type RuleVersion struct {
	ID        int       `db:"id" json:"id"`
	RuleID    int       `db:"rule_id" json:"rule_id"`
	Version   int       `db:"version" json:"version"`
	Content   string    `db:"content" json:"content"`
	CreatedBy int       `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// RuleAssignment represents a rule assigned to a node or group
type RuleAssignment struct {
	ID        int       `db:"id" json:"id"`
	RuleID    int       `db:"rule_id" json:"rule_id"`
	NodeID    *int      `db:"node_id" json:"node_id,omitempty"`
	GroupID   *int      `db:"group_id" json:"group_id,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// RuleFilter defines filters for listing rules
type RuleFilter struct {
	Active bool
	Search string
	Limit  int
	Offset int
}

// GetRuleByID retrieves a rule by its ID
func (s *Service) GetRuleByID(id int) (*Rule, error) {
	var rule Rule
	err := s.db.Get(&rule, "SELECT * FROM rules WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// GetRuleByName retrieves a rule by its name
func (s *Service) GetRuleByName(name string) (*Rule, error) {
	var rule Rule
	err := s.db.Get(&rule, "SELECT * FROM rules WHERE name = $1", name)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListRules retrieves a list of rules based on filters
func (s *Service) ListRules(filter RuleFilter) ([]Rule, error) {
	query := "SELECT * FROM rules WHERE 1=1"
	var args []interface{}
	var argCount int

	if filter.Active {
		argCount++
		query += " AND active = $" + string(argCount)
		args = append(args, true)
	}

	if filter.Search != "" {
		argCount++
		query += " AND (name LIKE $" + string(argCount) + " OR description LIKE $" + string(argCount) + ")"
		args = append(args, "%"+filter.Search+"%")
	}

	// Add ordering
	query += " ORDER BY name"

	// Add limit and offset
	if filter.Limit > 0 {
		argCount++
		query += " LIMIT $" + string(argCount)
		args = append(args, filter.Limit)

		if filter.Offset > 0 {
			argCount++
			query += " OFFSET $" + string(argCount)
			args = append(args, filter.Offset)
		}
	}

	var rules []Rule
	err := s.db.Select(&rules, query, args...)
	return rules, err
}

// CreateRule creates a new rule
func (s *Service) CreateRule(rule *Rule) error {
	// Start a transaction
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert the rule
	query := `
		INSERT INTO rules (
			name, description, content, version, active, created_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, NOW(), NOW()
		) RETURNING id, created_at, updated_at
	`

	row := tx.QueryRow(
		query,
		rule.Name, rule.Description, rule.Content, 1, rule.Active, rule.CreatedBy,
	)

	if err := row.Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return err
	}

	// Insert the initial version
	_, err = tx.Exec(
		`INSERT INTO rule_versions (rule_id, version, content, created_by, created_at)
		VALUES ($1, $2, $3, $4, NOW())`,
		rule.ID, 1, rule.Content, rule.CreatedBy,
	)
	if err != nil {
		return err
	}

	// Commit the transaction
	return tx.Commit()
}

// UpdateRule updates an existing rule
func (s *Service) UpdateRule(rule *Rule) error {
	// Start a transaction
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get current version
	var currentVersion int
	err = tx.Get(&currentVersion, "SELECT version FROM rules WHERE id = $1", rule.ID)
	if err != nil {
		return err
	}

	// Set new version
	newVersion := currentVersion + 1

	// Update the rule
	query := `
		UPDATE rules SET
			name = $1, description = $2, content = $3, version = $4, active = $5,
			updated_at = NOW()
		WHERE id = $6
		RETURNING updated_at
	`

	row := tx.QueryRow(
		query,
		rule.Name, rule.Description, rule.Content, newVersion, rule.Active, rule.ID,
	)

	if err := row.Scan(&rule.UpdatedAt); err != nil {
		return err
	}

	// Insert the new version
	_, err = tx.Exec(
		`INSERT INTO rule_versions (rule_id, version, content, created_by, created_at)
		VALUES ($1, $2, $3, $4, NOW())`,
		rule.ID, newVersion, rule.Content, rule.CreatedBy,
	)
	if err != nil {
		return err
	}

	// Commit the transaction
	return tx.Commit()
}

// DeleteRule deletes a rule
func (s *Service) DeleteRule(id int) error {
	_, err := s.db.Exec("DELETE FROM rules WHERE id = $1", id)
	return err
}

// GetRuleVersions retrieves all versions of a rule
func (s *Service) GetRuleVersions(ruleID int) ([]RuleVersion, error) {
	var versions []RuleVersion
	err := s.db.Select(
		&versions,
		"SELECT * FROM rule_versions WHERE rule_id = $1 ORDER BY version DESC",
		ruleID,
	)
	return versions, err
}

// GetRuleVersion retrieves a specific version of a rule
func (s *Service) GetRuleVersion(ruleID, version int) (*RuleVersion, error) {
	var ruleVersion RuleVersion
	err := s.db.Get(
		&ruleVersion,
		"SELECT * FROM rule_versions WHERE rule_id = $1 AND version = $2",
		ruleID, version,
	)
	if err != nil {
		return nil, err
	}
	return &ruleVersion, nil
}

// ListRuleAssignments retrieves all rule assignments
func (s *Service) ListRuleAssignments(ruleID int) ([]RuleAssignment, error) {
	var assignments []RuleAssignment
	var err error

	if ruleID > 0 {
		err = s.db.Select(
			&assignments,
			"SELECT * FROM rule_assignments WHERE rule_id = $1",
			ruleID,
		)
	} else {
		err = s.db.Select(&assignments, "SELECT * FROM rule_assignments")
	}

	return assignments, err
}

// CreateRuleAssignment creates a new rule assignment
type CreateRuleAssignmentRequest struct {
	RuleID  int  `json:"rule_id"`
	NodeID  *int `json:"node_id"`
	GroupID *int `json:"group_id"`
}

func (s *Service) CreateRuleAssignment(req *CreateRuleAssignmentRequest) (*RuleAssignment, error) {
	// Validate request
	if req.NodeID == nil && req.GroupID == nil {
		return nil, errors.New("either node_id or group_id must be provided")
	}
	if req.NodeID != nil && req.GroupID != nil {
		return nil, errors.New("only one of node_id or group_id can be provided")
	}

	// Check if rule exists
	var ruleExists bool
	err := s.db.Get(&ruleExists, "SELECT EXISTS(SELECT 1 FROM rules WHERE id = $1)", req.RuleID)
	if err != nil {
		return nil, err
	}
	if !ruleExists {
		return nil, errors.New("rule not found")
	}

	// Check if node exists if node_id is provided
	if req.NodeID != nil {
		var nodeExists bool
		err := s.db.Get(&nodeExists, "SELECT EXISTS(SELECT 1 FROM nodes WHERE id = $1)", *req.NodeID)
		if err != nil {
			return nil, err
		}
		if !nodeExists {
			return nil, errors.New("node not found")
		}

		// Check if assignment already exists
		var exists bool
		err = s.db.Get(
			&exists,
			"SELECT EXISTS(SELECT 1 FROM rule_assignments WHERE rule_id = $1 AND node_id = $2)",
			req.RuleID, *req.NodeID,
		)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("rule is already assigned to this node")
		}
	}

	// Check if group exists if group_id is provided
	if req.GroupID != nil {
		var groupExists bool
		err := s.db.Get(&groupExists, "SELECT EXISTS(SELECT 1 FROM node_groups WHERE id = $1)", *req.GroupID)
		if err != nil {
			return nil, err
		}
		if !groupExists {
			return nil, errors.New("group not found")
		}

		// Check if assignment already exists
		var exists bool
		err = s.db.Get(
			&exists,
			"SELECT EXISTS(SELECT 1 FROM rule_assignments WHERE rule_id = $1 AND group_id = $2)",
			req.RuleID, *req.GroupID,
		)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("rule is already assigned to this group")
		}
	}

	// Create the assignment
	assignment := &RuleAssignment{
		RuleID:  req.RuleID,
		NodeID:  req.NodeID,
		GroupID: req.GroupID,
	}

	query := `
		INSERT INTO rule_assignments (rule_id, node_id, group_id, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	row := s.db.QueryRow(query, assignment.RuleID, assignment.NodeID, assignment.GroupID)
	err = row.Scan(&assignment.ID, &assignment.CreatedAt, &assignment.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return assignment, nil
}

// DeleteRuleAssignment deletes a rule assignment
func (s *Service) DeleteRuleAssignment(id int) error {
	_, err := s.db.Exec("DELETE FROM rule_assignments WHERE id = $1", id)
	return err
}

// GetRulesForNode retrieves all rules assigned to a node
func (s *Service) GetRulesForNode(nodeID int) ([]Rule, error) {
	query := `
		SELECT r.* FROM rules r
		WHERE r.active = true AND (
			r.id IN (
				-- Rules directly assigned to the node
				SELECT ra.rule_id FROM rule_assignments ra
				WHERE ra.node_id = $1
			)
			OR r.id IN (
				-- Rules assigned to groups the node belongs to
				SELECT ra.rule_id FROM rule_assignments ra
				JOIN node_group_mapping ngm ON ra.group_id = ngm.group_id
				WHERE ngm.node_id = $1
			)
		)
		ORDER BY r.name
	`

	var rules []Rule
	err := s.db.Select(&rules, query, nodeID)
	return rules, err
}

// GetRulesForNodeByHostname retrieves all rules assigned to a node by hostname
func (s *Service) GetRulesForNodeByHostname(hostname string) ([]Rule, error) {
	query := `
		SELECT r.* FROM rules r
		WHERE r.active = true AND (
			r.id IN (
				-- Rules directly assigned to the node
				SELECT ra.rule_id FROM rule_assignments ra
				JOIN nodes n ON ra.node_id = n.id
				WHERE n.hostname = $1
			)
			OR r.id IN (
				-- Rules assigned to groups the node belongs to
				SELECT ra.rule_id FROM rule_assignments ra
				JOIN node_group_mapping ngm ON ra.group_id = ngm.group_id
				JOIN nodes n ON ngm.node_id = n.id
				WHERE n.hostname = $1
			)
		)
		ORDER BY r.name
	`

	var rules []Rule
	err := s.db.Select(&rules, query, hostname)
	return rules, err
}

// LoadDefaultRules loads default rules from the filesystem
func (s *Service) LoadDefaultRules() error {
	if s.defaultRulesPath == "" {
		return errors.New("default rules path not set")
	}

	files, err := ioutil.ReadDir(s.defaultRulesPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
			continue
		}

		// Read the file
		path := filepath.Join(s.defaultRulesPath, file.Name())
		content, err := ioutil.ReadFile(path)
		if err != nil {
			logrus.WithError(err).WithField("path", path).Warn("Failed to read default rule file")
			continue
		}

		// Use filename (without extension) as the rule name
		name := filepath.Base(file.Name())
		name = name[:len(name)-len(filepath.Ext(name))]

		// Check if rule already exists
		existingRule, err := s.GetRuleByName(name)
		if err == nil && existingRule != nil {
			logrus.WithField("name", name).Debug("Default rule already exists, skipping")
			continue
		}

		// Create the rule
		rule := &Rule{
			Name:        name,
			Description: fmt.Sprintf("Default rule: %s", name),
			Content:     string(content),
			Active:      true,
			CreatedBy:   0, // System user
		}

		if err := s.CreateRule(rule); err != nil {
			logrus.WithError(err).WithField("name", name).Warn("Failed to create default rule")
			continue
		}

		logrus.WithField("name", name).Info("Loaded default rule")
	}

	return nil
}

// ValidateRuleContent validates the content of a rule
// This is a simple validation for now - just checking if it's not empty
func (s *Service) ValidateRuleContent(content string) error {
	if content == "" {
		return errors.New("rule content cannot be empty")
	}
	return nil
}

// ExportRulesToFS exports all active rules to the filesystem
func (s *Service) ExportRulesToFS(outputDir string) error {
	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Get all active rules
	rules, err := s.ListRules(RuleFilter{Active: true})
	if err != nil {
		return err
	}

	// Export each rule
	for _, rule := range rules {
		filename := filepath.Join(outputDir, rule.Name+".yml")
		if err := ioutil.WriteFile(filename, []byte(rule.Content), 0644); err != nil {
			logrus.WithError(err).WithField("name", rule.Name).Warn("Failed to export rule")
			continue
		}
	}

	return nil
}