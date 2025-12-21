// Package redfish provides a client for interacting with Dell iDRAC Redfish API.
package redfish

// ============================================================================
// Redfish API Response Structures
// ============================================================================

// Collection represents a Redfish collection response with multiple members.
type Collection struct {
	OdataContext string `json:"@odata.context"`
	OdataID      string `json:"@odata.id"`
	OdataType    string `json:"@odata.type"`
	Name         string `json:"Name"`
	Description  string `json:"Description"`
	Count        int    `json:"Members@odata.count"`
	Members      []Link `json:"Members"`
}

// Link represents a reference to another Redfish resource.
type Link struct {
	OdataID string `json:"@odata.id"`
}

// Status represents the status of a Redfish component.
type Status struct {
	State        string `json:"State"`
	Health       string `json:"Health"`
	HealthRollup string `json:"HealthRollup"`
}

// System represents a Redfish ComputerSystem resource.
type System struct {
	OdataID     string `json:"@odata.id"`
	OdataType   string `json:"@odata.type"`
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`

	// Identification
	Model        string `json:"Model"`
	Manufacturer string `json:"Manufacturer"`
	SerialNumber string `json:"SerialNumber"`
	SKU          string `json:"SKU"` // Service Tag on Dell
	PartNumber   string `json:"PartNumber"`
	UUID         string `json:"UUID"`

	// Firmware
	BiosVersion string `json:"BiosVersion"`

	// Network
	HostName string `json:"HostName"`

	// State
	PowerState   string `json:"PowerState"`
	IndicatorLED string `json:"IndicatorLED"`

	// Summaries
	MemorySummary    MemorySummary    `json:"MemorySummary"`
	ProcessorSummary ProcessorSummary `json:"ProcessorSummary"`

	// Links to other resources
	Processors Link `json:"Processors"`
	Memory     Link `json:"Memory"`
	Storage    Link `json:"Storage"`

	Status Status `json:"Status"`
}

// MemorySummary provides a summary of memory in the system.
type MemorySummary struct {
	TotalSystemMemoryGiB float64 `json:"TotalSystemMemoryGiB"`
	Status               Status  `json:"Status"`
	MemoryMirroring      string  `json:"MemoryMirroring"`
}

// ProcessorSummary provides a summary of processors in the system.
type ProcessorSummary struct {
	Count                 int    `json:"Count"`
	Model                 string `json:"Model"`
	LogicalProcessorCount int    `json:"LogicalProcessorCount"`
	Status                Status `json:"Status"`
}

// Processor represents a Redfish Processor resource.
type Processor struct {
	OdataID     string `json:"@odata.id"`
	OdataType   string `json:"@odata.type"`
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`

	// Identification
	Socket       string `json:"Socket"`
	Model        string `json:"Model"`
	Manufacturer string `json:"Manufacturer"`

	// Capabilities
	ProcessorType         string `json:"ProcessorType"`
	ProcessorArchitecture string `json:"ProcessorArchitecture"`
	InstructionSet        string `json:"InstructionSet"`
	MaxSpeedMHz           int    `json:"MaxSpeedMHz"`
	OperatingSpeedMHz     int    `json:"OperatingSpeedMHz"`
	TotalCores            int    `json:"TotalCores"`
	TotalEnabledCores     int    `json:"TotalEnabledCores"`
	TotalThreads          int    `json:"TotalThreads"`

	Status Status `json:"Status"`
}

// IsInstalled returns true if the processor is present and enabled.
func (p *Processor) IsInstalled() bool {
	return p.Status.State == StateEnabled
}

// Memory represents a Redfish Memory (DIMM) resource.
type Memory struct {
	OdataID     string `json:"@odata.id"`
	OdataType   string `json:"@odata.type"`
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`

	// Identification
	DeviceLocator  string         `json:"DeviceLocator"`
	MemoryLocation MemoryLocation `json:"MemoryLocation"`
	PartNumber     string         `json:"PartNumber"`
	SerialNumber   string         `json:"SerialNumber"`
	Manufacturer   string         `json:"Manufacturer"`

	// Capabilities
	MemoryDeviceType  string `json:"MemoryDeviceType"`
	MemoryType        string `json:"MemoryType"`
	BaseModuleType    string `json:"BaseModuleType"`
	CapacityMiB       int    `json:"CapacityMiB"`
	DataWidthBits     int    `json:"DataWidthBits"`
	BusWidthBits      int    `json:"BusWidthBits"`
	OperatingSpeedMhz int    `json:"OperatingSpeedMhz"`
	AllowedSpeedsMHz  []int  `json:"AllowedSpeedsMHz"`
	RankCount         int    `json:"RankCount"`

	// ECC
	ErrorCorrection string `json:"ErrorCorrection"`

	Status Status `json:"Status"`
}

// MemoryLocation describes the physical location of a memory module.
type MemoryLocation struct {
	Socket           int `json:"Socket"`
	MemoryController int `json:"MemoryController"`
	Channel          int `json:"Channel"`
	Slot             int `json:"Slot"`
}

// IsPopulated returns true if this memory slot contains a DIMM.
func (m *Memory) IsPopulated() bool {
	return m.Status.State == StateEnabled
}

// IsEmpty returns true if this memory slot is empty.
func (m *Memory) IsEmpty() bool {
	return m.Status.State == StateAbsent
}

// CapacityGB returns the capacity in gigabytes.
func (m *Memory) CapacityGB() float64 {
	return float64(m.CapacityMiB) / 1024
}

// Storage represents a Redfish Storage controller resource.
type Storage struct {
	OdataID     string `json:"@odata.id"`
	OdataType   string `json:"@odata.type"`
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`

	// Controller info
	StorageControllers []StorageController `json:"StorageControllers"`

	// Links to drives
	Drives      []Link `json:"Drives"`
	DrivesCount int    `json:"Drives@odata.count"`

	Status Status `json:"Status"`
}

// StorageController represents information about a storage controller.
type StorageController struct {
	MemberID                 string   `json:"MemberId"`
	Name                     string   `json:"Name"`
	Manufacturer             string   `json:"Manufacturer"`
	Model                    string   `json:"Model"`
	FirmwareVersion          string   `json:"FirmwareVersion"`
	SpeedGbps                float64  `json:"SpeedGbps"`
	SupportedDeviceProtocols []string `json:"SupportedDeviceProtocols"`
	Status                   Status   `json:"Status"`
}

// Drive represents a Redfish Drive resource.
type Drive struct {
	OdataID     string `json:"@odata.id"`
	OdataType   string `json:"@odata.type"`
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`

	// Identification
	Model        string `json:"Model"`
	Manufacturer string `json:"Manufacturer"`
	SerialNumber string `json:"SerialNumber"`
	PartNumber   string `json:"PartNumber"`
	Revision     string `json:"Revision"`
	SKU          string `json:"SKU"`

	// Capabilities
	CapacityBytes      int64   `json:"CapacityBytes"`
	BlockSizeBytes     int     `json:"BlockSizeBytes"`
	RotationSpeedRPM   int     `json:"RotationSpeedRPM"`
	NegotiatedSpeedGbs float64 `json:"NegotiatedSpeedGbs"`
	CapableSpeedGbs    float64 `json:"CapableSpeedGbs"`

	// Type
	MediaType string `json:"MediaType"` // SSD, HDD
	Protocol  string `json:"Protocol"`  // SATA, SAS, NVMe

	// Health
	PredictedMediaLifeLeftPercent float64 `json:"PredictedMediaLifeLeftPercent"`
	FailurePredicted              bool    `json:"FailurePredicted"`

	// Encryption
	EncryptionAbility string `json:"EncryptionAbility"`
	EncryptionStatus  string `json:"EncryptionStatus"`

	// Location
	PhysicalLocation PhysicalLocation `json:"PhysicalLocation"`

	Status Status `json:"Status"`
}

// PhysicalLocation describes the physical location of a component.
type PhysicalLocation struct {
	PartLocation PartLocation `json:"PartLocation"`
}

// PartLocation describes the location within a system.
type PartLocation struct {
	LocationOrdinalValue int    `json:"LocationOrdinalValue"`
	LocationType         string `json:"LocationType"`
	ServiceLabel         string `json:"ServiceLabel"`
}

// CapacityGB returns the drive capacity in gigabytes.
func (d *Drive) CapacityGB() float64 {
	return float64(d.CapacityBytes) / 1024 / 1024 / 1024
}

// CapacityTB returns the drive capacity in terabytes.
func (d *Drive) CapacityTB() float64 {
	return d.CapacityGB() / 1024
}

// IsSSD returns true if this is a solid-state drive.
func (d *Drive) IsSSD() bool {
	return d.MediaType == "SSD"
}

// IsHDD returns true if this is a hard disk drive.
func (d *Drive) IsHDD() bool {
	return d.MediaType == "HDD"
}

// ============================================================================
// Redfish State Constants
// ============================================================================

const (
	StateEnabled            = "Enabled"
	StateDisabled           = "Disabled"
	StateAbsent             = "Absent"
	StateUnavailableOffline = "UnavailableOffline"
	StateStandbyOffline     = "StandbyOffline"
)

const (
	HealthOK       = "OK"
	HealthWarning  = "Warning"
	HealthCritical = "Critical"
)

const (
	PowerStateOn          = "On"
	PowerStateOff         = "Off"
	PowerStatePoweringOn  = "PoweringOn"
	PowerStatePoweringOff = "PoweringOff"
)

// ServiceRoot represents the Redfish service root.
type ServiceRoot struct {
	OdataID        string `json:"@odata.id"`
	OdataType      string `json:"@odata.type"`
	ID             string `json:"Id"`
	Name           string `json:"Name"`
	RedfishVersion string `json:"RedfishVersion"`
	UUID           string `json:"UUID"`
	Product        string `json:"Product"`
	Vendor         string `json:"Vendor"`
}
