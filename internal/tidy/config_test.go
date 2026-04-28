// SPDX-License-Identifier: MIT

package tidy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Rules: all true
	rules := []struct {
		name  string
		value bool
	}{
		{"Spelling", cfg.Rules.Spelling},
		{"Grammar", cfg.Rules.Grammar},
		{"Punctuation", cfg.Rules.Punctuation},
		{"Whitespace", cfg.Rules.Whitespace},
		{"Capitalization", cfg.Rules.Capitalization},
		{"RepeatedWords", cfg.Rules.RepeatedWords},
		{"MissingPunctuation", cfg.Rules.MissingPunctuation},
	}
	for _, r := range rules {
		if !r.value {
			t.Errorf("default %s = false, want true", r.name)
		}
	}

	if cfg.Rules.OxfordComma != "ignore" {
		t.Errorf("default OxfordComma = %q, want %q", cfg.Rules.OxfordComma, "ignore")
	}
	if cfg.Style.EmDashSpaces != false {
		t.Error("default EmDashSpaces = true, want false")
	}
	if cfg.Style.Ellipsis != "character" {
		t.Errorf("default Ellipsis = %q, want %q", cfg.Style.Ellipsis, "character")
	}
	if cfg.Style.TimeFormat != "ignore" {
		t.Errorf("default TimeFormat = %q, want %q", cfg.Style.TimeFormat, "ignore")
	}
	if cfg.API.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("default Model = %q, want %q", cfg.API.Model, "claude-haiku-4-5-20251001")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("LoadConfig missing file: got error %v, want nil", err)
	}
	def := DefaultConfig()
	if cfg.API.Model != def.API.Model {
		t.Errorf("model = %q, want %q", cfg.API.Model, def.API.Model)
	}
	if !cfg.Rules.Spelling {
		t.Error("Spelling = false, want true")
	}
}

func TestLoadConfig_PartialOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[api]
model = "claude-3-opus-20240229"

[rules]
spelling = false
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.API.Model != "claude-3-opus-20240229" {
		t.Errorf("model = %q, want %q", cfg.API.Model, "claude-3-opus-20240229")
	}
	if cfg.Rules.Spelling {
		t.Error("Spelling = true, want false")
	}
	// Unset fields should have defaults.
	if !cfg.Rules.Grammar {
		t.Error("Grammar = false, want true (default)")
	}
	if cfg.Rules.OxfordComma != "ignore" {
		t.Errorf("OxfordComma = %q, want %q", cfg.Rules.OxfordComma, "ignore")
	}
}

func TestLoadConfig_CustomInstructions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[style]
custom_instructions = ["Use active voice.", "Avoid jargon."]
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Style.CustomInstructions) != 2 {
		t.Fatalf("CustomInstructions len = %d, want 2", len(cfg.Style.CustomInstructions))
	}
	if cfg.Style.CustomInstructions[0] != "Use active voice." {
		t.Errorf("CustomInstructions[0] = %q", cfg.Style.CustomInstructions[0])
	}
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("not valid toml = = ="), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig invalid TOML: got nil error, want error")
	}
}

func TestLoadConfig_InvalidOxfordComma(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `[rules]
oxford_comma = "always"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig invalid oxford_comma: got nil error, want error")
	}
}

func TestLoadConfig_InvalidEllipsis(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `[style]
ellipsis = "unicode"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig invalid ellipsis: got nil error, want error")
	}
}

func TestLoadConfig_InvalidTimeFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `[style]
time_format = "dotted"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig invalid time_format: got nil error, want error")
	}
}

func TestApplyRuleOverrides(t *testing.T) {
	tests := []struct {
		name      string
		overrides []string
		wantErr   bool
		check     func(cfg Config) bool
	}{
		{
			name:      "set spelling false",
			overrides: []string{"spelling=false"},
			check:     func(cfg Config) bool { return !cfg.Rules.Spelling },
		},
		{
			name:      "set grammar true",
			overrides: []string{"grammar=true"},
			check:     func(cfg Config) bool { return cfg.Rules.Grammar },
		},
		{
			name:      "set oxford_comma remove",
			overrides: []string{"oxford_comma=remove"},
			check:     func(cfg Config) bool { return cfg.Rules.OxfordComma == "remove" },
		},
		{
			name:      "multiple overrides",
			overrides: []string{"spelling=false", "grammar=false"},
			check:     func(cfg Config) bool { return !cfg.Rules.Spelling && !cfg.Rules.Grammar },
		},
		{
			name:      "unknown key",
			overrides: []string{"unknown=true"},
			wantErr:   true,
		},
		{
			name:      "bad format no equals",
			overrides: []string{"spellingtrue"},
			wantErr:   true,
		},
		{
			name:      "bad bool value",
			overrides: []string{"spelling=yes"},
			wantErr:   true,
		},
		{
			name:      "invalid oxford_comma via override",
			overrides: []string{"oxford_comma=always"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			err := ApplyRuleOverrides(&cfg, tt.overrides)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ApplyRuleOverrides() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.check != nil && !tt.check(cfg) {
				t.Error("override check failed")
			}
		})
	}
}

func TestApplyStyleOverrides(t *testing.T) {
	tests := []struct {
		name      string
		overrides []string
		wantErr   bool
		check     func(cfg Config) bool
	}{
		{
			name:      "set em_dash_spaces true",
			overrides: []string{"em_dash_spaces=true"},
			check:     func(cfg Config) bool { return cfg.Style.EmDashSpaces },
		},
		{
			name:      "set ellipsis dots",
			overrides: []string{"ellipsis=dots"},
			check:     func(cfg Config) bool { return cfg.Style.Ellipsis == "dots" },
		},
		{
			name:      "set time_format uppercase",
			overrides: []string{"time_format=uppercase"},
			check:     func(cfg Config) bool { return cfg.Style.TimeFormat == "uppercase" },
		},
		{
			name:      "set time_format periods",
			overrides: []string{"time_format=periods"},
			check:     func(cfg Config) bool { return cfg.Style.TimeFormat == "periods" },
		},
		{
			name:      "invalid time_format value",
			overrides: []string{"time_format=dotted"},
			wantErr:   true,
		},
		{
			name:      "unknown key",
			overrides: []string{"unknown=true"},
			wantErr:   true,
		},
		{
			name:      "bad format no equals",
			overrides: []string{"em_dash_spaces"},
			wantErr:   true,
		},
		{
			name:      "invalid ellipsis value",
			overrides: []string{"ellipsis=unicode"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			err := ApplyStyleOverrides(&cfg, tt.overrides)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ApplyStyleOverrides() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.check != nil && !tt.check(cfg) {
				t.Error("override check failed")
			}
		})
	}
}

func TestResolveAPIKey(t *testing.T) {
	// Ensure env is clean for this test.
	t.Setenv("ANTHROPIC_API_KEY", "")

	t.Run("config takes precedence", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "env-key")
		cfg := DefaultConfig()
		cfg.API.APIKey = "config-key"
		got := ResolveAPIKey(cfg)
		if got != "config-key" {
			t.Errorf("ResolveAPIKey = %q, want %q", got, "config-key")
		}
	})

	t.Run("falls back to env", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "env-key")
		cfg := DefaultConfig()
		got := ResolveAPIKey(cfg)
		if got != "env-key" {
			t.Errorf("ResolveAPIKey = %q, want %q", got, "env-key")
		}
	})

	t.Run("empty when neither set", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		cfg := DefaultConfig()
		got := ResolveAPIKey(cfg)
		if got != "" {
			t.Errorf("ResolveAPIKey = %q, want empty", got)
		}
	})
}

func TestConfigString(t *testing.T) {
	cfg := DefaultConfig()
	s := ConfigString(cfg)
	if s == "" {
		t.Fatal("ConfigString returned empty string")
	}
	for _, want := range []string{"claude-haiku-4-5-20251001", "ignore", "character", "time_format"} {
		if !strings.Contains(s, want) {
			t.Errorf("ConfigString does not contain %q", want)
		}
	}
}
