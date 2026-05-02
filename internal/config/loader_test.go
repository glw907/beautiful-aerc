// SPDX-License-Identifier: MIT

package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolvePathFlagWins(t *testing.T) {
	t.Setenv("POPLAR_CONFIG", "/env/path/config.toml")
	got, src, err := Resolve("/flag/path/config.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/flag/path/config.toml" {
		t.Errorf("path = %q, want %q", got, "/flag/path/config.toml")
	}
	if src != SourceFlag {
		t.Errorf("source = %v, want SourceFlag", src)
	}
}

func TestResolveEnvBeatsDefault(t *testing.T) {
	t.Setenv("POPLAR_CONFIG", "/env/path/config.toml")
	got, src, err := Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/env/path/config.toml" {
		t.Errorf("path = %q, want %q", got, "/env/path/config.toml")
	}
	if src != SourceEnv {
		t.Errorf("source = %v, want SourceEnv", src)
	}
}

func TestResolveDefaultMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	t.Setenv("POPLAR_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/Users/test")
	got, src, err := Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/Users/test/.config/poplar/config.toml"
	if got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if src != SourceDefault {
		t.Errorf("source = %v, want SourceDefault", src)
	}
}

func TestResolveDefaultLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	t.Setenv("POPLAR_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/test")
	got, src, err := Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/home/test/.config/poplar/config.toml"
	if got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if src != SourceDefault {
		t.Errorf("source = %v, want SourceDefault", src)
	}
}

func TestLoadFirstRunWritesTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("POPLAR_CONFIG", filepath.Join(dir, "config.toml"))

	_, err := Load("")
	if !errors.Is(err, ErrFirstRun) {
		t.Fatalf("err = %v, want ErrFirstRun", err)
	}
	got, readErr := os.ReadFile(filepath.Join(dir, "config.toml"))
	if readErr != nil {
		t.Fatalf("template not written: %v", readErr)
	}
	if string(got) != Template() {
		t.Errorf("on-disk content does not match Template()")
	}
}

func TestLoadFlagPathMissingErrors(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "explicit.toml")
	_, err := Load(missing)
	if err == nil {
		t.Fatal("expected error for missing flag path")
	}
	if errors.Is(err, ErrFirstRun) {
		t.Errorf("first-run template-write should NOT trigger when path was explicit")
	}
}

func TestLoadDetectsLegacyAccountsToml(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("POPLAR_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "poplar"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "poplar", "accounts.toml"), []byte("[[account]]\nname=\"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load("")
	if !errors.Is(err, ErrOldAccountsToml) {
		t.Errorf("err = %v, want ErrOldAccountsToml", err)
	}
}
