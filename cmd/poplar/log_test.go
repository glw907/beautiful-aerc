package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenStateLog_CreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "poplar")
	t.Setenv("XDG_STATE_HOME", dir)

	f, err := openStateLog()
	if err != nil {
		t.Fatalf("openStateLog: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString("hello\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(sub, "poplar.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(data) != "hello\n" {
		t.Errorf("log content = %q, want %q", string(data), "hello\n")
	}
}

func TestOpenStateLog_Append(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	for _, msg := range []string{"first\n", "second\n"} {
		f, err := openStateLog()
		if err != nil {
			t.Fatalf("openStateLog: %v", err)
		}
		if _, err := f.WriteString(msg); err != nil {
			f.Close()
			t.Fatalf("write: %v", err)
		}
		f.Close()
	}

	data, err := os.ReadFile(filepath.Join(dir, "poplar", "poplar.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(data) != "first\nsecond\n" {
		t.Errorf("log content = %q, want %q", string(data), "first\nsecond\n")
	}
}

func TestInstallLogger_DebugLevel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("POPLAR_LOG", "debug")

	installLogger("")

	if !slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Error("expected debug level to be enabled")
	}
}

func TestInstallLogger_InfoLevel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("POPLAR_LOG", "")

	installLogger("")

	if slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Error("expected debug level to be disabled at info")
	}
	if !slog.Default().Enabled(nil, slog.LevelInfo) {
		t.Error("expected info level to be enabled")
	}
}

func TestInstallLogger_ConfigDebug(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("POPLAR_LOG", "")

	installLogger("debug")

	if !slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Error("expected debug level from config")
	}
}

func TestInstallLogger_EnvOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("POPLAR_LOG", "debug")

	installLogger("info")

	if !slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Error("expected env to override config: debug should be enabled")
	}
}
