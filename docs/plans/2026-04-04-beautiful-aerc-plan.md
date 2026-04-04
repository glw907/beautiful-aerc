# beautiful-aerc Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a themeable, distributable aerc email setup with a Go filter binary, multiple color themes, and optional nvim-mail/kitty integration.

**Architecture:** Single Go binary (`beautiful-aerc`) with three cobra subcommands (`headers`, `html`, `plain`) that replace the current shell/AWK/perl filter scripts. A shell-based theme generator produces a palette file consumed by the Go binary at runtime and an aerc styleset. Everything installs as one GNU Stow package.

**Tech Stack:** Go 1.23 (cobra), POSIX shell (generator), pandoc (HTML conversion), aerc built-ins (colorize, wrap)

**Reference:** Design spec at `docs/2026-04-04-beautiful-aerc-design.md`

**Source filters to port:** The current shell filters live at `~/.config/aerc/filters/`. Read these files for the exact regex patterns, header ordering logic, and pipeline behavior that the Go code must replicate:
- `~/.config/aerc/filters/format-headers.awk` - header formatting (118 lines)
- `~/.config/aerc/filters/html-to-text` - HTML pipeline (84 lines)
- `~/.config/aerc/filters/wrap-plain` - plain text routing (16 lines)
- `~/.config/aerc/filters/format-headers` - shell wrapper showing palette-to-env bridge
- `~/.config/aerc/filters/palette.sh` - palette format the Go binary must parse
- `~/.config/aerc/filters/unwrap-tables.lua` - pandoc Lua filter (unchanged, copied)
- `~/.config/aerc/themes/nord.sh` - theme file format

---

### Task 1: Project scaffolding

**Files:**
- Create: `CLAUDE.md`
- Create: `go.mod`
- Create: `Makefile`
- Create: `.golangci.yml`
- Create: `.gitignore`

- [ ] **Step 1: Create CLAUDE.md**

```markdown
# beautiful-aerc

Themeable aerc email filters and configuration, distributed as a
single GNU Stow package.

## MANDATORY: Go Conventions

**Read and follow `~/.claude/docs/go-conventions.md` before writing
ANY Go code.** Every Go file, function, test, and error message must
conform. Key rules:

- No unnecessary interfaces, goroutines, builder patterns
- `cmd/` for CLI wiring only, `internal/` for business logic
- cobra with `SilenceUsage: true`, flags in a struct
- `fmt.Errorf("context: %w", err)` at every error boundary
- Table-driven tests, no assertion libraries
- `make check` (vet + test) must pass before any commit

## MANDATORY: Go Skill

**Use superpowers:go skill for all Go development tasks.**

## Project Structure

```
cmd/beautiful-aerc/    CLI wiring (cobra root + subcommands)
internal/palette/      Parse generated/palette.sh, expose color tokens
internal/filter/       Filter implementations (headers, html, plain)
e2e/                   End-to-end tests (build binary, pipe fixtures)
e2e/testdata/          HTML email fixtures + golden output files
.config/aerc/          aerc configuration files
.config/aerc/themes/   Theme source files + generator script
.config/aerc/generated/ Generated palette.sh (produced by generator)
.config/aerc/stylesets/ Generated aerc stylesets
.config/aerc/filters/  pandoc Lua filter (unwrap-tables.lua)
.config/nvim-mail/     Neovim compose editor profile
.config/kitty/         kitty terminal profile for mail
.local/bin/            Launcher scripts (mail, nvim-mail)
```

## aerc Filter Protocol

aerc calls filters as shell commands. Each filter:
- Receives email content on **stdin**
- Writes ANSI-styled text to **stdout**
- Has access to `AERC_COLUMNS` env var (terminal width)
- `.headers` filter receives RFC 2822 headers (key: value, folded)
- `text/html` filter receives raw HTML body
- `text/plain` filter receives raw plain text body

## Theme System

Theme files (`.config/aerc/themes/*.sh`) define 16 semantic hex color
slots + markdown tokens. The generator (`themes/generate`) reads a
theme file and produces `generated/palette.sh` (ANSI tokens for the
Go binary) and `stylesets/<name>` (aerc UI colors).

The Go binary reads `palette.sh` at runtime for all color tokens.
It finds palette.sh by checking: `$AERC_CONFIG/generated/palette.sh`,
then relative to binary, then `~/.config/aerc/generated/palette.sh`.
If not found, it exits with a clear error.

## Testing

- **Unit tests:** table-driven, same package, alongside source files
- **E2E tests:** build binary in TestMain, pipe HTML fixtures, compare
  against golden files in `e2e/testdata/golden/`
- **Live verification:** tmux-based aerc testing (see global CLAUDE.md)

## Build

```
make build     # build binary
make test      # run tests
make vet       # go vet
make check     # vet + test (gate before commits)
make install   # go install
```
```

- [ ] **Step 2: Create go.mod**

```
go mod init github.com/glw907/beautiful-aerc
```

Then edit `go.mod` to set `go 1.23`.

- [ ] **Step 3: Create Makefile**

```makefile
BINARY := beautiful-aerc

build:
	go build -o $(BINARY) ./cmd/beautiful-aerc

test:
	go test ./...

vet:
	go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

install:
	go install ./cmd/beautiful-aerc

check: vet test

clean:
	rm -f $(BINARY)

.PHONY: build test vet lint install check clean
```

- [ ] **Step 4: Create .golangci.yml**

```yaml
linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gosimple

linters-settings:
  errcheck:
    check-type-assertions: true

issues:
  exclude-use-default: false
```

- [ ] **Step 5: Create .gitignore**

```
beautiful-aerc
.config/aerc/accounts.conf
.config/aerc/fastmail-*.age
.config/aerc/mailrules.json
```

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md go.mod Makefile .golangci.yml .gitignore
git commit -m "Scaffold project: CLAUDE.md, go.mod, Makefile, lint config"
```

---

### Task 2: Palette package

Port palette.sh parsing to Go. This is the foundation - every filter
depends on it.

Read `~/.config/aerc/filters/palette.sh` for the exact format. The
parser must handle:
- `KEY=value` (unquoted)
- `KEY="value"` (quoted)
- `# comments` and blank lines (skip)
- Lines after the override marker override earlier values

**Files:**
- Create: `internal/palette/palette.go`
- Create: `internal/palette/palette_test.go`

- [ ] **Step 1: Write failing tests for palette parsing**

```go
package palette

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		key     string
		want    string
	}{
		{
			name:    "unquoted value",
			content: "FG_BASE=#d8dee9",
			key:     "FG_BASE",
			want:    "#d8dee9",
		},
		{
			name:    "quoted value",
			content: `C_BOLD="1"`,
			key:     "C_BOLD",
			want:    "1",
		},
		{
			name:    "skip comments",
			content: "# comment\nFG_BASE=#d8dee9",
			key:     "FG_BASE",
			want:    "#d8dee9",
		},
		{
			name:    "skip blank lines",
			content: "\n\nFG_BASE=#d8dee9\n\n",
			key:     "FG_BASE",
			want:    "#d8dee9",
		},
		{
			name:    "override earlier value",
			content: "C_LINK_URL=\"4;38;2;163;190;140\"\n# --- overrides below this line are preserved across regeneration ---\nC_LINK_URL=\"38;2;97;110;136\"",
			key:     "C_LINK_URL",
			want:    "38;2;97;110;136",
		},
		{
			name:    "quoted with comment suffix",
			content: `C_RULE="38;2;97;110;136"    # FG_DIM`,
			key:     "C_RULE",
			want:    "38;2;97;110;136",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "palette.sh")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("writing test file: %v", err)
			}
			p, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := p.Get(tt.key)
			if got != tt.want {
				t.Errorf("Get(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestLoadNotFound(t *testing.T) {
	_, err := Load("/nonexistent/palette.sh")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "palette not found") {
		t.Errorf("error = %q, want it to contain 'palette not found'", err)
	}
}

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
			got, err := HexToANSI(tt.hex)
			if err != nil {
				t.Fatalf("HexToANSI(%q): %v", tt.hex, err)
			}
			if got != tt.want {
				t.Errorf("HexToANSI(%q) = %q, want %q", tt.hex, got, tt.want)
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
			_, err := HexToANSI(tt.hex)
			if err == nil {
				t.Errorf("HexToANSI(%q) should have returned error", tt.hex)
			}
		})
	}
}

func TestFindPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "palette.sh")
	if err := os.WriteFile(path, []byte("FG_BASE=#d8dee9"), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	t.Setenv("AERC_CONFIG", dir+"/..") // won't match
	got, err := FindPath(dir)
	if err != nil {
		t.Fatalf("FindPath: %v", err)
	}
	if got != path {
		t.Errorf("FindPath() = %q, want %q", got, path)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/Projects/beautiful-aerc && go test ./internal/palette/ -v`
Expected: FAIL - files don't exist yet

- [ ] **Step 3: Implement palette package**

```go
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
// For ANSI tokens (like C_HEADING), returns the value directly.
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
// Checks in order:
//  1. $AERC_CONFIG/generated/palette.sh
//  2. Provided directory + /palette.sh (for relative-to-binary)
//  3. ~/.config/aerc/generated/palette.sh
func FindPath(generatedDir string) (string, error) {
	candidates := []string{}

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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/Projects/beautiful-aerc && go test ./internal/palette/ -v`
Expected: PASS

- [ ] **Step 5: Run vet**

Run: `cd ~/Projects/beautiful-aerc && go vet ./internal/palette/`
Expected: clean

- [ ] **Step 6: Commit**

```bash
git add internal/palette/
git commit -m "Add palette package: parse palette.sh, hex-to-ANSI conversion"
```

---

### Task 3: Headers filter

Port the format-headers.awk logic to Go. Read
`~/.config/aerc/filters/format-headers.awk` for the exact behavior:
- Parse RFC 2822 headers (handle continuation lines)
- Reorder to: From, To, Cc, Bcc, Date, Subject
- Strip bare angle brackets from addresses
- Wrap address headers at recipient boundaries to fit terminal width
- Colorize keys (bold accent), values (foreground), angle brackets (dim)
- Draw separator line using AERC_COLUMNS width

Also read `~/.config/aerc/filters/format-headers` (shell wrapper) for
how the palette hex colors are converted to ANSI and passed as env vars.

**Files:**
- Create: `internal/filter/headers.go`
- Create: `internal/filter/headers_test.go`

- [ ] **Step 1: Write failing tests**

```go
package filter

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseHeaders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "simple headers",
			input: "From: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>\r\nSubject: Hello\r\n\r\n",
			want: map[string]string{
				"from":    " Alice <alice@example.com>",
				"to":      " Bob <bob@example.com>",
				"subject": " Hello",
			},
		},
		{
			name:  "folded header",
			input: "To: Alice <alice@example.com>,\r\n Bob <bob@example.com>\r\nSubject: Test\r\n\r\n",
			want: map[string]string{
				"to":      " Alice <alice@example.com>,\n Bob <bob@example.com>",
				"subject": " Test",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHeaders(strings.NewReader(tt.input))
			for k, want := range tt.want {
				if got.values[k] != want {
					t.Errorf("header[%q] = %q, want %q", k, got.values[k], want)
				}
			}
		})
	}
}

func TestStripBareAngles(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bare at start", "<alice@example.com>", "alice@example.com"},
		{"bare after comma", "Bob, <alice@example.com>", "Bob, alice@example.com"},
		{"with name", "Alice <alice@example.com>", "Alice <alice@example.com>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripBareAngles(tt.input)
			if got != tt.want {
				t.Errorf("stripBareAngles(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWrapAddresses(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		addrs  string
		cols   int
		want   int // expected number of output lines
	}{
		{
			name:  "short fits one line",
			key:   "To:",
			addrs: "alice@example.com",
			cols:  80,
			want:  1,
		},
		{
			name:  "long wraps",
			key:   "To:",
			addrs: "alice@example.com, bob@example.com, charlie@example.com, dave@example.com",
			cols:  40,
			want:  2, // at minimum wraps once
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := wrapAddresses(tt.key, tt.addrs, tt.cols)
			if len(lines) < tt.want {
				t.Errorf("wrapAddresses produced %d lines, want at least %d", len(lines), tt.want)
			}
		})
	}
}

func TestHeadersFilter(t *testing.T) {
	input := "From: Alice <alice@example.com>\r\nSubject: Hello World\r\nDate: Mon, 01 Jan 2026 00:00:00 +0000\r\nTo: Bob <bob@example.com>\r\nX-Mailer: test\r\n\r\n"

	var buf bytes.Buffer
	err := Headers(strings.NewReader(input), &buf, noColors(), 80)
	if err != nil {
		t.Fatalf("Headers: %v", err)
	}
	out := buf.String()

	// Verify header order: From before To before Date before Subject
	fromIdx := strings.Index(out, "From:")
	toIdx := strings.Index(out, "To:")
	dateIdx := strings.Index(out, "Date:")
	subjectIdx := strings.Index(out, "Subject:")

	if fromIdx < 0 || toIdx < 0 || dateIdx < 0 || subjectIdx < 0 {
		t.Fatalf("missing headers in output: %q", out)
	}
	if fromIdx > toIdx || toIdx > dateIdx || dateIdx > subjectIdx {
		t.Errorf("headers not in expected order (From, To, Date, Subject)")
	}

	// X-Mailer should be dropped
	if strings.Contains(out, "X-Mailer") {
		t.Error("X-Mailer should be dropped")
	}

	// Separator should be present
	if !strings.Contains(out, "─") {
		t.Error("separator line not found")
	}
}

// noColors returns a colorSet with no ANSI codes for testing.
func noColors() *ColorSet {
	return &ColorSet{}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/Projects/beautiful-aerc && go test ./internal/filter/ -v`
Expected: FAIL

- [ ] **Step 3: Implement headers filter**

Read `~/.config/aerc/filters/format-headers.awk` line by line and
port to Go. The implementation must:

1. Parse RFC 2822 headers - handle `\r\n` line endings and continuation
   lines (lines starting with whitespace)
2. Store headers keyed by lowercase name, preserve original case
3. On blank line (end of headers), output in order:
   from, to, cc, bcc, date, subject - skip any not present
4. For address headers (from/to/cc/bcc): strip bare `<email>` angles
   (where no name precedes), split by comma, wrap at `cols` width
5. Colorize: keys in bold accent, angle brackets in dim, values in fg
6. Draw separator line of `─` characters to `cols` width

The `colorSet` struct holds pre-computed ANSI sequences loaded from
the palette. `Headers()` is the public entry point called by the
cobra subcommand.

```go
package filter

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ColorSet holds pre-computed ANSI escape sequences for output.
type ColorSet struct {
	HdrKey string // bold accent for header keys
	HdrFG  string // foreground for header values
	HdrDim string // dim for angle brackets
	Reset  string
}

// ... (full implementation ported from format-headers.awk)
```

The full implementation should closely follow the AWK logic. Key
function signatures:

```go
func Headers(r io.Reader, w io.Writer, colors *ColorSet, cols int) error
func parseHeaders(r io.Reader) *headerBlock
func stripBareAngles(val string) string
func wrapAddresses(key, addrs string, cols int) []string
func colorizeValue(val string, colors *colorSet) string
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/Projects/beautiful-aerc && go test ./internal/filter/ -v -run TestHeaders -run TestParse -run TestStrip -run TestWrap`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/filter/headers.go internal/filter/headers_test.go
git commit -m "Add headers filter: reorder, colorize, wrap addresses"
```

---

### Task 4: HTML filter

Port the html-to-text pipeline to Go. Read
`~/.config/aerc/filters/html-to-text` for the exact pipeline:

1. Strip Mozilla/Thunderbird class attributes (sed)
2. Call pandoc for HTML-to-markdown conversion
3. Clean pandoc artifacts, normalize whitespace, highlight markdown (perl -0777)
4. Call colorize (aerc built-in) for quotes/diffs
5. Style links: colored [text], dimmed (url), strip colorize ANSI from URLs (perl)

The Go implementation absorbs stages 1, 3, and 5. Stages 2 (pandoc)
and 4 (colorize) remain as subprocess calls.

**Files:**
- Create: `internal/filter/html.go`
- Create: `internal/filter/html_test.go`

- [ ] **Step 1: Write failing tests for text cleanup**

Test the regex-based cleanup functions individually. These are the
perl regex patterns from html-to-text that need porting:

```go
package filter

import (
	"testing"
)

func TestCleanPandocArtifacts(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trailing backslash", "hello\\\n", "hello\n"},
		{"escaped punctuation", "hello\\!", "hello!"},
		{"escaped period", "end\\.", "end."},
		{"no change", "normal text", "normal text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanPandocArtifacts(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanImages(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"image link",
			"[![alt](img.png)](https://example.com)",
			"[alt](https://example.com)",
		},
		{
			"standalone image",
			"![logo](logo.png)\n",
			"",
		},
		{
			"empty text link",
			"[](https://example.com)\n",
			"",
		},
		{
			"empty url link",
			"[click here]()",
			"click here",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanImages(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"nbsp", "hello\u00a0world", "hello world"},
		{"zero-width chars", "he\u200cllo\u200bwor\uFEFFld", "helloworld"},
		{"trailing spaces on blank line", "hello\n   \nworld", "hello\n\nworld"},
		{"excessive blank lines", "hello\n\n\n\nworld", "hello\n\nworld"},
		{"leading blank lines", "\n\n\nhello", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHighlightMarkdown(t *testing.T) {
	colors := &markdownColors{
		Heading: "1;32",
		Bold:    "1",
		Italic:  "3",
		Rule:    "2",
		Reset:   "0",
	}
	tests := []struct {
		name  string
		input string
		check func(string) bool
		desc  string
	}{
		{
			"heading",
			"## Hello World",
			func(s string) bool { return contains(s, "\033[1;32m") && contains(s, "Hello World") },
			"should contain heading color + text",
		},
		{
			"bold",
			"this is **bold** text",
			func(s string) bool { return contains(s, "\033[1m") && contains(s, "bold") },
			"should contain bold ANSI + text",
		},
		{
			"italic",
			"this is *italic* text",
			func(s string) bool { return contains(s, "\033[3m") && contains(s, "italic") },
			"should contain italic ANSI + text",
		},
		{
			"horizontal rule dashes",
			"---",
			func(s string) bool { return contains(s, "\033[2m") },
			"should contain rule color",
		},
		{
			"horizontal rule underscores",
			"___",
			func(s string) bool { return contains(s, "\033[2m") },
			"should contain rule color",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := highlightMarkdown(tt.input, colors)
			if !tt.check(got) {
				t.Errorf("%s: got %q", tt.desc, got)
			}
		})
	}
}

func TestStyleLinks(t *testing.T) {
	colors := &linkColors{
		Text:  "38;2;136;192;208",
		URL:   "38;2;97;110;136",
		Reset: "0",
	}
	tests := []struct {
		name      string
		input     string
		clean     bool
		checkText string
	}{
		{
			"markdown mode",
			"[Click here](https://example.com)",
			false,
			"\033[38;2;136;192;208m[Click here]\033[0m\033[38;2;97;110;136m(https://example.com)\033[0m",
		},
		{
			"clean mode",
			"[Click here](https://example.com)",
			true,
			"Click here",
		},
		{
			"strip leading/trailing spaces in text",
			"[ Click here ](https://example.com)",
			false,
			"[Click here]",
		},
		{
			"strip colorize ANSI from URL",
			"[Click](\033[4;33mhttps://example.com\033[0m)",
			false,
			"(https://example.com)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := styleLinks(tt.input, colors, tt.clean)
			if !contains(got, tt.checkText) {
				t.Errorf("output %q does not contain %q", got, tt.checkText)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/Projects/beautiful-aerc && go test ./internal/filter/ -v -run "TestClean|TestNormalize|TestHighlight|TestStyleLinks"`
Expected: FAIL

- [ ] **Step 3: Implement cleanup and highlighting functions**

Port each perl regex pattern from html-to-text to Go. Use
`regexp` package. Key functions:

```go
func cleanMozAttributes(html string) string      // sed stage
func cleanPandocArtifacts(text string) string     // perl: \\$ and \\(punct)
func cleanImages(text string) string              // perl: image-links, standalone images, empty links
func joinMultilineLinks(text string) string        // perl: multi-line link text
func normalizeWhitespace(text string) string       // perl: nbsp, zero-width, blank lines, leading
func highlightMarkdown(text string, colors *markdownColors) string  // perl: headings, bold, italic, rules
func styleLinks(text string, colors *linkColors, clean bool) string // perl: link styling
func stripANSI(s string) string                   // perl: strip \e[...m from URLs
```

Read the exact regex patterns from `~/.config/aerc/filters/html-to-text`
lines 39-66 and 76-83 and replicate them precisely in Go.

- [ ] **Step 4: Implement pandoc subprocess call**

```go
func runPandoc(input io.Reader, luaFilter string, cols int) (string, error)
```

Calls pandoc with the same flags as the shell script:
```
pandoc -f html \
  -t markdown-raw_html-native_divs-native_spans-header_attributes-bracketed_spans-fenced_divs-inline_code_attributes-link_attributes \
  -L <luaFilter> \
  --wrap=auto --columns=<cols>
```

Finds `unwrap-tables.lua` relative to the binary or via
`$AERC_CONFIG/filters/unwrap-tables.lua`.

- [ ] **Step 5: Implement colorize subprocess call**

```go
func runColorize(input string) (string, error)
```

Pipes text through the aerc `colorize` built-in. Finds it at
`/usr/local/libexec/aerc/filters/colorize` or in PATH.

- [ ] **Step 6: Implement the HTML entry point**

```go
func HTML(r io.Reader, w io.Writer, p *palette.Palette, cols int, cleanLinks bool) error
```

Orchestrates the full pipeline:
1. Read all stdin
2. `cleanMozAttributes`
3. `runPandoc`
4. `cleanPandocArtifacts` + `cleanImages` + `joinMultilineLinks` + `normalizeWhitespace`
5. `highlightMarkdown`
6. `runColorize`
7. `styleLinks`
8. Write leading newline + result to stdout

- [ ] **Step 7: Run all tests**

Run: `cd ~/Projects/beautiful-aerc && go test ./internal/filter/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/filter/html.go internal/filter/html_test.go
git commit -m "Add HTML filter: pandoc pipeline, cleanup, markdown highlighting, link styling"
```

---

### Task 5: Plain text filter

Port wrap-plain to Go. Read `~/.config/aerc/filters/wrap-plain`:
- Read all stdin
- If first 50 lines contain HTML tags, delegate to HTML filter
- Otherwise, pipe through `wrap -w $AERC_COLUMNS -r | colorize`

**Files:**
- Create: `internal/filter/plain.go`
- Create: `internal/filter/plain_test.go`

- [ ] **Step 1: Write failing tests**

```go
package filter

import (
	"testing"
)

func TestDetectHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"plain text", "Hello world\nThis is a test", false},
		{"has div", "<div>Hello</div>", true},
		{"has html tag", "<html><body>test</body></html>", true},
		{"has br", "line one<br>line two", true},
		{"has table", "<table><tr><td>cell</td></tr></table>", true},
		{"has span", "text <span>styled</span> text", true},
		{"has p tag", "<p>paragraph</p>", true},
		{"angle bracket in text", "x < y and y > z", false},
		{"html deep in file", "line1\nline2\n" + repeatString("normal\n", 50) + "<div>late html</div>", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectHTML(tt.input)
			if got != tt.want {
				t.Errorf("detectHTML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func repeatString(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/Projects/beautiful-aerc && go test ./internal/filter/ -v -run TestDetectHTML`
Expected: FAIL

- [ ] **Step 3: Implement plain filter**

```go
package filter

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/glw907/beautiful-aerc/internal/palette"
)

var htmlTagRe = regexp.MustCompile(`(?i)<(div|html|body|table|span|br|p[ />])`)

func detectHTML(text string) bool {
	lines := strings.SplitN(text, "\n", 51)
	if len(lines) > 50 {
		lines = lines[:50]
	}
	sample := strings.Join(lines, "\n")
	return htmlTagRe.MatchString(sample)
}

// Plain handles the text/plain filter. If stdin looks like HTML,
// delegates to HTML filter. Otherwise pipes through wrap | colorize.
func Plain(r io.Reader, w io.Writer, p *palette.Palette, cols int, cleanLinks bool) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	text := string(body)

	fmt.Fprintln(w) // leading blank line

	if detectHTML(text) {
		return HTML(strings.NewReader(text), w, p, cols, cleanLinks)
	}

	colStr := "80"
	if cols > 0 {
		colStr = fmt.Sprintf("%d", cols)
	}

	wrap := exec.Command("wrap", "-w", colStr, "-r")
	wrap.Stdin = strings.NewReader(text)

	colorize, colorizeErr := findColorize()
	if colorizeErr != nil {
		// No colorize available - just wrap
		out, err := wrap.Output()
		if err != nil {
			return fmt.Errorf("running wrap: %w", err)
		}
		_, err = w.Write(out)
		return err
	}

	wrapOut, err := wrap.Output()
	if err != nil {
		return fmt.Errorf("running wrap: %w", err)
	}

	col := exec.Command(colorize)
	col.Stdin = strings.NewReader(string(wrapOut))
	col.Stdout = w
	col.Stderr = os.Stderr
	if err := col.Run(); err != nil {
		return fmt.Errorf("running colorize: %w", err)
	}
	return nil
}

func findColorize() (string, error) {
	// Check aerc's libexec directory first
	libexec := "/usr/local/libexec/aerc/filters/colorize"
	if _, err := os.Stat(libexec); err == nil {
		return libexec, nil
	}
	// Fall back to PATH
	return exec.LookPath("colorize")
}
```

- [ ] **Step 4: Run tests**

Run: `cd ~/Projects/beautiful-aerc && go test ./internal/filter/ -v -run TestDetectHTML`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/filter/plain.go internal/filter/plain_test.go
git commit -m "Add plain text filter: HTML detection, wrap+colorize delegation"
```

---

### Task 6: Cobra CLI

Wire up the three subcommands.

**Files:**
- Create: `cmd/beautiful-aerc/main.go`
- Create: `cmd/beautiful-aerc/root.go`
- Create: `cmd/beautiful-aerc/headers.go`
- Create: `cmd/beautiful-aerc/html.go`
- Create: `cmd/beautiful-aerc/plain.go`

- [ ] **Step 1: Add cobra dependency**

```bash
cd ~/Projects/beautiful-aerc && go get github.com/spf13/cobra
```

- [ ] **Step 2: Create main.go**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Create root.go**

```go
package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "beautiful-aerc",
		Short:        "Themeable filters for the aerc email client",
		SilenceUsage: true,
	}
	cmd.AddCommand(newHeadersCmd())
	cmd.AddCommand(newHTMLCmd())
	cmd.AddCommand(newPlainCmd())
	return cmd
}
```

- [ ] **Step 4: Create headers.go**

```go
package main

import (
	"os"
	"strconv"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/palette"
	"github.com/spf13/cobra"
)

func newHeadersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "headers",
		Short: "Format and colorize email headers",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadPalette()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.Headers(os.Stdin, os.Stdout, colorsFromPalette(p), cols)
		},
	}
	return cmd
}

func loadPalette() (*palette.Palette, error) {
	binDir, _ := os.Executable()
	genDir := ""
	if binDir != "" {
		// .local/bin/beautiful-aerc -> .config/aerc/generated
		genDir = binDir + "/../../.config/aerc/generated"
	}
	path, err := palette.FindPath(genDir)
	if err != nil {
		return nil, err
	}
	return palette.Load(path)
}

func colorsFromPalette(p *palette.Palette) *filter.ColorSet {
	hdrKey := p.Get("ACCENT_PRIMARY")
	fgBase := p.Get("FG_BASE")
	fgDim := p.Get("FG_DIM")

	ansiKey, _ := palette.HexToANSI(hdrKey)
	ansiFG, _ := palette.HexToANSI(fgBase)
	ansiDim, _ := palette.HexToANSI(fgDim)

	return &filter.ColorSet{
		HdrKey: "\033[1;" + ansiKey + "m",
		HdrFG:  "\033[" + ansiFG + "m",
		HdrDim: "\033[" + ansiDim + "m",
		Reset:  "\033[0m",
	}
}

func termCols() int {
	if s := os.Getenv("AERC_COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 80
}
```

- [ ] **Step 5: Create html.go**

```go
package main

import (
	"os"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/palette"
	"github.com/spf13/cobra"
)

type htmlFlags struct {
	cleanLinks bool
}

func newHTMLCmd() *cobra.Command {
	var f htmlFlags
	cmd := &cobra.Command{
		Use:   "html",
		Short: "Convert HTML email to styled markdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadPalette()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.HTML(os.Stdin, os.Stdout, p, cols, f.cleanLinks)
		},
	}
	cmd.Flags().BoolVar(&f.cleanLinks, "clean-links", false, "show link text only, hide URLs")
	return cmd
}
```

- [ ] **Step 6: Create plain.go**

```go
package main

import (
	"os"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/palette"
	"github.com/spf13/cobra"
)

type plainFlags struct {
	cleanLinks bool
}

func newPlainCmd() *cobra.Command {
	var f plainFlags
	cmd := &cobra.Command{
		Use:   "plain",
		Short: "Format plain text email (reflow and colorize)",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadPalette()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.Plain(os.Stdin, os.Stdout, p, cols, f.cleanLinks)
		},
	}
	cmd.Flags().BoolVar(&f.cleanLinks, "clean-links", false, "show link text only, hide URLs (when HTML detected)")
	return cmd
}
```

- [ ] **Step 7: Build and verify**

Run: `cd ~/Projects/beautiful-aerc && make build`
Expected: binary builds successfully

Run: `echo "From: Test <test@test.com>\nSubject: Hello\n\n" | ./beautiful-aerc headers`
Expected: formatted header output (may error on missing palette - that's OK, validates wiring)

- [ ] **Step 8: Commit**

```bash
git add cmd/ go.sum
git commit -m "Add cobra CLI: headers, html, plain subcommands"
```

---

### Task 7: Theme system

Update the theme file format to include markdown tokens. Rewrite the
generator to resolve slot references and produce both palette.sh and
the aerc styleset. No nvim dependency.

Read the current `~/.config/aerc/themes/nord.sh` and
`~/.config/aerc/filters/generate-palette` for the existing behavior
to preserve.

**Files:**
- Create: `.config/aerc/themes/nord.sh`
- Create: `.config/aerc/themes/generate`
- Create: `.config/aerc/themes/solarized-dark.sh`
- Create: `.config/aerc/themes/gruvbox-dark.sh`

- [ ] **Step 1: Create nord.sh with markdown tokens**

Port the existing `~/.config/aerc/themes/nord.sh` and add the
markdown token section. Read the original file for the exact hex
values and comments.

```sh
# Nord color theme for aerc
# https://www.nordtheme.com/docs/colors-and-palettes

# ── Background tones (dark to light) ────────────────────────────────
BG_BASE="#2e3440"
BG_ELEVATED="#3b4252"
BG_SELECTION="#394353"
BG_BORDER="#49576b"

# ── Foreground tones (light to dark) ────────────────────────────────
FG_BASE="#d8dee9"
FG_BRIGHT="#e5e9f0"
FG_BRIGHTEST="#eceff4"
FG_DIM="#616e88"

# ── Accent colors ───────────────────────────────────────────────────
ACCENT_PRIMARY="#81a1c1"
ACCENT_SECONDARY="#88c0d0"
ACCENT_TERTIARY="#8fbcbb"

# ── Semantic colors ─────────────────────────────────────────────────
COLOR_ERROR="#bf616a"
COLOR_WARNING="#d08770"
COLOR_SUCCESS="#a3be8c"
COLOR_INFO="#ebcb8b"
COLOR_SPECIAL="#b48ead"

# ── Markdown tokens (reference slots + style modifiers) ─────────────
C_HEADING="$COLOR_SUCCESS bold"
C_BOLD="bold"
C_ITALIC="italic"
C_LINK_TEXT="$ACCENT_SECONDARY"
C_LINK_URL="$FG_DIM"
C_RULE="$FG_DIM"
```

- [ ] **Step 2: Create the generator**

Rewrite `generate-palette` as `themes/generate`. Must:
- Source the theme file to get all variables
- Resolve `$VARIABLE` references in markdown tokens
- Convert hex colors to ANSI for markdown tokens
- Combine style modifiers (bold->1, italic->3, underline->4) with color
- Write `generated/palette.sh` with hex vars + resolved ANSI tokens
- Write `stylesets/<theme-name>` with hex values
- Preserve overrides below marker in both output files
- Name styleset after theme file (nord.sh -> nord, solarized-dark.sh -> solarized-dark)

Read `~/.config/aerc/filters/generate-palette` for the styleset
template (lines 135-260) - the exact styleset format must be
replicated.

Key new logic: token resolution. For `C_HEADING="$COLOR_SUCCESS bold"`:
1. Resolve `$COLOR_SUCCESS` -> `#a3be8c`
2. Convert `#a3be8c` -> `38;2;163;190;140`
3. Convert `bold` -> `1`
4. Combine: `1;38;2;163;190;140`

```sh
resolve_token() {
    local value="$1"
    local ansi_parts=""

    for part in $value; do
        case "$part" in
            \$*)
                varname="${part#\$}"
                eval "hex=\$$varname"
                ansi_parts="$ansi_parts$(hex_to_ansi "$hex")"
                ;;
            bold)    ansi_parts="${ansi_parts:+$ansi_parts;}1" ;;
            italic)  ansi_parts="${ansi_parts:+$ansi_parts;}3" ;;
            underline) ansi_parts="${ansi_parts:+$ansi_parts;}4" ;;
            *)
                echo "Unknown token part: $part" >&2
                ;;
        esac
    done
    echo "$ansi_parts"
}
```

- [ ] **Step 3: Test the generator with nord.sh**

```bash
cd ~/Projects/beautiful-aerc/.config/aerc
themes/generate themes/nord.sh
cat generated/palette.sh
cat stylesets/nord
```

Verify palette.sh contains resolved ANSI tokens and styleset
contains hex values matching the original nord-custom.

- [ ] **Step 4: Create solarized-dark.sh**

Research Solarized Dark hex values. Map to the 16 semantic slots:

```sh
# Solarized Dark color theme for aerc
# https://ethanschoonover.com/solarized/

BG_BASE="#002b36"
BG_ELEVATED="#073642"
BG_SELECTION="#073642"
BG_BORDER="#586e75"
FG_BASE="#839496"
FG_BRIGHT="#93a1a1"
FG_BRIGHTEST="#eee8d5"
FG_DIM="#657b83"
ACCENT_PRIMARY="#268bd2"
ACCENT_SECONDARY="#2aa198"
ACCENT_TERTIARY="#2aa198"
COLOR_ERROR="#dc322f"
COLOR_WARNING="#cb4b16"
COLOR_SUCCESS="#859900"
COLOR_INFO="#b58900"
COLOR_SPECIAL="#6c71c4"

C_HEADING="$COLOR_SUCCESS bold"
C_BOLD="bold"
C_ITALIC="italic"
C_LINK_TEXT="$ACCENT_SECONDARY"
C_LINK_URL="$FG_DIM"
C_RULE="$FG_DIM"
```

- [ ] **Step 5: Create gruvbox-dark.sh**

```sh
# Gruvbox Dark color theme for aerc
# https://github.com/morhetz/gruvbox

BG_BASE="#282828"
BG_ELEVATED="#3c3836"
BG_SELECTION="#3c3836"
BG_BORDER="#665c54"
FG_BASE="#ebdbb2"
FG_BRIGHT="#fbf1c7"
FG_BRIGHTEST="#fbf1c7"
FG_DIM="#928374"
ACCENT_PRIMARY="#83a598"
ACCENT_SECONDARY="#8ec07c"
ACCENT_TERTIARY="#8ec07c"
COLOR_ERROR="#fb4934"
COLOR_WARNING="#fe8019"
COLOR_SUCCESS="#b8bb26"
COLOR_INFO="#fabd2f"
COLOR_SPECIAL="#d3869b"

C_HEADING="$COLOR_SUCCESS bold"
C_BOLD="bold"
C_ITALIC="italic"
C_LINK_TEXT="$ACCENT_SECONDARY"
C_LINK_URL="$FG_DIM"
C_RULE="$FG_DIM"
```

- [ ] **Step 6: Test generator with all three themes**

```bash
themes/generate themes/solarized-dark.sh
cat stylesets/solarized-dark | head -20
themes/generate themes/gruvbox-dark.sh
cat stylesets/gruvbox-dark | head -20
themes/generate themes/nord.sh  # restore Nord
```

- [ ] **Step 7: Commit**

```bash
git add .config/aerc/themes/ .config/aerc/generated/ .config/aerc/stylesets/
git commit -m "Add theme system: generator, Nord, Solarized Dark, Gruvbox Dark"
```

---

### Task 8: Config files

Copy and clean aerc config, binds, accounts example, nvim-mail, kitty-mail,
launcher scripts, and desktop file.

**Files:**
- Create: `.config/aerc/aerc.conf`
- Create: `.config/aerc/binds.conf`
- Create: `.config/aerc/accounts.conf.example`
- Create: `.config/aerc/filters/unwrap-tables.lua`
- Create: `.config/nvim-mail/init.lua`
- Create: `.config/nvim-mail/syntax/aercmail.vim`
- Create: `.config/kitty/kitty-mail.conf`
- Create: `.local/bin/mail`
- Create: `.local/bin/nvim-mail`
- Create: `.local/share/applications/aerc-mail.desktop`

- [ ] **Step 1: Create aerc.conf**

Copy from `~/.config/aerc/aerc.conf`. Changes:
- `[filters]` section: replace filter references with `beautiful-aerc` subcommands
- Remove `[hooks]` aerc-rules line
- Add comment about styleset-name needing to match generated theme
- Keep all other settings (index-columns, sidebar, icons, threading, etc.)

Key filter lines:
```ini
[filters]
text/plain=beautiful-aerc plain
text/html=beautiful-aerc html
.headers=beautiful-aerc headers
```

- [ ] **Step 2: Create binds.conf**

Copy from `~/.config/aerc/binds.conf` as-is. No changes needed.

- [ ] **Step 3: Create accounts.conf.example**

```ini
# Copy this file to accounts.conf and fill in your details.
#
# See aerc-accounts(5) for all options.

[Mail]
source = jmap://you@example.com
# source-cred-cmd = your-credential-helper
outgoing = jmap://you@example.com
# outgoing-cred-cmd = your-credential-helper
default = Inbox
from = Your Name <you@example.com>
copy-to = Sent
cache-headers = true
```

- [ ] **Step 4: Copy unwrap-tables.lua**

Copy from `~/.config/aerc/filters/unwrap-tables.lua` unchanged.

- [ ] **Step 5: Create nvim-mail files**

Copy `~/.dotfiles/nvim-mail/.config/nvim-mail/init.lua`. Changes:
- Replace signature block with placeholder:

```lua
vim.keymap.set("n", "<leader>sig", function()
  local sig = {
    "-- ",
    "**Your Name**",
    "your-email@example.com",
  }
  local row = vim.api.nvim_win_get_cursor(0)[1]
  vim.api.nvim_buf_set_lines(0, row, row, false, sig)
end, { desc = "Insert email signature" })
```

Copy `syntax/aercmail.vim` unchanged.

- [ ] **Step 6: Copy kitty-mail.conf**

Copy from `~/.dotfiles/kitty/.config/kitty/kitty-mail.conf` unchanged.

- [ ] **Step 7: Create launcher scripts**

`mail`:
```bash
#!/usr/bin/env bash
exec kitty --class aerc-mail --config ~/.config/kitty/kitty-mail.conf --title Mail aerc
```

`nvim-mail`:
```bash
#!/usr/bin/env bash
NVIM_APPNAME=nvim-mail exec nvim "$@"
```

- [ ] **Step 8: Create desktop file**

Copy from `~/.local/share/applications/aerc-mail.desktop`. Verify
it contains no personal paths.

- [ ] **Step 9: Commit**

```bash
git add .config/ .local/
git commit -m "Add config files: aerc, nvim-mail, kitty, launchers"
```

---

### Task 9: E2E test fixtures

Collect real HTML email samples and create golden file tests.

**Files:**
- Create: `e2e/e2e_test.go`
- Create: `e2e/testdata/` (multiple fixture files)
- Create: `e2e/testdata/golden/` (expected output files)

- [ ] **Step 1: Create test fixture HTML files**

Save representative HTML email bodies as fixture files. Source these
from the user's actual email by extracting the HTML body from
messages in aerc (view source, copy HTML part). Categories:

- `e2e/testdata/marketing-reebelo.html` - zero-width preheader junk
- `e2e/testdata/transactional-google.html` - Google security alert
- `e2e/testdata/developer-github.html` - GitHub notification
- `e2e/testdata/simple-text.html` - simple paragraph HTML
- `e2e/testdata/edge-empty-links.html` - empty link text, image links

Since we don't have access to the raw HTML files outside aerc,
create representative synthetic fixtures that exercise the same
patterns. For example, the marketing fixture should include
`&#8204;` (zero-width non-joiner) preheader text, layout tables,
and tracking URLs.

- [ ] **Step 2: Create e2e test harness**

```go
package e2e

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	binary       string
	updateGolden = flag.Bool("update-golden", false, "regenerate golden files")
)

func TestMain(m *testing.M) {
	flag.Parse()

	// Build the binary once
	tmp, err := os.MkdirTemp("", "beautiful-aerc-test")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(tmp, "beautiful-aerc")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/beautiful-aerc")
	cmd.Dir = filepath.Join("..")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("build failed: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

func TestHTMLFixtures(t *testing.T) {
	fixtures, err := filepath.Glob("testdata/*.html")
	if err != nil {
		t.Fatalf("globbing fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no HTML fixtures found in testdata/")
	}

	for _, fixture := range fixtures {
		name := strings.TrimSuffix(filepath.Base(fixture), ".html")
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}

			cmd := exec.Command(binary, "html")
			cmd.Stdin = bytes.NewReader(input)
			cmd.Env = append(os.Environ(), "AERC_COLUMNS=80")
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("running html filter: %v", err)
			}

			goldenPath := filepath.Join("testdata", "golden", name+".txt")
			if *updateGolden {
				os.MkdirAll(filepath.Dir(goldenPath), 0755)
				if err := os.WriteFile(goldenPath, out, 0644); err != nil {
					t.Fatalf("writing golden file: %v", err)
				}
				return
			}

			golden, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("reading golden file (run with -update-golden to create): %v", err)
			}
			if !bytes.Equal(out, golden) {
				t.Errorf("output differs from golden file %s\ngot:\n%s\nwant:\n%s", goldenPath, out, golden)
			}
		})
	}
}

func TestHeadersFixture(t *testing.T) {
	input := "From: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>, Charlie <charlie@example.com>\r\nDate: Mon, 01 Jan 2026 00:00:00 +0000\r\nSubject: Test Message\r\nX-Mailer: test\r\n\r\n"

	cmd := exec.Command(binary, "headers")
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "AERC_COLUMNS=80")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running headers filter: %v", err)
	}

	output := string(out)
	// Verify order and content
	if !strings.Contains(output, "From:") {
		t.Error("missing From header")
	}
	if !strings.Contains(output, "Subject:") {
		t.Error("missing Subject header")
	}
	if strings.Contains(output, "X-Mailer") {
		t.Error("X-Mailer should be stripped")
	}
}
```

- [ ] **Step 3: Generate initial golden files**

```bash
cd ~/Projects/beautiful-aerc && go test ./e2e/ -v -update-golden
```

- [ ] **Step 4: Run tests against golden files**

```bash
cd ~/Projects/beautiful-aerc && go test ./e2e/ -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add e2e/
git commit -m "Add e2e tests with HTML fixtures and golden file comparison"
```

---

### Task 10: Full build verification

Run the complete quality gate.

- [ ] **Step 1: Run make check**

```bash
cd ~/Projects/beautiful-aerc && make check
```
Expected: vet passes, all tests pass

- [ ] **Step 2: Build and install**

```bash
cd ~/Projects/beautiful-aerc && make install
```
Expected: binary installed to `~/go/bin/beautiful-aerc`

- [ ] **Step 3: Verify binary works**

```bash
echo '<html><body><h2>Hello</h2><p>This is <b>bold</b> and <a href="https://example.com">a link</a>.</p></body></html>' | beautiful-aerc html
```
Expected: styled output with heading, bold, and link formatting
(will error about palette - generate it first)

```bash
cd ~/Projects/beautiful-aerc/.config/aerc && themes/generate themes/nord.sh
echo '<html><body><h2>Hello</h2><p>This is <b>bold</b> and <a href="https://example.com">a link</a>.</p></body></html>' | beautiful-aerc html
```
Expected: properly styled output

- [ ] **Step 4: Commit any fixes**

If any issues found, fix and commit.

---

### Task 11: Documentation

Write the four docs described in the spec.

**Files:**
- Create: `README.md`
- Create: `docs/themes.md`
- Create: `docs/filters.md`
- Create: `docs/contributing.md`

- [ ] **Step 1: Write README.md**

Cover: what it is, prerequisites, install steps (clone, build,
generate theme, stow, configure account), quick usage, links to
other docs. No screenshots yet (placeholder for later).

- [ ] **Step 2: Write docs/themes.md**

Cover: theme file format, 16 semantic slots with descriptions,
markdown token syntax, creating custom themes, running the generator,
override mechanism, updating kitty/nvim colors.

- [ ] **Step 3: Write docs/filters.md**

Cover: the three subcommands, HTML pipeline stages, link display
modes (markdown vs. clean), header formatting, plain text handling,
palette token reference, troubleshooting.

- [ ] **Step 4: Write docs/contributing.md**

Cover: Go project layout, build/test commands, architecture (aerc
filter protocol, stdin/stdout), how to add a filter stage, how to
add a theme, code conventions.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/
git commit -m "Add documentation: README, themes, filters, contributing"
```

---

### Task 12: Consumer setup (dotfiles integration)

Switch the user's dotfiles to consume from the project.

- [ ] **Step 1: Create symlink in dotfiles**

```bash
ln -s ~/Projects/beautiful-aerc ~/.dotfiles/beautiful-aerc
```

- [ ] **Step 2: Unstow old packages**

```bash
cd ~/.dotfiles
stow -D aerc
stow -D nvim-mail
# kitty and bin have non-mail files, so we only remove specific conflicts
```

- [ ] **Step 3: Stow beautiful-aerc**

```bash
cd ~/.dotfiles && stow beautiful-aerc
```

This will create symlinks for everything in the repo's `.config/`,
`.local/bin/`, and `.local/share/` directories.

- [ ] **Step 4: Copy personal files**

```bash
cp ~/.config/aerc/accounts.conf ~/Projects/beautiful-aerc/.config/aerc/accounts.conf
```

This file is gitignored so it won't be committed.

- [ ] **Step 5: Generate palette for the active theme**

```bash
cd ~/.config/aerc && themes/generate themes/nord.sh
```

- [ ] **Step 6: Update aerc.conf styleset name**

Edit `aerc.conf` to set `styleset-name=nord` (matching the generated
styleset).

- [ ] **Step 7: Live test in aerc**

```bash
tmux kill-session -t test 2>/dev/null
tmux new-session -d -s test -x 140 -y 50 'aerc'
sleep 8
# Check message list renders
tmux capture-pane -t test -p | head -15
# Open a message and check rendering
tmux send-keys -t test Enter && sleep 3
tmux capture-pane -t test -p | head -25
tmux kill-session -t test
```

Verify: headers are colorized, message body renders correctly,
links are styled, no errors.

- [ ] **Step 8: Test with problem emails**

Navigate to a marketing email (Reebelo-style) and a GitHub
notification to verify the Go binary handles them correctly.

- [ ] **Step 9: Clean up old stow packages**

Once everything works, remove the mail-related files from the old
stow packages in `~/.dotfiles/`:
- `aerc/` package - can be fully removed
- `nvim-mail/` package - can be fully removed
- `kitty/.config/kitty/kitty-mail.conf` - remove (now in beautiful-aerc)
- `bin/.local/bin/mail` - remove (now in beautiful-aerc)
- `bin/.local/bin/nvim-mail` - remove (now in beautiful-aerc)

- [ ] **Step 10: Commit dotfiles changes**

```bash
cd ~/.dotfiles
git add -A
git commit -m "Switch mail stack to beautiful-aerc project"
```
