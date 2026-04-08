package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHexToANSI(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want string
	}{
		{"nord blue", "#81a1c1", "38;2;129;161;193"},
		{"pure white", "#ffffff", "38;2;255;255;255"},
		{"pure black", "#000000", "38;2;0;0;0"},
		{"uppercase", "#81A1C1", "38;2;129;161;193"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hexToANSI(tt.hex)
			if err != nil {
				t.Fatalf("hexToANSI(%q): %v", tt.hex, err)
			}
			if got != tt.want {
				t.Errorf("hexToANSI(%q) = %q, want %q", tt.hex, got, tt.want)
			}
		})
	}
}

func TestHexToANSIErrors(t *testing.T) {
	tests := []struct {
		name string
		hex  string
	}{
		{"no hash", "81a1c1"},
		{"too short", "#81a"},
		{"invalid hex", "#zzzzzz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := hexToANSI(tt.hex)
			if err == nil {
				t.Errorf("hexToANSI(%q) should have returned error", tt.hex)
			}
		})
	}
}

func writeTheme(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test theme: %v", err)
	}
	return path
}

const validTheme = `name = "Test"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"

[tokens]
heading = { color = "color_success", bold = true }
bold = { bold = true }
italic = { italic = true }
link_text = { color = "accent_primary", underline = true }
hdr_key = { color = "accent_primary", bold = true }
hdr_dim = { color = "fg_dim" }
`

func TestLoad(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if th.Name != "Test" {
		t.Errorf("Name = %q, want %q", th.Name, "Test")
	}
}

func TestLoadColors(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tests := []struct {
		name string
		want string
	}{
		{"bg_base", "#2e3440"},
		{"accent_primary", "#81a1c1"},
		{"fg_dim", "#616e88"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := th.Color(tt.name)
			if got != tt.want {
				t.Errorf("Color(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestLoadTokenResolution(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tests := []struct {
		name string
		want string
	}{
		// color_success=#a3be8c -> 38;2;163;190;140 + bold=1
		{"heading", "38;2;163;190;140;1"},
		// bold only
		{"bold", "1"},
		// italic only
		{"italic", "3"},
		// accent_primary=#81a1c1 -> 38;2;129;161;193 + underline=4
		{"link_text", "38;2;129;161;193;4"},
		// accent_primary=#81a1c1 -> 38;2;129;161;193 + bold=1
		{"hdr_key", "38;2;129;161;193;1"},
		// fg_dim=#616e88 -> 38;2;97;110;136
		{"hdr_dim", "38;2;97;110;136"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := th.ANSI(tt.name)
			want := "\033[" + tt.want + "m"
			if got != want {
				t.Errorf("ANSI(%q) = %q, want %q", tt.name, got, want)
			}
		})
	}
}

func TestLoadMissingToken(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := th.ANSI("nonexistent")
	if got != "" {
		t.Errorf("ANSI(nonexistent) = %q, want empty", got)
	}
}

func TestReset(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := th.Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() = %q, want %q", got, "\033[0m")
	}
}

func TestLoadMissingColor(t *testing.T) {
	// Missing accent_primary from required colors
	content := `name = "Bad"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"
`
	path := writeTheme(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing color slot")
	}
	if !strings.Contains(err.Error(), "accent_primary") {
		t.Errorf("error = %q, want it to mention accent_primary", err)
	}
}

func TestLoadInvalidHex(t *testing.T) {
	content := `name = "Bad"

[colors]
bg_base = "not-a-color"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"
`
	path := writeTheme(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
	if !strings.Contains(err.Error(), "bg_base") {
		t.Errorf("error = %q, want it to mention bg_base", err)
	}
}

func TestLoadBadColorReference(t *testing.T) {
	bad := `name = "Bad"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"

[tokens]
bad = { color = "nonexistent" }
`
	path := writeTheme(t, bad)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for undefined color reference")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error = %q, want it to mention nonexistent", err)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/theme.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
