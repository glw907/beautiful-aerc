package palette

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		key     string
		want    string
	}{
		{
			name:    "unquoted value",
			content: "FG_BASE=#d8dee9",
			key:     "FG_BASE",
			want:    "#d8dee9",
		},
		{
			name:    "quoted value",
			content: `C_BOLD="1"`,
			key:     "C_BOLD",
			want:    "1",
		},
		{
			name:    "skip comments",
			content: "# comment\nFG_BASE=#d8dee9",
			key:     "FG_BASE",
			want:    "#d8dee9",
		},
		{
			name:    "skip blank lines",
			content: "\n\nFG_BASE=#d8dee9\n\n",
			key:     "FG_BASE",
			want:    "#d8dee9",
		},
		{
			name:    "override earlier value",
			content: "C_LINK_URL=\"4;38;2;163;190;140\"\n# --- overrides below this line are preserved across regeneration ---\nC_LINK_URL=\"38;2;97;110;136\"",
			key:     "C_LINK_URL",
			want:    "38;2;97;110;136",
		},
		{
			name:    "quoted with comment suffix",
			content: `C_RULE="38;2;97;110;136"    # FG_DIM`,
			key:     "C_RULE",
			want:    "38;2;97;110;136",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "palette.sh")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("writing test file: %v", err)
			}
			p, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := p.Get(tt.key)
			if got != tt.want {
				t.Errorf("Get(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestLoadNotFound(t *testing.T) {
	_, err := Load("/nonexistent/palette.sh")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "palette not found") {
		t.Errorf("error = %q, want it to contain 'palette not found'", err)
	}
}

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
			got, err := HexToANSI(tt.hex)
			if err != nil {
				t.Fatalf("HexToANSI(%q): %v", tt.hex, err)
			}
			if got != tt.want {
				t.Errorf("HexToANSI(%q) = %q, want %q", tt.hex, got, tt.want)
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
			_, err := HexToANSI(tt.hex)
			if err == nil {
				t.Errorf("HexToANSI(%q) should have returned error", tt.hex)
			}
		})
	}
}

func TestFindPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "palette.sh")
	if err := os.WriteFile(path, []byte("FG_BASE=#d8dee9"), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	t.Setenv("AERC_CONFIG", dir+"/..")  // won't match
	got, err := FindPath(dir)
	if err != nil {
		t.Fatalf("FindPath: %v", err)
	}
	if got != path {
		t.Errorf("FindPath() = %q, want %q", got, path)
	}
}
