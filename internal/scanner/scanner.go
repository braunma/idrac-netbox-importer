// Package scanner provides functionality for scanning Dell iDRAC servers
// via the Redfish API to collect hardware inventory information.
package scanner

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"idrac-inventory/internal/config"
	"idrac-inventory/internal/models"
	"idrac-inventory/internal/redfish"
	"idrac-inventory/pkg/defaults"
	"idrac-inventory/pkg/errors"
	"idrac-inventory/pkg/logging"
)

// Scanner manages hardware inventory scanning across multiple iDRAC servers.
type Scanner struct {
	cfg         *config.Config
	concurrency int
	httpClient  *http.Client
	logger      *zap.SugaredLogger
}

// New creates a new Scanner instance with the provided configuration.
func New(cfg *config.Config) *Scanner {
	// Set concurrency, defaulting to 5 if not configured
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	// Create HTTP client with connection pooling and TLS config
	httpClient := &http.Client{
		Timeout: cfg.Defaults.Timeout(),
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.Defaults.GetInsecureSkipVerify(),
			},
			MaxIdleConns:        cfg.HTTP.GetMaxIdleConns(),
			IdleConnTimeout:     cfg.HTTP.GetIdleConnTimeout(),
			MaxIdleConnsPerHost: 2,
		},
	}

	return &Scanner{
		cfg:         cfg,
		concurrency: concurrency,
		httpClient:  httpClient,
		logger:      logging.WithComponent("scanner"),
	}
}

// ScanAll scans all configured servers in parallel and returns the results with statistics.
func (s *Scanner) ScanAll(ctx context.Context) ([]models.ServerInfo, models.CollectionStats) {
	s.logger.Infow("starting parallel scan",
		"server_count", len(s.cfg.Servers),
		"concurrency", s.concurrency,
	)

	startTime := time.Now()

	// Create buffered channels for work distribution
	jobs := make(chan config.ServerConfig, len(s.cfg.Servers))
	results := make(chan scanResult, len(s.cfg.Servers))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go s.worker(ctx, jobs, results, &wg)
	}

	// Send jobs to workers
	for _, server := range s.cfg.Servers {
		jobs <- server
	}
	close(jobs)

	// Wait for all workers to complete in a separate goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var serverInfos []models.ServerInfo
	var durations []time.Duration

	for result := range results {
		serverInfos = append(serverInfos, result.info)
		durations = append(durations, result.duration)
	}

	totalDuration := time.Since(startTime)

	// Calculate statistics
	stats := s.calculateStats(serverInfos, durations, totalDuration)

	s.logger.Infow("scan completed",
		"total_servers", stats.TotalServers,
		"successful", stats.SuccessfulCount,
		"failed", stats.FailedCount,
		"duration", totalDuration,
	)

	return serverInfos, stats
}

// ValidateConnections tests connectivity to all configured servers without collecting inventory.
func (s *Scanner) ValidateConnections(ctx context.Context) map[string]error {
	s.logger.Infow("validating connections", "server_count", len(s.cfg.Servers))

	results := make(map[string]error)
	var mu sync.Mutex

	// Create buffered channels
	jobs := make(chan config.ServerConfig, len(s.cfg.Servers))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < s.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for server := range jobs {
				err := s.validateConnection(ctx, server)
				mu.Lock()
				results[server.Host] = err
				mu.Unlock()
			}
		}()
	}

	// Send jobs
	for _, server := range s.cfg.Servers {
		jobs <- server
	}
	close(jobs)

	wg.Wait()

	return results
}

// scanResult holds the result of scanning a single server.
type scanResult struct {
	info     models.ServerInfo
	duration time.Duration
}

// worker processes scan jobs from the jobs channel.
func (s *Scanner) worker(ctx context.Context, jobs <-chan config.ServerConfig, results chan<- scanResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for server := range jobs {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			// Context cancelled, return error result
			results <- scanResult{
				info: models.ServerInfo{
					Host:        server.Host,
					Name:        server.Name,
					CollectedAt: time.Now(),
					Error:       ctx.Err(),
				},
				duration: 0,
			}
			continue
		default:
		}

		// Scan the server
		startTime := time.Now()
		info := s.scanServer(ctx, server)
		duration := time.Since(startTime)

		results <- scanResult{
			info:     info,
			duration: duration,
		}
	}
}

// scanServer scans a single iDRAC server and collects hardware information.
func (s *Scanner) scanServer(ctx context.Context, server config.ServerConfig) models.ServerInfo {
	info := models.ServerInfo{
		Host:        server.Host,
		Name:        server.Name,
		CollectedAt: time.Now(),
	}

	s.logger.Debugw("scanning server", "host", server.Host)

	// Get credentials (server-specific or defaults)
	username := server.GetUsername(s.cfg.Defaults.Username)
	password := server.GetPassword(s.cfg.Defaults.Password)
	timeout := server.GetTimeout(s.cfg.Defaults.Timeout())

	// Create context with timeout
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create authenticated client for this server
	client := &redfishClient{
		baseURL:    fmt.Sprintf("https://%s", server.Host),
		username:   username,
		password:   password,
		httpClient: s.httpClient,
		logger:     s.logger,
	}

	// Collect system information
	if err := s.collectSystemInfo(scanCtx, client, &info); err != nil {
		info.Error = err
		s.logger.Warnw("failed to collect system info",
			"host", server.Host,
			"error", err,
		)
		return info
	}

	// Collect processor information
	if err := s.collectProcessors(scanCtx, client, &info); err != nil {
		s.logger.Warnw("failed to collect processor info",
			"host", server.Host,
			"error", err,
		)
		// Don't fail the whole scan, just log the error
	}

	// Collect memory information
	if err := s.collectMemory(scanCtx, client, &info); err != nil {
		s.logger.Warnw("failed to collect memory info",
			"host", server.Host,
			"error", err,
		)
		// Don't fail the whole scan
	}

	// Collect storage information
	if err := s.collectStorage(scanCtx, client, &info); err != nil {
		s.logger.Warnw("failed to collect storage info",
			"host", server.Host,
			"error", err,
		)
		// Don't fail the whole scan
	}

	// Collect power information
	if err := s.collectPowerInfo(scanCtx, client, &info); err != nil {
		s.logger.Debugw("failed to collect power info",
			"host", server.Host,
			"error", err,
		)
		// Don't fail the whole scan - power data is optional
	}

	s.logger.Infow("server scan completed",
		"host", server.Host,
		"model", info.Model,
		"serial_number", info.SerialNumber,
		"service_tag", info.ServiceTag,
		"cpus", info.CPUCount,
		"gpus", info.GPUCount,
		"ram_gb", info.TotalMemoryGiB,
		"drives", info.DriveCount,
	)

	return info
}

// validateConnection tests basic connectivity to an iDRAC server.
func (s *Scanner) validateConnection(ctx context.Context, server config.ServerConfig) error {
	username := server.GetUsername(s.cfg.Defaults.Username)
	password := server.GetPassword(s.cfg.Defaults.Password)
	timeout := server.GetTimeout(s.cfg.Defaults.Timeout())

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := &redfishClient{
		baseURL:    fmt.Sprintf("https://%s", server.Host),
		username:   username,
		password:   password,
		httpClient: s.httpClient,
		logger:     s.logger,
	}

	// Try to fetch the service root
	var root redfish.ServiceRoot
	if err := client.get(ctx, defaults.RedfishBasePath, &root); err != nil {
		return err
	}

	s.logger.Debugw("connection validated",
		"host", server.Host,
		"redfish_version", root.RedfishVersion,
	)

	return nil
}

// collectSystemInfo retrieves system-level information from iDRAC.
func (s *Scanner) collectSystemInfo(ctx context.Context, client *redfishClient, info *models.ServerInfo) error {
	var system redfish.System

	if err := client.get(ctx, defaults.RedfishSystemPath, &system); err != nil {
		return errors.NewCollectionError(info.Host, "system", err)
	}

	// Map system information
	info.Model = system.Model
	info.Manufacturer = system.Manufacturer
	info.SerialNumber = system.SerialNumber
	info.ServiceTag = system.SKU // Dell uses SKU for service tag
	info.BiosVersion = system.BiosVersion
	info.HostName = system.HostName
	info.PowerState = system.PowerState

	// Use processor summary for CPU count and model
	info.CPUCount = system.ProcessorSummary.Count
	info.CPUModel = system.ProcessorSummary.Model

	// Use memory summary for total RAM
	info.TotalMemoryGiB = system.MemorySummary.TotalSystemMemoryGiB

	// Extract Dell OEM memory information if available
	if system.Oem.Dell != nil && system.Oem.Dell.DellSystem != nil {
		dellSys := system.Oem.Dell.DellSystem
		if dellSys.MaxDIMMSlots > 0 {
			info.MemorySlotsTotal = dellSys.MaxDIMMSlots
			s.logger.Debugw("extracted Dell OEM memory slot info",
				"host", info.Host,
				"max_dimm_slots", dellSys.MaxDIMMSlots,
				"populated_slots", dellSys.PopulatedSlots,
			)
		}
	}

	// Log extracted system information
	s.logger.Infow("extracted system information",
		"host", info.Host,
		"manufacturer", info.Manufacturer,
		"model", info.Model,
		"serial_number", info.SerialNumber,
		"service_tag", info.ServiceTag,
		"bios_version", info.BiosVersion,
		"hostname", info.HostName,
		"power_state", info.PowerState,
	)

	return nil
}

// collectProcessors retrieves detailed processor information, including GPUs/accelerators.
func (s *Scanner) collectProcessors(ctx context.Context, client *redfishClient, info *models.ServerInfo) error {
	// Get processor collection
	var collection redfish.Collection
	if err := client.get(ctx, defaults.RedfishProcessorsPath, &collection); err != nil {
		return errors.NewCollectionError(info.Host, "processors", err)
	}

	// Fetch each processor and classify as CPU or GPU/accelerator
	var cpus []models.CPUInfo
	var gpus []models.GPUInfo

	for _, member := range collection.Members {
		var processor redfish.Processor
		if err := client.get(ctx, member.OdataID, &processor); err != nil {
			s.logger.Warnw("failed to get processor details",
				"host", info.Host,
				"path", member.OdataID,
				"error", err,
			)
			continue
		}

		// Only include installed processors
		if !processor.IsInstalled() {
			continue
		}

		if processor.IsGPU() {
			// Collect as GPU/accelerator ("Beschleuniger" in German iDRAC)
			gpu := s.buildGPUInfo(processor)
			gpus = append(gpus, gpu)

			s.logger.Infow("GPU/accelerator details",
				"host", info.Host,
				"slot", gpu.Slot,
				"model", gpu.Model,
				"manufacturer", gpu.Manufacturer,
				"memory_mib", gpu.MemoryMiB,
				"memory_type", gpu.MemoryType,
				"health", gpu.Health,
			)
		} else {
			// Collect as standard CPU
			brand := processor.Model
			if processor.Manufacturer != "" && processor.Model != "" {
				brand = processor.Manufacturer + " " + processor.Model
			}

			cpu := models.CPUInfo{
				Socket:            processor.Socket,
				Model:             processor.Model,
				Manufacturer:      processor.Manufacturer,
				Brand:             brand,
				Cores:             processor.TotalCores,
				Threads:           processor.TotalThreads,
				MaxSpeedMHz:       processor.MaxSpeedMHz,
				OperatingSpeedMHz: processor.OperatingSpeedMHz,
				ProcessorType:     processor.ProcessorType,
				Architecture:      processor.ProcessorArchitecture,
				InstructionSet:    processor.InstructionSet,
				Health:            processor.Status.Health,
			}
			cpus = append(cpus, cpu)
		}
	}

	info.CPUs = cpus
	info.GPUs = gpus
	info.GPUCount = len(gpus)

	// Update count from actual installed CPUs if different from summary
	if len(cpus) > 0 {
		info.CPUCount = len(cpus)
		// Use first CPU's model if more detailed than summary
		if info.CPUModel == "" && cpus[0].Model != "" {
			info.CPUModel = cpus[0].Model
		}

		s.logger.Infow("extracted CPU information",
			"host", info.Host,
			"cpu_count", len(cpus),
		)
		for i, cpu := range cpus {
			s.logger.Infow("CPU details",
				"host", info.Host,
				"cpu_index", i+1,
				"socket", cpu.Socket,
				"brand", cpu.Brand,
				"manufacturer", cpu.Manufacturer,
				"model", cpu.Model,
				"cores", cpu.Cores,
				"threads", cpu.Threads,
				"max_speed_mhz", cpu.MaxSpeedMHz,
				"architecture", cpu.Architecture,
				"instruction_set", cpu.InstructionSet,
			)
		}
	}

	if len(gpus) > 0 {
		s.logger.Infow("extracted GPU/accelerator information",
			"host", info.Host,
			"gpu_count", len(gpus),
		)
	}

	return nil
}

// buildGPUInfo constructs a GPUInfo model from a Redfish Processor entry typed as GPU/Accelerator.
func (s *Scanner) buildGPUInfo(processor redfish.Processor) models.GPUInfo {
	gpu := models.GPUInfo{
		Slot:         processor.Socket,
		Model:        processor.Model,
		Manufacturer: processor.Manufacturer,
		Health:       processor.Status.Health,
	}

	// Use Name as Slot identifier if Socket is empty (common for GPU entries)
	if gpu.Slot == "" {
		gpu.Slot = processor.Name
	}

	// Extract VRAM from inline ProcessorMemory array
	for _, mem := range processor.ProcessorMemory {
		if mem.CapacityMiB > 0 {
			gpu.MemoryMiB += mem.CapacityMiB
			if gpu.MemoryType == "" {
				gpu.MemoryType = mem.MemoryType
			}
		}
	}

	return gpu
}

// collectMemory retrieves detailed memory module information.
func (s *Scanner) collectMemory(ctx context.Context, client *redfishClient, info *models.ServerInfo) error {
	// Get memory collection
	var collection redfish.Collection
	if err := client.get(ctx, defaults.RedfishMemoryPath, &collection); err != nil {
		return errors.NewCollectionError(info.Host, "memory", err)
	}

	// Fetch each memory module
	var memoryModules []models.MemoryInfo
	var totalMemoryMiB int
	slotsUsed := 0
	slotsFree := 0

	for _, member := range collection.Members {
		var memory redfish.Memory
		if err := client.get(ctx, member.OdataID, &memory); err != nil {
			s.logger.Warnw("failed to get memory details",
				"host", info.Host,
				"path", member.OdataID,
				"error", err,
			)
			continue
		}

		// Determine slot name
		slotName := memory.DeviceLocator
		if slotName == "" {
			slotName = memory.ID
		}

		mem := models.MemoryInfo{
			Slot:           slotName,
			CapacityMiB:    memory.CapacityMiB,
			Type:           memory.MemoryDeviceType,
			Technology:     memory.MemoryType,
			BaseModuleType: memory.BaseModuleType,
			SpeedMHz:       memory.OperatingSpeedMhz,
			Manufacturer:   memory.Manufacturer,
			PartNumber:     memory.PartNumber,
			SerialNumber:   memory.SerialNumber,
			RankCount:      memory.RankCount,
			DataWidthBits:  memory.DataWidthBits,
			State:          memory.Status.State,
			Health:         memory.Status.Health,
		}

		memoryModules = append(memoryModules, mem)

		// Count slots and capacity
		if memory.IsPopulated() {
			slotsUsed++
			totalMemoryMiB += memory.CapacityMiB
		} else if memory.IsEmpty() {
			slotsFree++
		}
	}

	info.Memory = memoryModules
	info.MemorySlotsUsed = slotsUsed

	// Set total slots if not already set by OEM data
	if info.MemorySlotsTotal == 0 {
		info.MemorySlotsTotal = len(memoryModules)
	}

	// Calculate free slots
	info.MemorySlotsFree = info.MemorySlotsTotal - slotsUsed

	// Update total memory if we calculated it from DIMMs
	if totalMemoryMiB > 0 {
		calculatedGiB := float64(totalMemoryMiB) / 1024
		// Use calculated value if summary was missing or different
		if info.TotalMemoryGiB == 0 || calculatedGiB > info.TotalMemoryGiB {
			info.TotalMemoryGiB = calculatedGiB
		}
	}

	// Log extracted memory information
	s.logger.Infow("extracted memory information",
		"host", info.Host,
		"total_memory_gib", info.TotalMemoryGiB,
		"slots_total", info.MemorySlotsTotal,
		"slots_used", info.MemorySlotsUsed,
		"slots_free", info.MemorySlotsFree,
	)
	for i, mem := range memoryModules {
		if mem.IsPopulated() {
			s.logger.Infow("memory module details",
				"host", info.Host,
				"module_index", i+1,
				"slot", mem.Slot,
				"capacity_gib", mem.CapacityGB(),
				"type", mem.Type,
				"technology", mem.Technology,
				"base_module_type", mem.BaseModuleType,
				"speed_mhz", mem.SpeedMHz,
				"manufacturer", mem.Manufacturer,
				"rank_count", mem.RankCount,
			)
		}
	}

	return nil
}

// collectStorage retrieves storage controller and drive information.
func (s *Scanner) collectStorage(ctx context.Context, client *redfishClient, info *models.ServerInfo) error {
	// Get storage collection
	var collection redfish.Collection
	if err := client.get(ctx, defaults.RedfishStoragePath, &collection); err != nil {
		return errors.NewCollectionError(info.Host, "storage", err)
	}

	var allDrives []models.DriveInfo
	var totalCapacityBytes int64

	// Iterate through storage controllers
	for _, member := range collection.Members {
		var storage redfish.Storage
		if err := client.get(ctx, member.OdataID, &storage); err != nil {
			s.logger.Warnw("failed to get storage controller",
				"host", info.Host,
				"path", member.OdataID,
				"error", err,
			)
			continue
		}

		// Fetch each drive
		for _, driveLink := range storage.Drives {
			var drive redfish.Drive
			if err := client.get(ctx, driveLink.OdataID, &drive); err != nil {
				s.logger.Warnw("failed to get drive details",
					"host", info.Host,
					"path", driveLink.OdataID,
					"error", err,
				)
				continue
			}

			// Map drive info
			driveInfo := models.DriveInfo{
				Name:         drive.Name,
				Model:        drive.Model,
				Manufacturer: drive.Manufacturer,
				SerialNumber: drive.SerialNumber,
				CapacityGB:   drive.CapacityGB(),
				MediaType:    drive.MediaType,
				Protocol:     drive.Protocol,
				LifeLeftPct:  drive.PredictedMediaLifeLeftPercent,
				Health:       drive.Status.Health,
			}

			allDrives = append(allDrives, driveInfo)
			totalCapacityBytes += drive.CapacityBytes
		}
	}

	info.Drives = allDrives
	info.DriveCount = len(allDrives)

	// Calculate total storage in TB
	if totalCapacityBytes > 0 {
		info.TotalStorageTB = float64(totalCapacityBytes) / 1024 / 1024 / 1024 / 1024
	}

	// Log extracted storage information
	s.logger.Infow("extracted storage information",
		"host", info.Host,
		"total_drives", info.DriveCount,
		"total_storage_tb", fmt.Sprintf("%.2f", info.TotalStorageTB),
	)
	for i, drive := range allDrives {
		s.logger.Infow("drive details",
			"host", info.Host,
			"drive_index", i+1,
			"name", drive.Name,
			"model", drive.Model,
			"manufacturer", drive.Manufacturer,
			"serial_number", drive.SerialNumber,
			"capacity_gb", fmt.Sprintf("%.2f", drive.CapacityGB),
			"capacity_tb", fmt.Sprintf("%.2f", drive.CapacityTB()),
			"media_type", drive.MediaType,
			"protocol", drive.Protocol,
			"health", drive.Health,
		)
	}

	return nil
}

// collectPowerInfo retrieves power consumption information from the chassis.
// This function is resilient - it will not fail if power data is unavailable.
func (s *Scanner) collectPowerInfo(ctx context.Context, client *redfishClient, info *models.ServerInfo) error {
	var power redfish.Power
	if err := client.get(ctx, defaults.RedfishPowerPath, &power); err != nil {
		// Power data may not be available on all systems
		return errors.NewCollectionError(info.Host, "power", err)
	}

	// Extract power consumption data from the first PowerControl entry
	if len(power.PowerControl) > 0 {
		pc := power.PowerControl[0]

		// Set current power consumption if available
		if pc.PowerConsumedWatts > 0 {
			info.PowerConsumedWatts = pc.PowerConsumedWatts
		}

		// Set peak power consumption from metrics if available
		if pc.PowerMetrics.MaxConsumedWatts > 0 {
			info.PowerPeakWatts = pc.PowerMetrics.MaxConsumedWatts
		}

		s.logger.Infow("extracted power information",
			"host", info.Host,
			"power_consumed_watts", info.PowerConsumedWatts,
			"power_peak_watts", info.PowerPeakWatts,
			"metrics_interval_min", pc.PowerMetrics.IntervalInMin,
		)
	}

	return nil
}

// calculateStats computes statistics from scan results.
func (s *Scanner) calculateStats(results []models.ServerInfo, durations []time.Duration, totalDuration time.Duration) models.CollectionStats {
	stats := models.CollectionStats{
		TotalServers:  len(results),
		TotalDuration: totalDuration,
	}

	if len(results) == 0 {
		return stats
	}

	// Count successes and failures
	for _, result := range results {
		if result.Error == nil {
			stats.SuccessfulCount++
		} else {
			stats.FailedCount++
		}
	}

	// Calculate duration statistics
	if len(durations) > 0 {
		var totalDur time.Duration
		fastest := durations[0]
		slowest := durations[0]

		for _, dur := range durations {
			totalDur += dur
			if dur < fastest {
				fastest = dur
			}
			if dur > slowest {
				slowest = dur
			}
		}

		stats.AverageDuration = totalDur / time.Duration(len(durations))
		stats.FastestDuration = fastest
		stats.SlowestDuration = slowest
	}

	return stats
}

// ============================================================================
// Redfish HTTP Client
// ============================================================================

// redfishClient handles HTTP communication with a Redfish API endpoint.
type redfishClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	logger     *zap.SugaredLogger
}

// get performs a GET request to the Redfish API and unmarshals the response.
func (c *redfishClient) get(ctx context.Context, path string, target interface{}) error {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	req.SetBasicAuth(c.username, c.password)

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "idrac-inventory/1.0")

	// Make request
	c.logger.Debugw("making redfish request",
		"method", "GET",
		"url", url,
	)

	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.NewRedfishError(c.baseURL, path, 0, "", err.Error())
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	c.logger.Debugw("redfish request completed",
		"url", url,
		"status", resp.StatusCode,
		"duration", duration,
	)

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		c.logger.Errorw("redfish API error",
			"url", url,
			"status", resp.StatusCode,
			"body", string(body),
		)

		// Check for authentication error
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return errors.ErrAuthenticationFailed
		}

		if resp.StatusCode == 404 {
			return errors.ErrNotFound
		}

		return errors.NewRedfishError(c.baseURL, path, resp.StatusCode, resp.Status, string(body))
	}

	// Unmarshal JSON
	if target != nil {
		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}
