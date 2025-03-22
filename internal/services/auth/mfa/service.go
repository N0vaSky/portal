package mfa

import (
	"fmt"
	"strings"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Service handles MFA operations
type Service struct{}

// NewService creates a new instance of Service
func NewService() *Service {
	return &Service{}
}

// GenerateSecret generates a new TOTP secret
func (s *Service) GenerateSecret(username string) (*otp.Key, error) {
	// Clean up the username
	username = strings.Replace(username, " ", "_", -1)
	username = strings.Replace(username, "@", "_at_", -1)

	// Generate a new TOTP key
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "FibratusPortal",
		AccountName: username,
	})
	if err != nil {
		return nil, err
	}

	return key, nil
}

// ValidateCode validates a TOTP code
func (s *Service) ValidateCode(secret, code string) bool {
	// Validate the code
	valid, err := totp.ValidateCustom(
		code,
		secret,
		fmt.Sprint(totp.Now().Unix()),
		totp.ValidateOpts{
			Period:    30,
			Skew:      1,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		},
	)
	if err != nil {
		return false
	}

	return valid
}