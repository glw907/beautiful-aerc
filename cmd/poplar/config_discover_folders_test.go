// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// execConfigDiscoverFolders invokes "poplar config discover-folders [args...]" through the full
// command tree so the persistent --config flag (defined on root) is visible.
func execConfigDiscoverFolders(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCmd()
	root.AddCommand(newConfigCmd())
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"config", "discover-folders"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("config discover-folders failed: %v", err)
	}
	return buf.String()
}

func writeStubConfig(t *testing.T, dir, contents string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

const minimalMockConfig = `[[account]]
name = "Mock"
provider = "mock"
source = "mock://local"

[ui]
threading = true
`

func TestConfigDiscoverFolders_DryRunShowsDiscoveredFolders(t *testing.T) {
	dir := t.TempDir()
	path := writeStubConfig(t, dir, minimalMockConfig)
	out := execConfigDiscoverFolders(t, "--config", path)

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

func TestConfigDiscoverFolders_WriteAppends(t *testing.T) {
	dir := t.TempDir()
	path := writeStubConfig(t, dir, minimalMockConfig)
	_ = execConfigDiscoverFolders(t, "--config", path, "--write")

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

func TestConfigDiscoverFolders_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := writeStubConfig(t, dir, minimalMockConfig)
	_ = execConfigDiscoverFolders(t, "--config", path, "--write")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = execConfigDiscoverFolders(t, "--config", path, "--write")
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("second run should be no-op\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
