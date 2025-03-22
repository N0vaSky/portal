package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	LogLevel string        `yaml:"log_level"`
	Server   ServerConfig  `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Fibratus FibratusConfig `yaml:"fibratus"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	TLS  TLSConfig `yaml:"tls"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"ssl_mode"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret    string `yaml:"jwt_secret"`
	MFAEnabled   bool   `yaml:"mfa_enabled"`
	SessionKey   string `yaml:"session_key"`
	SessionName  string `yaml:"session_name"`
	CookieSecure bool   `yaml:"cookie_secure"`
}

// FibratusConfig holds Fibratus-specific configuration
type FibratusConfig struct {
	HeartbeatInterval   int    `yaml:"heartbeat_interval"`
	HeartbeatTimeout    int    `yaml:"heartbeat_timeout"`
	AlertsJsonPath      string `yaml:"alerts_json_path"`
	DefaultRulesDirPath string `yaml:"default_rules_dir_path"`
}

// Load loads config from a YAML file
func Load(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		LogLevel: "info",
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
			TLS: TLSConfig{
				Enabled:  false,
				CertFile: "/etc/fibratus/cert.pem",
				KeyFile:  "/etc/fibratus/key.pem",
			},
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Username: "fibratus",
			Password: "fibratus",
			Database: "fibratus",
			SSLMode:  "disable",
		},
		Auth: AuthConfig{
			JWTSecret:    "change-this-in-production",
			MFAEnabled:   true,
			SessionKey:   "change-this-in-production",
			SessionName:  "fibratus_session",
			CookieSecure: true,
		},
		Fibratus: FibratusConfig{
			HeartbeatInterval:   60,
			HeartbeatTimeout:    180,
			AlertsJsonPath:      "/var/lib/fibratus/alerts.json",
			DefaultRulesDirPath: "/etc/fibratus/rules",
		},
	}
}