package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"idrac-inventory/internal/models"
)

// MarkdownFormatter generates a GitLab-flavoured Markdown inventory report.
// The output renders well in GitLab's web UI:
//   - Tables for hardware specs and server lists
//   - <details> collapsible sections per group for large deployments
//   - A summary table linking all groups at the top
type MarkdownFormatter struct{}

// NewMarkdownFormatter creates a new MarkdownFormatter.
func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{}
}

// FormatAggregated writes the aggregated inventory as Markdown to w.
func (f *MarkdownFormatter) FormatAggregated(w io.Writer, inv models.AggregatedInventory) error {
	// Title block
	fmt.Fprintf(w, "# Hardware Inventory Report\n\n")
	fmt.Fprintf(w, "> **Generated:** %s  \n", inv.GeneratedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(w, "> **Scanned:** %d servers &nbsp;|&nbsp; **Success:** %d &nbsp;|&nbsp; **Failed:** %d\n\n",
		inv.TotalServers, inv.SuccessfulCount, inv.FailedCount)

	fmt.Fprintf(w, "---\n\n")

	// Quick-reference summary table — one row per model group.
	fmt.Fprintf(w, "## Summary\n\n")
	fmt.Fprintf(w, "| # | Count | Model | Configs | CPUs | RAM | RAM Type | RAM Slots | Storage |\n")
	fmt.Fprintf(w, "|---|-------|-------|---------|------|-----|----------|-----------|--------|\n")
	for i, mg := range inv.ModelGroups {
		// Show CPU/RAM from the largest config subgroup (first after sort).
		cpuCol := "-"
		ramCol := "-"
		ramTypeCol := "-"
		ramSlotsCol := "-"
		storageCol := "-"
		if len(mg.ConfigGroups) > 0 {
			fp := mg.ConfigGroups[0].Fingerprint
			cpuCol = fmt.Sprintf("%d×", fp.CPUCount)
			if fp.CPUModel != "" {
				cpuCol += " " + shortenCPUModel(fp.CPUModel)
			}
			ramCol = fmt.Sprintf("%d GiB", fp.RAMTotalGiB)
			if fp.RAMType != "" {
				ramTypeCol = fp.RAMType
				if fp.RAMSpeedMHz > 0 {
					ramTypeCol += fmt.Sprintf(" @ %s MHz", formatWithCommas(fp.RAMSpeedMHz))
				}
			}
			if fp.RAMSlotsTotal > 0 && len(mg.ConfigGroups[0].Servers) > 0 {
				s := mg.ConfigGroups[0].Servers[0]
				if fp.RAMModuleSizeGiB > 0 {
					ramSlotsCol = fmt.Sprintf("%d/%d × %d GiB (%d free)",
						s.MemorySlotsUsed, fp.RAMSlotsTotal, fp.RAMModuleSizeGiB, s.MemorySlotsFree)
				} else {
					ramSlotsCol = fmt.Sprintf("%d/%d (%d free)", s.MemorySlotsUsed, fp.RAMSlotsTotal, s.MemorySlotsFree)
				}
			}
			storageCol = fp.StorageSummary
			if len(mg.ConfigGroups) > 1 {
				storageCol += " *(varies)*"
			}
		}
		fmt.Fprintf(w, "| [%d](#model-%d) | **%d** | %s | %d | %s | %s | %s | %s | %s |\n",
			i+1, i+1,
			mg.TotalCount, mg.Model,
			len(mg.ConfigGroups),
			cpuCol, ramCol, ramTypeCol, ramSlotsCol, storageCol,
		)
	}
	if len(inv.FailedServers) > 0 {
		fmt.Fprintf(w, "| — | **%d** | ❌ Failed | — | — | — | — | — | — |\n", inv.FailedCount)
	}

	fmt.Fprintf(w, "\n")

	// Scan timing stats (if available)
	if inv.Stats.TotalDuration > 0 {
		fmt.Fprintf(w, "### Scan Timing\n\n")
		fmt.Fprintf(w, "| Metric | Value |\n")
		fmt.Fprintf(w, "|--------|-------|\n")
		fmt.Fprintf(w, "| Total duration | `%s` |\n", inv.Stats.TotalDuration.Round(time.Millisecond))
		fmt.Fprintf(w, "| Average per server | `%s` |\n", inv.Stats.AverageDuration.Round(time.Millisecond))
		fmt.Fprintf(w, "| Fastest | `%s` |\n", inv.Stats.FastestDuration.Round(time.Millisecond))
		fmt.Fprintf(w, "| Slowest | `%s` |\n\n", inv.Stats.SlowestDuration.Round(time.Millisecond))
	}

	fmt.Fprintf(w, "---\n\n")

	// Per-model detail sections
	fmt.Fprintf(w, "## Hardware Groups\n\n")
	for i, mg := range inv.ModelGroups {
		f.writeModelGroup(w, i+1, mg)
	}

	// Failed servers section
	if len(inv.FailedServers) > 0 {
		f.writeFailedServers(w, inv.FailedServers)
	}

	return nil
}

func (f *MarkdownFormatter) writeModelGroup(w io.Writer, idx int, mg models.ModelGroup) {
	fmt.Fprintf(w, "<a id=\"model-%d\"></a>\n\n", idx)
	fmt.Fprintf(w, "### Model %d — %d× %s\n\n", idx, mg.TotalCount, mg.DisplayModel())

	if len(mg.ConfigGroups) == 1 {
		// Single config: render inline without a nested header.
		f.writeConfigGroup(w, 0, mg.ConfigGroups[0], false)
	} else {
		// Multiple configs: render each as a sub-section.
		fmt.Fprintf(w, "> **%d configuration variants** found for this model.\n\n", len(mg.ConfigGroups))
		for j, cg := range mg.ConfigGroups {
			f.writeConfigGroup(w, j+1, cg, true)
		}
	}

	fmt.Fprintf(w, "---\n\n")
}

// writeConfigGroup renders one hardware-config subgroup.
// If showSubheader is true a "#### Configuration N" heading is emitted first.
func (f *MarkdownFormatter) writeConfigGroup(w io.Writer, idx int, group models.HardwareGroup, showSubheader bool) {
	fp := group.Fingerprint

	if showSubheader {
		fmt.Fprintf(w, "#### Configuration %d — %d server", idx, group.Count)
		if group.Count != 1 {
			fmt.Fprintf(w, "s")
		}
		fmt.Fprintf(w, "\n\n")
	}

	// Hardware spec table
	fmt.Fprintf(w, "| Property | Value |\n")
	fmt.Fprintf(w, "|----------|-------|\n")

	// CPU rows
	if fp.CPUModel != "" {
		fmt.Fprintf(w, "| **CPUs** | %d× %s |\n", fp.CPUCount, mdEscape(fp.CPUModel))
	} else {
		fmt.Fprintf(w, "| **CPUs** | %d sockets |\n", fp.CPUCount)
	}
	if fp.CPUCoresPerSocket > 0 {
		totalCores := fp.CPUCoresPerSocket * fp.CPUCount
		fmt.Fprintf(w, "| **CPU Cores** | %d cores/CPU · %d total |\n", fp.CPUCoresPerSocket, totalCores)
	}
	if fp.CPUSpeedMHz > 0 {
		fmt.Fprintf(w, "| **CPU Speed** | %s MHz (%.2f GHz) |\n",
			formatWithCommas(fp.CPUSpeedMHz), float64(fp.CPUSpeedMHz)/1000)
	}

	// RAM rows
	fmt.Fprintf(w, "| **RAM** | %d GiB |\n", fp.RAMTotalGiB)
	if fp.RAMType != "" {
		ramTypeLine := fp.RAMType
		if fp.RAMSpeedMHz > 0 {
			ramTypeLine += fmt.Sprintf(" @ %s MHz", formatWithCommas(fp.RAMSpeedMHz))
		}
		fmt.Fprintf(w, "| **RAM Type** | %s |\n", ramTypeLine)
	}
	if fp.RAMSlotsTotal > 0 && len(group.Servers) > 0 {
		s := group.Servers[0]
		if fp.RAMModuleSizeGiB > 0 {
			fmt.Fprintf(w, "| **RAM Slots** | %d/%d × %d GiB (%d free) |\n",
				s.MemorySlotsUsed, fp.RAMSlotsTotal, fp.RAMModuleSizeGiB, s.MemorySlotsFree)
		} else {
			fmt.Fprintf(w, "| **RAM Slots** | %d/%d (%d free) |\n",
				s.MemorySlotsUsed, fp.RAMSlotsTotal, s.MemorySlotsFree)
		}
	}

	// GPU / Accelerator rows ("Beschleuniger" in German iDRAC)
	if fp.GPUCount > 0 {
		gpuLine := fmt.Sprintf("%d×", fp.GPUCount)
		if fp.GPUModel != "" {
			gpuLine += " " + fp.GPUModel
		}
		if fp.GPUMemoryGiB > 0 {
			gpuLine += fmt.Sprintf(" · %d GB VRAM each", fp.GPUMemoryGiB)
		}
		fmt.Fprintf(w, "| **GPUs/Accelerators** | %s |\n", gpuLine)
	}

	// Storage rows
	fmt.Fprintf(w, "| **Storage** | %s |\n", mdEscape(fp.StorageSummary))
	if group.TotalStorageTB > 0 {
		fmt.Fprintf(w, "| **Total Storage** | %.2f TB |\n", group.TotalStorageTB)
	}

	fmt.Fprintf(w, "\n")

	// Collapsible server list — GitLab renders <details> natively
	fmt.Fprintf(w, "<details>\n")
	fmt.Fprintf(w, "<summary>Servers in this group (%d) — click to expand</summary>\n\n", group.Count)

	fmt.Fprintf(w, "| # | IP Address | Hostname | Service Tag | Power | Scanned At |\n")
	fmt.Fprintf(w, "|---|-----------|---------|-------------|-------|------------|\n")
	for j, srv := range group.Servers {
		hostname := srv.HostName
		if hostname == "" {
			hostname = srv.Name
		}
		if hostname == "" {
			hostname = "-"
		}
		scannedAt := "-"
		if !srv.CollectedAt.IsZero() {
			scannedAt = srv.CollectedAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(w, "| %d | `%s` | %s | %s | %s | %s |\n",
			j+1,
			srv.Host,
			mdEscape(hostname),
			dashIfEmpty(srv.ServiceTag),
			dashIfEmpty(srv.PowerState),
			scannedAt,
		)
	}

	fmt.Fprintf(w, "\n</details>\n\n")
}

func (f *MarkdownFormatter) writeFailedServers(w io.Writer, failed []models.ServerInfo) {
	fmt.Fprintf(w, "## Failed Scans\n\n")
	fmt.Fprintf(w, "| IP Address | Error |\n")
	fmt.Fprintf(w, "|-----------|-------|\n")
	for _, srv := range failed {
		errMsg := "unknown error"
		if srv.Error != nil {
			errMsg = srv.Error.Error()
		}
		// Pipe characters inside table cells must be escaped
		fmt.Fprintf(w, "| `%s` | %s |\n", srv.Host, strings.ReplaceAll(errMsg, "|", "\\|"))
	}
	fmt.Fprintf(w, "\n")
}

// shortenCPUModel trims verbose Intel/AMD model strings to a compact version.
// E.g. "Intel(R) Xeon(R) Gold 6140 CPU @ 2.30GHz" → "Xeon Gold 6140"
func shortenCPUModel(model string) string {
	// Remove (R) / (TM) noise
	model = strings.ReplaceAll(model, "(R)", "")
	model = strings.ReplaceAll(model, "(TM)", "")
	// Strip everything from " CPU @" onwards
	if idx := strings.Index(model, " CPU"); idx >= 0 {
		model = model[:idx]
	}
	// Remove double spaces and trim
	for strings.Contains(model, "  ") {
		model = strings.ReplaceAll(model, "  ", " ")
	}
	return strings.TrimSpace(model)
}

// formatWithCommas formats an integer with thousands separators.
func formatWithCommas(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// mdEscape escapes characters that would break Markdown table cells.
func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
