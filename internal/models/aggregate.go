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
	RAMModuleSizeGiB  int    `json:"ram_module_size_gib"` // size of a single DIMM in GiB
	RAMType           string `json:"ram_type"`
	RAMSpeedMHz       int    `json:"ram_speed_mhz"`
	RAMSlotsTotal     int    `json:"ram_slots_total"`
	StorageSummary    string `json:"storage_summary"` // e.g. "2×745GB SSD, 4×14306GB HDD"
	// GPU / Accelerator ("Beschleuniger" in German iDRAC)
	GPUCount     int    `json:"gpu_count"`
	GPUModel     string `json:"gpu_model"`     // model of the first GPU (all assumed identical)
	GPUMemoryGiB int    `json:"gpu_memory_gib"` // VRAM per GPU in GiB
}

// Key returns a stable string key for hardware config (excludes manufacturer/model —
// those are the model-group key). Used as the config-subgroup discriminator.
func (f HardwareFingerprint) Key() string {
	return fmt.Sprintf("%d|%s|%d|%d|%d|%d|%s|%d|%d|%s|%d|%s|%d",
		f.CPUCount, f.CPUModel, f.CPUCoresPerSocket, f.CPUSpeedMHz,
		f.RAMTotalGiB, f.RAMModuleSizeGiB, f.RAMType, f.RAMSpeedMHz, f.RAMSlotsTotal,
		f.StorageSummary,
		f.GPUCount, f.GPUModel, f.GPUMemoryGiB,
	)
}

// HardwareGroup represents a set of servers that share identical hardware configuration
// within a model group.
type HardwareGroup struct {
	Fingerprint    HardwareFingerprint `json:"fingerprint"`
	Count          int                 `json:"count"`
	Servers        []ServerInfo        `json:"servers"`
	TotalStorageTB float64             `json:"total_storage_tb,omitempty"` // from first server
}

// ModelGroup represents all servers of a specific hardware model (e.g. "Dell PowerEdge R440").
// Servers with different hardware configurations within the same model appear as separate
// ConfigGroups, making it easy to spot e.g. "50× R440: 45 with config A, 5 with config B".
type ModelGroup struct {
	Manufacturer string         `json:"manufacturer"`
	Model        string         `json:"model"`
	TotalCount   int            `json:"total_count"`
	ConfigGroups []HardwareGroup `json:"config_groups"`
}

// DisplayModel returns a human-friendly model string including manufacturer.
func (g ModelGroup) DisplayModel() string {
	if g.Manufacturer != "" && !strings.HasPrefix(strings.ToLower(g.Model), strings.ToLower(g.Manufacturer)) {
		return g.Manufacturer + " " + g.Model
	}
	return g.Model
}

// AggregatedInventory is the top-level structure for the aggregated hardware report.
type AggregatedInventory struct {
	GeneratedAt     time.Time     `json:"generated_at"`
	TotalServers    int           `json:"total_servers"`
	SuccessfulCount int           `json:"successful_count"`
	FailedCount     int           `json:"failed_count"`
	ModelGroups     []ModelGroup  `json:"model_groups"`
	FailedServers   []ServerInfo  `json:"failed_servers,omitempty"`
	Stats           CollectionStats `json:"stats"`
}

// TotalConfigGroups returns the total number of distinct hardware-config sub-groups
// across all model groups.
func (inv AggregatedInventory) TotalConfigGroups() int {
	total := 0
	for _, mg := range inv.ModelGroups {
		total += len(mg.ConfigGroups)
	}
	return total
}

// GroupByConfiguration groups servers using a two-level hierarchy:
//  1. Model group — all servers of the same Manufacturer+Model
//  2. Config subgroup — servers within a model that share the same hardware config
//
// Failed servers are collected separately.
// Model groups are sorted by total count (descending); config subgroups within each model
// are also sorted by count (descending).
func GroupByConfiguration(servers []ServerInfo, stats CollectionStats) AggregatedInventory {
	inv := AggregatedInventory{
		GeneratedAt:  time.Now().UTC(),
		TotalServers: len(servers),
		Stats:        stats,
	}

	type modelKey struct {
		manufacturer string
		model        string
	}

	modelMap := make(map[modelKey]*ModelGroup)
	// configIdxMap maps "manufacturer|model\x00fpKey" → index in ModelGroup.ConfigGroups.
	configIdxMap := make(map[string]int)
	var modelOrder []modelKey

	for _, srv := range servers {
		if srv.Error != nil {
			inv.FailedServers = append(inv.FailedServers, srv)
			inv.FailedCount++
			continue
		}

		inv.SuccessfulCount++

		mk := modelKey{manufacturer: srv.Manufacturer, model: srv.Model}
		if _, exists := modelMap[mk]; !exists {
			modelMap[mk] = &ModelGroup{
				Manufacturer: srv.Manufacturer,
				Model:        srv.Model,
			}
			modelOrder = append(modelOrder, mk)
		}
		mg := modelMap[mk]
		mg.TotalCount++

		fp := buildFingerprint(srv)
		combKey := fmt.Sprintf("%s|%s\x00%s", mk.manufacturer, mk.model, fp.Key())

		if idx, exists := configIdxMap[combKey]; exists {
			mg.ConfigGroups[idx].Servers = append(mg.ConfigGroups[idx].Servers, srv)
			mg.ConfigGroups[idx].Count++
		} else {
			configIdxMap[combKey] = len(mg.ConfigGroups)
			mg.ConfigGroups = append(mg.ConfigGroups, HardwareGroup{
				Fingerprint:    fp,
				Count:          1,
				Servers:        []ServerInfo{srv},
				TotalStorageTB: srv.TotalStorageTB,
			})
		}
	}

	for _, mk := range modelOrder {
		inv.ModelGroups = append(inv.ModelGroups, *modelMap[mk])
	}

	// Sort model groups by total count descending.
	sort.Slice(inv.ModelGroups, func(i, j int) bool {
		return inv.ModelGroups[i].TotalCount > inv.ModelGroups[j].TotalCount
	})

	// Sort config subgroups within each model by count descending.
	for i := range inv.ModelGroups {
		sort.Slice(inv.ModelGroups[i].ConfigGroups, func(a, b int) bool {
			return inv.ModelGroups[i].ConfigGroups[a].Count > inv.ModelGroups[i].ConfigGroups[b].Count
		})
	}

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

	// Pull memory type/speed/module-size from the first populated DIMM.
	for _, mem := range s.Memory {
		if mem.IsPopulated() {
			fp.RAMType = mem.Type
			fp.RAMSpeedMHz = mem.SpeedMHz
			fp.RAMModuleSizeGiB = (mem.CapacityMiB + 512) / 1024 // round to nearest GiB
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
