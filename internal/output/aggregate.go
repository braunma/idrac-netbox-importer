package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"idrac-inventory/internal/models"
)

// AggregatedConsoleFormatter prints an aggregated hardware inventory to the terminal.
// Servers are first grouped by model, then by hardware configuration within each model.
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
	dotted := strings.Repeat("·", width)

	// Header
	fmt.Fprintf(w, "\n%s\n", line)
	fmt.Fprintf(w, "  HARDWARE INVENTORY REPORT\n")
	fmt.Fprintf(w, "  Generated: %s\n", inv.GeneratedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(w, "%s\n", line)
	fmt.Fprintf(w, "  Total: %d servers  |  Success: %d  |  Failed: %d  |  Models: %d  |  Config groups: %d\n",
		inv.TotalServers, inv.SuccessfulCount, inv.FailedCount,
		len(inv.ModelGroups), inv.TotalConfigGroups())
	if inv.Stats.TotalDuration > 0 {
		fmt.Fprintf(w, "  Scan time: %s total  |  avg %s/server\n",
			inv.Stats.TotalDuration.Round(time.Millisecond),
			inv.Stats.AverageDuration.Round(time.Millisecond))
	}
	fmt.Fprintf(w, "\n")

	// Model groups
	for i, mg := range inv.ModelGroups {
		fmt.Fprintf(w, "%s\n", thin)
		fmt.Fprintf(w, "  MODEL %d — %s%d× %s%s\n",
			i+1,
			f.bold(), mg.TotalCount, mg.DisplayModel(), f.reset())
		fmt.Fprintf(w, "%s\n", thin)

		for j, cg := range mg.ConfigGroups {
			fp := cg.Fingerprint

			// If there is more than one config subgroup, label each one.
			if len(mg.ConfigGroups) > 1 {
				fmt.Fprintf(w, "\n  %sConfiguration %d/%d%s  (%d server",
					f.bold(), j+1, len(mg.ConfigGroups), f.reset(), cg.Count)
				if cg.Count != 1 {
					fmt.Fprintf(w, "s")
				}
				fmt.Fprintf(w, ")\n")
				fmt.Fprintf(w, "  %s\n", dotted[:60])
			} else {
				fmt.Fprintf(w, "\n")
			}

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
			if fp.RAMModuleSizeGiB > 0 {
				moduleCount := 0
				if len(cg.Servers) > 0 {
					moduleCount = cg.Servers[0].MemorySlotsUsed
				}
				if moduleCount > 0 {
					ramSpec += fmt.Sprintf("  (%d× %d GiB modules)", moduleCount, fp.RAMModuleSizeGiB)
				} else {
					ramSpec += fmt.Sprintf("  (%d GiB/module)", fp.RAMModuleSizeGiB)
				}
			}
			fmt.Fprintf(w, "  %-15s %s\n", "RAM:", ramSpec)

			if fp.RAMSlotsTotal > 0 && len(cg.Servers) > 0 {
				s := cg.Servers[0]
				fmt.Fprintf(w, "  %-15s %d total  /  %d used  /  %d free\n",
					"RAM Slots:", fp.RAMSlotsTotal, s.MemorySlotsUsed, s.MemorySlotsFree)
			}

			// GPU line
			if fp.GPUCount > 0 {
				gpuSpec := fmt.Sprintf("%d×", fp.GPUCount)
				if fp.GPUModel != "" {
					gpuSpec += " " + fp.GPUModel
				}
				if fp.GPUMemoryGiB > 0 {
					gpuSpec += fmt.Sprintf(" (%d GB VRAM each)", fp.GPUMemoryGiB)
				}
				fmt.Fprintf(w, "  %-15s %s\n", "GPUs:", gpuSpec)
			}

			// Storage
			storageSpec := fp.StorageSummary
			if cg.TotalStorageTB > 0 {
				storageSpec += fmt.Sprintf("  (%.2f TB total)", cg.TotalStorageTB)
			}
			fmt.Fprintf(w, "  %-15s %s\n", "Storage:", storageSpec)

			// Server list
			fmt.Fprintf(w, "\n  Servers (%d):\n", cg.Count)
			fmt.Fprintf(w, "    %-18s %-22s %-14s %s\n", "IP Address", "Hostname", "Service Tag", "Power")
			fmt.Fprintf(w, "    %s\n", strings.Repeat("-", 64))
			for _, srv := range cg.Servers {
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
