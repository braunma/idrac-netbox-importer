package models

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerInfo_IsValid(t *testing.T) {
	t.Run("valid server", func(t *testing.T) {
		s := ServerInfo{Host: "192.168.1.10", Model: "R750"}
		assert.True(t, s.IsValid())
	})

	t.Run("server with error", func(t *testing.T) {
		s := ServerInfo{Host: "192.168.1.10", Error: errors.New("connection failed")}
		assert.False(t, s.IsValid())
	})
}

func TestServerInfo_Summary(t *testing.T) {
	t.Run("successful collection", func(t *testing.T) {
		s := ServerInfo{
			Host:             "192.168.1.10",
			Model:            "PowerEdge R750",
			CPUCount:         2,
			TotalMemoryGiB:   512,
			MemorySlotsUsed:  16,
			MemorySlotsTotal: 32,
			DriveCount:       8,
			TotalStorageTB:   7.28,
		}

		summary := s.Summary()
		assert.Contains(t, summary, "192.168.1.10")
		assert.Contains(t, summary, "PowerEdge R750")
		assert.Contains(t, summary, "2 CPUs")
		assert.Contains(t, summary, "512 GiB RAM")
		assert.Contains(t, summary, "16/32 slots")
		assert.Contains(t, summary, "8 drives")
	})

	t.Run("failed collection", func(t *testing.T) {
		s := ServerInfo{
			Host:  "192.168.1.10",
			Error: errors.New("authentication failed"),
		}

		summary := s.Summary()
		assert.Contains(t, summary, "ERROR")
		assert.Contains(t, summary, "authentication failed")
	})
}

func TestServerInfo_MarshalJSON(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		s := ServerInfo{
			Host:  "192.168.1.10",
			Error: errors.New("connection timeout"),
		}

		data, err := json.Marshal(s)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, "192.168.1.10", result["host"])
		assert.Equal(t, "connection timeout", result["error"])
	})

	t.Run("without error", func(t *testing.T) {
		s := ServerInfo{
			Host:  "192.168.1.10",
			Model: "R750",
		}

		data, err := json.Marshal(s)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, "192.168.1.10", result["host"])
		assert.Nil(t, result["error"])
	})
}

func TestServerInfo_GetDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		server   ServerInfo
		expected string
	}{
		{
			name:     "with name set",
			server:   ServerInfo{Name: "web-server-01", HostName: "web01.local", Host: "192.168.1.10"},
			expected: "web-server-01",
		},
		{
			name:     "with hostname only",
			server:   ServerInfo{HostName: "web01.local", Host: "192.168.1.10"},
			expected: "web01.local",
		},
		{
			name:     "with only IP",
			server:   ServerInfo{Host: "192.168.1.10"},
			expected: "192.168.1.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.server.GetDisplayName())
		})
	}
}

func TestCPUInfo_String(t *testing.T) {
	cpu := CPUInfo{
		Socket:      "CPU.Socket.1",
		Model:       "Intel Xeon Gold 6342",
		Cores:       24,
		Threads:     48,
		MaxSpeedMHz: 2800,
	}

	str := cpu.String()
	assert.Contains(t, str, "CPU.Socket.1")
	assert.Contains(t, str, "Intel Xeon Gold 6342")
	assert.Contains(t, str, "24 cores")
	assert.Contains(t, str, "48 threads")
	assert.Contains(t, str, "2800 MHz")
}

func TestCPUInfo_TotalSpeed(t *testing.T) {
	cpu := CPUInfo{Cores: 24, MaxSpeedMHz: 2800}
	assert.Equal(t, 67200, cpu.TotalSpeed())
}

func TestMemoryInfo_State(t *testing.T) {
	t.Run("populated DIMM", func(t *testing.T) {
		m := MemoryInfo{State: MemoryStateEnabled, CapacityMiB: 32768}
		assert.True(t, m.IsPopulated())
		assert.False(t, m.IsEmpty())
	})

	t.Run("empty slot", func(t *testing.T) {
		m := MemoryInfo{State: MemoryStateAbsent}
		assert.False(t, m.IsPopulated())
		assert.True(t, m.IsEmpty())
	})

	t.Run("disabled slot", func(t *testing.T) {
		m := MemoryInfo{State: MemoryStateDisabled}
		assert.False(t, m.IsPopulated())
		assert.False(t, m.IsEmpty())
	})
}

func TestMemoryInfo_CapacityGB(t *testing.T) {
	m := MemoryInfo{CapacityMiB: 32768}
	assert.Equal(t, float64(32), m.CapacityGB())
}

func TestMemoryInfo_String(t *testing.T) {
	t.Run("populated", func(t *testing.T) {
		m := MemoryInfo{
			Slot:         "DIMM.A1",
			State:        MemoryStateEnabled,
			CapacityMiB:  32768,
			Type:         "DDR4",
			SpeedMHz:     3200,
			Manufacturer: "Samsung",
		}

		str := m.String()
		assert.Contains(t, str, "DIMM.A1")
		assert.Contains(t, str, "32 GB")
		assert.Contains(t, str, "DDR4")
		assert.Contains(t, str, "3200 MHz")
		assert.Contains(t, str, "Samsung")
	})

	t.Run("empty", func(t *testing.T) {
		m := MemoryInfo{Slot: "DIMM.A2", State: MemoryStateAbsent}

		str := m.String()
		assert.Contains(t, str, "DIMM.A2")
		assert.Contains(t, str, "[empty]")
	})
}

func TestDriveInfo_Capacity(t *testing.T) {
	d := DriveInfo{CapacityGB: 1024}
	assert.Equal(t, float64(1), d.CapacityTB())
}

func TestDriveInfo_MediaType(t *testing.T) {
	ssd := DriveInfo{MediaType: "SSD"}
	hdd := DriveInfo{MediaType: "HDD"}
	nvme := DriveInfo{MediaType: "NVMe"}

	assert.True(t, ssd.IsSSD())
	assert.False(t, ssd.IsHDD())

	assert.False(t, hdd.IsSSD())
	assert.True(t, hdd.IsHDD())

	assert.False(t, nvme.IsSSD())
	assert.False(t, nvme.IsHDD())
}

func TestDriveInfo_String(t *testing.T) {
	t.Run("SSD with life info", func(t *testing.T) {
		d := DriveInfo{
			Name:        "Disk.Bay.0",
			Model:       "SAMSUNG MZ7LH960",
			CapacityGB:  960,
			MediaType:   "SSD",
			Protocol:    "SATA",
			LifeLeftPct: 98,
		}

		str := d.String()
		assert.Contains(t, str, "Disk.Bay.0")
		assert.Contains(t, str, "960 GB")
		assert.Contains(t, str, "SSD")
		assert.Contains(t, str, "98% life remaining")
	})

	t.Run("HDD without life info", func(t *testing.T) {
		d := DriveInfo{
			Name:       "Disk.Bay.1",
			Model:      "ST4000NM",
			CapacityGB: 4000,
			MediaType:  "HDD",
			Protocol:   "SAS",
		}

		str := d.String()
		assert.Contains(t, str, "4000 GB")
		assert.Contains(t, str, "HDD")
		assert.NotContains(t, str, "life remaining")
	})
}

func TestCollectionStats_SuccessRate(t *testing.T) {
	tests := []struct {
		name     string
		stats    CollectionStats
		expected float64
	}{
		{
			name:     "all successful",
			stats:    CollectionStats{TotalServers: 10, SuccessfulCount: 10},
			expected: 100,
		},
		{
			name:     "all failed",
			stats:    CollectionStats{TotalServers: 10, SuccessfulCount: 0, FailedCount: 10},
			expected: 0,
		},
		{
			name:     "mixed",
			stats:    CollectionStats{TotalServers: 10, SuccessfulCount: 7, FailedCount: 3},
			expected: 70,
		},
		{
			name:     "empty",
			stats:    CollectionStats{TotalServers: 0},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.stats.SuccessRate())
		})
	}
}

func TestCollectionStats_String(t *testing.T) {
	stats := CollectionStats{
		TotalServers:    10,
		SuccessfulCount: 8,
		FailedCount:     2,
		TotalDuration:   5 * time.Second,
	}

	str := stats.String()
	assert.Contains(t, str, "10 servers")
	assert.Contains(t, str, "8 successful")
	assert.Contains(t, str, "2 failed")
	assert.Contains(t, str, "80.0% success rate")
}

func TestConstants(t *testing.T) {
	// Verify constants are defined
	assert.Equal(t, "Enabled", MemoryStateEnabled)
	assert.Equal(t, "Absent", MemoryStateAbsent)
	assert.Equal(t, "Disabled", MemoryStateDisabled)

	assert.Equal(t, "OK", HealthOK)
	assert.Equal(t, "Warning", HealthWarning)
	assert.Equal(t, "Critical", HealthCritical)

	assert.Equal(t, "On", PowerStateOn)
	assert.Equal(t, "Off", PowerStateOff)
}
