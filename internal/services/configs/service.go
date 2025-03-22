package configs

import (
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

// Service handles configuration management operations
type Service struct {
	db *sqlx.DB
}

// NewService creates a new instance of Service
func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

// Config represents a Fibratus configuration
type Config struct {
	ID          int       `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Content     string    `db:"content" json:"content"`
	Version     int       `db:"version" json:"version"`
	CreatedBy   int       `db:"created_by" json:"created_by"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// ConfigVersion represents a historical version of a configuration
type ConfigVersion struct {
	ID        int       `db:"id" json:"id"`
	ConfigID  int       `db:"config_id" json:"config_id"`
	Version   int       `db:"version" json:"version"`
	Content   string    `db:"content" json:"content"`
	CreatedBy int       `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ConfigAssignment represents a configuration assigned to a node or group
type ConfigAssignment struct {
	ID        int       `db:"id" json:"id"`
	ConfigID  int       `db:"config_id" json:"config_id"`
	NodeID    *int      `db:"node_id" json:"node_id,omitempty"`
	GroupID   *int      `db:"group_id" json:"group_id,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ConfigFilter defines filters for listing configurations
type ConfigFilter struct {
	Search string
	Limit  int
	Offset int
}

// GetConfigByID retrieves a configuration by its ID
func (s *Service) GetConfigByID(id int) (*Config, error) {
	var config Config
	err := s.db.Get(&config, "SELECT * FROM configs WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetConfigByName retrieves a configuration by its name
func (s *Service) GetConfigByName(name string) (*Config, error) {
	var config Config
	err := s.db.Get(&config, "SELECT * FROM configs WHERE name = $1", name)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// ListConfigs retrieves a list of configurations based on filters
func (s *Service) ListConfigs(filter ConfigFilter) ([]Config, error) {
	query := "SELECT * FROM configs WHERE 1=1"
	var args []interface{}
	var argCount int

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

	var configs []Config
	err := s.db.Select(&configs, query, args...)
	return configs, err
}

// CreateConfig creates a new configuration
func (s *Service) CreateConfig(config *Config) error {
	// Start a transaction
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert the configuration
	query := `
		INSERT INTO configs (
			name, description, content, version, created_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, NOW(), NOW()
		) RETURNING id, created_at, updated_at
	`

	row := tx.QueryRow(
		query,
		config.Name, config.Description, config.Content, 1, config.CreatedBy,
	)

	if err := row.Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt); err != nil {
		return err
	}

	// Insert the initial version
	_, err = tx.Exec(
		`INSERT INTO config_versions (config_id, version, content, created_by, created_at)
		VALUES ($1, $2, $3, $4, NOW())`,
		config.ID, 1, config.Content, config.CreatedBy,
	)
	if err != nil {
		return err
	}

	// Commit the transaction
	return tx.Commit()
}

// UpdateConfig updates an existing configuration
func (s *Service) UpdateConfig(config *Config) error {
	// Start a transaction
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get current version
	var currentVersion int
	err = tx.Get(&currentVersion, "SELECT version FROM configs WHERE id = $1", config.ID)
	if err != nil {
		return err
	}

	// Set new version
	newVersion := currentVersion + 1

	// Update the configuration
	query := `
		UPDATE configs SET
			name = $1, description = $2, content = $3, version = $4,
			updated_at = NOW()
		WHERE id = $5
		RETURNING updated_at
	`

	row := tx.QueryRow(
		query,
		config.Name, config.Description, config.Content, newVersion, config.ID,
	)

	if err := row.Scan(&config.UpdatedAt); err != nil {
		return err
	}

	// Insert the new version
	_, err = tx.Exec(
		`INSERT INTO config_versions (config_id, version, content, created_by, created_at)
		VALUES ($1, $2, $3, $4, NOW())`,
		config.ID, newVersion, config.Content, config.CreatedBy,
	)
	if err != nil {
		return err
	}

	// Commit the transaction
	return tx.Commit()
}

// DeleteConfig deletes a configuration
func (s *Service) DeleteConfig(id int) error {
	_, err := s.db.Exec("DELETE FROM configs WHERE id = $1", id)
	return err
}

// GetConfigVersions retrieves all versions of a configuration
func (s *Service) GetConfigVersions(configID int) ([]ConfigVersion, error) {
	var versions []ConfigVersion
	err := s.db.Select(
		&versions,
		"SELECT * FROM config_versions WHERE config_id = $1 ORDER BY version DESC",
		configID,
	)
	return versions, err
}

// GetConfigVersion retrieves a specific version of a configuration
func (s *Service) GetConfigVersion(configID, version int) (*ConfigVersion, error) {
	var configVersion ConfigVersion
	err := s.db.Get(
		&configVersion,
		"SELECT * FROM config_versions WHERE config_id = $1 AND version = $2",
		configID, version,
	)
	if err != nil {
		return nil, err
	}
	return &configVersion, nil
}

// ListConfigAssignments retrieves all configuration assignments
func (s *Service) ListConfigAssignments(configID int) ([]ConfigAssignment, error) {
	var assignments []ConfigAssignment
	var err error

	if configID > 0 {
		err = s.db.Select(
			&assignments,
			"SELECT * FROM config_assignments WHERE config_id = $1",
			configID,
		)
	} else {
		err = s.db.Select(&assignments, "SELECT * FROM config_assignments")
	}

	return assignments, err
}

// CreateConfigAssignment creates a new configuration assignment
type CreateConfigAssignmentRequest struct {
	ConfigID int  `json:"config_id"`
	NodeID   *int `json:"node_id"`
	GroupID  *int `json:"group_id"`
}

func (s *Service) CreateConfigAssignment(req *CreateConfigAssignmentRequest) (*ConfigAssignment, error) {
	// Validate request
	if req.NodeID == nil && req.GroupID == nil {
		return nil, errors.New("either node_id or group_id must be provided")
	}
	if req.NodeID != nil && req.GroupID != nil {
		return nil, errors.New("only one of node_id or group_id can be provided")
	}

	// Check if configuration exists
	var configExists bool
	err := s.db.Get(&configExists, "SELECT EXISTS(SELECT 1 FROM configs WHERE id = $1)", req.ConfigID)
	if err != nil {
		return nil, err
	}
	if !configExists {
		return nil, errors.New("configuration not found")
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
			"SELECT EXISTS(SELECT 1 FROM config_assignments WHERE config_id = $1 AND node_id = $2)",
			req.ConfigID, *req.NodeID,
		)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("configuration is already assigned to this node")
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
			"SELECT EXISTS(SELECT 1 FROM config_assignments WHERE config_id = $1 AND group_id = $2)",
			req.ConfigID, *req.GroupID,
		)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, errors.New("configuration is already assigned to this group")
		}
	}

	// Create the assignment
	assignment := &ConfigAssignment{
		ConfigID: req.ConfigID,
		NodeID:   req.NodeID,
		GroupID:  req.GroupID,
	}

	query := `
		INSERT INTO config_assignments (config_id, node_id, group_id, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	row := s.db.QueryRow(query, assignment.ConfigID, assignment.NodeID, assignment.GroupID)
	err = row.Scan(&assignment.ID, &assignment.CreatedAt, &assignment.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return assignment, nil
}

// DeleteConfigAssignment deletes a configuration assignment
func (s *Service) DeleteConfigAssignment(id int) error {
	_, err := s.db.Exec("DELETE FROM config_assignments WHERE id = $1", id)
	return err
}

// GetConfigsForNode retrieves all configurations assigned to a node
func (s *Service) GetConfigsForNode(nodeID int) ([]Config, error) {
	query := `
		SELECT c.* FROM configs c
		WHERE c.id IN (
			-- Configs directly assigned to the node
			SELECT ca.config_id FROM config_assignments ca
			WHERE ca.node_id = $1
		)
		OR c.id IN (
			-- Configs assigned to groups the node belongs to
			SELECT ca.config_id FROM config_assignments ca
			JOIN node_group_mapping ngm ON ca.group_id = ngm.group_id
			WHERE ngm.node_id = $1
		)
		ORDER BY c.name
	`

	var configs []Config
	err := s.db.Select(&configs, query, nodeID)
	return configs, err
}

// GetConfigForNodeByHostname retrieves the merged configuration for a node by hostname
func (s *Service) GetConfigForNodeByHostname(hostname string) (map[string]interface{}, error) {
	// This is a placeholder for actual configuration merging logic
	// In a real implementation, you would:
	// 1. Get the node ID from hostname
	// 2. Get all configurations assigned to the node or its groups
	// 3. Merge the configurations based on some priority rules
	// 4. Return the merged configuration

	// For now, return a default configuration
	return map[string]interface{}{
		"version": "1.0.0",
		"logging": map[string]interface{}{
			"level":     "info",
			"file_path": "C:\\Program Files\\Fibratus\\logs\\fibratus.log",
		},
		"heartbeat": map[string]interface{}{
			"interval": 60,
			"timeout":  180,
		},
		"server": map[string]interface{}{
			"url": "https://fibratus-server:8080",
		},
	}, nil
}