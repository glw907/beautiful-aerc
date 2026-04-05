package corpus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"html tag", "<html><body>hello</body></html>", true},
		{"doctype", "<!DOCTYPE html><html>", true},
		{"head tag", "<head><meta charset='utf-8'></head>", true},
		{"body tag", "<body>content</body>", true},
		{"table tag", "<table><tr><td>cell</td></tr></table>", true},
		{"case insensitive", "<HTML><BODY>hello</BODY></HTML>", true},
		{"plain text", "Hello, this is a plain email.", false},
		{"markdown", "# Heading\n\nSome **bold** text.", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHTML([]byte(tt.input))
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSave(t *testing.T) {
	t.Run("html content", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte("<html><body>test</body></html>")
		path, err := Save(dir, content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Ext(path) != ".html" {
			t.Errorf("expected .html extension, got %s", filepath.Ext(path))
		}
		got, _ := os.ReadFile(path)
		if string(got) != string(content) {
			t.Errorf("content mismatch")
		}
	})

	t.Run("plain text content", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte("Hello, plain text email.")
		path, err := Save(dir, content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Ext(path) != ".txt" {
			t.Errorf("expected .txt extension, got %s", filepath.Ext(path))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Save(dir, []byte{})
		if err == nil {
			t.Error("expected error for empty input")
		}
	})

	t.Run("collision avoidance", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte("plain text")
		p1, _ := Save(dir, content)
		p2, _ := Save(dir, content)
		if p1 == p2 {
			t.Error("expected different paths for same-second saves")
		}
	})
}

func TestFindDir(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (envVal string, binHint string)
		wantErr bool
	}{
		{
			"env override",
			func(t *testing.T) (string, string) {
				dir := t.TempDir()
				corpus := filepath.Join(dir, "corpus")
				os.MkdirAll(corpus, 0755)
				return dir, ""
			},
			false,
		},
		{
			"relative to binary hint",
			func(t *testing.T) (string, string) {
				dir := t.TempDir()
				aercDir := filepath.Join(dir, ".config", "aerc")
				os.MkdirAll(aercDir, 0755)
				corpus := filepath.Join(dir, "corpus")
				os.MkdirAll(corpus, 0755)
				return "", aercDir
			},
			false,
		},
		{
			"creates corpus dir if missing",
			func(t *testing.T) (string, string) {
				dir := t.TempDir()
				aercDir := filepath.Join(dir, ".config", "aerc")
				os.MkdirAll(aercDir, 0755)
				return "", aercDir
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVal, binHint := tt.setup(t)
			if envVal != "" {
				t.Setenv("AERC_CONFIG", envVal)
			} else {
				t.Setenv("AERC_CONFIG", "")
			}
			dir, err := FindDir(binHint)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, err := os.Stat(dir); err != nil {
				t.Errorf("corpus dir does not exist: %v", err)
			}
		})
	}
}
