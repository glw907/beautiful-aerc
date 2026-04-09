package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateStyleset(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	output, err := GenerateStyleset(th)
	if err != nil {
		t.Fatalf("GenerateStyleset: %v", err)
	}

	// Verify key structural elements
	checks := []struct {
		name string
		want string
	}{
		{"title bg", "title.bg=#81a1c1"},
		{"title fg", "title.fg=#2e3440"},
		{"error", "error.fg=#bf616a"},
		{"warning", "warning.fg=#d08770"},
		{"success", "success.fg=#a3be8c"},
		{"msglist unread", "msglist_unread.fg=#8fbcbb"},
		{"tab selected", "tab.selected.bg=#88c0d0"},
		{"selection", "*.selected.bg=#394353"},
		{"quote 1", "quote_1.fg=#8fbcbb"},
		{"diff add", "diff_add.fg=#a3be8c"},
		{"diff del", "diff_del.fg=#bf616a"},
		{"border", "border.fg=#49576b"},
		{"viewer section", "[viewer]"},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(output, c.want) {
				t.Errorf("output missing %q", c.want)
			}
		})
	}
}

func TestGenerateStylesetWriteFile(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "testtheme")
	if err := WriteStyleset(th, outPath); err != nil {
		t.Fatalf("WriteStyleset: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if !strings.Contains(string(data), "title.bg=#81a1c1") {
		t.Error("written file missing expected content")
	}
}
