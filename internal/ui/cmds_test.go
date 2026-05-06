// SPDX-License-Identifier: MIT

package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeAttachFilename(t *testing.T) {
	cases := []struct {
		name, partID, want string
	}{
		{"report.pdf", "2", "report.pdf"},
		{"", "2.1", "attachment-2.1"},
		{"a/b/c.txt", "1", "a_b_c.txt"},
		{"  spaced.bin  ", "3", "spaced.bin"},
	}
	for _, c := range cases {
		if got := sanitizeAttachFilename(c.name, c.partID); got != c.want {
			t.Errorf("sanitize(%q, %q) = %q, want %q", c.name, c.partID, got, c.want)
		}
	}
}

func TestResolveSaveTarget_Collision(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.pdf"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a-1.pdf"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := resolveSaveTarget(dir, "a.pdf")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := filepath.Join(dir, "a-2.pdf")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveSaveTarget_Fresh(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveSaveTarget(dir, "fresh.bin")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != filepath.Join(dir, "fresh.bin") {
		t.Errorf("got %q", got)
	}
}
