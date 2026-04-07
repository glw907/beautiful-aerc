package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRulesFile(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}
	defaultPath := filepath.Join(home, ".config", "aerc", "mailrules.json")

	tests := []struct {
		name      string
		flagValue string
		envValue  string
		want      string
	}{
		{
			name: "default when no flag or env",
			want: defaultPath,
		},
		{
			name:     "env overrides default",
			envValue: "/tmp/custom-rules.json",
			want:     "/tmp/custom-rules.json",
		},
		{
			name:      "flag overrides env",
			flagValue: "/tmp/flag-rules.json",
			envValue:  "/tmp/env-rules.json",
			want:      "/tmp/flag-rules.json",
		},
		{
			name:      "flag overrides default",
			flagValue: "/tmp/flag-rules.json",
			want:      "/tmp/flag-rules.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("AERC_RULES_FILE")
			if tt.envValue != "" {
				t.Setenv("AERC_RULES_FILE", tt.envValue)
			}
			got := resolveRulesFile(tt.flagValue)
			if got != tt.want {
				t.Errorf("resolveRulesFile(%q) = %q, want %q", tt.flagValue, got, tt.want)
			}
		})
	}
}

func TestResolveExportDest(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}
	defaultPath := filepath.Join(home, "Documents", "mailrules.json")

	tests := []struct {
		name      string
		flagValue string
		envValue  string
		want      string
	}{
		{
			name: "default when no flag or env",
			want: defaultPath,
		},
		{
			name:     "env overrides default",
			envValue: "/tmp/custom-export.json",
			want:     "/tmp/custom-export.json",
		},
		{
			name:      "flag overrides env",
			flagValue: "/tmp/flag-export.json",
			envValue:  "/tmp/env-export.json",
			want:      "/tmp/flag-export.json",
		},
		{
			name:      "flag overrides default",
			flagValue: "/tmp/flag-export.json",
			want:      "/tmp/flag-export.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("AERC_RULES_EXPORT_DEST")
			if tt.envValue != "" {
				t.Setenv("AERC_RULES_EXPORT_DEST", tt.envValue)
			}
			got := resolveExportDest(tt.flagValue)
			if got != tt.want {
				t.Errorf("resolveExportDest(%q) = %q, want %q", tt.flagValue, got, tt.want)
			}
		})
	}
}
