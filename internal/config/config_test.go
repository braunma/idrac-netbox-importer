package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"idrac-inventory/pkg/logging"
)

func init() {
	_ = logging.Init(logging.Config{Level: "error", Format: "console"})
}

// clearTestEnv clears all environment variables that might affect config parsing
func clearTestEnv(t *testing.T) {
	t.Helper()
	envVars := []string{
		"NETBOX_URL",
		"NETBOX_TOKEN",
		"IDRAC_DEFAULT_USER",
		"IDRAC_DEFAULT_PASS",
		"IDRAC_LOG_LEVEL",
		"IDRAC_LOG_FORMAT",
		"IDRAC_DEFAULT_TIMEOUT",
		"IDRAC_CONCURRENCY",
		"IDRAC_INSECURE_SKIP_VERIFY",
		"NETBOX_TIMEOUT",
		"NETBOX_INSECURE_SKIP_VERIFY",
	}

	for _, env := range envVars {
		os.Unsetenv(env)
	}

	// Cleanup after test
	t.Cleanup(func() {
		for _, env := range envVars {
			os.Unsetenv(env)
		}
	})
}

func TestParse_ValidConfig(t *testing.T) {
	clearTestEnv(t) // Clear environment variables that might interfere

	yaml := `
netbox:
  url: "https://netbox.example.com"
  token: "abc123"

defaults:
  username: "root"
  password: "password"
  timeout_seconds: 30

concurrency: 10

servers:
  - host: "192.168.1.10"
  - host: "192.168.1.11"
    username: "admin"
    password: "different"
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	assert.Equal(t, "https://netbox.example.com", cfg.NetBox.URL)
	assert.Equal(t, "abc123", cfg.NetBox.Token)
	assert.Equal(t, 10, cfg.Concurrency)
	assert.Len(t, cfg.Servers, 2)

	// First server uses defaults
	assert.Equal(t, "192.168.1.10", cfg.Servers[0].Host)
	assert.Equal(t, "root", cfg.Servers[0].GetUsername(cfg.Defaults.Username))
	assert.Equal(t, "password", cfg.Servers[0].GetPassword(cfg.Defaults.Password))

	// Second server uses custom credentials
	assert.Equal(t, "admin", cfg.Servers[1].GetUsername(cfg.Defaults.Username))
	assert.Equal(t, "different", cfg.Servers[1].GetPassword(cfg.Defaults.Password))
}

func TestParse_MinimalConfig(t *testing.T) {
	yaml := `
defaults:
  username: "root"
  password: "password"

servers:
  - host: "192.168.1.10"
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	// Check defaults are applied
	assert.Equal(t, 5, cfg.Concurrency)
	assert.Equal(t, 60, cfg.Defaults.TimeoutSeconds)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "console", cfg.Logging.Format)
}

func TestParse_NoServers(t *testing.T) {
	yaml := `
defaults:
  username: "root"
  password: "password"

servers: []
`
	_, err := Parse([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no servers configured")
}

func TestParse_MissingCredentials(t *testing.T) {
	clearTestEnv(t) // Clear environment variables that might interfere

	yaml := `
servers:
  - host: "192.168.1.10"
`
	_, err := Parse([]byte(yaml))
	require.Error(t, err)
	// MultiError shows "2 errors occurred; first: ..." when there are multiple errors
	assert.Contains(t, err.Error(), "2 errors occurred")
	assert.Contains(t, err.Error(), "no username configured")
}

func TestParse_MissingHost(t *testing.T) {
	yaml := `
defaults:
  username: "root"
  password: "password"

servers:
  - name: "server without host"
`
	_, err := Parse([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestParse_InvalidNetBoxConfig(t *testing.T) {
	clearTestEnv(t) // Clear environment variables that might interfere

	yaml := `
netbox:
  url: "https://netbox.example.com"
  # token missing

defaults:
  username: "root"
  password: "password"

servers:
  - host: "192.168.1.10"
`
	_, err := Parse([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token is required")
}

func TestParse_NetBoxURLNormalization(t *testing.T) {
	clearTestEnv(t) // Clear environment variables that might interfere

	tests := []struct {
		name        string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "URL without scheme gets https:// prefix",
			inputURL:    "netbox.example.com",
			expectedURL: "https://netbox.example.com",
		},
		{
			name:        "URL with https:// stays unchanged",
			inputURL:    "https://netbox.example.com",
			expectedURL: "https://netbox.example.com",
		},
		{
			name:        "URL with http:// stays unchanged",
			inputURL:    "http://netbox.example.com",
			expectedURL: "http://netbox.example.com",
		},
		{
			name:        "URL without scheme with path",
			inputURL:    "netbox.example.com/api",
			expectedURL: "https://netbox.example.com/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := `
netbox:
  url: "` + tt.inputURL + `"
  token: "test-token"

defaults:
  username: "root"
  password: "password"

servers:
  - host: "192.168.1.10"
`
			cfg, err := Parse([]byte(yaml))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedURL, cfg.NetBox.URL, "URL should be normalized correctly")
		})
	}
}

func TestParse_InvalidLogLevel(t *testing.T) {
	clearTestEnv(t) // Clear environment variables that might interfere

	yaml := `
defaults:
  username: "root"
  password: "password"

logging:
  level: "verbose"

servers:
  - host: "192.168.1.10"
`
	_, err := Parse([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid level")
}

func TestParse_InvalidLogFormat(t *testing.T) {
	clearTestEnv(t) // Clear environment variables that might interfere

	yaml := `
defaults:
  username: "root"
  password: "password"

logging:
  format: "xml"

servers:
  - host: "192.168.1.10"
`
	_, err := Parse([]byte(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid format")
}

func TestEnvOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("NETBOX_URL", "https://env-netbox.example.com")
	os.Setenv("NETBOX_TOKEN", "env-token")
	os.Setenv("IDRAC_DEFAULT_USER", "env-user")
	os.Setenv("IDRAC_DEFAULT_PASS", "env-pass")
	os.Setenv("IDRAC_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("NETBOX_URL")
		os.Unsetenv("NETBOX_TOKEN")
		os.Unsetenv("IDRAC_DEFAULT_USER")
		os.Unsetenv("IDRAC_DEFAULT_PASS")
		os.Unsetenv("IDRAC_LOG_LEVEL")
	}()

	yaml := `
netbox:
  url: "https://yaml-netbox.example.com"
  token: "yaml-token"

defaults:
  username: "yaml-user"
  password: "yaml-pass"

logging:
  level: "info"

servers:
  - host: "192.168.1.10"
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	// Environment should override YAML
	assert.Equal(t, "https://env-netbox.example.com", cfg.NetBox.URL)
	assert.Equal(t, "env-token", cfg.NetBox.Token)
	assert.Equal(t, "env-user", cfg.Defaults.Username)
	assert.Equal(t, "env-pass", cfg.Defaults.Password)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestServerConfig_GetDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		server   ServerConfig
		expected string
	}{
		{
			name:     "with name",
			server:   ServerConfig{Host: "192.168.1.10", Name: "web-server"},
			expected: "web-server",
		},
		{
			name:     "without name",
			server:   ServerConfig{Host: "192.168.1.10"},
			expected: "192.168.1.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.server.GetDisplayName())
		})
	}
}

func TestServerConfig_GetTimeout(t *testing.T) {
	defaultTimeout := 60 * time.Second

	t.Run("with custom timeout", func(t *testing.T) {
		timeout := 30
		s := ServerConfig{TimeoutSeconds: &timeout}
		assert.Equal(t, 30*time.Second, s.GetTimeout(defaultTimeout))
	})

	t.Run("without custom timeout", func(t *testing.T) {
		s := ServerConfig{}
		assert.Equal(t, defaultTimeout, s.GetTimeout(defaultTimeout))
	})
}

func TestNetBoxConfig_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   NetBoxConfig
		expected bool
	}{
		{
			name:     "both set",
			config:   NetBoxConfig{URL: "https://netbox.example.com", Token: "abc"},
			expected: true,
		},
		{
			name:     "only url",
			config:   NetBoxConfig{URL: "https://netbox.example.com"},
			expected: false,
		},
		{
			name:     "only token",
			config:   NetBoxConfig{Token: "abc"},
			expected: false,
		},
		{
			name:     "neither",
			config:   NetBoxConfig{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.IsEnabled())
		})
	}
}

func TestNetBoxConfig_Timeout(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int
		expected time.Duration
	}{
		{"positive", 30, 30 * time.Second},
		{"zero", 0, 30 * time.Second},
		{"negative", -5, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NetBoxConfig{TimeoutSeconds: tt.seconds}
			assert.Equal(t, tt.expected, cfg.Timeout())
		})
	}
}

func TestDefaultsConfig_Timeout(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int
		expected time.Duration
	}{
		{"positive", 30, 30 * time.Second},
		{"zero", 0, 60 * time.Second},
		{"negative", -5, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultsConfig{TimeoutSeconds: tt.seconds}
			assert.Equal(t, tt.expected, cfg.Timeout())
		})
	}
}

func TestNewSingleServerConfig(t *testing.T) {
	cfg := NewSingleServerConfig("192.168.1.10", "admin", "secret")

	require.Len(t, cfg.Servers, 1)
	assert.Equal(t, "192.168.1.10", cfg.Servers[0].Host)
	assert.Equal(t, "admin", cfg.Servers[0].Username)
	assert.Equal(t, "secret", cfg.Servers[0].Password)
	assert.Equal(t, 1, cfg.Concurrency)
	assert.Equal(t, 60, cfg.Defaults.TimeoutSeconds)
}

func TestConcurrencyLimits(t *testing.T) {
	t.Run("upper limit", func(t *testing.T) {
		yaml := `
concurrency: 100

defaults:
  username: "root"
  password: "password"

servers:
  - host: "192.168.1.10"
`
		cfg, err := Parse([]byte(yaml))
		require.NoError(t, err)
		assert.Equal(t, 50, cfg.Concurrency)
	})

	t.Run("zero defaults to 5", func(t *testing.T) {
		yaml := `
concurrency: 0

defaults:
  username: "root"
  password: "password"

servers:
  - host: "192.168.1.10"
`
		cfg, err := Parse([]byte(yaml))
		require.NoError(t, err)
		assert.Equal(t, 5, cfg.Concurrency)
	})
}
