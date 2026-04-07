package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// errNoExportNeeded signals that export-check found no changes.
// main() treats this as a silent exit 1 (no error message).
var errNoExportNeeded = errors.New("no export needed")

func statePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "fastmail-cli", "last-export")
}

type exportFlags struct {
	rulesFile  string
	exportDest string
}

func newExportCmd() *cobra.Command {
	var f exportFlags

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Copy rules file to export destination",
		Long: `Copy the local rules file to an export destination for upload to
Fastmail's web UI. Updates the last-export timestamp so export-check
knows whether a fresh export is needed.

Used in the aerc shutdown hook:
  aerc-shutdown=fastmail-cli rules export-check && fastmail-cli rules export`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExport(resolveRulesFile(f.rulesFile), resolveExportDest(f.exportDest))
		},
	}

	cmd.Flags().StringVar(&f.rulesFile, "rules-file", "", "Path to rules file (default: $AERC_RULES_FILE or ~/.config/aerc/mailrules.json)")
	cmd.Flags().StringVar(&f.exportDest, "export-dest", "", "Export destination (default: $AERC_RULES_EXPORT_DEST or ~/Documents/mailrules.json)")

	return cmd
}

func runExport(src, dst string) error {
	stamp := statePath()

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening rules file: %w", err)
	}
	defer in.Close()

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// Atomic write: temp file in same directory, then rename
	tmp, err := os.CreateTemp(dir, ".tmp-export-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		return fmt.Errorf("copying rules: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing export file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing export file: %w", err)
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("renaming export file: %w", err)
	}
	ok = true

	// Update last-export timestamp
	if err := os.MkdirAll(filepath.Dir(stamp), 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	if err := os.WriteFile(stamp, nil, 0644); err != nil {
		return fmt.Errorf("updating export timestamp: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Exported to %s\n", dst)
	return nil
}

type exportCheckFlags struct {
	rulesFile string
}

func newExportCheckCmd() *cobra.Command {
	var f exportCheckFlags

	cmd := &cobra.Command{
		Use:   "export-check",
		Short: "Exit 0 if rules changed since last export, 1 if unchanged",
		Long: `Check whether the rules file has been modified since the last export.
Exits 0 if an export is needed, exits 1 if unchanged.

Used in the aerc shutdown hook before export:
  aerc-shutdown=fastmail-cli rules export-check && fastmail-cli rules export`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExportCheck(resolveRulesFile(f.rulesFile))
		},
	}

	cmd.Flags().StringVar(&f.rulesFile, "rules-file", "", "Path to rules file (default: $AERC_RULES_FILE or ~/.config/aerc/mailrules.json)")

	return cmd
}

func runExportCheck(src string) error {
	stamp := statePath()

	srcInfo, err := os.Stat(src)
	if err != nil {
		// No rules file means nothing to export
		return errNoExportNeeded
	}

	stampInfo, err := os.Stat(stamp)
	if err != nil {
		// No timestamp means never exported - export needed
		return nil
	}

	if srcInfo.ModTime().After(stampInfo.ModTime()) {
		// Rules changed since last export
		return nil
	}

	return errNoExportNeeded
}
