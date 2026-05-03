// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome turns a leading "~" into the user's home directory.
// "~" alone resolves to $HOME; "~/x" resolves to $HOME/x. Other
// paths pass through. The empty string also passes through (callers
// supply their own default).
func ExpandHome(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~/")), nil
}
