// Package models defines the core data structures used throughout the application.
// This file provides aggregation models for grouping servers by hardware configuration.
package models

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// HardwareFingerprint uniquely identifies a hardware configuration.
// Servers with identical fingerprints are grouped together in the report.
type HardwareFingerprint struct {
	Manufacturer      string `json:"manufacturer"`
	Model             string `json:"model"`
	CPUCount          int    `json:"cpu_count"`
	CPUModel          string `json:"cpu_model"`
	CPUCoresPerSocket int    `json:"cpu_cores_per_socket"`
	CPUSpeedMHz       int    `json:"cpu_speed_mhz"`
	RAMTotalGiB       int    `json:"ram_total_gib"`
	RAMType           string `json:"ram_type"`
	RAMSpeedMHz       int    `json:"ram_speed_mhz"`
	RAMSlotsTotal     int    `json:"ram_slots_total"`
	StorageSummary    string `json:"storage_summary"` // e.g. "2×745GB SSD, 4×14306GB HDD"
	// GPU / Accelerator ("Beschleuniger" in German iDRAC)
	GPUCount    int    `json:"gpu_count"`
	GPUModel    string `json:"gpu_model"`    // model of the first GPU (all assumed identical)
	GPUMemoryGiB int   `json:"gpu_memory_gib"` // VRAM per GPU in GiB
}

// Key returns a stable string key suitable for use in a map.
func (f HardwareFingerprint) Key() string {
	return fmt.Sprintf("%s|%s|%d|%s|%d|%d|%d|%s|%d|%d|%s|%d|%s|%d",
		f.Manufacturer, f.Model,
		f.CPUCount, f.CPUModel, f.CPUCoresPerSocket, f.CPUSpeedMHz,
		f.RAMTotalGiB, f.RAMType, f.RAMSpeedMHz, f.RAMSlotsTotal,
		f.StorageSummary,
		f.GPUCount, f.GPUModel, f.GPUMemoryGiB,
	)
}

// DisplayModel returns a human-friendly model string including manufacturer.
func (f HardwareFingerprint) DisplayModel() string {
	if f.Manufacturer != "" && !strings.HasPrefix(strings.ToLower(f.Model), strings.ToLower(f.Manufacturer)) {
		return f.Manufacturer + " " + f.Model
	}
	return f.Model
}

// HardwareGroup represents a set of servers that share identical hardware configuration.
type HardwareGroup struct {
	Fingerprint    HardwareFingerprint `json:"fingerprint"`
	Count          int                 `json:"count"`
	Servers        []ServerInfo        `json:"servers"`
	TotalStorageTB float64             `json:"total_storage_tb,omitempty"` // from first server
}

// AggregatedInventory is the top-level structure for the aggregated hardware report.
type AggregatedInventory struct {
	GeneratedAt    time.Time       `json:"generated_at"`
	TotalServers   int             `json:"total_servers"`
	SuccessfulCount int            `json:"successful_count"`
	FailedCount    int             `json:"failed_count"`
	Groups         []HardwareGroup `json:"groups"`
	FailedServers  []ServerInfo    `json:"failed_servers,omitempty"`
	Stats          CollectionStats `json:"stats"`
}

// GroupByConfiguration groups servers that share the same hardware configuration.
// Failed servers are collected separately. Groups are sorted by count (descending).
func GroupByConfiguration(servers []ServerInfo, stats CollectionStats) AggregatedInventory {
	inv := AggregatedInventory{
		GeneratedAt:  time.Now().UTC(),
		TotalServers: len(servers),
		Stats:        stats,
	}

	groupMap := make(map[string]*HardwareGroup)
	var groupOrder []string

	for _, srv := range servers {
		if srv.Error != nil {
			inv.FailedServers = append(inv.FailedServers, srv)
			inv.FailedCount++
			continue
		}

		inv.SuccessfulCount++
		fp := buildFingerprint(srv)
		key := fp.Key()

		if _, exists := groupMap[key]; !exists {
			groupMap[key] = &HardwareGroup{
				Fingerprint:    fp,
				TotalStorageTB: srv.TotalStorageTB,
			}
			groupOrder = append(groupOrder, key)
		}
		groupMap[key].Servers = append(groupMap[key].Servers, srv)
		groupMap[key].Count++
	}

	for _, key := range groupOrder {
		inv.Groups = append(inv.Groups, *groupMap[key])
	}

	// Sort groups by count descending so the largest groups appear first.
	sort.Slice(inv.Groups, func(i, j int) bool {
		return inv.Groups[i].Count > inv.Groups[j].Count
	})

	return inv
}

// buildFingerprint derives a HardwareFingerprint from a successfully scanned server.
func buildFingerprint(s ServerInfo) HardwareFingerprint {
	fp := HardwareFingerprint{
		Manufacturer:   s.Manufacturer,
		Model:          s.Model,
		CPUCount:       s.CPUCount,
		CPUModel:       s.CPUModel,
		RAMTotalGiB:    int(s.TotalMemoryGiB + 0.5), // round to nearest GiB
		RAMSlotsTotal:  s.MemorySlotsTotal,
		StorageSummary: NormalizeStorageSummary(s.Drives),
		GPUCount:       s.GPUCount,
	}

	// Pull per-socket CPU details from the first populated CPU socket.
	for _, cpu := range s.CPUs {
		if cpu.Cores > 0 {
			fp.CPUCoresPerSocket = cpu.Cores
			fp.CPUSpeedMHz = cpu.MaxSpeedMHz
			if fp.CPUModel == "" {
				fp.CPUModel = cpu.Model
			}
			break
		}
	}

	// Pull memory type/speed from the first populated DIMM.
	for _, mem := range s.Memory {
		if mem.IsPopulated() {
			fp.RAMType = mem.Type
			fp.RAMSpeedMHz = mem.SpeedMHz
			break
		}
	}

	// Pull GPU model and VRAM from the first GPU (assumes homogeneous GPU config).
	if len(s.GPUs) > 0 {
		fp.GPUModel = s.GPUs[0].Model
		fp.GPUMemoryGiB = int(s.GPUs[0].MemoryGB() + 0.5) // round to nearest GiB
	}

	return fp
}

// NormalizeStorageSummary builds a canonical, sorted storage summary string.
// Drives are grouped by rounded capacity and media type.
// Example output: "2×745GB SSD, 4×14306GB HDD"
func NormalizeStorageSummary(drives []DriveInfo) string {
	if len(drives) == 0 {
		return "no drives"
	}

	type driveKey struct {
		capacityGB int
		mediaType  string
	}

	counts := make(map[driveKey]int)
	var keys []driveKey

	for _, d := range drives {
		k := driveKey{
			capacityGB: int(d.CapacityGB + 0.5),
			mediaType:  d.MediaType,
		}
		if _, exists := counts[k]; !exists {
			keys = append(keys, k)
		}
		counts[k]++
	}

	// Sort: SSDs first, then by capacity descending.
	sort.Slice(keys, func(i, j int) bool {
		ki, kj := keys[i], keys[j]
		if ki.mediaType != kj.mediaType {
			if ki.mediaType == "SSD" {
				return true
			}
			if kj.mediaType == "SSD" {
				return false
			}
			return ki.mediaType < kj.mediaType
		}
		return ki.capacityGB > kj.capacityGB
	})

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d×%dGB %s", counts[k], k.capacityGB, k.mediaType))
	}

	return strings.Join(parts, ", ")
}
