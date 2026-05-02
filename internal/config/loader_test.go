// SPDX-License-Identifier: MIT

package config

import (
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
