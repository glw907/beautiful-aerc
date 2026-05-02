// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Source records where the config path came from.
type Source int

const (
	SourceFlag    Source = iota
	SourceEnv
	SourceDefault
)

// Resolve returns the config-file path to use, plus how it was
// chosen. Precedence: --config flag, then $POPLAR_CONFIG, then the
// OS default.
//
// Linux/macOS default: ~/.config/poplar/config.toml.
// Windows default:     %APPDATA%\poplar\config.toml.
//
// macOS deliberately uses ~/.config/ rather than the OS-default
// ~/Library/Application Support/, matching the convention used by
// pass, nvim, tmux, and git.
func Resolve(flagPath string) (string, Source, error) {
	if flagPath != "" {
		return flagPath, SourceFlag, nil
	}
	if env := os.Getenv("POPLAR_CONFIG"); env != "" {
		return env, SourceEnv, nil
	}
	dir, err := defaultConfigDir()
	if err != nil {
		return "", SourceDefault, err
	}
	return filepath.Join(dir, "poplar", "config.toml"), SourceDefault, nil
}

func defaultConfigDir() (string, error) {
	switch runtime.GOOS {
	case "darwin", "linux", "freebsd", "openbsd", "netbsd":
		if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
			return v, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config"), nil
	default:
		return os.UserConfigDir()
	}
}
