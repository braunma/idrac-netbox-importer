// Package gitlab provides functionality for exporting hardware inventory data
// to a local git repository that is connected to a GitLab instance.
//
// Workflow:
//  1. Write hardware-inventory.md  (human-readable, renders in GitLab)
//  2. Write hardware-inventory.json (machine-readable, full detail)
//  3. git add <files>
//  4. git commit -m "inventory: update hardware report <timestamp>"
//  5. (optional) git push origin <branch>
package gitlab

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"idrac-inventory/internal/models"
	"idrac-inventory/internal/output"
	"idrac-inventory/pkg/logging"
)

// Config holds configuration for the GitLab exporter.
type Config struct {
	// RepoPath is the absolute path to the local git repository.
	// The directory must already contain a .git folder.
	RepoPath string

	// Branch is the git branch to commit to (default: "main").
	Branch string

	// InventoryDir is the directory within the repository where the inventory
	// files will be written (default: "inventory").
	InventoryDir string

	// AuthorName is the git commit author name (default: "iDRAC Inventory Bot").
	AuthorName string

	// AuthorEmail is the git commit author email (default: "idrac-inventory@localhost").
	AuthorEmail string

	// Push controls whether to push to the remote after committing.
	Push bool
}

// Exporter writes inventory reports into a local git repository and optionally
// pushes the resulting commit to the configured remote.
type Exporter struct {
	cfg Config
}

// New creates a new Exporter, applying sensible defaults to any unset fields.
func New(cfg Config) *Exporter {
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.InventoryDir == "" {
		cfg.InventoryDir = "inventory"
	}
	if cfg.AuthorName == "" {
		cfg.AuthorName = "iDRAC Inventory Bot"
	}
	if cfg.AuthorEmail == "" {
		cfg.AuthorEmail = "idrac-inventory@localhost"
	}
	return &Exporter{cfg: cfg}
}

// Export writes the inventory files, commits them, and optionally pushes.
func (e *Exporter) Export(inv models.AggregatedInventory) error {
	if e.cfg.RepoPath == "" {
		return fmt.Errorf("gitlab.repo_path is not configured")
	}

	// Verify the target is an actual git repository.
	if _, err := os.Stat(filepath.Join(e.cfg.RepoPath, ".git")); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s (missing .git directory)", e.cfg.RepoPath)
	}

	// Ensure the inventory sub-directory exists.
	inventoryDir := filepath.Join(e.cfg.RepoPath, e.cfg.InventoryDir)
	if err := os.MkdirAll(inventoryDir, 0o755); err != nil {
		return fmt.Errorf("failed to create inventory directory %s: %w", inventoryDir, err)
	}

	// Write Markdown report.
	mdFile := "hardware-inventory.md"
	mdPath := filepath.Join(inventoryDir, mdFile)
	if err := e.writeMarkdown(mdPath, inv); err != nil {
		return fmt.Errorf("failed to write Markdown report: %w", err)
	}
	logging.Info("Wrote Markdown report", "path", mdPath)

	// Write JSON report.
	jsonFile := "hardware-inventory.json"
	jsonPath := filepath.Join(inventoryDir, jsonFile)
	if err := e.writeJSON(jsonPath, inv); err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}
	logging.Info("Wrote JSON report", "path", jsonPath)

	// Stage both files.
	relMD := filepath.Join(e.cfg.InventoryDir, mdFile)
	relJSON := filepath.Join(e.cfg.InventoryDir, jsonFile)
	if err := e.gitRun("add", relMD, relJSON); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// Commit.
	msg := fmt.Sprintf(
		"inventory: update hardware report %s\n\nScanned: %d | Success: %d | Failed: %d | Groups: %d",
		inv.GeneratedAt.Format("2006-01-02 15:04:05 UTC"),
		inv.TotalServers, inv.SuccessfulCount, inv.FailedCount, len(inv.Groups),
	)
	if err := e.gitCommit(msg); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}
	logging.Info("Committed inventory",
		"repo", e.cfg.RepoPath,
		"branch", e.cfg.Branch,
		"servers", inv.TotalServers,
		"groups", len(inv.Groups),
	)

	// Optionally push.
	if e.cfg.Push {
		if err := e.gitRun("push", "origin", e.cfg.Branch); err != nil {
			return fmt.Errorf("git push failed: %w", err)
		}
		logging.Info("Pushed inventory to remote", "branch", e.cfg.Branch)
	}

	return nil
}

// writeMarkdown renders the aggregated inventory as Markdown and writes it to path.
func (e *Exporter) writeMarkdown(path string, inv models.AggregatedInventory) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return output.NewMarkdownFormatter().FormatAggregated(f, inv)
}

// writeJSON serialises the aggregated inventory as indented JSON and writes it to path.
func (e *Exporter) writeJSON(path string, inv models.AggregatedInventory) error {
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// gitCommit runs git commit, setting the configured author identity via -c flags.
func (e *Exporter) gitCommit(message string) error {
	// Build args manually to keep gitRun simple.
	args := []string{
		"-C", e.cfg.RepoPath,
		"-c", "user.name=" + e.cfg.AuthorName,
		"-c", "user.email=" + e.cfg.AuthorEmail,
		"commit", "-m", message,
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	logging.Debug("git commit", "output", strings.TrimSpace(string(out)))
	return nil
}

// gitRun executes a git sub-command inside RepoPath.
func (e *Exporter) gitRun(subArgs ...string) error {
	args := append([]string{"-C", e.cfg.RepoPath}, subArgs...)
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	logging.Debug("git", "args", subArgs, "output", strings.TrimSpace(string(out)))
	return nil
}
