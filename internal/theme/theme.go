// Package theme loads TOML theme files and resolves color tokens to ANSI escape sequences.
package theme

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// requiredColors lists the 16 color slots every theme must define.
var requiredColors = []string{
	"bg_base", "bg_elevated", "bg_selection", "bg_border",
	"fg_base", "fg_bright", "fg_brightest", "fg_dim",
	"accent_primary", "accent_secondary", "accent_tertiary",
	"color_error", "color_warning", "color_success", "color_info", "color_special",
}

// Theme holds parsed color slots and resolved ANSI tokens.
type Theme struct {
	Name   string
	colors map[string]string // "accent_primary" → "#81a1c1"
	tokens map[string]string // "hdr_key" → "38;2;129;161;193;1"
}

// themeFile is the TOML deserialization target.
type themeFile struct {
	Name   string                     `toml:"name"`
	Colors map[string]string          `toml:"colors"`
	Tokens map[string]tokenDefinition `toml:"tokens"`
}

type tokenDefinition struct {
	Color     string `toml:"color"`
	Bold      bool   `toml:"bold"`
	Italic    bool   `toml:"italic"`
	Underline bool   `toml:"underline"`
}

// Load reads and validates a TOML theme file. All tokens are resolved
// to ANSI SGR parameter strings at load time.
func Load(path string) (*Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading theme: %w", err)
	}

	var f themeFile
	if err := toml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing theme %s: %w", path, err)
	}

	if f.Colors == nil {
		return nil, fmt.Errorf("theme %s: missing [colors] section", path)
	}

	// Validate required color slots and hex format.
	for _, name := range requiredColors {
		hex, ok := f.Colors[name]
		if !ok {
			return nil, fmt.Errorf("theme %s: missing required color %q", path, name)
		}
		if _, err := hexToANSI(hex); err != nil {
			return nil, fmt.Errorf("theme %s: color %q: %w", path, name, err)
		}
	}

	// Resolve tokens.
	resolved := make(map[string]string, len(f.Tokens))
	for name, def := range f.Tokens {
		sgr, err := resolveToken(def, f.Colors)
		if err != nil {
			return nil, fmt.Errorf("theme %s: token %q: %w", path, name, err)
		}
		resolved[name] = sgr
	}

	return &Theme{
		Name:   f.Name,
		colors: f.Colors,
		tokens: resolved,
	}, nil
}

// ANSI returns the ANSI escape sequence for a token, or "" if not found.
func (t *Theme) ANSI(name string) string {
	v := t.tokens[name]
	if v == "" {
		return ""
	}
	return "\033[" + v + "m"
}

// Color returns the hex value for a color slot, or "" if not found.
func (t *Theme) Color(name string) string {
	return t.colors[name]
}

// Reset returns the ANSI reset sequence.
func (t *Theme) Reset() string {
	return "\033[0m"
}

// resolveToken converts a token definition to an ANSI SGR parameter string.
func resolveToken(def tokenDefinition, colors map[string]string) (string, error) {
	var parts []string

	if def.Color != "" {
		hex, ok := colors[def.Color]
		if !ok {
			return "", fmt.Errorf("references undefined color %q", def.Color)
		}
		ansi, err := hexToANSI(hex)
		if err != nil {
			return "", err
		}
		parts = append(parts, ansi)
	}

	if def.Bold {
		parts = append(parts, "1")
	}
	if def.Italic {
		parts = append(parts, "3")
	}
	if def.Underline {
		parts = append(parts, "4")
	}

	return strings.Join(parts, ";"), nil
}

// hexToANSI converts "#rrggbb" to "38;2;R;G;B".
func hexToANSI(hex string) (string, error) {
	if len(hex) != 7 || hex[0] != '#' {
		return "", fmt.Errorf("invalid hex color %q: must be #rrggbb", hex)
	}
	r, err := strconv.ParseUint(hex[1:3], 16, 8)
	if err != nil {
		return "", fmt.Errorf("invalid hex color %q: %w", hex, err)
	}
	g, err := strconv.ParseUint(hex[3:5], 16, 8)
	if err != nil {
		return "", fmt.Errorf("invalid hex color %q: %w", hex, err)
	}
	b, err := strconv.ParseUint(hex[5:7], 16, 8)
	if err != nil {
		return "", fmt.Errorf("invalid hex color %q: %w", hex, err)
	}
	return fmt.Sprintf("38;2;%d;%d;%d", r, g, b), nil
}
