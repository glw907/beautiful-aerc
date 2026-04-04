// Package palette parses generated palette files and exposes color tokens.
package palette

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Palette holds parsed color tokens from a palette.sh file.
type Palette struct {
	values map[string]string
}

// Load reads and parses a palette.sh file.
func Load(path string) (*Palette, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("palette not found: %w", err)
	}
	defer f.Close()

	p := &Palette{values: make(map[string]string)}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := parseAssignment(line)
		if !ok {
			continue
		}
		p.values[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading palette %s: %w", path, err)
	}
	return p, nil
}

// Get returns the value for a key, or empty string if not found.
func (p *Palette) Get(key string) string {
	return p.values[key]
}

// ANSI returns the ANSI escape sequence for a token key.
// Wraps as \033[<value>m for use in terminal output.
func (p *Palette) ANSI(key string) string {
	v := p.values[key]
	if v == "" {
		return ""
	}
	return "\033[" + v + "m"
}

// Reset returns the ANSI reset sequence.
func (p *Palette) Reset() string {
	return "\033[0m"
}

// parseAssignment parses "KEY=value" or KEY="value" lines.
// Strips inline comments after quoted values.
func parseAssignment(line string) (string, string, bool) {
	eq := strings.IndexByte(line, '=')
	if eq < 1 {
		return "", "", false
	}
	key := line[:eq]
	val := line[eq+1:]

	// Strip quotes
	if len(val) >= 2 && val[0] == '"' {
		end := strings.IndexByte(val[1:], '"')
		if end >= 0 {
			val = val[1 : end+1]
		}
	}
	return key, val, true
}

// HexToANSI converts a hex color like "#81a1c1" to ANSI "38;2;129;161;193".
func HexToANSI(hex string) (string, error) {
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

// FindPath locates palette.sh by checking standard locations.
func FindPath(generatedDir string) (string, error) {
	var candidates []string

	if aercConfig := os.Getenv("AERC_CONFIG"); aercConfig != "" {
		candidates = append(candidates, filepath.Join(aercConfig, "generated", "palette.sh"))
	}

	if generatedDir != "" {
		candidates = append(candidates, filepath.Join(generatedDir, "palette.sh"))
	}

	home, err := os.UserHomeDir()
	if err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "aerc", "generated", "palette.sh"))
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", fmt.Errorf("palette not found - run themes/generate to set up your theme (checked: %s)", strings.Join(candidates, ", "))
}
