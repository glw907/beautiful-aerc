package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Resolve returns the config-file path. Precedence: --config flag,
// $POPLAR_CONFIG, then the OS default (~/.config/poplar/config.toml
// on Linux and macOS, %APPDATA%\poplar\config.toml on Windows). macOS
// uses ~/.config/ rather than ~/Library/Application Support/ to match
// pass, nvim, tmux, and git.
func Resolve(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if env := os.Getenv("POPLAR_CONFIG"); env != "" {
		return env, nil
	}
	dir, err := defaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "poplar", "config.toml"), nil
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

// ErrFirstRun signals that Load wrote a fresh template at the default
// path. Callers should print the path with a re-run hint and exit
// with status 78 (EX_CONFIG).
var ErrFirstRun = errors.New("first-run: template written")

// ErrOldAccountsToml signals a pre-1.0 accounts.toml with no
// config.toml.
var ErrOldAccountsToml = errors.New("old accounts.toml detected, rename to config.toml")

// Load resolves the config path and returns the parsed accounts plus
// the resolved path (callers reuse it for sibling loads like LoadUI).
// A missing default-path file triggers a template write and returns
// ErrFirstRun. A missing --config path returns a plain error with no
// template write.
func Load(flagPath string) ([]AccountConfig, string, error) {
	path, err := Resolve(flagPath)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		accts, err := ParseAccountsFromBytes(data)
		return accts, path, err
	}
	if !os.IsNotExist(err) {
		return nil, path, err
	}
	if flagPath != "" {
		return nil, path, fmt.Errorf("config file %s not found", path)
	}
	dir := filepath.Dir(path)
	legacy := filepath.Join(dir, "accounts.toml")
	if _, statErr := os.Stat(legacy); statErr == nil {
		return nil, path, fmt.Errorf("%w: found %s", ErrOldAccountsToml, legacy)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, path, fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(Template()), 0o600); err != nil {
		return nil, path, fmt.Errorf("write template: %w", err)
	}
	return nil, path, fmt.Errorf("%w: %s", ErrFirstRun, path)
}
