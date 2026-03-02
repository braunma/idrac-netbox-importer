// Package models defines the core data structures used throughout the application.
// These models represent hardware information collected from iDRAC systems.
package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// ServerInfo contains all hardware information collected from a single server.
type ServerInfo struct {
	// Connection details
	Host        string    `json:"host"`
	Name        string    `json:"name,omitempty"`
	CollectedAt time.Time `json:"collected_at"`

	// Error tracking - nil if collection succeeded
	Error error `json:"-"`
	// ErrorMessage is the string representation for JSON serialization
	ErrorMessage string `json:"error,omitempty"`

	// System identification
	Model        string `json:"model"`
	Manufacturer string `json:"manufacturer"`
	SerialNumber string `json:"serial_number"`
	ServiceTag   string `json:"service_tag"`
	BiosVersion  string `json:"bios_version"`
	HostName     string `json:"hostname"`
	PowerState   string `json:"power_state"`

	// CPU information
	CPUs     []CPUInfo `json:"cpus"`
	CPUCount int       `json:"cpu_count"`
	CPUModel string    `json:"cpu_model"`

	// Memory information
	Memory           []MemoryInfo `json:"memory"`
	TotalMemoryGiB   float64      `json:"total_memory_gib"`
	MemorySlotsTotal int          `json:"memory_slots_total"`
	MemorySlotsUsed  int          `json:"memory_slots_used"`
	MemorySlotsFree  int          `json:"memory_slots_free"`

	// Storage information
	Drives         []DriveInfo `json:"drives"`
	DriveCount     int         `json:"drive_count"`
	TotalStorageTB float64     `json:"total_storage_tb"`

	// GPU/Accelerator information ("Beschleuniger" in German iDRAC)
	GPUs     []GPUInfo `json:"gpus,omitempty"`
	GPUCount int       `json:"gpu_count"`

	// Power information
	PowerConsumedWatts int `json:"power_consumed_watts,omitempty"`
	PowerPeakWatts     int `json:"power_peak_watts,omitempty"`
}

// IsValid returns true if the server info was collected without errors.
func (s *ServerInfo) IsValid() bool {
	return s.Error == nil
}

// Summary returns a brief one-line summary of the server.
func (s *ServerInfo) Summary() string {
	if s.Error != nil {
		return fmt.Sprintf("%s: ERROR - %v", s.Host, s.Error)
	}
	return fmt.Sprintf("%s: %s, %d CPUs, %.0f GiB RAM (%d/%d slots), %d drives (%.2f TB)",
		s.Host, s.Model, s.CPUCount, s.TotalMemoryGiB,
		s.MemorySlotsUsed, s.MemorySlotsTotal, s.DriveCount, s.TotalStorageTB)
}

// MarshalJSON implements custom JSON marshaling to include error message.
func (s ServerInfo) MarshalJSON() ([]byte, error) {
	type Alias ServerInfo
	aux := struct {
		Alias
		ErrorMessage string `json:"error,omitempty"`
	}{
		Alias: Alias(s),
	}
	if s.Error != nil {
		aux.ErrorMessage = s.Error.Error()
	}
	return json.Marshal(aux)
}

// GetDisplayName returns the best available name for the server.
func (s *ServerInfo) GetDisplayName() string {
	if s.Name != "" {
		return s.Name
	}
	if s.HostName != "" {
		return s.HostName
	}
	return s.Host
}

// CPUInfo contains detailed information about a single processor.
type CPUInfo struct {
	Socket            string `json:"socket"`
	Model             string `json:"model"`
	Manufacturer      string `json:"manufacturer"`
	Brand             string `json:"brand"`              // CPU brand (e.g., "Intel Xeon", "AMD EPYC")
	Cores             int    `json:"cores"`              // Physical core count
	Threads           int    `json:"threads"`            // Logical thread count
	MaxSpeedMHz       int    `json:"max_speed_mhz"`
	OperatingSpeedMHz int    `json:"operating_speed_mhz"`
	ProcessorType     string `json:"processor_type"`     // e.g., "CPU"
	Architecture      string `json:"architecture"`       // e.g., "x86", "ARM"
	InstructionSet    string `json:"instruction_set"`    // e.g., "x86-64"
	Health            string `json:"health"`
}

// String returns a human-readable representation of the CPU.
func (c CPUInfo) String() string {
	brand := c.Model
	if c.Brand != "" {
		brand = c.Brand
	}
	return fmt.Sprintf("%s: %s (%d cores/%d threads @ %d MHz)",
		c.Socket, brand, c.Cores, c.Threads, c.MaxSpeedMHz)
}

// TotalSpeed returns the total theoretical speed (cores * speed).
func (c CPUInfo) TotalSpeed() int {
	return c.Cores * c.MaxSpeedMHz
}

// MemoryInfo contains detailed information about a single memory module or slot.
type MemoryInfo struct {
	Slot           string `json:"slot"`
	CapacityMiB    int    `json:"capacity_mib"`        // Module size in MiB
	Type           string `json:"type"`                // Memory device type (e.g., "DDR4", "DDR5")
	Technology     string `json:"technology"`          // Memory technology detail
	BaseModuleType string `json:"base_module_type"`    // Module type (e.g., "RDIMM", "UDIMM", "LRDIMM")
	SpeedMHz       int    `json:"speed_mhz"`           // Operating speed
	Manufacturer   string `json:"manufacturer"`
	PartNumber     string `json:"part_number"`
	SerialNumber   string `json:"serial_number"`
	RankCount      int    `json:"rank_count"`          // Number of ranks
	DataWidthBits  int    `json:"data_width_bits"`     // Data width
	State          string `json:"state"`
	Health         string `json:"health"`
}

// Memory state constants as returned by Redfish API.
const (
	MemoryStateEnabled  = "Enabled"
	MemoryStateAbsent   = "Absent"
	MemoryStateDisabled = "Disabled"
)

// IsPopulated returns true if this memory slot contains a DIMM.
func (m MemoryInfo) IsPopulated() bool {
	return m.State == MemoryStateEnabled
}

// IsEmpty returns true if this memory slot is empty.
func (m MemoryInfo) IsEmpty() bool {
	return m.State == MemoryStateAbsent
}

// CapacityGB returns the capacity in gigabytes.
func (m MemoryInfo) CapacityGB() float64 {
	return float64(m.CapacityMiB) / 1024
}

// String returns a human-readable representation of the memory module.
func (m MemoryInfo) String() string {
	if m.IsEmpty() {
		return fmt.Sprintf("%s: [empty]", m.Slot)
	}

	// Include base module type if available (e.g., "DDR4 RDIMM")
	memType := m.Type
	if m.BaseModuleType != "" && m.BaseModuleType != m.Type {
		memType = fmt.Sprintf("%s %s", m.Type, m.BaseModuleType)
	}

	return fmt.Sprintf("%s: %.0f GB %s @ %d MHz (%s)",
		m.Slot, m.CapacityGB(), memType, m.SpeedMHz, m.Manufacturer)
}

// DriveInfo contains detailed information about a single storage drive.
type DriveInfo struct {
	Name         string  `json:"name"`
	Model        string  `json:"model"`
	Manufacturer string  `json:"manufacturer"`
	SerialNumber string  `json:"serial_number"`
	CapacityGB   float64 `json:"capacity_gb"`
	MediaType    string  `json:"media_type"`
	Protocol     string  `json:"protocol"`
	LifeLeftPct  float64 `json:"life_left_pct,omitempty"`
	Health       string  `json:"health"`
}

// CapacityTB returns the capacity in terabytes.
func (d DriveInfo) CapacityTB() float64 {
	return d.CapacityGB / 1024
}

// IsSSD returns true if this is a solid-state drive.
func (d DriveInfo) IsSSD() bool {
	return d.MediaType == "SSD"
}

// IsHDD returns true if this is a hard disk drive.
func (d DriveInfo) IsHDD() bool {
	return d.MediaType == "HDD"
}

// String returns a human-readable representation of the drive.
func (d DriveInfo) String() string {
	lifeInfo := ""
	if d.LifeLeftPct > 0 && d.IsSSD() {
		lifeInfo = fmt.Sprintf(" [%.0f%% life remaining]", d.LifeLeftPct)
	}
	return fmt.Sprintf("%s: %.0f GB %s %s (%s)%s",
		d.Name, d.CapacityGB, d.MediaType, d.Protocol, d.Model, lifeInfo)
}

// GPUInfo contains information about a GPU or accelerator ("Beschleuniger" in German iDRAC).
type GPUInfo struct {
	Slot         string `json:"slot"`
	Model        string `json:"model"`
	Manufacturer string `json:"manufacturer"`
	MemoryMiB    int    `json:"memory_mib"`  // VRAM size in MiB (0 if unknown)
	MemoryType   string `json:"memory_type"` // e.g. "GDDR6", "HBM2"
	Health       string `json:"health"`
}

// MemoryGB returns the GPU VRAM in gigabytes.
func (g GPUInfo) MemoryGB() float64 {
	return float64(g.MemoryMiB) / 1024
}

// String returns a human-readable representation of the GPU.
func (g GPUInfo) String() string {
	if g.MemoryMiB > 0 {
		return fmt.Sprintf("%s: %s (%.0f GB VRAM)", g.Slot, g.Model, g.MemoryGB())
	}
	return fmt.Sprintf("%s: %s", g.Slot, g.Model)
}

// Health status constants.
const (
	HealthOK       = "OK"
	HealthWarning  = "Warning"
	HealthCritical = "Critical"
)

// Power state constants.
const (
	PowerStateOn          = "On"
	PowerStateOff         = "Off"
	PowerStatePoweringOn  = "PoweringOn"
	PowerStatePoweringOff = "PoweringOff"
)

// ScanResult contains the result of scanning multiple servers.
type ScanResult struct {
	Servers   []ServerInfo    `json:"servers"`
	Stats     CollectionStats `json:"stats"`
	StartTime time.Time       `json:"start_time"`
	EndTime   time.Time       `json:"end_time"`
}

// CollectionStats provides statistics about a batch collection operation.
type CollectionStats struct {
	TotalServers    int           `json:"total_servers"`
	SuccessfulCount int           `json:"successful_count"`
	FailedCount     int           `json:"failed_count"`
	TotalDuration   time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	FastestDuration time.Duration `json:"fastest_duration"`
	SlowestDuration time.Duration `json:"slowest_duration"`
}

// SuccessRate returns the percentage of successful collections.
func (s CollectionStats) SuccessRate() float64 {
	if s.TotalServers == 0 {
		return 0
	}
	return float64(s.SuccessfulCount) / float64(s.TotalServers) * 100
}

// String returns a human-readable summary of the collection stats.
func (s CollectionStats) String() string {
	return fmt.Sprintf("Scanned %d servers: %d successful, %d failed (%.1f%% success rate) in %s",
		s.TotalServers, s.SuccessfulCount, s.FailedCount, s.SuccessRate(), s.TotalDuration)
}
