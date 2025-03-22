package remediation

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

// Service handles remediation operations
type Service struct {
	db *sqlx.DB
}

// NewService creates a new instance of Service
func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

// CommandType defines the types of commands that can be executed
type CommandType string

// Command types
const (
	CommandTypeRemoveFile      CommandType = "remove-file"
	CommandTypeQuarantineFile  CommandType = "quarantine-file"
	CommandTypeListFiles       CommandType = "list-files"
	CommandTypeGetFileHash     CommandType = "get-file-hash"
	CommandTypeRevertRegistry  CommandType = "revert-registry-key"
	CommandTypeSetRegistry     CommandType = "set-registry-value"
	CommandTypeGetRegistry     CommandType = "get-registry-value"
	CommandTypeExportRegistry  CommandType = "export-registry-hive"
	CommandTypeKillProcess     CommandType = "kill-process"
	CommandTypeGetProcess      CommandType = "get-process-details"
	CommandTypeGetProcessTree  CommandType = "get-process-tree"
	CommandTypeScanProcess     CommandType = "scan-process-memory"
	CommandTypeIsolateHost     CommandType = "isolate-host"
	CommandTypeUnisolateHost   CommandType = "unisolate-host"
	CommandTypeGetNetworkConns CommandType = "get-network-connections"
	CommandTypeBlockIP         CommandType = "block-ip"
	CommandTypeRebootNode      CommandType = "reboot-node"
	CommandTypeUpdateAgent     CommandType = "update-agent"
	CommandTypeUninstallAgent  CommandType = "uninstall-agent"
	CommandTypeGetSystemInfo   CommandType = "get-system-info"
	CommandTypeCollectWinLogs  CommandType = "collect-windows-logs"
	CommandTypeCollectAgentLogs CommandType = "collect-fibratus-logs"
	CommandTypeClearLogs       CommandType = "clear-local-logs"
	CommandTypeEnableDebugLogs CommandType = "enable-debug-logging"
)

// CommandStatus defines the status of a command
type CommandStatus string

// Command statuses
const (
	CommandStatusPending  CommandStatus = "pending"
	CommandStatusRunning  CommandStatus = "running"
	CommandStatusSuccess  CommandStatus = "success"
	CommandStatusFailed   CommandStatus = "failed"
	CommandStatusTimeout  CommandStatus = "timeout"
	CommandStatusCanceled CommandStatus = "canceled"
)

// Command represents a remediation command
type Command struct {
	ID             int             `db:"id" json:"id"`
	NodeID         int             `db:"node_id" json:"node_id"`
	UserID         int             `db:"user_id" json:"user_id"`
	CommandType    CommandType     `db:"command_type" json:"command_type"`
	CommandDetails json.RawMessage `db:"command_details" json:"command_details"`
	Status         CommandStatus   `db:"status" json:"status"`
	ResultDetails  json.RawMessage `db:"result_details" json:"result_details,omitempty"`
	ExecutedAt     time.Time       `db:"executed_at" json:"executed_at"`
	CompletedAt    *time.Time      `db:"completed_at" json:"completed_at,omitempty"`
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ExecuteCommand creates a new command to be executed on a node
func (s *Service) ExecuteCommand(cmd *Command) error {
	// Validate command
	if cmd.NodeID == 0 {
		return errors.New("node ID is required")
	}
	if cmd.UserID == 0 {
		return errors.New("user ID is required")
	}
	if cmd.CommandType == "" {
		return errors.New("command type is required")
	}

	// Set initial status and timestamp
	cmd.Status = CommandStatusPending
	cmd.ExecutedAt = time.Now()

	// Insert command into database
	query := `
		INSERT INTO command_history (
			node_id, user_id, command_type, command_details, status, executed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6
		) RETURNING id
	`

	row := s.db.QueryRow(
		query,
		cmd.NodeID, cmd.UserID, cmd.CommandType, cmd.CommandDetails, cmd.Status, cmd.ExecutedAt,
	)

	return row.Scan(&cmd.ID)
}

// GetCommand retrieves a command by its ID
func (s *Service) GetCommand(id int) (*Command, error) {
	var cmd Command
	err := s.db.Get(&cmd, "SELECT * FROM command_history WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

// ListCommands retrieves a list of commands
func (s *Service) ListCommands(nodeID int, limit, offset int) ([]Command, error) {
	query := "SELECT * FROM command_history WHERE 1=1"
	var args []interface{}
	var argCount int

	if nodeID > 0 {
		argCount++
		query += " AND node_id = $" + string(argCount)
		args = append(args, nodeID)
	}

	// Add ordering
	query += " ORDER BY executed_at DESC"

	// Add limit and offset
	if limit > 0 {
		argCount++
		query += " LIMIT $" + string(argCount)
		args = append(args, limit)

		if offset > 0 {
			argCount++
			query += " OFFSET $" + string(argCount)
			args = append(args, offset)
		}
	}

	var commands []Command
	err := s.db.Select(&commands, query, args...)
	return commands, err
}

// GetPendingCommands retrieves pending commands for a node
func (s *Service) GetPendingCommands(nodeID int) ([]Command, error) {
	var commands []Command
	err := s.db.Select(
		&commands,
		"SELECT * FROM command_history WHERE node_id = $1 AND status = $2 ORDER BY executed_at",
		nodeID, CommandStatusPending,
	)
	return commands, err
}

// GetPendingCommandsByHostname retrieves pending commands for a node by hostname
func (s *Service) GetPendingCommandsByHostname(hostname string) ([]Command, error) {
	var commands []Command
	err := s.db.Select(
		&commands,
		`SELECT ch.* FROM command_history ch
		JOIN nodes n ON ch.node_id = n.id
		WHERE n.hostname = $1 AND ch.status = $2
		ORDER BY ch.executed_at`,
		hostname, CommandStatusPending,
	)
	return commands, err
}

// UpdateCommandResult updates the result of a command
func (s *Service) UpdateCommandResult(commandID int, result *CommandResult) error {
	// Serialize result details
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}

	// Determine status based on result
	status := CommandStatusSuccess
	if !result.Success {
		status = CommandStatusFailed
	}

	// Update command
	now := time.Now()
	_, err = s.db.Exec(
		`UPDATE command_history SET
			status = $1, result_details = $2, completed_at = $3
		WHERE id = $4`,
		status, resultJSON, now, commandID,
	)
	return err
}

// CancelCommand cancels a pending command
func (s *Service) CancelCommand(commandID int) error {
	// Check if the command is pending
	var status CommandStatus
	err := s.db.Get(&status, "SELECT status FROM command_history WHERE id = $1", commandID)
	if err != nil {
		return err
	}

	if status != CommandStatusPending {
		return errors.New("only pending commands can be canceled")
	}

	// Update command status
	now := time.Now()
	_, err = s.db.Exec(
		`UPDATE command_history SET
			status = $1, completed_at = $2
		WHERE id = $3`,
		CommandStatusCanceled, now, commandID,
	)
	return err
}

// GetCommandsByType retrieves commands by type
func (s *Service) GetCommandsByType(nodeID int, commandType CommandType) ([]Command, error) {
	var commands []Command
	err := s.db.Select(
		&commands,
		"SELECT * FROM command_history WHERE node_id = $1 AND command_type = $2 ORDER BY executed_at DESC",
		nodeID, commandType,
	)
	return commands, err
}

// CleanupOldCommands removes old completed commands
func (s *Service) CleanupOldCommands(olderThan time.Time) (int, error) {
	result, err := s.db.Exec(
		`DELETE FROM command_history
		WHERE status IN ($1, $2, $3, $4)
		AND completed_at < $5`,
		CommandStatusSuccess, CommandStatusFailed, CommandStatusTimeout, CommandStatusCanceled,
		olderThan,
	)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(affected), nil
}

// TimeoutStalePendingCommands updates the status of commands that have been pending for too long
func (s *Service) TimeoutStalePendingCommands(timeout time.Duration) (int, error) {
	cutoff := time.Now().Add(-timeout)
	now := time.Now()

	result, err := s.db.Exec(
		`UPDATE command_history
		SET status = $1, completed_at = $2
		WHERE status = $3 AND executed_at < $4`,
		CommandStatusTimeout, now, CommandStatusPending, cutoff,
	)
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(affected), nil
}

// ValidateCommandType checks if a command type is valid
func ValidateCommandType(cmdType CommandType) bool {
	switch cmdType {
	case CommandTypeRemoveFile, CommandTypeQuarantineFile, CommandTypeListFiles, CommandTypeGetFileHash,
		CommandTypeRevertRegistry, CommandTypeSetRegistry, CommandTypeGetRegistry, CommandTypeExportRegistry,
		CommandTypeKillProcess, CommandTypeGetProcess, CommandTypeGetProcessTree, CommandTypeScanProcess,
		CommandTypeIsolateHost, CommandTypeUnisolateHost, CommandTypeGetNetworkConns, CommandTypeBlockIP,
		CommandTypeRebootNode, CommandTypeUpdateAgent, CommandTypeUninstallAgent, CommandTypeGetSystemInfo,
		CommandTypeCollectWinLogs, CommandTypeCollectAgentLogs, CommandTypeClearLogs, CommandTypeEnableDebugLogs:
		return true
	default:
		return false
	}
}