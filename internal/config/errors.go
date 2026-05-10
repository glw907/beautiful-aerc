package config

import (
	"errors"
	"fmt"
	"strings"
)

// ErrConfigInvalid is the sentinel for any structured config error.
// Callers test errors.Is(err, ErrConfigInvalid) to branch on the
// invalid-config family. Specific instances are *ConfigError.
var ErrConfigInvalid = errors.New("invalid config")

// ConfigError describes one validation failure with enough context
// to guide the user to a fix. Path is the resolved config-file path.
// Line is 1-based and 0 when unknown. Account, Field, and Message
// narrow the failure. Suggest is an optional fix hint rendered on
// its own line.
type ConfigError struct {
	Path    string
	Line    int
	Account string
	Field   string
	Message string
	Suggest string
}

func (e *ConfigError) Error() string {
	var b strings.Builder
	if e.Path != "" {
		b.WriteString(e.Path)
		if e.Line > 0 {
			fmt.Fprintf(&b, ":%d", e.Line)
		}
		b.WriteString(": ")
	}
	if e.Account != "" {
		fmt.Fprintf(&b, "account %q: ", e.Account)
	}
	if e.Field != "" {
		fmt.Fprintf(&b, "field %q: ", e.Field)
	}
	b.WriteString(e.Message)
	if e.Suggest != "" {
		b.WriteString("\n  fix: ")
		b.WriteString(e.Suggest)
	}
	return b.String()
}

// Is treats every ConfigError as a member of the ErrConfigInvalid family.
func (e *ConfigError) Is(target error) bool {
	return target == ErrConfigInvalid
}
