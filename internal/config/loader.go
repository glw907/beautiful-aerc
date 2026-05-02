// SPDX-License-Identifier: MIT

package config

import (
	"errors"
	"fmt"
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

// ErrFirstRun is returned by Load when the default config path
// did not exist and a fresh template was written. The caller
// should print a "created <path> — edit and run again" message
// and exit with status 78 (EX_CONFIG).
var ErrFirstRun = errors.New("first-run: template written")

// ErrOldAccountsToml is returned when the user has an old
// accounts.toml file (pre-1.0 carryover) and no config.toml.
var ErrOldAccountsToml = errors.New("old accounts.toml detected; rename to config.toml")

// Load resolves the config path and returns the parsed accounts.
// When src is SourceDefault or SourceEnv and no file exists, it
// writes the template and returns ErrFirstRun. When src is
// SourceFlag and the file is missing, it returns a plain error
// (the user explicitly chose that path; no template is written).
func Load(flagPath string) ([]AccountConfig, error) {
	path, src, err := Resolve(flagPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		return ParseAccountsFromBytes(data)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if src == SourceFlag {
		return nil, fmt.Errorf("config file %s not found", path)
	}
	dir := filepath.Dir(path)
	legacy := filepath.Join(dir, "accounts.toml")
	if _, statErr := os.Stat(legacy); statErr == nil {
		return nil, fmt.Errorf("%w: found %s", ErrOldAccountsToml, legacy)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(Template()), 0o600); err != nil {
		return nil, fmt.Errorf("write template: %w", err)
	}
	return nil, fmt.Errorf("%w: %s", ErrFirstRun, path)
}
