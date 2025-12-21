package scanner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yourusername/idrac-inventory/internal/config"
	"github.com/yourusername/idrac-inventory/internal/models"
	"github.com/yourusername/idrac-inventory/pkg/logging"
)

func init() {
	_ = logging.Init(logging.Config{
		Level:  "error",
		Format: "console",
	})
}

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Concurrency: 10,
	}

	scanner := New(cfg)

	assert.NotNil(t, scanner)
	assert.Equal(t, 10, scanner.concurrency)
}

func TestNew_DefaultConcurrency(t *testing.T) {
	cfg := &config.Config{
		Concurrency: 0,
	}

	scanner := New(cfg)

	assert.Equal(t, 5, scanner.concurrency)
}

func TestCalculateStats(t *testing.T) {
	scanner := New(&config.Config{Concurrency: 5})

	results := []models.ServerInfo{
		{Host: "host1", Model: "R750"},         // Success
		{Host: "host2", Model: "R650"},         // Success
		{Host: "host3", Error: assert.AnError}, // Failure
	}

	durations := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		500 * time.Millisecond,
	}

	stats := scanner.calculateStats(results, durations, 3*time.Second)

	assert.Equal(t, 3, stats.TotalServers)
	assert.Equal(t, 2, stats.SuccessfulCount)
	assert.Equal(t, 1, stats.FailedCount)
	assert.InDelta(t, 66.67, stats.SuccessRate(), 0.1)
	assert.Equal(t, 500*time.Millisecond, stats.FastestDuration)
	assert.Equal(t, 2*time.Second, stats.SlowestDuration)
}

func TestCalculateStats_Empty(t *testing.T) {
	scanner := New(&config.Config{Concurrency: 5})

	stats := scanner.calculateStats([]models.ServerInfo{}, []time.Duration{}, 0)

	assert.Equal(t, 0, stats.TotalServers)
	assert.Equal(t, 0, stats.SuccessfulCount)
	assert.Equal(t, 0, stats.FailedCount)
	assert.Equal(t, float64(0), stats.SuccessRate())
}

func TestCalculateStats_AllSuccess(t *testing.T) {
	scanner := New(&config.Config{Concurrency: 5})

	results := []models.ServerInfo{
		{Host: "host1", Model: "R750"},
		{Host: "host2", Model: "R650"},
	}

	durations := []time.Duration{
		1 * time.Second,
		2 * time.Second,
	}

	stats := scanner.calculateStats(results, durations, 2*time.Second)

	assert.Equal(t, 2, stats.TotalServers)
	assert.Equal(t, 2, stats.SuccessfulCount)
	assert.Equal(t, 0, stats.FailedCount)
	assert.Equal(t, float64(100), stats.SuccessRate())
}

func TestCalculateStats_AllFailed(t *testing.T) {
	scanner := New(&config.Config{Concurrency: 5})

	results := []models.ServerInfo{
		{Host: "host1", Error: assert.AnError},
		{Host: "host2", Error: assert.AnError},
	}

	durations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
	}

	stats := scanner.calculateStats(results, durations, 200*time.Millisecond)

	assert.Equal(t, 2, stats.TotalServers)
	assert.Equal(t, 0, stats.SuccessfulCount)
	assert.Equal(t, 2, stats.FailedCount)
	assert.Equal(t, float64(0), stats.SuccessRate())
}

func TestScanAll_ContextCancellation(t *testing.T) {
	cfg := &config.Config{
		Servers: []config.ServerConfig{
			{Host: "192.168.1.1", Username: "admin", Password: "pass"},
			{Host: "192.168.1.2", Username: "admin", Password: "pass"},
		},
		Defaults: config.DefaultsConfig{
			TimeoutSeconds: 1,
		},
		Concurrency: 2,
	}

	scanner := New(cfg)

	// Create already cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, stats := scanner.ScanAll(ctx)

	// All should fail due to context cancellation
	assert.Equal(t, 2, len(results))
	assert.Equal(t, 2, stats.FailedCount)
}

func TestCollectionStats_SuccessRate(t *testing.T) {
	tests := []struct {
		name     string
		stats    models.CollectionStats
		expected float64
	}{
		{
			name: "all success",
			stats: models.CollectionStats{
				TotalServers:    10,
				SuccessfulCount: 10,
			},
			expected: 100.0,
		},
		{
			name: "half success",
			stats: models.CollectionStats{
				TotalServers:    10,
				SuccessfulCount: 5,
			},
			expected: 50.0,
		},
		{
			name: "none success",
			stats: models.CollectionStats{
				TotalServers:    10,
				SuccessfulCount: 0,
			},
			expected: 0.0,
		},
		{
			name: "no servers",
			stats: models.CollectionStats{
				TotalServers:    0,
				SuccessfulCount: 0,
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.stats.SuccessRate())
		})
	}
}
