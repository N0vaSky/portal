package users

import (
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// Service handles user-related operations
type Service struct {
	db *sqlx.DB
}

// NewService creates a new instance of Service
func NewService(db *sqlx.DB) *Service {
	return &Service{db: db}
}

// User represents a user of the portal
type User struct {
	ID           int       `db:"id" json:"id"`
	Username     string    `db:"username" json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	MFASecret    *string   `db:"mfa_secret" json:"-"`
	MFAEnabled   bool      `db:"mfa_enabled" json:"mfa_enabled"`
	Role         string    `db:"role" json:"role"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// UserFilter defines filters for listing users
type UserFilter struct {
	Role   string
	Search string
	Limit  int
	Offset int
}

// AvailableRoles defines the available user roles
var AvailableRoles = []string{"admin", "analyst", "readonly"}

// CreateUser creates a new user
func (s *Service) CreateUser(user *User, password string) error {
	// Validate role
	if !isValidRole(user.Role) {
		return errors.New("invalid role")
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Set the password hash
	user.PasswordHash = string(hashedPassword)

	// Insert the user
	query := `
		INSERT INTO users (
			username, password_hash, mfa_secret, mfa_enabled, role, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, NOW(), NOW()
		) RETURNING id, created_at, updated_at
	`

	row := s.db.QueryRow(
		query,
		user.Username, user.PasswordHash, user.MFASecret, user.MFAEnabled, user.Role,
	)

	return row.Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

// UpdateUser updates an existing user
func (s *Service) UpdateUser(user *User) error {
	// Validate role
	if !isValidRole(user.Role) {
		return errors.New("invalid role")
	}

	// Update the user
	query := `
		UPDATE users SET
			username = $1, mfa_enabled = $2, role = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at
	`

	row := s.db.QueryRow(
		query,
		user.Username, user.MFAEnabled, user.Role, user.ID,
	)

	return row.Scan(&user.UpdatedAt)
}

// UpdatePassword updates a user's password
func (s *Service) UpdatePassword(userID int, currentPassword, newPassword string) error {
	// Get the current password hash
	var currentHash string
	err := s.db.Get(&currentHash, "SELECT password_hash FROM users WHERE id = $1", userID)
	if err != nil {
		return err
	}

	// Verify the current password
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(currentPassword)); err != nil {
		return errors.New("incorrect current password")
	}

	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Update the password
	_, err = s.db.Exec(
		"UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2",
		string(hashedPassword), userID,
	)
	return err
}

// ResetPassword resets a user's password (admin function)
func (s *Service) ResetPassword(userID int, newPassword string) error {
	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Update the password
	_, err = s.db.Exec(
		"UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2",
		string(hashedPassword), userID,
	)
	return err
}

// DeleteUser deletes a user
func (s *Service) DeleteUser(id int) error {
	// Get the user count
	var count int
	err := s.db.Get(&count, "SELECT COUNT(*) FROM users WHERE role = 'admin'")
	if err != nil {
		return err
	}

	// Get the user's role
	var role string
	err = s.db.Get(&role, "SELECT role FROM users WHERE id = $1", id)
	if err != nil {
		return err
	}

	// Check if this is the last admin
	if role == "admin" && count <= 1 {
		return errors.New("cannot delete the last admin user")
	}

	// Delete the user
	_, err = s.db.Exec("DELETE FROM users WHERE id = $1", id)
	return err
}

// GetUserByID retrieves a user by their ID
func (s *Service) GetUserByID(id int) (*User, error) {
	var user User
	err := s.db.Get(&user, "SELECT * FROM users WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by their username
func (s *Service) GetUserByUsername(username string) (*User, error) {
	var user User
	err := s.db.Get(&user, "SELECT * FROM users WHERE username = $1", username)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers retrieves a list of users based on filters
func (s *Service) ListUsers(filter UserFilter) ([]User, error) {
	query := "SELECT * FROM users WHERE 1=1"
	var args []interface{}
	var argCount int

	if filter.Role != "" {
		argCount++
		query += " AND role = $" + string(argCount)
		args = append(args, filter.Role)
	}

	if filter.Search != "" {
		argCount++
		query += " AND username LIKE $" + string(argCount)
		args = append(args, "%"+filter.Search+"%")
	}

	// Add ordering
	query += " ORDER BY username"

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

	var users []User
	err := s.db.Select(&users, query, args...)
	return users, err
}

// AuthenticateUser authenticates a user with their username and password
func (s *Service) AuthenticateUser(username, password string) (*User, error) {
	// Get the user
	user, err := s.GetUserByUsername(username)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	// Verify the password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		logrus.WithError(err).WithField("username", username).Info("Failed login attempt")
		return nil, errors.New("invalid username or password")
	}

	return user, nil
}

// SetupMFA sets up MFA for a user
func (s *Service) SetupMFA(userID int, secret string) error {
	_, err := s.db.Exec(
		"UPDATE users SET mfa_secret = $1, updated_at = NOW() WHERE id = $2",
		secret, userID,
	)
	return err
}

// EnableMFA enables MFA for a user
func (s *Service) EnableMFA(userID int) error {
	_, err := s.db.Exec(
		"UPDATE users SET mfa_enabled = TRUE, updated_at = NOW() WHERE id = $1",
		userID,
	)
	return err
}

// DisableMFA disables MFA for a user
func (s *Service) DisableMFA(userID int) error {
	_, err := s.db.Exec(
		"UPDATE users SET mfa_enabled = FALSE, mfa_secret = NULL, updated_at = NOW() WHERE id = $1",
		userID,
	)
	return err
}

// GetMFASecret retrieves a user's MFA secret
func (s *Service) GetMFASecret(userID int) (string, error) {
	var secret string
	err := s.db.Get(&secret, "SELECT mfa_secret FROM users WHERE id = $1", userID)
	return secret, err
}

// IsMFAEnabled checks if MFA is enabled for a user
func (s *Service) IsMFAEnabled(userID int) (bool, error) {
	var enabled bool
	err := s.db.Get(&enabled, "SELECT mfa_enabled FROM users WHERE id = $1", userID)
	return enabled, err
}

// isValidRole checks if a role is valid
func isValidRole(role string) bool {
	for _, r := range AvailableRoles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyUser checks if there are any users in the system
func (s *Service) HasAnyUser() (bool, error) {
	var count int
	err := s.db.Get(&count, "SELECT COUNT(*) FROM users")
	return count > 0, err
}

// InitializeAdminUser creates an initial admin user if no users exist
func (s *Service) InitializeAdminUser(username, password string) (*User, error) {
	// Check if users already exist
	hasUsers, err := s.HasAnyUser()
	if err != nil {
		return nil, err
	}
	if hasUsers {
		return nil, errors.New("users already exist")
	}

	// Create admin user
	user := &User{
		Username:   username,
		MFAEnabled: false,
		Role:       "admin",
	}

	if err := s.CreateUser(user, password); err != nil {
		return nil, err
	}

	logrus.WithField("username", username).Info("Initialized admin user")
	return user, nil
}