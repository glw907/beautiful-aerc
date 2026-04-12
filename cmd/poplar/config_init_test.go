package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runConfigInit invokes the subcommand with the given flags and
// returns its captured stdout.
func execConfigInit(t *testing.T, args ...string) string {
	t.Helper()
	cmd := newConfigInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("config init failed: %v", err)
	}
	return buf.String()
}

func writeStubConfig(t *testing.T, dir, contents string) string {
	t.Helper()
	path := filepath.Join(dir, "accounts.toml")
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

const minimalMockConfig = `[[account]]
name = "Mock"
backend = "mock"
source = "mock://local"

[ui]
threading = true
`

func TestConfigInit_DryRunShowsDiscoveredFolders(t *testing.T) {
	dir := t.TempDir()
	path := writeStubConfig(t, dir, minimalMockConfig)
	out := execConfigInit(t, "--config", path)

	wantKeys := []string{
		"[ui.folders.Inbox]",
		"[ui.folders.Drafts]",
		"[ui.folders.Sent]",
		"[ui.folders.Archive]",
		"[ui.folders.Spam]",
		"[ui.folders.Trash]",
		`[ui.folders.Notifications]`,
		`[ui.folders.Remind]`,
		`[ui.folders."Lists/golang"]`,
		`[ui.folders."Lists/rust"]`,
	}
	for _, k := range wantKeys {
		if !strings.Contains(out, k) {
			t.Errorf("expected %q in dry-run output", k)
		}
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != minimalMockConfig {
		t.Errorf("dry-run should not modify file; got:\n%s", got)
	}
}

func TestConfigInit_WriteAppends(t *testing.T) {
	dir := t.TempDir()
	path := writeStubConfig(t, dir, minimalMockConfig)
	_ = execConfigInit(t, "--config", path, "--write")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "[ui.folders.Inbox]") {
		t.Errorf("expected Inbox subsection after --write\ngot:\n%s", got)
	}
	if !strings.Contains(string(got), `name = "Mock"`) {
		t.Errorf("original config lost\ngot:\n%s", got)
	}
}

func TestConfigInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := writeStubConfig(t, dir, minimalMockConfig)
	_ = execConfigInit(t, "--config", path, "--write")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = execConfigInit(t, "--config", path, "--write")
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("second run should be no-op\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
