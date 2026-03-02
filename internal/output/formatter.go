// Package output provides formatters for displaying scan results in various formats.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"idrac-inventory/internal/models"
)

// Formatter defines the interface for output formatters.
type Formatter interface {
	Format(w io.Writer, results []models.ServerInfo, stats models.CollectionStats) error
}

// ConsoleFormatter outputs results in a human-readable console format.
type ConsoleFormatter struct {
	Verbose bool
	NoColor bool
}

// JSONFormatter outputs results as JSON.
type JSONFormatter struct {
	Indent bool
}

// TableFormatter outputs results in a tabular format.
type TableFormatter struct{}

// NewConsoleFormatter creates a new console formatter.
func NewConsoleFormatter(verbose, noColor bool) *ConsoleFormatter {
	return &ConsoleFormatter{
		Verbose: verbose,
		NoColor: noColor,
	}
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter(indent bool) *JSONFormatter {
	return &JSONFormatter{Indent: indent}
}

// NewTableFormatter creates a new table formatter.
func NewTableFormatter() *TableFormatter {
	return &TableFormatter{}
}

// Format outputs results in console format.
func (f *ConsoleFormatter) Format(w io.Writer, results []models.ServerInfo, stats models.CollectionStats) error {
	for _, info := range results {
		f.formatServer(w, info)
	}

	f.formatSummary(w, stats)
	return nil
}

func (f *ConsoleFormatter) formatServer(w io.Writer, info models.ServerInfo) {
	if info.Error != nil {
		fmt.Fprintf(w, "\n%s %s - Error: %v\n", f.icon("❌"), info.Host, info.Error)
		return
	}

	// Header
	fmt.Fprintf(w, "\n%s\n", strings.Repeat("═", 72))
	fmt.Fprintf(w, "%s  %s (%s)\n", f.icon("🖥️"), info.Host, info.Model)
	fmt.Fprintf(w, "%s\n", strings.Repeat("═", 72))

	// System Information
	fmt.Fprintf(w, "\n%s System Information:\n", f.icon("📋"))
	fmt.Fprintf(w, "   %-14s %s %s\n", "Model:", info.Manufacturer, info.Model)
	fmt.Fprintf(w, "   %-14s %s\n", "Service Tag:", f.valueOrNA(info.ServiceTag))
	fmt.Fprintf(w, "   %-14s %s\n", "Serial:", f.valueOrNA(info.SerialNumber))
	fmt.Fprintf(w, "   %-14s %s\n", "BIOS:", f.valueOrNA(info.BiosVersion))
	fmt.Fprintf(w, "   %-14s %s\n", "Hostname:", f.valueOrNA(info.HostName))
	fmt.Fprintf(w, "   %-14s %s\n", "Power State:", f.formatPowerState(info.PowerState))

	// CPUs
	fmt.Fprintf(w, "\n%s CPUs: %d installed\n", f.icon("🔲"), info.CPUCount)
	if f.Verbose {
		for _, cpu := range info.CPUs {
			fmt.Fprintf(w, "   └─ %s\n", cpu.Socket)
			fmt.Fprintf(w, "      %s\n", cpu.Model)
			fmt.Fprintf(w, "      %d Cores / %d Threads @ %d MHz\n",
				cpu.Cores, cpu.Threads, cpu.MaxSpeedMHz)
			fmt.Fprintf(w, "      Health: %s\n", f.formatHealth(cpu.Health))
		}
	} else if len(info.CPUs) > 0 {
		cpu := info.CPUs[0]
		fmt.Fprintf(w, "   └─ %s (%d Cores / %d Threads)\n",
			cpu.Model, cpu.Cores, cpu.Threads)
	}

	// Memory
	memoryLine := fmt.Sprintf("%.0f GiB total", info.TotalMemoryGiB)
	for _, mem := range info.Memory {
		if mem.IsPopulated() {
			moduleGiB := int(mem.CapacityGB() + 0.5)
			if mem.Type != "" {
				memoryLine += fmt.Sprintf("  (%d× %d GiB %s)", info.MemorySlotsUsed, moduleGiB, mem.Type)
			} else {
				memoryLine += fmt.Sprintf("  (%d× %d GiB)", info.MemorySlotsUsed, moduleGiB)
			}
			break
		}
	}
	fmt.Fprintf(w, "\n%s Memory: %s\n", f.icon("💾"), memoryLine)
	fmt.Fprintf(w, "   └─ Slots: %d/%d used (%d free)\n",
		info.MemorySlotsUsed, info.MemorySlotsTotal, info.MemorySlotsFree)

	if f.Verbose {
		for _, mem := range info.Memory {
			if mem.IsPopulated() {
				fmt.Fprintf(w, "   └─ %s: %.0f GiB %s @ %d MHz\n",
					mem.Slot, mem.CapacityGB(), mem.Type, mem.SpeedMHz)
				fmt.Fprintf(w, "      %s %s (S/N: %s)\n",
					mem.Manufacturer, mem.PartNumber, mem.SerialNumber)
			} else if f.Verbose {
				fmt.Fprintf(w, "   └─ %s: [empty]\n", mem.Slot)
			}
		}
	}

	// Storage
	fmt.Fprintf(w, "\n%s Storage: %d drive(s), %.2f TB total\n",
		f.icon("💿"), info.DriveCount, info.TotalStorageTB)

	if f.Verbose {
		for _, drive := range info.Drives {
			lifeInfo := ""
			if drive.LifeLeftPct > 0 {
				lifeInfo = fmt.Sprintf(" [%.0f%% life]", drive.LifeLeftPct)
			}
			fmt.Fprintf(w, "   └─ %s: %.0f GB %s (%s)\n",
				drive.Name, drive.CapacityGB, drive.MediaType, drive.Protocol)
			fmt.Fprintf(w, "      %s (S/N: %s) %s %s\n",
				drive.Model, drive.SerialNumber, f.formatHealth(drive.Health), lifeInfo)
		}
	} else {
		// Group by media type
		ssdCount, hddCount := 0, 0
		var ssdCapacity, hddCapacity float64
		for _, d := range info.Drives {
			if d.MediaType == "SSD" {
				ssdCount++
				ssdCapacity += d.CapacityGB
			} else {
				hddCount++
				hddCapacity += d.CapacityGB
			}
		}
		if ssdCount > 0 {
			fmt.Fprintf(w, "   └─ %d× SSD (%.0f GB total)\n", ssdCount, ssdCapacity)
		}
		if hddCount > 0 {
			fmt.Fprintf(w, "   └─ %d× HDD (%.0f GB total)\n", hddCount, hddCapacity)
		}
	}

	// GPUs / Accelerators ("Beschleuniger" in German iDRAC)
	if info.GPUCount > 0 {
		fmt.Fprintf(w, "\n%s GPUs/Accelerators: %d installed\n", f.icon("🎮"), info.GPUCount)
		if f.Verbose {
			for _, gpu := range info.GPUs {
				fmt.Fprintf(w, "   └─ %s\n", gpu.Slot)
				fmt.Fprintf(w, "      %s %s\n", gpu.Manufacturer, gpu.Model)
				if gpu.MemoryMiB > 0 {
					memType := gpu.MemoryType
					if memType == "" {
						memType = "VRAM"
					}
					fmt.Fprintf(w, "      %.0f GB %s\n", gpu.MemoryGB(), memType)
				}
				fmt.Fprintf(w, "      Health: %s\n", f.formatHealth(gpu.Health))
			}
		} else {
			gpu := info.GPUs[0]
			if gpu.MemoryMiB > 0 {
				fmt.Fprintf(w, "   └─ %s (%.0f GB VRAM each)\n", gpu.Model, gpu.MemoryGB())
			} else {
				fmt.Fprintf(w, "   └─ %s\n", gpu.Model)
			}
		}
	}

	// Power Consumption
	if info.PowerConsumedWatts > 0 || info.PowerPeakWatts > 0 {
		fmt.Fprintf(w, "\n%s Power Consumption:\n", f.icon("⚡"))
		if info.PowerConsumedWatts > 0 {
			fmt.Fprintf(w, "   └─ Current: %d W\n", info.PowerConsumedWatts)
		}
		if info.PowerPeakWatts > 0 {
			fmt.Fprintf(w, "   └─ Peak:    %d W\n", info.PowerPeakWatts)
		}
	}
}

func (f *ConsoleFormatter) formatSummary(w io.Writer, stats models.CollectionStats) {
	fmt.Fprintf(w, "\n%s\n", strings.Repeat("═", 72))
	fmt.Fprintf(w, "%s Summary\n", f.icon("📊"))
	fmt.Fprintf(w, "%s\n", strings.Repeat("═", 72))

	fmt.Fprintf(w, "   Total Servers:   %d\n", stats.TotalServers)
	fmt.Fprintf(w, "   %s Successful:    %d\n", f.icon("✅"), stats.SuccessfulCount)
	fmt.Fprintf(w, "   %s Failed:        %d\n", f.icon("❌"), stats.FailedCount)
	fmt.Fprintf(w, "   Success Rate:    %.1f%%\n", stats.SuccessRate())
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "   Total Duration:  %s\n", stats.TotalDuration.Round(time.Millisecond))
	fmt.Fprintf(w, "   Avg per Server:  %s\n", stats.AverageDuration.Round(time.Millisecond))
	fmt.Fprintf(w, "   Fastest:         %s\n", stats.FastestDuration.Round(time.Millisecond))
	fmt.Fprintf(w, "   Slowest:         %s\n", stats.SlowestDuration.Round(time.Millisecond))
}

func (f *ConsoleFormatter) icon(emoji string) string {
	if f.NoColor {
		return ""
	}
	return emoji
}

func (f *ConsoleFormatter) valueOrNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

// statusMapping defines icon mappings for different status values
type statusMapping map[string]string

var (
	powerStateIcons = statusMapping{
		"On":  "🟢",
		"Off": "🔴",
	}

	healthIcons = statusMapping{
		"OK":       "✓",
		"Warning":  "⚠",
		"Critical": "✗",
	}
)

// formatWithIcon formats a status value with an icon if colors are enabled
func (f *ConsoleFormatter) formatWithIcon(value string, mapping statusMapping, defaultIcon string) string {
	if f.NoColor {
		return value
	}
	if icon, ok := mapping[value]; ok {
		return icon + " " + value
	}
	if defaultIcon != "" {
		return defaultIcon + " " + value
	}
	return value
}

func (f *ConsoleFormatter) formatPowerState(state string) string {
	return f.formatWithIcon(state, powerStateIcons, "🟡")
}

func (f *ConsoleFormatter) formatHealth(health string) string {
	return f.formatWithIcon(health, healthIcons, "")
}

// Format outputs results as JSON.
func (f *JSONFormatter) Format(w io.Writer, results []models.ServerInfo, stats models.CollectionStats) error {
	output := struct {
		Servers []models.ServerInfo    `json:"servers"`
		Stats   models.CollectionStats `json:"stats"`
	}{
		Servers: results,
		Stats:   stats,
	}

	encoder := json.NewEncoder(w)
	if f.Indent {
		encoder.SetIndent("", "  ")
	}

	return encoder.Encode(output)
}

// Format outputs results as a table.
func (f *TableFormatter) Format(w io.Writer, results []models.ServerInfo, stats models.CollectionStats) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintln(tw, "HOST\tMODEL\tSERVICE TAG\tCPUs\tRAM (GB)\tRAM SLOTS\tGPUs\tGPU MODEL\tDRIVES\tPOWER (W)\tSTATUS")
	fmt.Fprintln(tw, "----\t-----\t-----------\t----\t--------\t---------\t----\t---------\t------\t---------\t------")

	for _, info := range results {
		status := "OK"
		if info.Error != nil {
			status = "ERROR"
		}

		ramSlots := fmt.Sprintf("%d/%d", info.MemorySlotsUsed, info.MemorySlotsTotal)
		power := ""
		if info.PowerConsumedWatts > 0 {
			power = fmt.Sprintf("%d", info.PowerConsumedWatts)
		} else {
			power = "-"
		}

		gpuModel := "-"
		if len(info.GPUs) > 0 {
			gpuModel = info.GPUs[0].Model
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%.0f\t%s\t%d\t%s\t%d\t%s\t%s\n",
			info.Host,
			info.Model,
			info.ServiceTag,
			info.CPUCount,
			info.TotalMemoryGiB,
			ramSlots,
			info.GPUCount,
			gpuModel,
			info.DriveCount,
			power,
			status,
		)
	}

	tw.Flush()

	// Summary
	fmt.Fprintf(w, "\nTotal: %d servers (%d successful, %d failed) in %s\n",
		stats.TotalServers, stats.SuccessfulCount, stats.FailedCount,
		stats.TotalDuration.Round(time.Millisecond))

	return nil
}

// CSVFormatter outputs results as CSV.
type CSVFormatter struct{}

// NewCSVFormatter creates a new CSV formatter.
func NewCSVFormatter() *CSVFormatter {
	return &CSVFormatter{}
}

// Format outputs results as CSV.
func (f *CSVFormatter) Format(w io.Writer, results []models.ServerInfo, stats models.CollectionStats) error {
	// Header
	fmt.Fprintln(w, "host,model,manufacturer,service_tag,serial,bios_version,power_state,cpu_count,cpu_model,ram_total_gb,ram_slots_total,ram_slots_used,ram_slots_free,gpu_count,gpu_model,gpu_memory_gb,drive_count,storage_total_tb,power_consumed_watts,power_peak_watts,status,error")

	for _, info := range results {
		status := "OK"
		errorMsg := ""
		if info.Error != nil {
			status = "ERROR"
			errorMsg = info.Error.Error()
		}

		gpuModel := ""
		gpuMemoryGB := 0
		if len(info.GPUs) > 0 {
			gpuModel = info.GPUs[0].Model
			for _, g := range info.GPUs {
				gpuMemoryGB += int(g.MemoryGB())
			}
		}

		fmt.Fprintf(w, "%s,%s,%s,%s,%s,%s,%s,%d,%s,%.0f,%d,%d,%d,%d,%s,%d,%d,%.2f,%d,%d,%s,%s\n",
			csvEscape(info.Host),
			csvEscape(info.Model),
			csvEscape(info.Manufacturer),
			csvEscape(info.ServiceTag),
			csvEscape(info.SerialNumber),
			csvEscape(info.BiosVersion),
			csvEscape(info.PowerState),
			info.CPUCount,
			csvEscape(info.CPUModel),
			info.TotalMemoryGiB,
			info.MemorySlotsTotal,
			info.MemorySlotsUsed,
			info.MemorySlotsFree,
			info.GPUCount,
			csvEscape(gpuModel),
			gpuMemoryGB,
			info.DriveCount,
			info.TotalStorageTB,
			info.PowerConsumedWatts,
			info.PowerPeakWatts,
			status,
			csvEscape(errorMsg),
		)
	}

	return nil
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
