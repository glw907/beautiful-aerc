package main

import (
	"os"
	"path/filepath"
)

// resolveRulesFile returns the path to the rules JSON file.
// Resolution order: flag > env var > default.
func resolveRulesFile(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("AERC_RULES_FILE"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "aerc", "mailrules.json")
}

// resolveExportDest returns the path to the export destination file.
// Resolution order: flag > env var > default.
func resolveExportDest(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("AERC_RULES_EXPORT_DEST"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "mailrules.json")
}
