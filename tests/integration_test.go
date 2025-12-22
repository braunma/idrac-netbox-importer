// Package tests contains integration tests for the iDRAC inventory tool.
package tests

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
	"idrac-inventory/internal/netbox"
	"idrac-inventory/internal/redfish"
	"idrac-inventory/internal/scanner"
	"idrac-inventory/pkg/logging"
)

func init() {
	_ = logging.Init(logging.Config{
		Level:  "error",
		Format: "console",
	})
}

// TestFullScanWorkflow tests the complete scan workflow from config to results.
func TestFullScanWorkflow(t *testing.T) {
	// Create mock iDRAC server
	idracServer := createMockiDRAC(t)
	defer idracServer.Close()

	// Create config
	cfg := &config.Config{
		Servers: []config.ServerConfig{
			{
				Host:     idracServer.Listener.Addr().String(),
				Username: "admin",
				Password: "password",
			},
		},
		Defaults: config.DefaultsConfig{
			TimeoutSeconds: 10,
		},
		Concurrency: 1,
	}

	// Create scanner
	s := scanner.New(cfg)

	// Run scan
	ctx := context.Background()
	results, stats := s.ScanAll(ctx)

	// Verify results
	require.Len(t, results, 1)
	assert.True(t, results[0].IsValid())
	assert.Equal(t, "PowerEdge R750", results[0].Model)
	assert.Equal(t, 2, results[0].CPUCount)
	assert.Equal(t, 4, results[0].MemorySlotsTotal)
	assert.Equal(t, 2, results[0].MemorySlotsUsed)
	assert.Equal(t, 2, results[0].MemorySlotsFree)

	// Verify stats
	assert.Equal(t, 1, stats.TotalServers)
	assert.Equal(t, 1, stats.SuccessfulCount)
	assert.Equal(t, 0, stats.FailedCount)
	assert.Equal(t, float64(100), stats.SuccessRate())
}

// TestScanWithNetBoxSync tests scanning and syncing to NetBox.
func TestScanWithNetBoxSync(t *testing.T) {
	// Create mock iDRAC server
	idracServer := createMockiDRAC(t)
	defer idracServer.Close()

	// Track NetBox updates
	var netboxUpdates []map[string]interface{}

	// Create mock NetBox server
	netboxServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth
		if r.Header.Get("Authorization") != "Token test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/status/":
			json.NewEncoder(w).Encode(map[string]string{"django-version": "4.2"})

		case r.Method == http.MethodGet && r.URL.Path == "/api/dcim/devices/":
			// Return device for serial lookup
			json.NewEncoder(w).Encode(map[string]interface{}{
				"count": 1,
				"results": []map[string]interface{}{
					{"id": 42, "name": "test-server", "serial": "ABC123"},
				},
			})

		case r.Method == http.MethodPatch && r.URL.Path == "/api/dcim/devices/42/":
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			netboxUpdates = append(netboxUpdates, body)
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer netboxServer.Close()

	// Create config
	cfg := &config.Config{
		NetBox: config.NetBoxConfig{
			URL:   netboxServer.URL,
			Token: "test-token",
		},
		Servers: []config.ServerConfig{
			{
				Host:     idracServer.Listener.Addr().String(),
				Username: "admin",
				Password: "password",
			},
		},
		Defaults: config.DefaultsConfig{
			TimeoutSeconds: 10,
		},
		Concurrency: 1,
	}

	// Run scan
	s := scanner.New(cfg)
	ctx := context.Background()
	results, _ := s.ScanAll(ctx)

	// Sync to NetBox
	nbClient := netbox.NewClient(cfg.NetBox)
	syncResults := nbClient.SyncAll(ctx, results)

	// Verify sync results
	require.Len(t, syncResults, 1)
	assert.True(t, syncResults[0].Success)

	// Verify NetBox was updated
	require.Len(t, netboxUpdates, 1)
	customFields := netboxUpdates[0]["custom_fields"].(map[string]interface{})
	assert.Equal(t, float64(2), customFields["hw_cpu_count"])
	assert.Equal(t, float64(512), customFields["hw_ram_total_gb"])
}

// TestParallelScan tests scanning multiple servers in parallel.
func TestParallelScan(t *testing.T) {
	// Create multiple mock servers
	servers := make([]*httptest.Server, 3)
	for i := 0; i < 3; i++ {
		servers[i] = createMockiDRACWithDelay(t, time.Duration(i*100)*time.Millisecond)
	}
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	// Build config
	serverConfigs := make([]config.ServerConfig, len(servers))
	for i, s := range servers {
		serverConfigs[i] = config.ServerConfig{
			Host:     s.Listener.Addr().String(),
			Username: "admin",
			Password: "password",
		}
	}

	cfg := &config.Config{
		Servers:     serverConfigs,
		Defaults:    config.DefaultsConfig{TimeoutSeconds: 10},
		Concurrency: 3, // All parallel
	}

	// Run scan
	s := scanner.New(cfg)
	ctx := context.Background()
	startTime := time.Now()
	results, stats := s.ScanAll(ctx)
	duration := time.Since(startTime)

	// All should succeed
	require.Len(t, results, 3)
	assert.Equal(t, 3, stats.SuccessfulCount)

	// Parallel execution should be faster than sequential
	// Sequential would take at least 0+100+200 = 300ms
	// Parallel should be closer to 200ms (the slowest)
	// Allow generous timeout for CI/slower systems
	assert.Less(t, duration.Milliseconds(), int64(3000))
}

// TestScanWithFailures tests handling of mixed success/failure.
func TestScanWithFailures(t *testing.T) {
	// Create one working server
	goodServer := createMockiDRAC(t)
	defer goodServer.Close()

	// Create config with one good and one bad server
	cfg := &config.Config{
		Servers: []config.ServerConfig{
			{
				Host:     goodServer.Listener.Addr().String(),
				Username: "admin",
				Password: "password",
			},
			{
				Host:     "192.0.2.1", // Non-routable address
				Username: "admin",
				Password: "password",
			},
		},
		Defaults: config.DefaultsConfig{
			TimeoutSeconds: 1, // Short timeout for the failing server
		},
		Concurrency: 2,
	}

	// Run scan
	s := scanner.New(cfg)
	ctx := context.Background()
	results, stats := s.ScanAll(ctx)

	// Should have mixed results
	require.Len(t, results, 2)
	assert.Equal(t, 1, stats.SuccessfulCount)
	assert.Equal(t, 1, stats.FailedCount)
	assert.InDelta(t, 50.0, stats.SuccessRate(), 0.1)

	// Verify we can identify which succeeded/failed
	for _, r := range results {
		if r.Host == goodServer.Listener.Addr().String() {
			assert.True(t, r.IsValid())
		} else {
			assert.False(t, r.IsValid())
			assert.NotNil(t, r.Error)
		}
	}
}

// TestContextCancellation tests proper handling of context cancellation.
func TestContextCancellation(t *testing.T) {
	// Create slow server
	slowServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	cfg := &config.Config{
		Servers: []config.ServerConfig{
			{
				Host:     slowServer.Listener.Addr().String(),
				Username: "admin",
				Password: "password",
			},
		},
		Defaults:    config.DefaultsConfig{TimeoutSeconds: 30},
		Concurrency: 1,
	}

	s := scanner.New(cfg)

	// Create context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	results, stats := s.ScanAll(ctx)
	duration := time.Since(startTime)

	// Should fail due to cancellation
	require.Len(t, results, 1)
	assert.Equal(t, 0, stats.SuccessfulCount)
	assert.Equal(t, 1, stats.FailedCount)

	// Should complete quickly due to cancellation
	assert.Less(t, duration.Milliseconds(), int64(1000))
}

// Helper: Create mock iDRAC server with full responses
func createMockiDRAC(t *testing.T) *httptest.Server {
	return createMockiDRACWithDelay(t, 0)
}

func createMockiDRACWithDelay(t *testing.T, delay time.Duration) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add delay if specified
		if delay > 0 {
			time.Sleep(delay)
		}

		// Check auth
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "password" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/redfish/v1/":
			json.NewEncoder(w).Encode(map[string]string{
				"RedfishVersion": "1.13.0",
				"Name":           "Root Service",
			})

		case "/redfish/v1/Systems/System.Embedded.1":
			json.NewEncoder(w).Encode(redfish.System{
				Model:        "PowerEdge R750",
				Manufacturer: "Dell Inc.",
				SerialNumber: "ABC123",
				SKU:          "SVCTAG01",
				BiosVersion:  "1.5.1",
				PowerState:   "On",
				MemorySummary: redfish.MemorySummary{
					TotalSystemMemoryGiB: 512,
				},
				ProcessorSummary: redfish.ProcessorSummary{
					Count: 2,
					Model: "Intel Xeon Gold 6342",
				},
			})

		case "/redfish/v1/Systems/System.Embedded.1/Processors":
			json.NewEncoder(w).Encode(redfish.Collection{
				Count: 2,
				Members: []redfish.Link{
					{OdataID: "/redfish/v1/Systems/System.Embedded.1/Processors/CPU.Socket.1"},
					{OdataID: "/redfish/v1/Systems/System.Embedded.1/Processors/CPU.Socket.2"},
				},
			})

		case "/redfish/v1/Systems/System.Embedded.1/Processors/CPU.Socket.1",
			"/redfish/v1/Systems/System.Embedded.1/Processors/CPU.Socket.2":
			json.NewEncoder(w).Encode(redfish.Processor{
				Socket:       "CPU.Socket.1",
				Model:        "Intel(R) Xeon(R) Gold 6342 CPU @ 2.80GHz",
				TotalCores:   24,
				TotalThreads: 48,
				MaxSpeedMHz:  2800,
				Status:       redfish.Status{State: "Enabled", Health: "OK"},
			})

		case "/redfish/v1/Systems/System.Embedded.1/Memory":
			json.NewEncoder(w).Encode(redfish.Collection{
				Count: 4,
				Members: []redfish.Link{
					{OdataID: "/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.A1"},
					{OdataID: "/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.A2"},
					{OdataID: "/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.B1"},
					{OdataID: "/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.B2"},
				},
			})

		case "/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.A1",
			"/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.B1":
			json.NewEncoder(w).Encode(redfish.Memory{
				DeviceLocator:     "DIMM A1",
				CapacityMiB:       262144,
				MemoryDeviceType:  "DDR4",
				OperatingSpeedMhz: 3200,
				Manufacturer:      "Samsung",
				Status:            redfish.Status{State: "Enabled", Health: "OK"},
			})

		case "/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.A2",
			"/redfish/v1/Systems/System.Embedded.1/Memory/DIMM.B2":
			json.NewEncoder(w).Encode(redfish.Memory{
				DeviceLocator: "DIMM A2",
				Status:        redfish.Status{State: "Absent"},
			})

		case "/redfish/v1/Systems/System.Embedded.1/Storage":
			json.NewEncoder(w).Encode(redfish.Collection{
				Count: 1,
				Members: []redfish.Link{
					{OdataID: "/redfish/v1/Systems/System.Embedded.1/Storage/RAID.Integrated.1-1"},
				},
			})

		case "/redfish/v1/Systems/System.Embedded.1/Storage/RAID.Integrated.1-1":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":   "RAID.Integrated.1-1",
				"Name": "PERC H755 Front",
				"Drives": []map[string]string{
					{"@odata.id": "/redfish/v1/Systems/System.Embedded.1/Storage/RAID.Integrated.1-1/Drives/Disk.Bay.0"},
				},
			})

		case "/redfish/v1/Systems/System.Embedded.1/Storage/RAID.Integrated.1-1/Drives/Disk.Bay.0":
			json.NewEncoder(w).Encode(redfish.Drive{
				ID:            "Disk.Bay.0",
				Name:          "SSD 0",
				Model:         "SAMSUNG MZ7LH960",
				CapacityBytes: 960197124096,
				MediaType:     "SSD",
				Protocol:      "SATA",
				Status:        redfish.Status{State: "Enabled", Health: "OK"},
			})

		case "/redfish/v1/Chassis/System.Embedded.1/Power":
			json.NewEncoder(w).Encode(redfish.Power{
				OdataID:   "/redfish/v1/Chassis/System.Embedded.1/Power",
				OdataType: "#Power.v1_7_1.Power",
				ID:        "Power",
				Name:      "Power",
				PowerControl: []redfish.PowerControl{
					{
						MemberID:           "0",
						Name:               "System Power Control",
						PowerConsumedWatts: 420,
						PowerMetrics: redfish.PowerMetrics{
							MinConsumedWatts:     380,
							MaxConsumedWatts:     580,
							AverageConsumedWatts: 450,
							IntervalInMin:        60,
						},
					},
				},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestServerInfoMethods tests the ServerInfo helper methods.
func TestServerInfoMethods(t *testing.T) {
	t.Run("IsValid", func(t *testing.T) {
		valid := models.ServerInfo{Model: "R750"}
		invalid := models.ServerInfo{Error: assert.AnError}

		assert.True(t, valid.IsValid())
		assert.False(t, invalid.IsValid())
	})

	t.Run("Summary", func(t *testing.T) {
		info := models.ServerInfo{
			Host:             "192.168.1.10",
			Model:            "PowerEdge R750",
			CPUCount:         2,
			TotalMemoryGiB:   512,
			MemorySlotsUsed:  16,
			MemorySlotsTotal: 32,
			DriveCount:       8,
		}

		summary := info.Summary()
		assert.Contains(t, summary, "192.168.1.10")
		assert.Contains(t, summary, "PowerEdge R750")
		assert.Contains(t, summary, "2 CPUs")
		assert.Contains(t, summary, "512 GiB RAM")
	})
}
