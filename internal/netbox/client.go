// Package netbox provides a client for syncing hardware inventory to NetBox.
package netbox

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	"idrac-inventory/internal/config"
	"idrac-inventory/internal/models"
	"idrac-inventory/pkg/defaults"
	"idrac-inventory/pkg/logging"
)

// Client provides methods for interacting with the NetBox API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     *zap.SugaredLogger
	fieldNames FieldNames
}

// FieldNames holds the configurable NetBox custom field names.
type FieldNames struct {
	CPUCount            string
	CPUModel            string
	CPUCores            string
	RAMTotalGB          string
	RAMSlotsTotal       string
	RAMSlotsUsed        string
	RAMSlotsAvailable   string
	RAMType             string
	RAMSpeedMHz         string
	DiskCount           string
	StorageSummary      string
	StorageTotalTB      string
	BIOSVersion         string
	PowerState          string
	PowerConsumedWatts  string
	PowerPeakWatts      string
	LastInventory       string
	// GPU / Accelerator fields ("Beschleuniger" in German iDRAC)
	GPUCount    string
	GPUModel    string
	GPUMemoryGB string
}

// DefaultFieldNames returns the default field names from the defaults package.
func DefaultFieldNames() FieldNames {
	return FieldNames{
		CPUCount:           defaults.NetBoxFieldCPUCount,
		CPUModel:           defaults.NetBoxFieldCPUModel,
		CPUCores:           defaults.NetBoxFieldCPUCores,
		RAMTotalGB:         defaults.NetBoxFieldRAMTotalGB,
		RAMSlotsTotal:      defaults.NetBoxFieldRAMSlotsTotal,
		RAMSlotsUsed:       defaults.NetBoxFieldRAMSlotsUsed,
		RAMSlotsAvailable:  defaults.NetBoxFieldRAMSlotsAvailable,
		RAMType:            defaults.NetBoxFieldRAMType,
		RAMSpeedMHz:        defaults.NetBoxFieldRAMSpeedMHz,
		DiskCount:          defaults.NetBoxFieldDiskCount,
		StorageSummary:     defaults.NetBoxFieldStorageSummary,
		StorageTotalTB:     defaults.NetBoxFieldStorageTotalTB,
		BIOSVersion:        defaults.NetBoxFieldBIOSVersion,
		PowerState:         defaults.NetBoxFieldPowerState,
		PowerConsumedWatts: defaults.NetBoxFieldPowerConsumedWatts,
		PowerPeakWatts:     defaults.NetBoxFieldPowerPeakWatts,
		LastInventory:      defaults.NetBoxFieldLastInventory,
		GPUCount:           defaults.NetBoxFieldGPUCount,
		GPUModel:           defaults.NetBoxFieldGPUModel,
		GPUMemoryGB:        defaults.NetBoxFieldGPUMemoryGB,
	}
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithFieldNames sets custom field names for the NetBox client.
func WithFieldNames(names FieldNames) ClientOption {
	return func(c *Client) {
		c.fieldNames = names
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new NetBox API client.
func NewClient(cfg config.NetBoxConfig, opts ...ClientOption) *Client {
	// Build TLS config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	// Add custom CA certificate if provided
	if cfg.CACert != "" {
		certPool := x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM([]byte(cfg.CACert)); !ok {
			logging.Warn("Failed to parse CA certificate, using system cert pool")
		} else {
			tlsConfig.RootCAs = certPool
			logging.Debug("Custom CA certificate loaded for NetBox connection")
		}
	}

	c := &Client{
		baseURL: cfg.URL,
		token:   cfg.Token,
		httpClient: &http.Client{
			Timeout: cfg.Timeout(),
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
				MaxIdleConns:    defaults.DefaultHTTPMaxIdleConns,
				IdleConnTimeout: defaults.GetHTTPIdleConnTimeout(),
			},
		},
		logger:     logging.WithComponent("netbox"),
		fieldNames: DefaultFieldNames(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Device represents a NetBox device.
type Device struct {
	ID           int                    `json:"id"`
	URL          string                 `json:"url"`
	Name         string                 `json:"name"`
	Serial       string                 `json:"serial"`
	AssetTag     string                 `json:"asset_tag"`
	CustomFields map[string]interface{} `json:"custom_fields"`
}

// DeviceList represents a paginated list of devices.
type DeviceList struct {
	Count    int      `json:"count"`
	Next     string   `json:"next"`
	Previous string   `json:"previous"`
	Results  []Device `json:"results"`
}

// request performs an HTTP request to the NetBox API.
func (c *Client) request(ctx context.Context, method, path string, body interface{}, target interface{}) error {
	fullURL := c.baseURL + path

	c.logger.Debugw("performing API request",
		"method", method,
		"path", path,
	)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Errorw("API request failed",
			"method", method,
			"path", path,
			"error", err,
		)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	c.logger.Debugw("API request completed",
		"method", method,
		"path", path,
		"status_code", resp.StatusCode,
		"duration", duration,
	)

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		c.logger.Errorw("API error response",
			"method", method,
			"path", path,
			"status_code", resp.StatusCode,
			"body", string(respBody),
		)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	// Decode response if target provided
	if target != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, target); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// FindDeviceBySerial searches for a device by its serial number.
func (c *Client) FindDeviceBySerial(ctx context.Context, serial string) (*Device, error) {
	c.logger.Debugw("searching for device by serial",
		"serial", serial,
	)

	path := fmt.Sprintf("%s?serial=%s", defaults.NetBoxDevicesPath, url.QueryEscape(serial))

	var result DeviceList
	if err := c.request(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}

	if result.Count == 0 {
		c.logger.Debugw("device not found",
			"serial", serial,
		)
		return nil, nil
	}

	c.logger.Debugw("device found",
		"serial", serial,
		"device_id", result.Results[0].ID,
		"device_name", result.Results[0].Name,
	)

	return &result.Results[0], nil
}

// FindDeviceByServiceTag searches for a device by its Dell service tag (asset tag).
func (c *Client) FindDeviceByServiceTag(ctx context.Context, serviceTag string) (*Device, error) {
	c.logger.Debugw("searching for device by service tag",
		"service_tag", serviceTag,
	)

	// Try asset_tag first (common for service tags)
	path := fmt.Sprintf("%s?asset_tag=%s", defaults.NetBoxDevicesPath, url.QueryEscape(serviceTag))

	var result DeviceList
	if err := c.request(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}

	if result.Count > 0 {
		return &result.Results[0], nil
	}

	// Fall back to serial number search
	return c.FindDeviceBySerial(ctx, serviceTag)
}

// UpdateDeviceCustomFields updates the custom fields of a device.
func (c *Client) UpdateDeviceCustomFields(ctx context.Context, deviceID int, fields map[string]interface{}) error {
	c.logger.Debugw("updating device custom fields",
		"device_id", deviceID,
		"field_count", len(fields),
	)

	path := fmt.Sprintf("%s%d/", defaults.NetBoxDevicesPath, deviceID)
	body := map[string]interface{}{
		"custom_fields": fields,
	}

	if err := c.request(ctx, http.MethodPatch, path, body, nil); err != nil {
		return fmt.Errorf("failed to update device %d: %w", deviceID, err)
	}

	c.logger.Infow("device custom fields updated",
		"device_id", deviceID,
	)

	return nil
}

// SyncServerInfo syncs a server's hardware information to NetBox.
func (c *Client) SyncServerInfo(ctx context.Context, info models.ServerInfo) error {
	c.logger.Infow("syncing server info to NetBox",
		"host", info.Host,
		"service_tag", info.ServiceTag,
		"serial", info.SerialNumber,
	)

	// Find device using consolidated lookup logic
	device, err := c.findDevice(ctx, info)
	if err != nil {
		return err
	}

	if device == nil {
		c.logger.Warnw("device not found in NetBox",
			"host", info.Host,
			"service_tag", info.ServiceTag,
			"serial", info.SerialNumber,
		)
		return fmt.Errorf("device not found in NetBox (service_tag=%s, serial=%s)",
			info.ServiceTag, info.SerialNumber)
	}

	// Build custom fields payload
	fields := c.buildCustomFields(info)

	// Update the device
	if err := c.UpdateDeviceCustomFields(ctx, device.ID, fields); err != nil {
		return err
	}

	c.logger.Infow("server info synced to NetBox",
		"host", info.Host,
		"device_id", device.ID,
		"device_name", device.Name,
	)

	return nil
}

// buildCustomFields creates the custom fields map for a server.
// Uses configurable field names from the defaults package.
func (c *Client) buildCustomFields(info models.ServerInfo) map[string]interface{} {
	fields := map[string]interface{}{
		c.fieldNames.CPUCount:         info.CPUCount,
		c.fieldNames.CPUModel:         info.CPUModel,
		c.fieldNames.RAMTotalGB:       int(info.TotalMemoryGiB),
		c.fieldNames.RAMSlotsTotal:    info.MemorySlotsTotal,
		c.fieldNames.RAMSlotsUsed:     info.MemorySlotsUsed,
		c.fieldNames.RAMSlotsAvailable: info.MemorySlotsFree,
		c.fieldNames.StorageTotalTB:   fmt.Sprintf("%.2f", info.TotalStorageTB),
		c.fieldNames.BIOSVersion:    info.BiosVersion,
		c.fieldNames.PowerState:     info.PowerState,
		c.fieldNames.LastInventory:  info.CollectedAt.Format(time.RFC3339),
	}

	// Add CPU cores if available
	if len(info.CPUs) > 0 {
		fields[c.fieldNames.CPUCores] = info.CPUs[0].Cores
	}

	// Add memory details if available
	if len(info.Memory) > 0 {
		// Get memory type and speed from first populated module
		for _, mem := range info.Memory {
			if mem.IsPopulated() {
				fields[c.fieldNames.RAMType] = mem.Type
				fields[c.fieldNames.RAMSpeedMHz] = mem.SpeedMHz
				break
			}
		}
	}

	// Add storage information
	fields[c.fieldNames.DiskCount] = info.DriveCount
	if len(info.Drives) > 0 {
		fields[c.fieldNames.StorageSummary] = c.buildStorageSummary(info.Drives)
	}

	// Add power consumption data if available
	if info.PowerConsumedWatts > 0 {
		fields[c.fieldNames.PowerConsumedWatts] = info.PowerConsumedWatts
	}
	if info.PowerPeakWatts > 0 {
		fields[c.fieldNames.PowerPeakWatts] = info.PowerPeakWatts
	}

	// Add GPU/accelerator data ("Beschleuniger" in German iDRAC)
	fields[c.fieldNames.GPUCount] = info.GPUCount
	if len(info.GPUs) > 0 {
		fields[c.fieldNames.GPUModel] = c.buildGPUSummary(info.GPUs)
		totalVRAM := 0
		for _, gpu := range info.GPUs {
			totalVRAM += gpu.MemoryMiB
		}
		if totalVRAM > 0 {
			fields[c.fieldNames.GPUMemoryGB] = int(totalVRAM / 1024)
		}
	}

	return fields
}

// buildGPUSummary returns a compact summary of installed GPUs.
// Example: "4× NVIDIA A100 (80 GB)" or "2× NVIDIA H100, 2× NVIDIA A30"
func (c *Client) buildGPUSummary(gpus []models.GPUInfo) string {
	type gpuKey struct {
		model    string
		memoryGB int
	}
	counts := make(map[gpuKey]int)
	var order []gpuKey

	for _, g := range gpus {
		k := gpuKey{model: g.Model, memoryGB: int(g.MemoryGB())}
		if counts[k] == 0 {
			order = append(order, k)
		}
		counts[k]++
	}

	parts := make([]string, 0, len(order))
	for _, k := range order {
		entry := fmt.Sprintf("%d× %s", counts[k], k.model)
		if k.memoryGB > 0 {
			entry += fmt.Sprintf(" (%d GB)", k.memoryGB)
		}
		parts = append(parts, entry)
	}

	return strings.Join(parts, ", ")
}

// buildStorageSummary creates a grouped summary of drives by capacity.
// Example output: "2x745GB, 16x14306GB"
func (c *Client) buildStorageSummary(drives []models.DriveInfo) string {
	// Group drives by capacity (rounded to nearest GB)
	capacityGroups := make(map[int]int)
	for _, drive := range drives {
		capacityGB := int(drive.CapacityGB)
		capacityGroups[capacityGB]++
	}

	// Sort capacities for consistent output.
	capacities := make([]int, 0, len(capacityGroups))
	for capacity := range capacityGroups {
		capacities = append(capacities, capacity)
	}
	sort.Ints(capacities)

	summary := make([]string, 0, len(capacities))
	for _, capacity := range capacities {
		summary = append(summary, fmt.Sprintf("%dx%dGB", capacityGroups[capacity], capacity))
	}

	return strings.Join(summary, ", ")
}

// findDevice searches for a device in NetBox using service tag and serial number.
// It tries service tag first (which includes fallback to serial), then tries
// serial number directly if service tag is empty.
func (c *Client) findDevice(ctx context.Context, info models.ServerInfo) (*Device, error) {
	c.logger.Infow("searching for device in NetBox",
		"host", info.Host,
		"service_tag", info.ServiceTag,
		"serial_number", info.SerialNumber,
	)

	// Try service tag first (includes serial fallback internally)
	if info.ServiceTag != "" {
		c.logger.Debugw("attempting lookup by service tag",
			"service_tag", info.ServiceTag,
		)
		device, err := c.FindDeviceByServiceTag(ctx, info.ServiceTag)
		if err != nil || device != nil {
			if device != nil {
				c.logger.Infow("device found by service tag",
					"service_tag", info.ServiceTag,
					"device_id", device.ID,
					"device_name", device.Name,
				)
			}
			return device, err
		}
	}

	// Try serial number directly if service tag wasn't provided or didn't match
	if info.SerialNumber != "" {
		c.logger.Debugw("attempting lookup by serial number",
			"serial_number", info.SerialNumber,
		)
		device, err := c.FindDeviceBySerial(ctx, info.SerialNumber)
		if device != nil {
			c.logger.Infow("device found by serial number",
				"serial_number", info.SerialNumber,
				"device_id", device.ID,
				"device_name", device.Name,
			)
		}
		return device, err
	}

	c.logger.Warnw("no service tag or serial number available for device lookup",
		"host", info.Host,
	)
	return nil, nil
}

// TestConnection verifies connectivity to the NetBox API.
func (c *Client) TestConnection(ctx context.Context) error {
	c.logger.Debug("testing connection to NetBox")

	var status struct {
		DjangoVersion string   `json:"django-version"`
		InstalledApps struct{} `json:"installed-apps"`
	}

	if err := c.request(ctx, http.MethodGet, defaults.NetBoxStatusPath, nil, &status); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	c.logger.Infow("connection test successful",
		"django_version", status.DjangoVersion,
	)

	return nil
}

// SyncResult contains the result of syncing a single server.
type SyncResult struct {
	Host    string
	Success bool
	Error   error
}

// SyncAll syncs all provided server information to NetBox.
func (c *Client) SyncAll(ctx context.Context, servers []models.ServerInfo) []SyncResult {
	c.logger.Infow("syncing all servers to NetBox",
		"count", len(servers),
	)

	results := make([]SyncResult, 0, len(servers))

	for _, info := range servers {
		result := SyncResult{Host: info.Host}

		if !info.IsValid() {
			result.Error = fmt.Errorf("skipped: collection failed with error: %v", info.Error)
			results = append(results, result)
			continue
		}

		if err := c.SyncServerInfo(ctx, info); err != nil {
			result.Error = err
		} else {
			result.Success = true
		}

		results = append(results, result)
	}

	// Log summary
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	c.logger.Infow("sync completed",
		"total", len(results),
		"successful", successCount,
		"failed", len(results)-successCount,
	)

	return results
}
