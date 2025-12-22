package netbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"idrac-inventory/internal/config"
	"idrac-inventory/internal/models"
	"idrac-inventory/pkg/logging"
)

func init() {
	_ = logging.Init(logging.Config{
		Level:  "error",
		Format: "console",
	})
}

func mockNetBoxServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization
		auth := r.Header.Get("Authorization")
		if auth != "Token test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		handler(w, r)
	}))
}

func TestClient_FindDeviceBySerial(t *testing.T) {
	server := mockNetBoxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/dcim/devices/" && r.URL.Query().Get("serial") == "ABC123" {
			json.NewEncoder(w).Encode(DeviceList{
				Count: 1,
				Results: []Device{
					{
						ID:     42,
						Name:   "server01",
						Serial: "ABC123",
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	client := NewClient(config.NetBoxConfig{
		URL:   server.URL,
		Token: "test-token",
	})

	ctx := context.Background()
	device, err := client.FindDeviceBySerial(ctx, "ABC123")

	require.NoError(t, err)
	require.NotNil(t, device)
	assert.Equal(t, 42, device.ID)
	assert.Equal(t, "server01", device.Name)
	assert.Equal(t, "ABC123", device.Serial)
}

func TestClient_FindDeviceBySerial_NotFound(t *testing.T) {
	server := mockNetBoxServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(DeviceList{
			Count:   0,
			Results: []Device{},
		})
	})
	defer server.Close()

	client := NewClient(config.NetBoxConfig{
		URL:   server.URL,
		Token: "test-token",
	})

	ctx := context.Background()
	device, err := client.FindDeviceBySerial(ctx, "NOTFOUND")

	require.NoError(t, err)
	assert.Nil(t, device)
}

func TestClient_UpdateDeviceCustomFields(t *testing.T) {
	var receivedBody map[string]interface{}

	server := mockNetBoxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && r.URL.Path == "/api/dcim/devices/42/" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	client := NewClient(config.NetBoxConfig{
		URL:   server.URL,
		Token: "test-token",
	})

	ctx := context.Background()
	fields := map[string]interface{}{
		"hw_cpu_count":    2,
		"hw_ram_total_gb": 512,
	}

	err := client.UpdateDeviceCustomFields(ctx, 42, fields)

	require.NoError(t, err)
	assert.NotNil(t, receivedBody["custom_fields"])

	customFields := receivedBody["custom_fields"].(map[string]interface{})
	assert.Equal(t, float64(2), customFields["hw_cpu_count"])
	assert.Equal(t, float64(512), customFields["hw_ram_total_gb"])
}

func TestClient_SyncServerInfo(t *testing.T) {
	var patchedDeviceID int
	var patchedFields map[string]interface{}

	server := mockNetBoxServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("asset_tag") == "SVCTAG01":
			json.NewEncoder(w).Encode(DeviceList{
				Count: 1,
				Results: []Device{
					{ID: 42, Name: "server01", Serial: "ABC123"},
				},
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/dcim/devices/42/":
			patchedDeviceID = 42
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			patchedFields = body["custom_fields"].(map[string]interface{})
			w.WriteHeader(http.StatusOK)
		default:
			// For serial search fallback
			json.NewEncoder(w).Encode(DeviceList{Count: 0, Results: []Device{}})
		}
	})
	defer server.Close()

	client := NewClient(config.NetBoxConfig{
		URL:   server.URL,
		Token: "test-token",
	})

	ctx := context.Background()
	info := models.ServerInfo{
		Host:             "192.168.1.10",
		ServiceTag:       "SVCTAG01",
		SerialNumber:     "ABC123",
		Model:            "PowerEdge R750",
		CPUCount:         2,
		CPUModel:         "Intel Xeon Gold 6342",
		TotalMemoryGiB:   512,
		MemorySlotsTotal: 32,
		MemorySlotsUsed:  16,
		MemorySlotsFree:  16,
		DriveCount:       8,
		TotalStorageTB:   7.68,
		BiosVersion:      "1.5.1",
		PowerState:       "On",
		CollectedAt:      time.Now(),
		CPUs: []models.CPUInfo{
			{Cores: 24, Threads: 48, MaxSpeedMHz: 2800},
		},
		Memory: []models.MemoryInfo{
			{CapacityMiB: 32768, Type: "DDR5", SpeedMHz: 4800, State: models.MemoryStateEnabled},
			{CapacityMiB: 32768, Type: "DDR5", SpeedMHz: 4800, State: models.MemoryStateEnabled},
		},
		Drives: []models.DriveInfo{
			{CapacityGB: 960},
			{CapacityGB: 960},
			{CapacityGB: 960},
			{CapacityGB: 960},
			{CapacityGB: 1920},
			{CapacityGB: 1920},
			{CapacityGB: 1920},
			{CapacityGB: 1920},
		},
	}

	err := client.SyncServerInfo(ctx, info)

	require.NoError(t, err)
	assert.Equal(t, 42, patchedDeviceID)
	assert.Equal(t, float64(2), patchedFields["hw_cpu_count"])
	assert.Equal(t, float64(512), patchedFields["hw_ram_total_gb"])
	assert.Equal(t, float64(32), patchedFields["hw_ram_slots_total"])
	assert.Equal(t, float64(16), patchedFields["hw_ram_slots_used"])
	assert.Equal(t, float64(16), patchedFields["hw_ram_slots_free"])
	assert.Equal(t, "DDR5", patchedFields["hw_memory_type"])
	assert.Equal(t, float64(4800), patchedFields["hw_memory_speed_mhz"])
	assert.Equal(t, float64(1024), patchedFields["hw_memory_max_capacity_gb"]) // 32 slots × 32 GB
	assert.Equal(t, "4x960GB, 4x1920GB", patchedFields["hw_storage_summary"])
	assert.Equal(t, "1.5.1", patchedFields["hw_bios_version"])
	assert.Equal(t, float64(24), patchedFields["hw_cpu_cores"])
}

func TestClient_SyncServerInfo_DeviceNotFound(t *testing.T) {
	server := mockNetBoxServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(DeviceList{Count: 0, Results: []Device{}})
	})
	defer server.Close()

	client := NewClient(config.NetBoxConfig{
		URL:   server.URL,
		Token: "test-token",
	})

	ctx := context.Background()
	info := models.ServerInfo{
		Host:         "192.168.1.10",
		ServiceTag:   "NOTFOUND",
		SerialNumber: "NOTFOUND",
	}

	err := client.SyncServerInfo(ctx, info)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "device not found")
}

func TestClient_SyncAll(t *testing.T) {
	syncedDevices := make(map[string]bool)

	server := mockNetBoxServer(t, func(w http.ResponseWriter, r *http.Request) {
		serial := r.URL.Query().Get("serial")
		assetTag := r.URL.Query().Get("asset_tag")

		if r.Method == http.MethodGet {
			// Return device for SVCTAG01 and SVCTAG02
			if assetTag == "SVCTAG01" || assetTag == "SVCTAG02" || serial == "SN01" || serial == "SN02" {
				deviceID := 1
				if assetTag == "SVCTAG02" || serial == "SN02" {
					deviceID = 2
				}
				json.NewEncoder(w).Encode(DeviceList{
					Count:   1,
					Results: []Device{{ID: deviceID, Name: "server"}},
				})
				return
			}
			json.NewEncoder(w).Encode(DeviceList{Count: 0, Results: []Device{}})
			return
		}

		if r.Method == http.MethodPatch {
			if r.URL.Path == "/api/dcim/devices/1/" {
				syncedDevices["server1"] = true
			}
			if r.URL.Path == "/api/dcim/devices/2/" {
				syncedDevices["server2"] = true
			}
			w.WriteHeader(http.StatusOK)
		}
	})
	defer server.Close()

	client := NewClient(config.NetBoxConfig{
		URL:   server.URL,
		Token: "test-token",
	})

	ctx := context.Background()
	servers := []models.ServerInfo{
		{Host: "host1", ServiceTag: "SVCTAG01", SerialNumber: "SN01"},
		{Host: "host2", ServiceTag: "SVCTAG02", SerialNumber: "SN02"},
		{Host: "host3", Error: assert.AnError}, // Should be skipped
	}

	results := client.SyncAll(ctx, servers)

	require.Len(t, results, 3)
	assert.True(t, results[0].Success)
	assert.True(t, results[1].Success)
	assert.False(t, results[2].Success)
	assert.Contains(t, results[2].Error.Error(), "skipped")
}

func TestClient_TestConnection(t *testing.T) {
	server := mockNetBoxServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/status/" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"django-version": "4.2",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	client := NewClient(config.NetBoxConfig{
		URL:   server.URL,
		Token: "test-token",
	})

	ctx := context.Background()
	err := client.TestConnection(ctx)

	require.NoError(t, err)
}

func TestClient_AuthenticationFailure(t *testing.T) {
	server := mockNetBoxServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	client := NewClient(config.NetBoxConfig{
		URL:   server.URL,
		Token: "wrong-token", // Wrong token
	})

	ctx := context.Background()
	_, err := client.FindDeviceBySerial(ctx, "ABC123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestBuildCustomFields(t *testing.T) {
	client := NewClient(config.NetBoxConfig{})

	info := models.ServerInfo{
		CPUCount:         2,
		CPUModel:         "Intel Xeon",
		TotalMemoryGiB:   256,
		MemorySlotsTotal: 16,
		MemorySlotsUsed:  8,
		MemorySlotsFree:  8,
		DriveCount:       4,
		TotalStorageTB:   3.84,
		BiosVersion:      "2.0.0",
		PowerState:       "On",
		CollectedAt:      time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		CPUs: []models.CPUInfo{
			{Cores: 16, Threads: 32, MaxSpeedMHz: 3200},
		},
		Memory: []models.MemoryInfo{
			{CapacityMiB: 32768, Type: "DDR4", SpeedMHz: 2933, State: models.MemoryStateEnabled},
			{CapacityMiB: 32768, Type: "DDR4", SpeedMHz: 2933, State: models.MemoryStateEnabled},
			{State: models.MemoryStateAbsent},
		},
		Drives: []models.DriveInfo{
			{CapacityGB: 960},
			{CapacityGB: 960},
			{CapacityGB: 1920},
			{CapacityGB: 1920},
		},
	}

	fields := client.buildCustomFields(info)

	// CPU fields
	assert.Equal(t, 2, fields["hw_cpu_count"])
	assert.Equal(t, "Intel Xeon", fields["hw_cpu_model"])
	assert.Equal(t, 16, fields["hw_cpu_cores"])

	// Memory fields
	assert.Equal(t, 256, fields["hw_ram_total_gb"])
	assert.Equal(t, 16, fields["hw_ram_slots_total"])
	assert.Equal(t, 8, fields["hw_ram_slots_used"])
	assert.Equal(t, 8, fields["hw_ram_slots_free"])
	assert.Equal(t, "DDR4", fields["hw_memory_type"])
	assert.Equal(t, 2933, fields["hw_memory_speed_mhz"])
	assert.Equal(t, 512, fields["hw_memory_max_capacity_gb"]) // 16 slots × 32 GB

	// Storage fields
	assert.Equal(t, "2x960GB, 2x1920GB", fields["hw_storage_summary"])
	assert.Equal(t, "3.84", fields["hw_storage_total_tb"])

	// System fields
	assert.Equal(t, "2.0.0", fields["hw_bios_version"])
	assert.Equal(t, "On", fields["hw_power_state"])
}
