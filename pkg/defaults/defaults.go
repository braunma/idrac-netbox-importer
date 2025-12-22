// Package defaults provides centralized default values for the application.
// All values can be overridden via environment variables or configuration files.
// This package ensures no magic numbers or hardcoded values are scattered throughout the codebase.
package defaults

import (
	"os"
	"strconv"
	"time"
)

// Environment variable names - use these constants throughout the application
// to ensure consistency and make refactoring easier.
const (
	// Application
	EnvLogLevel  = "IDRAC_LOG_LEVEL"
	EnvLogFormat = "IDRAC_LOG_FORMAT"

	// iDRAC Connection
	EnvDefaultUsername    = "IDRAC_DEFAULT_USER"
	EnvDefaultPassword    = "IDRAC_DEFAULT_PASS"
	EnvDefaultTimeout     = "IDRAC_DEFAULT_TIMEOUT"
	EnvConcurrency        = "IDRAC_CONCURRENCY"
	EnvInsecureSkipVerify = "IDRAC_INSECURE_SKIP_VERIFY"

	// NetBox
	EnvNetBoxURL                = "NETBOX_URL"
	EnvNetBoxToken              = "NETBOX_TOKEN"
	EnvNetBoxTimeout            = "NETBOX_TIMEOUT"
	EnvNetBoxInsecureSkipVerify = "NETBOX_INSECURE_SKIP_VERIFY"
	EnvNetBoxCACert             = "CA_Chain"

	// HTTP Client
	EnvHTTPMaxIdleConns    = "HTTP_MAX_IDLE_CONNS"
	EnvHTTPIdleConnTimeout = "HTTP_IDLE_CONN_TIMEOUT"

	// Retry
	EnvRetryMaxAttempts = "IDRAC_RETRY_MAX_ATTEMPTS"
	EnvRetryBaseDelay   = "IDRAC_RETRY_BASE_DELAY"
	EnvRetryMaxDelay    = "IDRAC_RETRY_MAX_DELAY"
)

// Default values - these are used when no environment variable or config is set.
// All defaults are defined here to make them easy to find and modify.
var (
	// Application defaults
	DefaultLogLevel  = getEnvOrDefault(EnvLogLevel, "info")
	DefaultLogFormat = getEnvOrDefault(EnvLogFormat, "console")

	// iDRAC connection defaults
	DefaultUsername           = getEnvOrDefault(EnvDefaultUsername, "")
	DefaultPassword           = getEnvOrDefault(EnvDefaultPassword, "")
	DefaultTimeoutSeconds     = getEnvOrDefaultInt(EnvDefaultTimeout, 60)
	DefaultConcurrency        = getEnvOrDefaultInt(EnvConcurrency, 5)
	DefaultMaxConcurrency     = 50 // Safety limit
	DefaultInsecureSkipVerify = getEnvOrDefaultBool(EnvInsecureSkipVerify, true)

	// NetBox defaults
	DefaultNetBoxTimeoutSeconds     = getEnvOrDefaultInt(EnvNetBoxTimeout, 30)
	DefaultNetBoxInsecureSkipVerify = getEnvOrDefaultBool(EnvNetBoxInsecureSkipVerify, false)

	// HTTP client defaults
	DefaultHTTPMaxIdleConns       = getEnvOrDefaultInt(EnvHTTPMaxIdleConns, 10)
	DefaultHTTPIdleConnTimeoutSec = getEnvOrDefaultInt(EnvHTTPIdleConnTimeout, 30)

	// Retry defaults
	DefaultRetryMaxAttempts = getEnvOrDefaultInt(EnvRetryMaxAttempts, 3)
	DefaultRetryBaseDelay   = getEnvOrDefaultDuration(EnvRetryBaseDelay, 1*time.Second)
	DefaultRetryMaxDelay    = getEnvOrDefaultDuration(EnvRetryMaxDelay, 30*time.Second)
)

// Redfish API paths - centralized for easy maintenance
var (
	RedfishBasePath       = getEnvOrDefault("REDFISH_BASE_PATH", "/redfish/v1")
	RedfishSystemPath     = getEnvOrDefault("REDFISH_SYSTEM_PATH", "/redfish/v1/Systems/System.Embedded.1")
	RedfishProcessorsPath = getEnvOrDefault("REDFISH_PROCESSORS_PATH", "/redfish/v1/Systems/System.Embedded.1/Processors")
	RedfishMemoryPath     = getEnvOrDefault("REDFISH_MEMORY_PATH", "/redfish/v1/Systems/System.Embedded.1/Memory")
	RedfishStoragePath    = getEnvOrDefault("REDFISH_STORAGE_PATH", "/redfish/v1/Systems/System.Embedded.1/Storage")
)

// NetBox API paths
var (
	NetBoxDevicesPath = getEnvOrDefault("NETBOX_DEVICES_PATH", "/api/dcim/devices/")
	NetBoxStatusPath  = getEnvOrDefault("NETBOX_STATUS_PATH", "/api/status/")
)

// NetBox custom field names - configurable for different NetBox setups
var (
	NetBoxFieldCPUCount          = getEnvOrDefault("NETBOX_FIELD_CPU_COUNT", "hw_cpu_count")
	NetBoxFieldCPUModel          = getEnvOrDefault("NETBOX_FIELD_CPU_MODEL", "hw_cpu_model")
	NetBoxFieldCPUCores          = getEnvOrDefault("NETBOX_FIELD_CPU_CORES", "hw_cpu_cores")
	NetBoxFieldRAMTotalGB        = getEnvOrDefault("NETBOX_FIELD_RAM_TOTAL", "hw_ram_total_gb")
	NetBoxFieldRAMSlotsTotal     = getEnvOrDefault("NETBOX_FIELD_RAM_SLOTS_TOTAL", "hw_ram_slots_total")
	NetBoxFieldRAMSlotsUsed      = getEnvOrDefault("NETBOX_FIELD_RAM_SLOTS_USED", "hw_ram_slots_used")
	NetBoxFieldRAMSlotsFree      = getEnvOrDefault("NETBOX_FIELD_RAM_SLOTS_FREE", "hw_ram_slots_free")
	NetBoxFieldRAMType           = getEnvOrDefault("NETBOX_FIELD_RAM_TYPE", "hw_memory_type")
	NetBoxFieldRAMSpeedMHz       = getEnvOrDefault("NETBOX_FIELD_RAM_SPEED", "hw_memory_speed_mhz")
	NetBoxFieldRAMMaxCapacityGB  = getEnvOrDefault("NETBOX_FIELD_RAM_MAX_CAPACITY", "hw_memory_max_capacity_gb")
	NetBoxFieldDiskCount         = getEnvOrDefault("NETBOX_FIELD_DISK_COUNT", "hw_disk_count")
	NetBoxFieldStorageSummary    = getEnvOrDefault("NETBOX_FIELD_STORAGE_SUMMARY", "hw_storage_summary")
	NetBoxFieldStorageTotalTB    = getEnvOrDefault("NETBOX_FIELD_STORAGE_TOTAL", "hw_storage_total_tb")
	NetBoxFieldBIOSVersion       = getEnvOrDefault("NETBOX_FIELD_BIOS_VERSION", "hw_bios_version")
	NetBoxFieldPowerState        = getEnvOrDefault("NETBOX_FIELD_POWER_STATE", "hw_power_state")
	NetBoxFieldLastInventory     = getEnvOrDefault("NETBOX_FIELD_LAST_INVENTORY", "hw_last_inventory")
)

// Helper functions for reading environment variables with defaults

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvOrDefaultBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvOrDefaultDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// GetTimeout returns the iDRAC timeout as a Duration.
func GetTimeout() time.Duration {
	return time.Duration(DefaultTimeoutSeconds) * time.Second
}

// GetNetBoxTimeout returns the NetBox timeout as a Duration.
func GetNetBoxTimeout() time.Duration {
	return time.Duration(DefaultNetBoxTimeoutSeconds) * time.Second
}

// GetHTTPIdleConnTimeout returns the HTTP idle connection timeout as a Duration.
func GetHTTPIdleConnTimeout() time.Duration {
	return time.Duration(DefaultHTTPIdleConnTimeoutSec) * time.Second
}

// GetConcurrency returns the concurrency limit, capped at MaxConcurrency.
func GetConcurrency() int {
	if DefaultConcurrency > DefaultMaxConcurrency {
		return DefaultMaxConcurrency
	}
	if DefaultConcurrency <= 0 {
		return 5
	}
	return DefaultConcurrency
}
