package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"idrac-inventory/internal/models"
)

// AggregatedConsoleFormatter prints an aggregated hardware inventory to the terminal.
// Servers with identical hardware configurations are shown as a single group
// with a count, making it easy to spot e.g. "12× Dell R440 with the same spec".
type AggregatedConsoleFormatter struct {
	NoColor bool
}

// NewAggregatedConsoleFormatter creates a new AggregatedConsoleFormatter.
func NewAggregatedConsoleFormatter(noColor bool) *AggregatedConsoleFormatter {
	return &AggregatedConsoleFormatter{NoColor: noColor}
}

// FormatAggregated writes the aggregated inventory to w.
func (f *AggregatedConsoleFormatter) FormatAggregated(w io.Writer, inv models.AggregatedInventory) error {
	const width = 80
	line := strings.Repeat("═", width)
	thin := strings.Repeat("─", width)

	// Header
	fmt.Fprintf(w, "\n%s\n", line)
	fmt.Fprintf(w, "  HARDWARE INVENTORY REPORT\n")
	fmt.Fprintf(w, "  Generated: %s\n", inv.GeneratedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(w, "%s\n", line)
	fmt.Fprintf(w, "  Total: %d servers  |  Success: %d  |  Failed: %d  |  Config groups: %d\n",
		inv.TotalServers, inv.SuccessfulCount, inv.FailedCount, len(inv.Groups))
	if inv.Stats.TotalDuration > 0 {
		fmt.Fprintf(w, "  Scan time: %s total  |  avg %s/server\n",
			inv.Stats.TotalDuration.Round(time.Millisecond),
			inv.Stats.AverageDuration.Round(time.Millisecond))
	}
	fmt.Fprintf(w, "\n")

	// Groups
	for i, group := range inv.Groups {
		fp := group.Fingerprint
		fmt.Fprintf(w, "%s\n", thin)
		fmt.Fprintf(w, "  GROUP %d — %s%d× %s%s\n",
			i+1,
			f.bold(), group.Count, fp.DisplayModel(), f.reset())
		fmt.Fprintf(w, "%s\n", thin)

		// CPU line
		cpuSpec := ""
		if fp.CPUModel != "" {
			cpuSpec = fmt.Sprintf("%d× %s", fp.CPUCount, fp.CPUModel)
		} else {
			cpuSpec = fmt.Sprintf("%d sockets", fp.CPUCount)
		}
		fmt.Fprintf(w, "  %-15s %s\n", "CPUs:", cpuSpec)

		if fp.CPUCoresPerSocket > 0 {
			totalCores := fp.CPUCoresPerSocket * fp.CPUCount
			speedStr := ""
			if fp.CPUSpeedMHz > 0 {
				speedStr = fmt.Sprintf("  @  %.2f GHz", float64(fp.CPUSpeedMHz)/1000)
			}
			fmt.Fprintf(w, "  %-15s %d cores/CPU (%d total)%s\n",
				"CPU Cores:", fp.CPUCoresPerSocket, totalCores, speedStr)
		}

		// RAM line
		ramSpec := fmt.Sprintf("%d GiB", fp.RAMTotalGiB)
		if fp.RAMType != "" {
			ramSpec += "  " + fp.RAMType
			if fp.RAMSpeedMHz > 0 {
				ramSpec += fmt.Sprintf(" @ %d MHz", fp.RAMSpeedMHz)
			}
		}
		fmt.Fprintf(w, "  %-15s %s\n", "RAM:", ramSpec)

		if fp.RAMSlotsTotal > 0 && len(group.Servers) > 0 {
			s := group.Servers[0]
			fmt.Fprintf(w, "  %-15s %d total  /  %d used  /  %d free\n",
				"RAM Slots:", fp.RAMSlotsTotal, s.MemorySlotsUsed, s.MemorySlotsFree)
		}

		// Storage
		storageSpec := fp.StorageSummary
		if group.TotalStorageTB > 0 {
			storageSpec += fmt.Sprintf("  (%.2f TB total)", group.TotalStorageTB)
		}
		fmt.Fprintf(w, "  %-15s %s\n", "Storage:", storageSpec)

		// Server list
		fmt.Fprintf(w, "\n  Servers (%d):\n", group.Count)
		fmt.Fprintf(w, "    %-18s %-22s %-14s %s\n", "IP Address", "Hostname", "Service Tag", "Power")
		fmt.Fprintf(w, "    %s\n", strings.Repeat("-", 64))
		for _, srv := range group.Servers {
			hostname := srv.HostName
			if hostname == "" {
				hostname = srv.Name
			}
			if hostname == "" {
				hostname = "-"
			}
			serviceTag := srv.ServiceTag
			if serviceTag == "" {
				serviceTag = "-"
			}
			fmt.Fprintf(w, "    %-18s %-22s %-14s %s\n",
				srv.Host, hostname, serviceTag, srv.PowerState)
		}
		fmt.Fprintf(w, "\n")
	}

	// Failed servers
	if len(inv.FailedServers) > 0 {
		fmt.Fprintf(w, "%s\n", thin)
		fmt.Fprintf(w, "  FAILED SCANS (%d)\n", len(inv.FailedServers))
		fmt.Fprintf(w, "%s\n", thin)
		for _, srv := range inv.FailedServers {
			errMsg := "unknown error"
			if srv.Error != nil {
				errMsg = srv.Error.Error()
			}
			fmt.Fprintf(w, "  %-20s  %s\n", srv.Host, errMsg)
		}
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "%s\n\n", line)
	return nil
}

func (f *AggregatedConsoleFormatter) bold() string {
	if f.NoColor {
		return ""
	}
	return "\033[1m"
}

func (f *AggregatedConsoleFormatter) reset() string {
	if f.NoColor {
		return ""
	}
	return "\033[0m"
}
