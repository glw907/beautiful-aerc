package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Resolve returns the config-file path to use. Precedence:
// --config flag, then $POPLAR_CONFIG, then the OS default.
//
// Linux/macOS default: ~/.config/poplar/config.toml.
// Windows default:     %APPDATA%\poplar\config.toml.
//
// macOS deliberately uses ~/.config/ rather than the OS-default
// ~/Library/Application Support/, matching the convention used by
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

// ErrFirstRun is returned by Load when the default config path
// did not exist and a fresh template was written. The caller
// should print the path with a re-run hint and exit with status
// 78 (EX_CONFIG).
var ErrFirstRun = errors.New("first-run: template written")

// ErrOldAccountsToml is returned when the user has an old
// accounts.toml file (pre-1.0 carryover) and no config.toml.
var ErrOldAccountsToml = errors.New("old accounts.toml detected; rename to config.toml")

// Load resolves the config path and returns the parsed accounts
// alongside the resolved path (so callers can reuse it for sibling
// loads such as LoadUI without re-resolving). When the path comes
// from $POPLAR_CONFIG or the OS default and no file exists, it
// writes the template and returns ErrFirstRun. When the path was
// supplied via flagPath and the file is missing, it returns a plain
// error (the user explicitly chose that path). No template is written.
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
