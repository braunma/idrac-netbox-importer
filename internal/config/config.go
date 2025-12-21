// Package config handles loading, validation, and management of application configuration.
// It supports YAML configuration files with environment variable overrides.
// All default values are sourced from the defaults package to ensure consistency.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/yourusername/idrac-inventory/pkg/defaults"
	"github.com/yourusername/idrac-inventory/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure.
type Config struct {
	NetBox      NetBoxConfig   `yaml:"netbox"`
	Servers     []ServerConfig `yaml:"servers"`
	Defaults    DefaultsConfig `yaml:"defaults"`
	Concurrency int            `yaml:"concurrency"`
	Logging     LoggingConfig  `yaml:"logging"`
	Retry       RetryConfig    `yaml:"retry"`
	HTTP        HTTPConfig     `yaml:"http"`
}

// NetBoxConfig holds NetBox API configuration.
type NetBoxConfig struct {
	URL                string `yaml:"url"`
	Token              string `yaml:"token"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	TimeoutSeconds     int    `yaml:"timeout_seconds"`
}

// IsEnabled returns true if NetBox integration is configured.
func (n NetBoxConfig) IsEnabled() bool {
	return n.URL != "" && n.Token != ""
}

// Timeout returns the configured timeout as a Duration.
func (n NetBoxConfig) Timeout() time.Duration {
	return secondsToDuration(n.TimeoutSeconds, defaults.GetNetBoxTimeout())
}

// ServerConfig holds configuration for a single iDRAC server.
type ServerConfig struct {
	Host               string `yaml:"host"`
	Username           string `yaml:"username,omitempty"`
	Password           string `yaml:"password,omitempty"`
	Name               string `yaml:"name,omitempty"`
	InsecureSkipVerify *bool  `yaml:"insecure_skip_verify,omitempty"`
	TimeoutSeconds     *int   `yaml:"timeout_seconds,omitempty"`
}

// GetUsername returns the username, falling back to the provided default.
func (s ServerConfig) GetUsername(defaultUser string) string {
	return getStringOrDefault(s.Username, defaultUser)
}

// GetPassword returns the password, falling back to the provided default.
func (s ServerConfig) GetPassword(defaultPass string) string {
	return getStringOrDefault(s.Password, defaultPass)
}

// GetDisplayName returns a human-readable name for this server.
func (s ServerConfig) GetDisplayName() string {
	return getStringOrDefault(s.Name, s.Host)
}

// GetInsecureSkipVerify returns the TLS verification setting for this server.
func (s ServerConfig) GetInsecureSkipVerify(defaultValue bool) bool {
	return getBoolPtrOrDefault(s.InsecureSkipVerify, defaultValue)
}

// GetTimeout returns the timeout for this server.
func (s ServerConfig) GetTimeout(defaultTimeout time.Duration) time.Duration {
	return secondsPtrToDuration(s.TimeoutSeconds, defaultTimeout)
}

// DefaultsConfig holds default values for server connections.
type DefaultsConfig struct {
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	TimeoutSeconds     int    `yaml:"timeout_seconds"`
	InsecureSkipVerify *bool  `yaml:"insecure_skip_verify,omitempty"`
}

// Timeout returns the configured timeout as a Duration.
func (d DefaultsConfig) Timeout() time.Duration {
	return secondsToDuration(d.TimeoutSeconds, defaults.GetTimeout())
}

// GetInsecureSkipVerify returns the TLS verification setting.
func (d DefaultsConfig) GetInsecureSkipVerify() bool {
	return getBoolPtrOrDefault(d.InsecureSkipVerify, defaults.DefaultInsecureSkipVerify)
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, console
}

// RetryConfig holds retry configuration.
type RetryConfig struct {
	MaxAttempts int    `yaml:"max_attempts"`
	BaseDelay   string `yaml:"base_delay"`
	MaxDelay    string `yaml:"max_delay"`
}

// GetMaxAttempts returns the max retry attempts.
func (r RetryConfig) GetMaxAttempts() int {
	return getIntOrDefault(r.MaxAttempts, defaults.DefaultRetryMaxAttempts)
}

// GetBaseDelay returns the base retry delay.
func (r RetryConfig) GetBaseDelay() time.Duration {
	if r.BaseDelay == "" {
		return defaults.DefaultRetryBaseDelay
	}
	if d, err := time.ParseDuration(r.BaseDelay); err == nil {
		return d
	}
	return defaults.DefaultRetryBaseDelay
}

// GetMaxDelay returns the max retry delay.
func (r RetryConfig) GetMaxDelay() time.Duration {
	if r.MaxDelay == "" {
		return defaults.DefaultRetryMaxDelay
	}
	if d, err := time.ParseDuration(r.MaxDelay); err == nil {
		return d
	}
	return defaults.DefaultRetryMaxDelay
}

// HTTPConfig holds HTTP client configuration.
type HTTPConfig struct {
	MaxIdleConns       int `yaml:"max_idle_conns"`
	IdleConnTimeoutSec int `yaml:"idle_conn_timeout_seconds"`
}

// GetMaxIdleConns returns max idle connections.
func (h HTTPConfig) GetMaxIdleConns() int {
	return getIntOrDefault(h.MaxIdleConns, defaults.DefaultHTTPMaxIdleConns)
}

// GetIdleConnTimeout returns idle connection timeout.
func (h HTTPConfig) GetIdleConnTimeout() time.Duration {
	return secondsToDuration(h.IdleConnTimeoutSec, defaults.GetHTTPIdleConnTimeout())
}

// Load reads and parses a configuration file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return Parse(data)
}

// Parse parses configuration from YAML bytes.
func Parse(data []byte) (*Config, error) {
	var cfg Config

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	// Apply defaults
	cfg.applyDefaults()

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config.
func (c *Config) applyEnvOverrides() {
	// NetBox overrides
	if url := os.Getenv(defaults.EnvNetBoxURL); url != "" {
		c.NetBox.URL = url
	}
	if token := os.Getenv(defaults.EnvNetBoxToken); token != "" {
		c.NetBox.Token = token
	}

	// Default credentials overrides
	if user := os.Getenv(defaults.EnvDefaultUsername); user != "" {
		c.Defaults.Username = user
	}
	if pass := os.Getenv(defaults.EnvDefaultPassword); pass != "" {
		c.Defaults.Password = pass
	}

	// Logging overrides
	if level := os.Getenv(defaults.EnvLogLevel); level != "" {
		c.Logging.Level = level
	}
	if format := os.Getenv(defaults.EnvLogFormat); format != "" {
		c.Logging.Format = format
	}
}

// applyDefaults sets default values for unset fields.
func (c *Config) applyDefaults() {
	// Concurrency
	if c.Concurrency <= 0 {
		c.Concurrency = defaults.GetConcurrency()
	}
	if c.Concurrency > defaults.DefaultMaxConcurrency {
		c.Concurrency = defaults.DefaultMaxConcurrency
	}

	// Timeouts
	if c.Defaults.TimeoutSeconds <= 0 {
		c.Defaults.TimeoutSeconds = defaults.DefaultTimeoutSeconds
	}

	// Logging
	if c.Logging.Level == "" {
		c.Logging.Level = defaults.DefaultLogLevel
	}
	if c.Logging.Format == "" {
		c.Logging.Format = defaults.DefaultLogFormat
	}

	// NetBox
	if c.NetBox.TimeoutSeconds <= 0 {
		c.NetBox.TimeoutSeconds = defaults.DefaultNetBoxTimeoutSeconds
	}
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	multiErr := &errors.MultiError{}

	// Validate servers
	if len(c.Servers) == 0 {
		multiErr.Add(errors.NewConfigError("servers", "no servers configured"))
	}

	for i, srv := range c.Servers {
		if srv.Host == "" {
			multiErr.Add(errors.NewConfigError(
				fmt.Sprintf("server[%d].host", i),
				"host is required"))
		}

		// Check if we have credentials (either per-server or defaults)
		username := srv.GetUsername(c.Defaults.Username)
		password := srv.GetPassword(c.Defaults.Password)

		if username == "" {
			multiErr.Add(errors.NewConfigError(
				fmt.Sprintf("server[%d].username", i),
				fmt.Sprintf("no username configured for %s (set %s or per-server username)",
					srv.Host, defaults.EnvDefaultUsername)))
		}
		if password == "" {
			multiErr.Add(errors.NewConfigError(
				fmt.Sprintf("server[%d].password", i),
				fmt.Sprintf("no password configured for %s (set %s or per-server password)",
					srv.Host, defaults.EnvDefaultPassword)))
		}
	}

	// Validate NetBox config if provided
	if c.NetBox.URL != "" || c.NetBox.Token != "" {
		if c.NetBox.URL == "" {
			multiErr.Add(errors.NewConfigError(
				"netbox.url",
				fmt.Sprintf("url is required when token is set (or set %s)", defaults.EnvNetBoxURL)))
		}
		if c.NetBox.Token == "" {
			multiErr.Add(errors.NewConfigError(
				"netbox.token",
				fmt.Sprintf("token is required when url is set (or set %s)", defaults.EnvNetBoxToken)))
		}

		if c.NetBox.URL != "" {
			if _, err := url.Parse(c.NetBox.URL); err != nil {
				multiErr.Add(errors.NewConfigError("netbox.url", fmt.Sprintf("invalid url: %v", err)))
			}
		}
	}

	// Validate logging config
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(c.Logging.Level)] {
		multiErr.Add(errors.NewConfigError(
			"logging.level",
			fmt.Sprintf("invalid level %q (must be debug, info, warn, or error)", c.Logging.Level)))
	}

	validFormats := map[string]bool{"json": true, "console": true}
	if !validFormats[strings.ToLower(c.Logging.Format)] {
		multiErr.Add(errors.NewConfigError(
			"logging.format",
			fmt.Sprintf("invalid format %q (must be json or console)", c.Logging.Format)))
	}

	return multiErr.ErrorOrNil()
}

// NewSingleServerConfig creates a config for scanning a single server.
// This is useful for CLI single-server mode.
func NewSingleServerConfig(host, username, password string) *Config {
	return &Config{
		Servers: []ServerConfig{
			{
				Host:     host,
				Username: username,
				Password: password,
			},
		},
		Defaults: DefaultsConfig{
			TimeoutSeconds: defaults.DefaultTimeoutSeconds,
		},
		Concurrency: 1,
		Logging: LoggingConfig{
			Level:  defaults.DefaultLogLevel,
			Format: defaults.DefaultLogFormat,
		},
	}
}

// EnvVarHelp returns a list of all supported environment variables with descriptions.
func EnvVarHelp() map[string]string {
	return map[string]string{
		defaults.EnvLogLevel:                 "Log level: debug, info, warn, error (default: info)",
		defaults.EnvLogFormat:                "Log format: json, console (default: console)",
		defaults.EnvDefaultUsername:          "Default iDRAC username",
		defaults.EnvDefaultPassword:          "Default iDRAC password",
		defaults.EnvDefaultTimeout:           "Default connection timeout in seconds (default: 60)",
		defaults.EnvConcurrency:              "Max parallel server scans (default: 5, max: 50)",
		defaults.EnvInsecureSkipVerify:       "Skip TLS verification for iDRAC (default: true)",
		defaults.EnvNetBoxURL:                "NetBox API URL",
		defaults.EnvNetBoxToken:              "NetBox API token",
		defaults.EnvNetBoxTimeout:            "NetBox API timeout in seconds (default: 30)",
		defaults.EnvNetBoxInsecureSkipVerify: "Skip TLS verification for NetBox (default: false)",
		defaults.EnvRetryMaxAttempts:         "Max retry attempts on failure (default: 3)",
		defaults.EnvRetryBaseDelay:           "Base delay between retries (default: 1s)",
		defaults.EnvRetryMaxDelay:            "Max delay between retries (default: 30s)",
	}
}
