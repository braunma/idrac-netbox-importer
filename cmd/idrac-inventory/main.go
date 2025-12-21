// Package main provides the CLI entry point for the iDRAC inventory tool.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"idrac-inventory/internal/config"
	"idrac-inventory/internal/models"
	"idrac-inventory/internal/netbox"
	"idrac-inventory/internal/output"
	"idrac-inventory/internal/scanner"
	"idrac-inventory/pkg/logging"
)

// Build information, set via ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// CLI flags
type flags struct {
	// Config
	configFile string

	// Single server mode
	host     string
	username string
	password string

	// Output options
	outputFormat string
	verbose      bool
	noColor      bool

	// Actions
	syncNetBox          bool
	validateConnections bool

	// Misc
	version  bool
	logLevel string
}

func main() {
	f := parseFlags()

	if f.version {
		printVersion()
		os.Exit(0)
	}

	// Initialize logging
	if err := logging.Init(logging.Config{
		Level:  f.logLevel,
		Format: "console",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	defer logging.Sync()

	// Load configuration
	cfg, err := loadConfiguration(f)
	if err != nil {
		logging.Fatal("Configuration error", "error", err)
	}

	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupSignalHandler(cancel)

	// Run the appropriate action
	if err := run(ctx, cfg, f); err != nil {
		logging.Error("Execution failed", "error", err)
		os.Exit(1)
	}
}

func parseFlags() *flags {
	f := &flags{}

	// Config
	flag.StringVar(&f.configFile, "config", "config.yaml", "Path to configuration file")

	// Single server mode
	flag.StringVar(&f.host, "host", "", "Single host to scan (overrides config file)")
	flag.StringVar(&f.username, "user", "", "Username for single host mode")
	flag.StringVar(&f.password, "pass", "", "Password for single host mode")

	// Output options
	flag.StringVar(&f.outputFormat, "output", "console", "Output format: console, json, table, csv")
	flag.BoolVar(&f.verbose, "verbose", false, "Show detailed output")
	flag.BoolVar(&f.noColor, "no-color", false, "Disable colored output")

	// Actions
	flag.BoolVar(&f.syncNetBox, "sync", false, "Sync results to NetBox")
	flag.BoolVar(&f.validateConnections, "validate", false, "Only validate connections, don't collect inventory")

	// Misc
	flag.BoolVar(&f.version, "version", false, "Show version information")
	flag.StringVar(&f.logLevel, "log-level", "info", "Log level: debug, info, warn, error")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "iDRAC Hardware Inventory Tool\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Scan servers from config file\n")
		fmt.Fprintf(os.Stderr, "  %s -config config.yaml\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Scan single server\n")
		fmt.Fprintf(os.Stderr, "  %s -host 192.168.1.10 -user root -pass secret\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Scan and sync to NetBox\n")
		fmt.Fprintf(os.Stderr, "  %s -config config.yaml -sync\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Output as JSON\n")
		fmt.Fprintf(os.Stderr, "  %s -config config.yaml -output json\n\n", os.Args[0])
	}

	flag.Parse()

	return f
}

func loadConfiguration(f *flags) (*config.Config, error) {
	// Single host mode takes precedence
	if f.host != "" {
		if f.username == "" || f.password == "" {
			return nil, fmt.Errorf("single host mode requires -user and -pass flags")
		}

		logging.Debug("Using single server mode",
			"host", f.host,
		)

		return config.NewSingleServerConfig(f.host, f.username, f.password), nil
	}

	// Load from config file
	logging.Debug("Loading configuration",
		"file", f.configFile,
	)

	cfg, err := config.Load(f.configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", f.configFile, err)
	}

	logging.Info("Configuration loaded",
		"servers", len(cfg.Servers),
		"concurrency", cfg.Concurrency,
		"netbox_enabled", cfg.NetBox.IsEnabled(),
	)

	return cfg, nil
}

func setupSignalHandler(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logging.Warn("Received signal, shutting down",
			"signal", sig,
		)
		cancel()
	}()
}

func run(ctx context.Context, cfg *config.Config, f *flags) error {
	s := scanner.New(cfg)

	// Validate connections mode
	if f.validateConnections {
		return runValidateConnections(ctx, s)
	}

	// Scan all servers
	logging.Info("Starting inventory scan",
		"server_count", len(cfg.Servers),
	)

	results, stats := s.ScanAll(ctx)

	// Output results
	if err := outputResults(f, results, stats); err != nil {
		return fmt.Errorf("failed to output results: %w", err)
	}

	// Sync to NetBox if requested
	if f.syncNetBox {
		if !cfg.NetBox.IsEnabled() {
			logging.Warn("NetBox sync requested but not configured")
		} else {
			return runNetBoxSync(ctx, cfg, results)
		}
	}

	// Return error if any servers failed
	if stats.FailedCount > 0 {
		return fmt.Errorf("%d of %d servers failed", stats.FailedCount, stats.TotalServers)
	}

	return nil
}

func runValidateConnections(ctx context.Context, s *scanner.Scanner) error {
	logging.Info("Validating connections to all servers")

	results := s.ValidateConnections(ctx)

	successCount := printValidationResults(results)

	fmt.Printf("\nValidation complete: %d/%d successful\n", successCount, len(results))

	failCount := len(results) - successCount
	if failCount > 0 {
		return fmt.Errorf("%d connections failed", failCount)
	}

	return nil
}

func outputResults(f *flags, results []models.ServerInfo, stats models.CollectionStats) error {
	var formatter output.Formatter

	switch f.outputFormat {
	case "json":
		formatter = output.NewJSONFormatter(true)
	case "table":
		formatter = output.NewTableFormatter()
	case "csv":
		formatter = output.NewCSVFormatter()
	case "console":
		fallthrough
	default:
		formatter = output.NewConsoleFormatter(f.verbose, f.noColor)
	}

	return formatter.Format(os.Stdout, results, stats)
}

func runNetBoxSync(ctx context.Context, cfg *config.Config, results []models.ServerInfo) error {
	logging.Info("Syncing results to NetBox",
		"url", cfg.NetBox.URL,
	)

	client := netbox.NewClient(cfg.NetBox)

	// Test connection first
	if err := client.TestConnection(ctx); err != nil {
		return fmt.Errorf("NetBox connection failed: %w", err)
	}

	syncResults := client.SyncAll(ctx, results)

	// Print sync results and count failures
	fmt.Println("\nNetBox Sync Results:")
	failCount := printSyncResults(syncResults)

	if failCount > 0 {
		return fmt.Errorf("%d of %d servers failed to sync", failCount, len(syncResults))
	}

	return nil
}

func printVersion() {
	fmt.Printf("iDRAC Inventory Tool\n")
	fmt.Printf("  Version:    %s\n", Version)
	fmt.Printf("  Build Time: %s\n", BuildTime)
	fmt.Printf("  Git Commit: %s\n", GitCommit)
}

// printValidationResults prints validation results and returns the success count.
func printValidationResults(results map[string]error) int {
	successCount := 0
	for host, err := range results {
		if err != nil {
			fmt.Printf("❌ %s: %v\n", host, err)
		} else {
			fmt.Printf("✅ %s: OK\n", host)
			successCount++
		}
	}
	return successCount
}

// printSyncResults prints NetBox sync results and returns the failure count.
func printSyncResults(results []netbox.SyncResult) int {
	failCount := 0
	for _, r := range results {
		if r.Success {
			fmt.Printf("  ✅ %s: synced\n", r.Host)
		} else {
			fmt.Printf("  ❌ %s: %v\n", r.Host, r.Error)
			failCount++
		}
	}
	return failCount
}
