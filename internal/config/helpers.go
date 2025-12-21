// Package config provides helper functions for configuration value processing.
package config

import "time"

// secondsToDuration converts a seconds value to time.Duration,
// returning defaultDuration if seconds <= 0.
func secondsToDuration(seconds int, defaultDuration time.Duration) time.Duration {
	if seconds <= 0 {
		return defaultDuration
	}
	return time.Duration(seconds) * time.Second
}

// secondsPtrToDuration converts a pointer to seconds value to time.Duration,
// returning defaultDuration if the pointer is nil or points to a value <= 0.
func secondsPtrToDuration(seconds *int, defaultDuration time.Duration) time.Duration {
	if seconds == nil || *seconds <= 0 {
		return defaultDuration
	}
	return time.Duration(*seconds) * time.Second
}

// getIntOrDefault returns value if it's greater than 0, otherwise returns defaultValue.
func getIntOrDefault(value, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return value
}

// getStringOrDefault returns value if it's non-empty, otherwise returns defaultValue.
func getStringOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

// getBoolPtrOrDefault returns the boolean value pointed to by ptr,
// or defaultValue if ptr is nil.
func getBoolPtrOrDefault(ptr *bool, defaultValue bool) bool {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}
