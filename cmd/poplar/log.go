package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"golang.org/x/term"
)

func installLogger() {
	level := slog.LevelInfo
	if os.Getenv("POPLAR_LOG") == "debug" {
		level = slog.LevelDebug
	}
	var w io.Writer = os.Stderr
	if term.IsTerminal(int(os.Stdout.Fd())) {
		// TUI mode: stderr is hidden under altscreen.
		// Route to $XDG_STATE_HOME/poplar/poplar.log instead.
		if f, err := openStateLog(); err == nil {
			w = f
		}
	}
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}

func openStateLog() (*os.File, error) {
	dir := stateDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "poplar.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
