package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/logctx"
)

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

func TestInstallLogger_OpIDPropagation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("POPLAR_LOG", "debug")

	installLogger("")

	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(logctx.Handler{Handler: inner}))

	ctx := logctx.WithOpID(context.Background(), "42")
	slog.DebugContext(ctx, "probe")

	if !strings.Contains(buf.String(), "op_id=42") {
		t.Errorf("op_id not propagated: %s", buf.String())
	}
}
