// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailjmap"
)

// defaultConfigPath returns the default path to accounts.toml using
// the platform's user config directory.
func defaultConfigPath() (string, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(configHome, "poplar", "accounts.toml"), nil
}

// openBackend constructs a mail.Backend for the given account based on
// its backend type. Mock is the default for unconfigured (or test) accounts.
// IMAP support lands in Pass 8.
func openBackend(acct config.AccountConfig) (mail.Backend, error) {
	switch acct.Backend {
	case "mock", "":
		return mail.NewMockBackend(), nil
	case "jmap":
		return mailjmap.New(acct), nil
	case "imap":
		return nil, fmt.Errorf("imap backend lands in pass 8")
	default:
		return nil, fmt.Errorf("unknown backend %q for account %q", acct.Backend, acct.Name)
	}
}
