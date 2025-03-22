package nodes

import (
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

// Service handles node-related operations
type Service struct {
	db *sqlx.DB
}

// NewService creates a new instance of Service
func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

// Node represents a Fibratus node
type Node struct {
	ID              int       `db:"id" json:"id"`
	Hostname        string    `db:"hostname" json:"hostname"`
	IPAddress       string    `db:"ip_address" json:"ip_address"`
	OSVersion       string    `db:"os_version" json:"os_version"`
	FibratusVersion string    `db:"fibratus_version" json:"fibratus_version"`
	CPU             string    `db:"cpu" json:"cpu"`
	Memory          string    `db:"memory" json:"memory"`
	Disk            string    `db:"disk" json:"disk"`
	Status          string    `db:"status" json:"status"`
	Isolated        bool      `db:"isolated" json:"isolated"`
	LastHeartbeat   time.Time `db:"last_heartbeat" json:"last_heartbeat"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// NodeGroup represents a group of nodes
type NodeGroup struct {
	ID          int       `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// HeartbeatRequest represents a heartbeat request from a node
type HeartbeatRequest struct {
	Hostname        string `json:"hostname"`
	IPAddress       string `json:"ip_address"`
	OSVersion       string `json:"os_version"`
	FibratusVersion string `json:"fibratus_version"`
	CPU             string `json:"cpu"`
	Memory          string `json:"memory"`
	Disk            string `json:"disk"`
}

// NodeFilter defines filters for listing nodes
type NodeFilter struct {
	Status   string
	Isolated bool
	GroupID  int
	Search   string
	Limit    int
	Offset   int
}

// GetNodeByID retrieves a node by its ID
func (s *Service) GetNodeByID(id int) (*Node, error) {
	var node Node
	err := s.db.Get(&node, "SELECT * FROM nodes WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// GetNodeByHostname retrieves a node by its hostname
func (s *Service) GetNodeByHostname(hostname string) (*Node, error) {
	var node Node
	err := s.db.Get(&node, "SELECT * FROM nodes WHERE hostname = $1", hostname)
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// ListNodes retrieves a list of nodes based on filters
func (s *Service) ListNodes(filter NodeFilter) ([]Node, error) {
	query := "SELECT * FROM nodes WHERE 1=1"
	var args []interface{}
	var argCount int

	if filter.Status != "" {
		argCount++
		query += " AND status = $" + string(argCount)
		args = append(args, filter.Status)
	}

	if filter.Search != "" {
		argCount++
		query += " AND (hostname LIKE $" + string(argCount) + " OR ip_address LIKE $" + string(argCount) + ")"
		args = append(args, "%"+filter.Search+"%")
	}

	if filter.GroupID > 0 {
		argCount++
		query += " AND id IN (SELECT node_id FROM node_group_mapping WHERE group_id = $" + string(argCount) + ")"
		args = append(args, filter.GroupID)
	}

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

	var nodes []Node
	err := s.db.Select(&nodes, query, args...)
	return nodes, err
}

// GetNodesByGroupID retrieves all nodes assigned to a group
func (s *Service) GetNodesByGroupID(groupID int) ([]Node, error) {
	query := `
		SELECT n.* FROM nodes n
		JOIN node_group_mapping m ON n.id = m.node_id
		WHERE m.group_id = $1
	`
	var nodes []Node
	err := s.db.Select(&nodes, query, groupID)
	return nodes, err
}

// CreateNode creates a new node
func (s *Service) CreateNode(node *Node) error {
	query := `
		INSERT INTO nodes (
			hostname, ip_address, os_version, fibratus_version, 
			cpu, memory, disk, status, isolated, last_heartbeat
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		) RETURNING id, created_at, updated_at
	`
	
	row := s.db.QueryRow(
		query,
		node.Hostname, node.IPAddress, node.OSVersion, node.FibratusVersion,
		node.CPU, node.Memory, node.Disk, node.Status, node.Isolated, node.LastHeartbeat,
	)
	
	return row.Scan(&node.ID, &node.CreatedAt, &node.UpdatedAt)
}

// UpdateNode updates an existing node
func (s *Service) UpdateNode(node *Node) error {
	query := `
		UPDATE nodes SET
			hostname = $1, ip_address = $2, os_version = $3, fibratus_version = $4,
			cpu = $5, memory = $6, disk = $7, status = $8, isolated = $9, 
			last_heartbeat = $10, updated_at = NOW()
		WHERE id = $11
		RETURNING updated_at
	`
	
	row := s.db.QueryRow(
		query,
		node.Hostname, node.IPAddress, node.OSVersion, node.FibratusVersion,
		node.CPU, node.Memory, node.Disk, node.Status, node.Isolated,
		node.LastHeartbeat, node.ID,
	)
	
	return row.Scan(&node.UpdatedAt)
}

// DeleteNode deletes a node
func (s *Service) DeleteNode(id int) error {
	_, err := s.db.Exec("DELETE FROM nodes WHERE id = $1", id)
	return err
}

// ProcessHeartbeat processes a heartbeat from a node
func (s *Service) ProcessHeartbeat(req HeartbeatRequest) (*Node, error) {
	// Check if the node already exists
	existingNode, err := s.GetNodeByHostname(req.Hostname)
	if err == nil {
		// Node exists, update it
		existingNode.IPAddress = req.IPAddress
		existingNode.OSVersion = req.OSVersion
		existingNode.FibratusVersion = req.FibratusVersion
		existingNode.CPU = req.CPU
		existingNode.Memory = req.Memory
		existingNode.Disk = req.Disk
		existingNode.Status = "online"
		existingNode.LastHeartbeat = time.Now()

		if err := s.UpdateNode(existingNode); err != nil {
			return nil, err
		}
		return existingNode, nil
	}

	// Node doesn't exist, create it
	newNode := &Node{
		Hostname:        req.Hostname,
		IPAddress:       req.IPAddress,
		OSVersion:       req.OSVersion,
		FibratusVersion: req.FibratusVersion,
		CPU:             req.CPU,
		Memory:          req.Memory,
		Disk:            req.Disk,
		Status:          "online",
		Isolated:        false,
		LastHeartbeat:   time.Now(),
	}

	if err := s.CreateNode(newNode); err != nil {
		return nil, err
	}
	return newNode, nil
}

// UpdateNodeStatus updates the status of nodes based on heartbeat timeouts
func (s *Service) UpdateNodeStatus(heartbeatTimeout int) error {
	// Calculate the timeout threshold
	timeout := time.Now().Add(time.Duration(-heartbeatTimeout) * time.Second)

	// Update nodes that haven't sent a heartbeat within the timeout
	_, err := s.db.Exec(
		"UPDATE nodes SET status = 'offline', updated_at = NOW() WHERE status = 'online' AND last_heartbeat < $1",
		timeout,
	)
	return err
}

// CreateNodeGroup creates a new node group
func (s *Service) CreateNodeGroup(group *NodeGroup) error {
	query := `
		INSERT INTO node_groups (name, description)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at
	`
	
	row := s.db.QueryRow(query, group.Name, group.Description)
	return row.Scan(&group.ID, &group.CreatedAt, &group.UpdatedAt)
}

// GetNodeGroup retrieves a node group by its ID
func (s *Service) GetNodeGroup(id int) (*NodeGroup, error) {
	var group NodeGroup
	err := s.db.Get(&group, "SELECT * FROM node_groups WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// ListNodeGroups retrieves all node groups
func (s *Service) ListNodeGroups() ([]NodeGroup, error) {
	var groups []NodeGroup
	err := s.db.Select(&groups, "SELECT * FROM node_groups ORDER BY name")
	return groups, err
}

// UpdateNodeGroup updates a node group
func (s *Service) UpdateNodeGroup(group *NodeGroup) error {
	query := `
		UPDATE node_groups SET
			name = $1, description = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING updated_at
	`
	
	row := s.db.QueryRow(query, group.Name, group.Description, group.ID)
	return row.Scan(&group.UpdatedAt)
}

// DeleteNodeGroup deletes a node group
func (s *Service) DeleteNodeGroup(id int) error {
	_, err := s.db.Exec("DELETE FROM node_groups WHERE id = $1", id)
	return err
}

// AddNodeToGroup adds a node to a group
func (s *Service) AddNodeToGroup(nodeID, groupID int) error {
	// Verify that the node and group exist
	var nodeCount, groupCount int
	err := s.db.Get(&nodeCount, "SELECT COUNT(*) FROM nodes WHERE id = $1", nodeID)
	if err != nil {
		return err
	}
	if nodeCount == 0 {
		return errors.New("node not found")
	}

	err = s.db.Get(&groupCount, "SELECT COUNT(*) FROM node_groups WHERE id = $1", groupID)
	if err != nil {
		return err
	}
	if groupCount == 0 {
		return errors.New("group not found")
	}

	// Add the node to the group
	_, err = s.db.Exec(
		"INSERT INTO node_group_mapping (node_id, group_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		nodeID, groupID,
	)
	return err
}

// RemoveNodeFromGroup removes a node from a group
func (s *Service) RemoveNodeFromGroup(nodeID, groupID int) error {
	_, err := s.db.Exec(
		"DELETE FROM node_group_mapping WHERE node_id = $1 AND group_id = $2",
		nodeID, groupID,
	)
	return err
}

// GetNodeGroups retrieves all groups that a node belongs to
func (s *Service) GetNodeGroups(nodeID int) ([]NodeGroup, error) {
	query := `
		SELECT g.* FROM node_groups g
		JOIN node_group_mapping m ON g.id = m.group_id
		WHERE m.node_id = $1
	`
	var groups []NodeGroup
	err := s.db.Select(&groups, query, nodeID)
	return groups, err
}