package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "info", cfg.Level)
	assert.Equal(t, "console", cfg.Format)
	assert.Equal(t, []string{"stderr"}, cfg.OutputPaths)
	assert.False(t, cfg.Development)
}

func TestReinit(t *testing.T) {
	// Test with different configurations
	configs := []Config{
		{Level: "debug", Format: "console"},
		{Level: "info", Format: "json"},
		{Level: "warn", Format: "console"},
		{Level: "error", Format: "json"},
	}

	for _, cfg := range configs {
		t.Run(cfg.Level+"_"+cfg.Format, func(t *testing.T) {
			err := Reinit(cfg)
			require.NoError(t, err)

			logger := Logger()
			assert.NotNil(t, logger)
		})
	}
}

func TestSetLevel(t *testing.T) {
	err := Reinit(DefaultConfig())
	require.NoError(t, err)

	testCases := []struct {
		level   string
		wantErr bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"invalid", true},
	}

	for _, tc := range testCases {
		t.Run(tc.level, func(t *testing.T) {
			err := SetLevel(tc.level)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.level, GetLevel())
			}
		})
	}
}

func TestWithComponent(t *testing.T) {
	err := Reinit(DefaultConfig())
	require.NoError(t, err)

	logger := WithComponent("test-component")
	assert.NotNil(t, logger)
}

func TestWithFields(t *testing.T) {
	err := Reinit(DefaultConfig())
	require.NoError(t, err)

	logger := WithFields("key1", "value1", "key2", 42)
	assert.NotNil(t, logger)
}

func TestNewNopLogger(t *testing.T) {
	logger := NewNopLogger()
	assert.NotNil(t, logger)

	// Should not panic
	logger.Info("test message")
	logger.Debug("debug message")
	logger.Warn("warn message")
	logger.Error("error message")
}

func TestNewTestLogger(t *testing.T) {
	logger := NewTestLogger()
	assert.NotNil(t, logger)
}

func TestInvalidLevel(t *testing.T) {
	err := Reinit(Config{
		Level:  "invalid",
		Format: "console",
	})
	// Should not error, just default to info
	require.NoError(t, err)
}

func TestLoggingFunctions(t *testing.T) {
	err := Reinit(Config{
		Level:  "debug",
		Format: "console",
	})
	require.NoError(t, err)

	// These should not panic
	Debug("debug message", "key", "value")
	Info("info message", "key", "value")
	Warn("warn message", "key", "value")
	Error("error message", "key", "value")
}

func TestSync(t *testing.T) {
	err := Reinit(DefaultConfig())
	require.NoError(t, err)

	err = Sync()
	// Sync to stderr might return an error on some systems, that's OK
	_ = err
}
