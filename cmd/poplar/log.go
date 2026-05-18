package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/glw907/poplar/internal/logctx"
	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

func installLogger(cfgLevel string) {
	level := slog.LevelInfo
	if os.Getenv("POPLAR_LOG") == "debug" || cfgLevel == "debug" {
		level = slog.LevelDebug
	}
	var w io.Writer = os.Stderr
	if term.IsTerminal(int(os.Stdout.Fd())) {
		// TUI mode: stderr is hidden under altscreen.
		// Route to $XDG_STATE_HOME/poplar/poplar.log with size-based rotation.
		dir := stateDir()
		if err := os.MkdirAll(dir, 0o755); err == nil {
			w = &lumberjack.Logger{
				Filename:   filepath.Join(dir, "poplar.log"),
				MaxSize:    10,
				MaxBackups: 2,
			}
		}
	}
	h := logctx.Handler{Handler: slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})}
	slog.SetDefault(slog.New(h))
}

func stateDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "poplar")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "poplar")
	}
	return filepath.Join(home, ".local", "state", "poplar")
}
