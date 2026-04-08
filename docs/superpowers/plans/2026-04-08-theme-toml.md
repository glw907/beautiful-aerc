# Theme System Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace shell-based theme generation with direct TOML theme loading in Go, eliminating `palette.sh` and the `themes/generate` shell script.

**Architecture:** New `internal/theme` package loads `.toml` theme files, resolves color references + modifiers to ANSI SGR strings at load time, and exposes a lookup API. A `mailrender themes generate` subcommand produces aerc stylesets. `internal/palette` is deleted.

**Tech Stack:** Go, `github.com/BurntSushi/toml` (already in go.mod), cobra

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/theme/theme.go` | TOML loading, validation, token resolution, theme discovery |
| `internal/theme/theme_test.go` | Unit tests for loading, resolution, validation, discovery |
| `internal/theme/styleset.go` | Styleset template + generation function |
| `internal/theme/styleset_test.go` | Styleset generation tests |
| `cmd/mailrender/themes.go` | Cobra `themes` + `themes generate` subcommands |
| `.config/aerc/themes/nord.toml` | Nord theme (TOML format) |
| `.config/aerc/themes/solarized-dark.toml` | Solarized Dark theme (TOML format) |
| `.config/aerc/themes/gruvbox-dark.toml` | Gruvbox Dark theme (TOML format) |

---

### Task 1: Create `internal/theme/theme.go` with TOML loading and token resolution

This is the core package. It reads a TOML theme file, validates all 16 required color slots, resolves each token's color reference + modifiers to an ANSI SGR parameter string, and caches the results. The existing `HexToANSI` logic moves here from `internal/palette/palette.go`.

**Files:**
- Create: `internal/theme/theme.go`
- Create: `internal/theme/theme_test.go`

**Context:**
- The TOML format is defined in the spec: `docs/superpowers/specs/2026-04-08-theme-toml-design.md` (lines 23-63)
- The existing `HexToANSI` function in `internal/palette/palette.go:86-103` is moved here unchanged
- `BurntSushi/toml` is already in `go.mod` — no `go get` needed
- Token resolution: look up `color` field in `[colors]` map, convert hex to `38;2;R;G;B`, append modifier codes (bold=1, italic=3, underline=4), join with `;`

- [ ] **Step 1: Write the failing tests**

```go
package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHexToANSI(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want string
	}{
		{"nord blue", "#81a1c1", "38;2;129;161;193"},
		{"pure white", "#ffffff", "38;2;255;255;255"},
		{"pure black", "#000000", "38;2;0;0;0"},
		{"uppercase", "#81A1C1", "38;2;129;161;193"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hexToANSI(tt.hex)
			if err != nil {
				t.Fatalf("hexToANSI(%q): %v", tt.hex, err)
			}
			if got != tt.want {
				t.Errorf("hexToANSI(%q) = %q, want %q", tt.hex, got, tt.want)
			}
		})
	}
}

func TestHexToANSIErrors(t *testing.T) {
	tests := []struct {
		name string
		hex  string
	}{
		{"no hash", "81a1c1"},
		{"too short", "#81a"},
		{"invalid hex", "#zzzzzz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := hexToANSI(tt.hex)
			if err == nil {
				t.Errorf("hexToANSI(%q) should have returned error", tt.hex)
			}
		})
	}
}

func writeTheme(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test theme: %v", err)
	}
	return path
}

const validTheme = `name = "Test"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"

[tokens]
heading = { color = "color_success", bold = true }
bold = { bold = true }
italic = { italic = true }
link_text = { color = "accent_primary", underline = true }
hdr_key = { color = "accent_primary", bold = true }
hdr_dim = { color = "fg_dim" }
`

func TestLoad(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if th.Name != "Test" {
		t.Errorf("Name = %q, want %q", th.Name, "Test")
	}
}

func TestLoadColors(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tests := []struct {
		name string
		want string
	}{
		{"bg_base", "#2e3440"},
		{"accent_primary", "#81a1c1"},
		{"fg_dim", "#616e88"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := th.Color(tt.name)
			if got != tt.want {
				t.Errorf("Color(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestLoadTokenResolution(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tests := []struct {
		name string
		want string
	}{
		// color_success=#a3be8c -> 38;2;163;190;140 + bold=1
		{"heading", "38;2;163;190;140;1"},
		// bold only
		{"bold", "1"},
		// italic only
		{"italic", "3"},
		// accent_primary=#81a1c1 -> 38;2;129;161;193 + underline=4
		{"link_text", "38;2;129;161;193;4"},
		// accent_primary=#81a1c1 -> 38;2;129;161;193 + bold=1
		{"hdr_key", "38;2;129;161;193;1"},
		// fg_dim=#616e88 -> 38;2;97;110;136
		{"hdr_dim", "38;2;97;110;136"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ANSI wraps in \033[..m, so check the raw resolved value
			got := th.ANSI(tt.name)
			want := "\033[" + tt.want + "m"
			if got != want {
				t.Errorf("ANSI(%q) = %q, want %q", tt.name, got, want)
			}
		})
	}
}

func TestLoadMissingToken(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := th.ANSI("nonexistent")
	if got != "" {
		t.Errorf("ANSI(nonexistent) = %q, want empty", got)
	}
}

func TestReset(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := th.Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() = %q, want %q", got, "\033[0m")
	}
}

func TestLoadMissingColor(t *testing.T) {
	// Missing accent_primary from required colors
	content := `name = "Bad"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"
`
	path := writeTheme(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing color slot")
	}
	if !strings.Contains(err.Error(), "accent_primary") {
		t.Errorf("error = %q, want it to mention accent_primary", err)
	}
}

func TestLoadInvalidHex(t *testing.T) {
	content := `name = "Bad"

[colors]
bg_base = "not-a-color"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"
`
	path := writeTheme(t, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
	if !strings.Contains(err.Error(), "bg_base") {
		t.Errorf("error = %q, want it to mention bg_base", err)
	}
}

func TestLoadBadColorReference(t *testing.T) {
	content := validTheme + `

[tokens]
bad_token = { color = "nonexistent_color" }
`
	// Need to remove the existing [tokens] section and add a bad one.
	// Simpler: just build a minimal valid theme with a bad token.
	bad := `name = "Bad"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"

[tokens]
bad = { color = "nonexistent" }
`
	path := writeTheme(t, bad)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for undefined color reference")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error = %q, want it to mention nonexistent", err)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/theme.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./internal/theme/ -v -count=1`
Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Write the implementation**

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./internal/theme/ -v -count=1`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/theme/theme.go internal/theme/theme_test.go
git commit -m "Add internal/theme package with TOML loading and token resolution"
```

---

### Task 2: Add `FindPath` for theme discovery via `aerc.conf`

`FindPath` locates the active theme by reading `styleset-name` from `aerc.conf` and looking for the corresponding `.toml` file in the `themes/` directory. This replaces the old `palette.FindPath` which looked for `generated/palette.sh`.

**Files:**
- Modify: `internal/theme/theme.go`
- Modify: `internal/theme/theme_test.go`

**Context:**
- `aerc.conf` is an INI-like file. `styleset-name=nord` appears in the `[ui]` section.
- Lookup order: `$AERC_CONFIG/aerc.conf`, then `~/.config/aerc/aerc.conf`.
- After finding the name, look for `themes/<name>.toml` relative to the directory containing `aerc.conf`.
- The existing `palette.FindPath` is at `internal/palette/palette.go:106-129` — same search-path pattern but for `palette.sh`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/theme/theme_test.go`:

```go
func TestFindPathWithEnv(t *testing.T) {
	dir := t.TempDir()

	// Write aerc.conf with styleset-name
	aercConf := "# comment\n[ui]\nstyleset-name=testtheme\n"
	if err := os.WriteFile(filepath.Join(dir, "aerc.conf"), []byte(aercConf), 0644); err != nil {
		t.Fatal(err)
	}

	// Write the theme file
	themesDir := filepath.Join(dir, "themes")
	os.MkdirAll(themesDir, 0755)
	if err := os.WriteFile(filepath.Join(themesDir, "testtheme.toml"), []byte(validTheme), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AERC_CONFIG", dir)
	got, err := FindPath()
	if err != nil {
		t.Fatalf("FindPath: %v", err)
	}
	want := filepath.Join(themesDir, "testtheme.toml")
	if got != want {
		t.Errorf("FindPath() = %q, want %q", got, want)
	}
}

func TestFindPathNoAercConf(t *testing.T) {
	t.Setenv("AERC_CONFIG", "/nonexistent/path")
	t.Setenv("HOME", "/nonexistent/home")
	_, err := FindPath()
	if err == nil {
		t.Fatal("expected error when aerc.conf not found")
	}
	if !strings.Contains(err.Error(), "aerc.conf") {
		t.Errorf("error = %q, want mention of aerc.conf", err)
	}
}

func TestFindPathNoStylesetName(t *testing.T) {
	dir := t.TempDir()
	aercConf := "[ui]\n# no styleset-name\n"
	os.WriteFile(filepath.Join(dir, "aerc.conf"), []byte(aercConf), 0644)
	t.Setenv("AERC_CONFIG", dir)
	_, err := FindPath()
	if err == nil {
		t.Fatal("expected error when styleset-name missing")
	}
	if !strings.Contains(err.Error(), "styleset-name") {
		t.Errorf("error = %q, want mention of styleset-name", err)
	}
}

func TestFindPathNoThemeFile(t *testing.T) {
	dir := t.TempDir()
	aercConf := "[ui]\nstyleset-name=missing\n"
	os.WriteFile(filepath.Join(dir, "aerc.conf"), []byte(aercConf), 0644)
	os.MkdirAll(filepath.Join(dir, "themes"), 0755)
	t.Setenv("AERC_CONFIG", dir)
	_, err := FindPath()
	if err == nil {
		t.Fatal("expected error when theme file not found")
	}
	if !strings.Contains(err.Error(), "missing.toml") {
		t.Errorf("error = %q, want mention of missing.toml", err)
	}
}

func TestFindPathStylesetNameVariants(t *testing.T) {
	tests := []struct {
		name     string
		conf     string
		wantName string
	}{
		{"with spaces", "[ui]\nstyleset-name = nord\n", "nord"},
		{"no section header", "styleset-name=nord\n", "nord"},
		{"trailing whitespace", "styleset-name=nord  \n", "nord"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "aerc.conf"), []byte(tt.conf), 0644)
			themesDir := filepath.Join(dir, "themes")
			os.MkdirAll(themesDir, 0755)
			os.WriteFile(filepath.Join(themesDir, tt.wantName+".toml"), []byte(validTheme), 0644)
			t.Setenv("AERC_CONFIG", dir)
			got, err := FindPath()
			if err != nil {
				t.Fatalf("FindPath: %v", err)
			}
			want := filepath.Join(themesDir, tt.wantName+".toml")
			if got != want {
				t.Errorf("FindPath() = %q, want %q", got, want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./internal/theme/ -run 'FindPath' -v -count=1`
Expected: FAIL — `FindPath` not defined.

- [ ] **Step 3: Write the implementation**

Add to `internal/theme/theme.go`:

```go
// FindPath locates the active theme file. Reads styleset-name from
// aerc.conf, then looks for themes/<name>.toml relative to the
// aerc.conf directory.
func FindPath() (string, error) {
	confDir, err := FindConfigDir()
	if err != nil {
		return "", err
	}

	name, err := readStylesetName(filepath.Join(confDir, "aerc.conf"))
	if err != nil {
		return "", err
	}

	path := filepath.Join(confDir, "themes", name+".toml")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("theme file not found: %s", path)
	}
	return path, nil
}

// FindConfigDir returns the directory containing aerc.conf.
func FindConfigDir() (string, error) {
	if dir := os.Getenv("AERC_CONFIG"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "aerc.conf")); err == nil {
			return dir, nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", "aerc")
	if _, err := os.Stat(filepath.Join(dir, "aerc.conf")); err == nil {
		return dir, nil
	}

	return "", fmt.Errorf("aerc.conf not found (checked $AERC_CONFIG and ~/.config/aerc/)")
}

// readStylesetName extracts the styleset-name value from aerc.conf.
func readStylesetName(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading aerc.conf: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) == "styleset-name" {
			name := strings.TrimSpace(val)
			if name != "" {
				return name, nil
			}
		}
	}
	return "", fmt.Errorf("styleset-name not found in %s", path)
}
```

Add `"path/filepath"` to the import list in `theme.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./internal/theme/ -v -count=1`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/theme/theme.go internal/theme/theme_test.go
git commit -m "Add FindPath to discover active theme via aerc.conf"
```

---

### Task 3: Convert theme files from shell to TOML

Convert the three shipped theme files from `.sh` to `.toml` format. The hex values and token definitions are identical — only the syntax changes. The old `.sh` files are deleted.

**Files:**
- Create: `.config/aerc/themes/nord.toml`
- Create: `.config/aerc/themes/solarized-dark.toml`
- Create: `.config/aerc/themes/gruvbox-dark.toml`
- Delete: `.config/aerc/themes/nord.sh`
- Delete: `.config/aerc/themes/solarized-dark.sh`
- Delete: `.config/aerc/themes/gruvbox-dark.sh`

**Context:**
- Current shell files are at `.config/aerc/themes/nord.sh` (54 lines), `solarized-dark.sh` (46 lines), `gruvbox-dark.sh` (46 lines)
- All three have identical token definitions — only the hex values differ
- The TOML format is defined in the spec (lines 23-63)
- Token name mapping: `C_HEADING` → `heading`, `C_HDR_KEY` → `hdr_key`, etc.
- Modifiers: `"$COLOR_SUCCESS bold"` → `{ color = "color_success", bold = true }`

- [ ] **Step 1: Create `nord.toml`**

```toml
name = "Nord"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"

[tokens]
heading = { color = "color_success", bold = true }
bold = { bold = true }
italic = { italic = true }
link_text = { color = "accent_primary", underline = true }
link_url = { color = "fg_dim" }
rule = { color = "fg_dim" }
hdr_key = { color = "accent_primary", bold = true }
hdr_value = { color = "fg_base" }
hdr_dim = { color = "fg_dim" }
picker_num = { color = "accent_primary" }
picker_label = { color = "fg_base" }
picker_url = { color = "fg_dim" }
picker_sel_bg = { color = "bg_selection" }
picker_sel_fg = { color = "fg_bright" }
msg_marker = { color = "fg_dim", bold = true }
msg_title_success = { color = "color_success", bold = true }
msg_title_accent = { color = "accent_primary", bold = true }
msg_detail = { color = "fg_base" }
msg_dim = { color = "fg_dim" }
```

- [ ] **Step 2: Create `solarized-dark.toml`**

```toml
name = "Solarized Dark"

[colors]
bg_base = "#002b36"
bg_elevated = "#073642"
bg_selection = "#073642"
bg_border = "#586e75"
fg_base = "#839496"
fg_bright = "#93a1a1"
fg_brightest = "#eee8d5"
fg_dim = "#657b83"
accent_primary = "#268bd2"
accent_secondary = "#2aa198"
accent_tertiary = "#2aa198"
color_error = "#dc322f"
color_warning = "#cb4b16"
color_success = "#859900"
color_info = "#b58900"
color_special = "#6c71c4"

[tokens]
heading = { color = "color_success", bold = true }
bold = { bold = true }
italic = { italic = true }
link_text = { color = "accent_secondary" }
link_url = { color = "fg_dim" }
rule = { color = "fg_dim" }
hdr_key = { color = "accent_primary", bold = true }
hdr_value = { color = "fg_base" }
hdr_dim = { color = "fg_dim" }
picker_num = { color = "accent_primary" }
picker_label = { color = "fg_base" }
picker_url = { color = "fg_dim" }
picker_sel_bg = { color = "bg_selection" }
picker_sel_fg = { color = "fg_bright" }
msg_marker = { color = "fg_dim", bold = true }
msg_title_success = { color = "color_success", bold = true }
msg_title_accent = { color = "accent_primary", bold = true }
msg_detail = { color = "fg_base" }
msg_dim = { color = "fg_dim" }
```

- [ ] **Step 3: Create `gruvbox-dark.toml`**

```toml
name = "Gruvbox Dark"

[colors]
bg_base = "#282828"
bg_elevated = "#3c3836"
bg_selection = "#3c3836"
bg_border = "#665c54"
fg_base = "#ebdbb2"
fg_bright = "#fbf1c7"
fg_brightest = "#fbf1c7"
fg_dim = "#928374"
accent_primary = "#83a598"
accent_secondary = "#8ec07c"
accent_tertiary = "#8ec07c"
color_error = "#fb4934"
color_warning = "#fe8019"
color_success = "#b8bb26"
color_info = "#fabd2f"
color_special = "#d3869b"

[tokens]
heading = { color = "color_success", bold = true }
bold = { bold = true }
italic = { italic = true }
link_text = { color = "accent_secondary" }
link_url = { color = "fg_dim" }
rule = { color = "fg_dim" }
hdr_key = { color = "accent_primary", bold = true }
hdr_value = { color = "fg_base" }
hdr_dim = { color = "fg_dim" }
picker_num = { color = "accent_primary" }
picker_label = { color = "fg_base" }
picker_url = { color = "fg_dim" }
picker_sel_bg = { color = "bg_selection" }
picker_sel_fg = { color = "fg_bright" }
msg_marker = { color = "fg_dim", bold = true }
msg_title_success = { color = "color_success", bold = true }
msg_title_accent = { color = "accent_primary", bold = true }
msg_detail = { color = "fg_base" }
msg_dim = { color = "fg_dim" }
```

- [ ] **Step 4: Verify the TOML files load correctly**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./internal/theme/ -run TestLoad -v -count=1`
Expected: PASS (uses the `validTheme` constant, same structure).

Write a quick one-off test or script to verify the actual theme files parse:

```bash
cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep
go test ./internal/theme/ -run TestLoadRealThemes -v -count=1
```

Add to `theme_test.go`:

```go
func TestLoadRealThemes(t *testing.T) {
	themes := []string{
		"../../.config/aerc/themes/nord.toml",
		"../../.config/aerc/themes/solarized-dark.toml",
		"../../.config/aerc/themes/gruvbox-dark.toml",
	}
	for _, path := range themes {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			th, err := Load(path)
			if err != nil {
				t.Fatalf("Load(%s): %v", path, err)
			}
			if th.Name == "" {
				t.Error("theme name is empty")
			}
			// Verify all expected tokens resolve
			for _, tok := range []string{"heading", "bold", "hdr_key", "picker_num", "msg_dim"} {
				if th.ANSI(tok) == "" {
					t.Errorf("token %q resolved to empty", tok)
				}
			}
		})
	}
}
```

- [ ] **Step 5: Delete the old shell theme files**

```bash
git rm .config/aerc/themes/nord.sh .config/aerc/themes/solarized-dark.sh .config/aerc/themes/gruvbox-dark.sh
```

- [ ] **Step 6: Commit**

```bash
git add .config/aerc/themes/nord.toml .config/aerc/themes/solarized-dark.toml .config/aerc/themes/gruvbox-dark.toml internal/theme/theme_test.go
git commit -m "Convert theme files from shell to TOML format"
```

---

### Task 4: Add styleset generation

The `internal/theme/styleset.go` file contains the styleset template and a function to generate the aerc styleset from a loaded theme. The template is a Go string constant reproducing the exact output the shell script produces today.

**Files:**
- Create: `internal/theme/styleset.go`
- Create: `internal/theme/styleset_test.go`

**Context:**
- The current styleset output can be seen at `~/.config/aerc/stylesets/nord` — 80+ lines of `key.property=value` in aerc INI format
- The shell script writes the styleset by cat-ing a heredoc with `$VAR` substitutions
- The full template is at `.config/aerc/themes/generate:150-266`
- The Go version uses `text/template` with the theme's `Color()` method to substitute hex values
- The output must be byte-identical to what the shell script produces (minus the override section)

- [ ] **Step 1: Write the failing test**

```go
package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateStyleset(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	output, err := GenerateStyleset(th)
	if err != nil {
		t.Fatalf("GenerateStyleset: %v", err)
	}

	// Verify key structural elements
	checks := []struct {
		name string
		want string
	}{
		{"title bg", "title.bg=#81a1c1"},
		{"title fg", "title.fg=#2e3440"},
		{"error", "error.fg=#bf616a"},
		{"warning", "warning.fg=#d08770"},
		{"success", "success.fg=#a3be8c"},
		{"msglist unread", "msglist_unread.fg=#8fbcbb"},
		{"tab selected", "tab.selected.bg=#88c0d0"},
		{"selection", "*.selected.bg=#394353"},
		{"quote 1", "quote_1.fg=#8fbcbb"},
		{"diff add", "diff_add.fg=#a3be8c"},
		{"diff del", "diff_del.fg=#bf616a"},
		{"border", "border.fg=#49576b"},
		{"ui section", "[ui]"},
		{"viewer section", "[viewer]"},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(output, c.want) {
				t.Errorf("output missing %q", c.want)
			}
		})
	}
}

func TestGenerateStylesetWriteFile(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "testtheme")
	if err := WriteStyleset(th, outPath); err != nil {
		t.Fatalf("WriteStyleset: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if !strings.Contains(string(data), "title.bg=#81a1c1") {
		t.Error("written file missing expected content")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./internal/theme/ -run 'Styleset' -v -count=1`
Expected: FAIL — `GenerateStyleset` not defined.

- [ ] **Step 3: Write the implementation**

```go
package theme

import (
	"fmt"
	"os"
	"strings"
	"text/template"
)

const stylesetTemplate = `#
# aerc styleset -- auto-generated from {{.Name}} theme
# Re-generate with: mailrender themes generate
#

[ui]

*.default=true
*.normal=true

title.fg={{.C "bg_base"}}
title.bg={{.C "accent_primary"}}
title.bold=true
header.bold=true

*error.bold=true
error.fg={{.C "color_error"}}
warning.fg={{.C "color_warning"}}
success.fg={{.C "color_success"}}

statusline*.default=true
statusline_default.fg={{.C "bg_border"}}
statusline_default.reverse=true
statusline_error.fg={{.C "color_error"}}
statusline_error.reverse=true
statusline_success.fg={{.C "color_success"}}
statusline_success.reverse=true

completion_default.fg={{.C "fg_base"}}
completion_default.bg={{.C "bg_elevated"}}
completion_gutter.bg={{.C "bg_elevated"}}
completion_pill.fg={{.C "bg_base"}}
completion_pill.bg={{.C "accent_primary"}}
completion_description.fg={{.C "fg_dim"}}
completion_description.dim=true

border.fg={{.C "bg_border"}}

spinner.fg={{.C "accent_primary"}}

stack.fg={{.C "fg_base"}}

selector_default.fg={{.C "fg_base"}}
selector_default.bg={{.C "bg_base"}}
selector_focused.fg={{.C "bg_base"}}
selector_focused.bg={{.C "accent_primary"}}
selector_focused.bold=true
selector_chooser.fg={{.C "fg_base"}}
selector_chooser.bold=true

*.selected.bg={{.C "bg_selection"}}

msglist_default.fg={{.C "fg_base"}}
msglist_read.fg={{.C "fg_dim"}}
msglist_unread.fg={{.C "accent_tertiary"}}
msglist_unread.bold=true
msglist_flagged.fg={{.C "color_warning"}}
msglist_flagged.bold=true
msglist_answered.fg={{.C "color_special"}}
msglist_forwarded.fg={{.C "color_special"}}
msglist_forwarded.dim=true
msglist_deleted.fg={{.C "fg_dim"}}
msglist_deleted.dim=true
msglist_marked.bg={{.C "accent_primary"}}
msglist_marked.fg={{.C "bg_base"}}
msglist_result.fg={{.C "color_info"}}
msglist_gutter.fg={{.C "bg_border"}}
msglist_pill.fg={{.C "bg_base"}}
msglist_pill.bg={{.C "accent_primary"}}
msglist_thread_folded.fg={{.C "color_warning"}}
msglist_thread_context.fg={{.C "fg_dim"}}
msglist_thread_context.dim=true
msglist_thread_orphan.fg={{.C "fg_dim"}}

tab.fg={{.C "fg_bright"}}
tab.bg={{.C "bg_border"}}
tab.selected.bg={{.C "accent_secondary"}}
tab.selected.fg={{.C "bg_base"}}

dirlist_default.fg={{.C "fg_base"}}
dirlist_unread.fg={{.C "accent_secondary"}}
dirlist_recent.fg={{.C "accent_secondary"}}

part_*.fg={{.C "fg_brightest"}}
part_mimetype.fg={{.C "bg_selection"}}
part_*.selected.fg={{.C "fg_brightest"}}
part_filename.selected.bold=true

[viewer]
*.default=true
*.normal=true

header.fg={{.C "accent_primary"}}
header.bold=true

quote_1.fg={{.C "accent_tertiary"}}
quote_1.dim=false
quote_2.fg={{.C "fg_dim"}}
quote_2.dim=false
quote_3.fg={{.C "fg_dim"}}
quote_3.dim=true
quote_4.fg={{.C "fg_dim"}}
quote_4.dim=true
quote_x.fg={{.C "fg_dim"}}
quote_x.dim=true

signature.fg={{.C "fg_base"}}
signature.dim=true

url.fg={{.C "accent_tertiary"}}
url.underline=true

diff_meta.bold=true
diff_chunk.dim=true
diff_add.fg={{.C "color_success"}}
diff_del.fg={{.C "color_error"}}
`

// stylesetData wraps a Theme for template execution.
type stylesetData struct {
	Name  string
	theme *Theme
}

// C returns the hex color for a slot name. Used in the template as {{.C "name"}}.
func (d stylesetData) C(name string) string {
	return d.theme.Color(name)
}

var stylesetTmpl = template.Must(template.New("styleset").Parse(stylesetTemplate))

// GenerateStyleset renders the styleset template with the theme's colors.
func GenerateStyleset(t *Theme) (string, error) {
	var buf strings.Builder
	data := stylesetData{Name: t.Name, theme: t}
	if err := stylesetTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing styleset template: %w", err)
	}
	return buf.String(), nil
}

// WriteStyleset generates and writes the styleset to a file.
func WriteStyleset(t *Theme, path string) error {
	content, err := GenerateStyleset(t)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./internal/theme/ -v -count=1`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/theme/styleset.go internal/theme/styleset_test.go
git commit -m "Add styleset generation from TOML theme"
```

---

### Task 5: Add `mailrender themes generate` cobra subcommand

Wire the styleset generation into the `mailrender` CLI as a `themes generate` subcommand. This replaces the shell script `themes/generate`.

**Files:**
- Create: `cmd/mailrender/themes.go`
- Modify: `cmd/mailrender/root.go`

**Context:**
- `cmd/mailrender/root.go` currently adds `headers`, `html`, `plain` commands
- The new `themes` command has one subcommand: `generate [theme-name]`
- If `theme-name` is provided, loads `.config/aerc/themes/<name>.toml` relative to `aerc.conf` directory
- If omitted, uses `theme.FindPath()` to read `styleset-name` from `aerc.conf`
- Writes `stylesets/<name>` in the same directory
- Prints a summary: `Theme: nord.toml\nStyleset: stylesets/nord`

- [ ] **Step 1: Write the `themes generate` command**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/spf13/cobra"
)

func newThemesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "themes",
		Short: "Theme management commands",
	}
	cmd.AddCommand(newThemesGenerateCmd())
	return cmd
}

func newThemesGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [theme-name]",
		Short: "Generate aerc styleset from a TOML theme file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var themePath string
			var err error

			if len(args) == 1 {
				confDir, findErr := findAercConfigDir()
				if findErr != nil {
					return findErr
				}
				themePath = filepath.Join(confDir, "themes", args[0]+".toml")
			} else {
				themePath, err = theme.FindPath()
				if err != nil {
					return err
				}
			}

			th, err := theme.Load(themePath)
			if err != nil {
				return err
			}

			// Write styleset next to themes/ directory (i.e., in aerc config dir)
			themeDir := filepath.Dir(themePath)
			confDir := filepath.Dir(themeDir)
			stylesetDir := filepath.Join(confDir, "stylesets")
			if err := os.MkdirAll(stylesetDir, 0755); err != nil {
				return fmt.Errorf("creating stylesets directory: %w", err)
			}

			outPath := filepath.Join(stylesetDir, th.Name)
			if err := theme.WriteStyleset(th, outPath); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Theme:    %s\n", filepath.Base(themePath))
			fmt.Fprintf(os.Stderr, "Styleset: stylesets/%s\n", th.Name)
			return nil
		},
	}
	return cmd
}

// findAercConfigDir is a thin wrapper so the cmd package can locate
// the config directory for explicit theme-name resolution.
func findAercConfigDir() (string, error) {
	if dir := os.Getenv("AERC_CONFIG"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "aerc.conf")); err == nil {
			return dir, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", "aerc")
	if _, err := os.Stat(filepath.Join(dir, "aerc.conf")); err == nil {
		return dir, nil
	}
	return "", fmt.Errorf("aerc.conf not found (checked $AERC_CONFIG and ~/.config/aerc/)")
}
```

Wait — `findAercConfigDir` duplicates the unexported function in `internal/theme/theme.go`. Export it instead.

Revised approach: export `FindConfigDir` from `internal/theme`:

Add to `internal/theme/theme.go` (rename `findAercConfigDir` to `FindConfigDir` and export it):

```go
// FindConfigDir returns the directory containing aerc.conf.
func FindConfigDir() (string, error) {
	// ... same implementation as findAercConfigDir, just exported
}
```

Then `cmd/mailrender/themes.go` uses `theme.FindConfigDir()` instead of a local copy. Update `FindPath` to call `FindConfigDir` internally.

The `themes.go` file becomes:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/spf13/cobra"
)

func newThemesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "themes",
		Short: "Theme management commands",
	}
	cmd.AddCommand(newThemesGenerateCmd())
	return cmd
}

func newThemesGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [theme-name]",
		Short: "Generate aerc styleset from a TOML theme file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var themePath string
			var err error

			if len(args) == 1 {
				confDir, findErr := theme.FindConfigDir()
				if findErr != nil {
					return findErr
				}
				themePath = filepath.Join(confDir, "themes", args[0]+".toml")
			} else {
				themePath, err = theme.FindPath()
				if err != nil {
					return err
				}
			}

			th, err := theme.Load(themePath)
			if err != nil {
				return err
			}

			themeDir := filepath.Dir(themePath)
			confDir := filepath.Dir(themeDir)
			stylesetDir := filepath.Join(confDir, "stylesets")
			if err := os.MkdirAll(stylesetDir, 0755); err != nil {
				return fmt.Errorf("creating stylesets directory: %w", err)
			}

			outPath := filepath.Join(stylesetDir, th.Name)
			if err := theme.WriteStyleset(th, outPath); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Theme:    %s\n", filepath.Base(themePath))
			fmt.Fprintf(os.Stderr, "Styleset: stylesets/%s\n", th.Name)
			return nil
		},
	}
	return cmd
}
```

- [ ] **Step 2: Add the themes command to root**

Modify `cmd/mailrender/root.go` — add `cmd.AddCommand(newThemesCmd())`:

```go
func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "mailrender",
		Short:        "Themeable message rendering filters for the aerc email client",
		SilenceUsage: true,
	}
	cmd.AddCommand(newHeadersCmd())
	cmd.AddCommand(newHTMLCmd())
	cmd.AddCommand(newPlainCmd())
	cmd.AddCommand(newThemesCmd())
	return cmd
}
```

- [ ] **Step 3: Export `FindConfigDir` in theme package**

In `internal/theme/theme.go`, rename `findAercConfigDir` to `FindConfigDir` and export it. Update `FindPath` to call `FindConfigDir`:

```go
// FindConfigDir returns the directory containing aerc.conf.
func FindConfigDir() (string, error) {
	if dir := os.Getenv("AERC_CONFIG"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "aerc.conf")); err == nil {
			return dir, nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", "aerc")
	if _, err := os.Stat(filepath.Join(dir, "aerc.conf")); err == nil {
		return dir, nil
	}

	return "", fmt.Errorf("aerc.conf not found (checked $AERC_CONFIG and ~/.config/aerc/)")
}

func FindPath() (string, error) {
	confDir, err := FindConfigDir()
	if err != nil {
		return "", err
	}
	// ... rest unchanged
}
```

- [ ] **Step 4: Build and verify**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go build ./cmd/mailrender/`
Expected: builds without errors.

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go vet ./cmd/mailrender/`
Expected: no issues.

- [ ] **Step 5: Commit**

```bash
git add cmd/mailrender/themes.go cmd/mailrender/root.go internal/theme/theme.go
git commit -m "Add mailrender themes generate subcommand"
```

---

### Task 6: Migrate callers from `internal/palette` to `internal/theme`

Update all four caller files to import `internal/theme` instead of `internal/palette`, and update API calls to use the new names. The `loadPalette()` function in both `cmd/mailrender/headers.go` and `cmd/pick-link/root.go` becomes `loadTheme()`. Token names drop the `C_` prefix and use lowercase.

**Files:**
- Modify: `cmd/mailrender/headers.go`
- Modify: `cmd/mailrender/html.go`
- Modify: `cmd/mailrender/plain.go`
- Modify: `cmd/pick-link/root.go`
- Modify: `internal/filter/html.go`
- Modify: `internal/filter/plain.go`
- Modify: `internal/picker/picker.go`

**Context:**
- `cmd/mailrender/headers.go:30-47` — `loadPalette()` uses `palette.FindPath(genDir)` + `palette.Load(path)`. Replace with `theme.FindPath()` + `theme.Load(path)`. The `genDir` logic (navigate from binary location) is no longer needed — `FindPath` reads `aerc.conf` directly.
- `cmd/mailrender/headers.go:50-57` — `colorsFromPalette(p)` uses `p.ANSI("C_HDR_KEY")` etc. Change to `t.ANSI("hdr_key")`.
- `cmd/mailrender/html.go:14-23` — calls `loadPalette()`. Change to `loadTheme()`.
- `cmd/mailrender/plain.go:14-23` — calls `loadPalette()`. Change to `loadTheme()`.
- `cmd/pick-link/root.go:57-73` — duplicate `loadPalette()`. Same replacement.
- `internal/filter/html.go:466` — `HTML` function signature takes `*palette.Palette`. Change to `*theme.Theme`.
- `internal/filter/html.go:472-488` — uses `p.Get("C_LINK_TEXT")`, `p.Get("FG_DIM")`, etc. Change to `t.ANSI(...)`.
- `internal/filter/plain.go:31` — `Plain` function signature takes `*palette.Palette`. Change to `*theme.Theme`.
- `internal/picker/picker.go:63-79` — `ColorsFromPalette(p *palette.Palette)`. Rename to `ColorsFromTheme(t *theme.Theme)`, update token names.

- [ ] **Step 1: Update `internal/filter/html.go`**

Change the import from `palette` to `theme`:

```go
import (
	// ... other imports
	"github.com/glw907/beautiful-aerc/internal/theme"
)
```

Change the `HTML` function signature:

```go
func HTML(r io.Reader, w io.Writer, t *theme.Theme, cols int) error {
```

Change the body (lines 472-488):

```go
	fc := &footnoteColors{
		LinkText: t.ANSI("link_text"),
		Dim:      t.ANSI("msg_dim"),
		LinkURL:  t.ANSI("link_url"),
		Reset:    "\033[0m",
	}
	styled := styleFootnotes(body, refs, cols, fc)

	mc := &markdownColors{
		Heading: t.ANSI("heading"),
		Bold:    t.ANSI("bold"),
		Italic:  t.ANSI("italic"),
		Rule:    t.ANSI("rule"),
		Reset:   "\033[0m",
	}
```

**Important:** The old code stored raw SGR params in `footnoteColors`/`markdownColors` fields and wrapped them in `\033[..m` at point of use. Check if the downstream code wraps them or uses them raw. Read `internal/filter/footnotes.go` and the markdown highlighter to verify.

Actually, looking more carefully at the existing code: `p.Get("C_LINK_TEXT")` returns the raw SGR param string like `"38;2;129;161;193;4"` — NOT wrapped in escape codes. The `footnoteColors` and `markdownColors` structs store these raw params and the styling functions wrap them when building output. But `t.ANSI("link_text")` returns `"\033[38;2;129;161;193;4m"` — already wrapped.

This means either:
1. The caller code that wraps these values needs to change (stop wrapping), or
2. We add a `Raw()` method that returns the unwrapped SGR param string.

The cleanest fix: add a `Raw(name string) string` method to `Theme` that returns the resolved SGR params without wrapping. The callers that need raw params use `Raw()`, callers that need the full escape use `ANSI()`.

Add to `internal/theme/theme.go`:

```go
// Raw returns the resolved SGR parameter string for a token (no escape wrapping).
func (t *Theme) Raw(name string) string {
	return t.tokens[name]
}
```

Then the html.go migration uses `t.Raw(...)` for the color structs:

```go
	fc := &footnoteColors{
		LinkText: t.Raw("link_text"),
		Dim:      t.Raw("msg_dim"),
		LinkURL:  t.Raw("link_url"),
		Reset:    "0",
	}

	mc := &markdownColors{
		Heading: t.Raw("heading"),
		Bold:    t.Raw("bold"),
		Italic:  t.Raw("italic"),
		Rule:    t.Raw("rule"),
		Reset:   "0",
	}
```

And the picker/headers code that already used `p.ANSI(...)` (which returned wrapped sequences) continues to use `t.ANSI(...)`.

Similarly, check `internal/picker/picker.go:72-77` — `p.Get("C_PICKER_SEL_BG")` returns raw SGR params. The picker manually constructs `"\033[" + bgParam + "m"`. So picker needs `t.Raw(...)` for sel_bg/sel_fg.

- [ ] **Step 2: Update `internal/filter/plain.go`**

Change import and function signature:

```go
import (
	// ... other imports
	"github.com/glw907/beautiful-aerc/internal/theme"
)

func Plain(r io.Reader, w io.Writer, t *theme.Theme, cols int) error {
	// ... body unchanged except: HTML(strings.NewReader(text), w, p, cols)
	// becomes: HTML(strings.NewReader(text), w, t, cols)
```

Remove the `palette` import.

- [ ] **Step 3: Update `internal/picker/picker.go`**

Change import:

```go
import (
	// ... other imports
	"github.com/glw907/beautiful-aerc/internal/theme"
)
```

Rename function and update token names:

```go
func ColorsFromTheme(t *theme.Theme) *Colors {
	c := &Colors{
		Number: t.ANSI("picker_num"),
		Label:  t.ANSI("picker_label"),
		URL:    t.ANSI("picker_url"),
		Marker: t.ANSI("msg_marker"),
		Title:  t.ANSI("msg_title_accent"),
		Reset:  t.Reset(),
	}
	selBG := t.Raw("picker_sel_bg")
	selFG := t.Raw("picker_sel_fg")
	if selBG != "" && selFG != "" {
		bgParam := strings.Replace(selBG, "38;2;", "48;2;", 1)
		c.Selected = "\033[" + bgParam + "m\033[" + selFG + "m"
	}
	return c
}
```

Remove the `palette` import.

- [ ] **Step 4: Update `cmd/mailrender/headers.go`**

Replace `loadPalette` with `loadTheme`:

```go
package main

import (
	"os"
	"strconv"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/spf13/cobra"
)

func newHeadersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "headers",
		Short: "Format and colorize email headers",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := loadTheme()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.Headers(os.Stdin, os.Stdout, colorsFromTheme(t), cols)
		},
	}
	return cmd
}

func loadTheme() (*theme.Theme, error) {
	path, err := theme.FindPath()
	if err != nil {
		return nil, err
	}
	return theme.Load(path)
}

func colorsFromTheme(t *theme.Theme) *filter.ColorSet {
	return &filter.ColorSet{
		HdrKey: t.ANSI("hdr_key"),
		HdrFG:  t.ANSI("hdr_value"),
		HdrDim: t.ANSI("hdr_dim"),
		Reset:  t.Reset(),
	}
}
```

Remove `"path/filepath"` from imports (no longer needed — no genDir logic).

- [ ] **Step 5: Update `cmd/mailrender/html.go`**

```go
func newHTMLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "html",
		Short: "Convert HTML email to styled markdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := loadTheme()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.HTML(os.Stdin, os.Stdout, t, cols)
		},
	}
	return cmd
}
```

Remove `palette` import if present (it's not — html.go only imports `filter`).

- [ ] **Step 6: Update `cmd/mailrender/plain.go`**

Same pattern as html.go:

```go
func newPlainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plain",
		Short: "Format plain text email (reflow and colorize)",
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := loadTheme()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.Plain(os.Stdin, os.Stdout, t, cols)
		},
	}
	return cmd
}
```

- [ ] **Step 7: Update `cmd/pick-link/root.go`**

Replace `loadPalette` with `loadTheme`:

```go
package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/picker"
	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "pick-link",
		Short:        "Interactive URL picker for aerc email messages",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := loadTheme()
			if err != nil {
				return err
			}

			cols := termCols()
			links, err := filter.HTMLLinks(os.Stdin, cols)
			if err != nil {
				return err
			}

			colors := picker.ColorsFromTheme(t)
			url, err := picker.Run(links, cols, colors)
			if err != nil {
				return err
			}
			if url != "" {
				name := "xdg-open"
				if strings.HasPrefix(url, "mailto:") {
					name = "aerc"
				}
				open := exec.Command(name, url)
				open.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
				return open.Start()
			}
			return nil
		},
	}
	return cmd
}

func loadTheme() (*theme.Theme, error) {
	path, err := theme.FindPath()
	if err != nil {
		return nil, err
	}
	return theme.Load(path)
}
```

Remove `"path/filepath"` and `palette` imports.

- [ ] **Step 8: Add `Raw` method and test**

Add to `internal/theme/theme.go`:

```go
// Raw returns the resolved SGR parameter string for a token without escape wrapping.
func (t *Theme) Raw(name string) string {
	return t.tokens[name]
}
```

Add to `internal/theme/theme_test.go`:

```go
func TestRaw(t *testing.T) {
	path := writeTheme(t, validTheme)
	th, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// hdr_key = accent_primary (#81a1c1) + bold
	got := th.Raw("hdr_key")
	want := "38;2;129;161;193;1"
	if got != want {
		t.Errorf("Raw(hdr_key) = %q, want %q", got, want)
	}
}
```

- [ ] **Step 9: Build both binaries**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go build ./cmd/mailrender/ && go build ./cmd/pick-link/`
Expected: both build without errors.

- [ ] **Step 10: Run all tests**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./internal/theme/ ./internal/filter/ ./internal/picker/ -v -count=1`
Expected: All PASS. (Unit tests for filter and picker may need adjustment if they directly reference palette — check.)

- [ ] **Step 11: Commit**

```bash
git add cmd/mailrender/headers.go cmd/mailrender/html.go cmd/mailrender/plain.go cmd/pick-link/root.go internal/filter/html.go internal/filter/plain.go internal/picker/picker.go internal/theme/theme.go internal/theme/theme_test.go
git commit -m "Migrate all callers from internal/palette to internal/theme"
```

---

### Task 7: Update E2E tests for TOML theme format

The E2E test setup in `e2e/e2e_test.go` currently creates a `palette.sh` file for the test binary. Replace it with a TOML theme file and a minimal `aerc.conf` so the binary can discover the theme via `FindPath`.

**Files:**
- Modify: `e2e/e2e_test.go`

**Context:**
- The current test setup at `e2e/e2e_test.go:35-79` creates `generated/palette.sh` with pre-resolved ANSI values
- The new setup needs: `aerc.conf` with `styleset-name=test`, and `themes/test.toml` with the same hex values
- The resolved ANSI values must match what the test palette had to keep golden files stable
- **Critical:** The e2e test palette uses `C_LINK_TEXT="38;2;136;192;208"` (ACCENT_SECONDARY, no underline) while the real nord theme has `C_LINK_TEXT="$ACCENT_PRIMARY underline"`. The TOML theme for tests must use the same values as the e2e test palette, not the real nord theme. In TOML: `link_text = { color = "accent_secondary" }` (no underline).
- Similarly: `C_HEADING="1;38;2;163;190;140"` in the test has bold BEFORE color (order `1;38;2;...`) while the new resolver puts color first then bold (`38;2;...;1`). Check if order matters for ANSI SGR params.

**ANSI SGR parameter order:** SGR parameters are order-independent. `\033[1;38;2;163;190;140m` and `\033[38;2;163;190;140;1m` produce identical rendering. However, the golden files contain the literal escape sequences, so if the byte sequence changes, golden files need regeneration.

The resolution: the new resolver always puts color params first, then modifiers. The golden files were generated with the old palette (bold first for heading). So **the golden files will need to be regenerated** after this migration. Use `-update-golden` flag.

- [ ] **Step 1: Replace palette setup with TOML theme**

Replace `e2e/e2e_test.go` TestMain setup. Remove the `paletteDir` variable and palette creation. Add:

```go
var (
	binary    string
	configDir string
	updateGolden = flag.Bool("update-golden", false, "regenerate golden files")
)

func TestMain(m *testing.M) {
	flag.Parse()

	// Build the binary once
	tmp, err := os.MkdirTemp("", "mailrender-test")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(tmp, "mailrender")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/mailrender")
	cmd.Dir = filepath.Join("..")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("build failed: " + err.Error())
	}

	// Create test config directory with theme + aerc.conf
	configDir, err = os.MkdirTemp("", "mailrender-config")
	if err != nil {
		panic(err)
	}

	// Write aerc.conf
	os.WriteFile(filepath.Join(configDir, "aerc.conf"), []byte("[ui]\nstyleset-name=test\n"), 0644)

	// Write TOML theme
	themesDir := filepath.Join(configDir, "themes")
	os.MkdirAll(themesDir, 0755)
	themeContent := `name = "test"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"

[tokens]
heading = { color = "color_success", bold = true }
bold = { bold = true }
italic = { italic = true }
link_text = { color = "accent_secondary" }
link_url = { color = "fg_dim" }
rule = { color = "fg_dim" }
hdr_key = { color = "accent_primary", bold = true }
hdr_value = { color = "fg_base" }
hdr_dim = { color = "fg_dim" }
picker_num = { color = "accent_primary" }
picker_label = { color = "fg_base" }
picker_url = { color = "fg_dim" }
picker_sel_bg = { color = "bg_selection" }
picker_sel_fg = { color = "fg_bright" }
msg_marker = { color = "fg_dim", bold = true }
msg_title_success = { color = "color_success", bold = true }
msg_title_accent = { color = "accent_primary", bold = true }
msg_detail = { color = "fg_base" }
msg_dim = { color = "fg_dim" }
`
	os.WriteFile(filepath.Join(themesDir, "test.toml"), []byte(themeContent), 0644)

	// Copy unwrap-tables.lua into the test config dir
	filterDir := filepath.Join(configDir, "filters")
	os.MkdirAll(filterDir, 0755)
	luaSrc := filepath.Join("..", ".config", "aerc", "filters", "unwrap-tables.lua")
	luaData, err := os.ReadFile(luaSrc)
	if err != nil {
		panic("reading unwrap-tables.lua: " + err.Error())
	}
	os.WriteFile(filepath.Join(filterDir, "unwrap-tables.lua"), luaData, 0644)

	code := m.Run()
	os.RemoveAll(tmp)
	os.RemoveAll(configDir)
	os.Exit(code)
}
```

- [ ] **Step 2: Update env var references**

Replace all `"AERC_CONFIG="+paletteDir` with `"AERC_CONFIG="+configDir` in the test functions.

- [ ] **Step 3: Regenerate golden files**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./e2e/ -update-golden -count=1`
Expected: golden files updated. Verify the output looks correct (same structure, different ANSI byte order for heading bold).

- [ ] **Step 4: Run E2E tests without update flag**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./e2e/ -v -count=1`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add e2e/e2e_test.go e2e/testdata/golden/
git commit -m "Update E2E tests for TOML theme format"
```

---

### Task 8: Delete `internal/palette` and `themes/generate`

Remove all the old infrastructure. By this point, no code references `internal/palette` or `palette.sh`.

**Files:**
- Delete: `internal/palette/palette.go`
- Delete: `internal/palette/palette_test.go`
- Delete: `.config/aerc/themes/generate`
- Delete: `.config/aerc/generated/` directory (including `palette.sh` if present in repo)

**Context:**
- Verify no remaining references before deleting: `grep -r "internal/palette" --include="*.go" .` should return nothing
- The `generated/` directory may not be in the repo (it's gitignored or only exists on the live system). Check with `git ls-files .config/aerc/generated/`
- The live `~/.config/aerc/generated/palette.sh` is a runtime file on the user's system — not deleted by this task (user removes it manually or it sits harmlessly)

- [ ] **Step 1: Verify no remaining palette references**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && grep -r "internal/palette" --include="*.go" .`
Expected: no output.

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && grep -r "palette\." --include="*.go" . | grep -v "_test.go" | grep -v theme`
Expected: no output (or only test files / theme package references to "palette" as a word).

- [ ] **Step 2: Delete the files**

```bash
cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep
git rm internal/palette/palette.go internal/palette/palette_test.go
git rm .config/aerc/themes/generate
# Check if generated/ is tracked
git ls-files .config/aerc/generated/
# If any files listed, git rm them too
```

- [ ] **Step 3: Run full test suite**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go test ./... -count=1`
Expected: All PASS. No compilation errors from missing palette package.

- [ ] **Step 4: Build both binaries**

Run: `cd /home/glw907/Projects/beautiful-aerc/.claude/worktrees/compose-prep && go build ./cmd/mailrender/ && go build ./cmd/pick-link/`
Expected: both build cleanly.

- [ ] **Step 5: Commit**

```bash
git commit -m "Delete internal/palette and themes/generate shell script"
```

---

### Task 9: Update documentation

Update CLAUDE.md, README.md, docs/themes.md, and aerc-setup.md to reflect the new TOML theme system.

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`
- Modify: `docs/themes.md`
- Modify: `~/.claude/docs/aerc-setup.md`

**Context:**
- CLAUDE.md: Update "Theme System" section — replace references to `themes/generate` shell script with `mailrender themes generate`, remove `generated/palette.sh` references, update "Theme System" to describe TOML format
- README.md: Update "Generate a theme" section to show new command, remove `cd .config/aerc` + `themes/generate` workflow
- docs/themes.md: Major rewrite — TOML format, new command, remove override mechanism section, remove `palette.sh` references
- aerc-setup.md: Update architecture diagram, generator section, palette section, override section

- [ ] **Step 1: Update `CLAUDE.md`**

Key changes:
- Theme System section: "Theme files (`.config/aerc/themes/*.toml`) define 16 semantic hex color slots + TOML token definitions..."
- Remove: "The generator (`themes/generate`) reads a theme file and produces `generated/palette.sh`..."
- Add: "Go binaries read `.toml` theme files directly at runtime. The active theme is determined by `styleset-name` in `aerc.conf`."
- Remove: "palette.sh" from all references
- Update: "The Go binary reads `palette.sh` at runtime" → "The Go binary reads the theme TOML file at runtime"
- Add `mailrender themes generate` to the mailrender command structure

- [ ] **Step 2: Update `README.md`**

Change the "Generate a theme" section:

```markdown
**3. Generate a styleset**

Pick one of the three built-in themes and generate the aerc styleset:

```sh
mailrender themes generate nord
```

Then set in `aerc.conf`:

```ini
styleset-name=nord
```
```

- [ ] **Step 3: Rewrite `docs/themes.md`**

Full rewrite for TOML format. Remove override mechanism section. Update all code examples from shell to TOML. Update the generator command from `themes/generate themes/nord.sh` to `mailrender themes generate [theme-name]`. Remove `palette.sh` references.

- [ ] **Step 4: Update `~/.claude/docs/aerc-setup.md`**

Update the architecture diagram, generator section, and palette references. Replace the ASCII art showing `themes/generate` → `palette.sh` + `stylesets/` with the new flow: `themes/*.toml` → Go binary reads directly + `mailrender themes generate` → `stylesets/`.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md README.md docs/themes.md
git commit -m "Update documentation for TOML theme system"
```

Note: `~/.claude/docs/aerc-setup.md` is outside the repo — update it separately (not committed here).
