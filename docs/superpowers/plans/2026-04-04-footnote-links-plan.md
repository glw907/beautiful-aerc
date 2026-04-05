# Footnote-Style Link Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace inline markdown links with footnote-style references in the HTML filter, add a themeable `pick-link` subcommand for keyboard-driven URL selection.

**Architecture:** Pandoc outputs reference-style links; a new `convertToFootnotes` function renumbers them as `[^N]` footnotes; `styleFootnotes` applies ANSI colors. The `pick-link` subcommand reads URLs from stdin and presents a numbered picker with 1-9/0 instant select and vim-style navigation.

**Tech Stack:** Go 1.23, cobra, pandoc `--reference-links`, ANSI escape sequences, raw terminal mode for pick-link input.

---

## File Structure

```
internal/filter/
  html.go               Modify: add --reference-links to pandoc, replace styleLinks
                         with convertToFootnotes + styleFootnotes, remove cleanLinks param
  html_test.go           Modify: replace TestStyleLinks with TestConvertToFootnotes
                         and TestStyleFootnotes
  footnotes.go           Create: convertToFootnotes and styleFootnotes functions + regexes
  footnotes_test.go      Create: unit tests for footnote conversion and styling
  plain.go               Modify: remove cleanLinks param from Plain() signature
  plain_test.go          No change (plain tests don't cover link styling)

cmd/beautiful-aerc/
  html.go                Modify: remove --clean-links flag, remove cleanLinks from HTML call
  plain.go               Modify: remove --clean-links flag, remove cleanLinks from Plain call
  root.go                Modify: add pick-link subcommand
  picklink.go            Create: pick-link subcommand cobra wiring

internal/picker/
  picker.go              Create: link picker UI (read URLs, numbered display, key handling)
  picker_test.go         Create: unit tests for URL extraction and selection logic

.config/aerc/
  binds.conf             Modify: add Tab keybinding in [view] section

e2e/
  e2e_test.go            Modify: update golden files (footnotes change all HTML output)
  testdata/golden/       Modify: regenerate all golden files
```

---

### Task 1: Create footnotes.go with convertToFootnotes

**Files:**
- Create: `internal/filter/footnotes.go`
- Create: `internal/filter/footnotes_test.go`

- [ ] **Step 1: Write failing tests for convertToFootnotes**

Create `internal/filter/footnotes_test.go`:

```go
package filter

import (
	"strings"
	"testing"
)

func TestConvertToFootnotes(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantBody  string
		wantRefs  []string
	}{
		{
			"single link",
			"Click [here] to continue.\n\n  [here]: https://example.com\n",
			"Click here[^1] to continue.",
			[]string{"[^1]: https://example.com"},
		},
		{
			"multiple links",
			"Visit [home] and [about].\n\n  [home]: https://example.com\n  [about]: https://example.com/about\n",
			"Visit home[^1] and about[^2].",
			[]string{"[^1]: https://example.com", "[^2]: https://example.com/about"},
		},
		{
			"duplicate link text with numeric fallback",
			"[Click here] and [Click here][1]\n\n  [Click here]: https://example.com/a\n  [1]: https://example.com/b\n",
			"Click here[^1] and Click here[^2]",
			[]string{"[^1]: https://example.com/a", "[^2]: https://example.com/b"},
		},
		{
			"self-referencing link becomes plain URL",
			"Visit <https://example.com> for info.\n",
			"Visit https://example.com for info.",
			nil,
		},
		{
			"autolink with no ref defs",
			"See <https://example.com>.\n",
			"See https://example.com.",
			nil,
		},
		{
			"no links",
			"Just plain text.\n",
			"Just plain text.",
			nil,
		},
		{
			"self-ref link in ref defs skipped",
			"Visit [https://example.com] for info.\n\n  [https://example.com]: https://example.com\n",
			"Visit https://example.com for info.",
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, refs := convertToFootnotes(tt.input)
			body = strings.TrimSpace(body)
			if body != tt.wantBody {
				t.Errorf("body:\ngot:  %q\nwant: %q", body, tt.wantBody)
			}
			if len(refs) != len(tt.wantRefs) {
				t.Errorf("refs count: got %d, want %d\nrefs: %v", len(refs), len(tt.wantRefs), refs)
				return
			}
			for i, want := range tt.wantRefs {
				if refs[i] != want {
					t.Errorf("refs[%d]:\ngot:  %q\nwant: %q", i, refs[i], want)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/filter/ -run TestConvertToFootnotes -v`
Expected: FAIL - `convertToFootnotes` undefined

- [ ] **Step 3: Write convertToFootnotes implementation**

Create `internal/filter/footnotes.go`:

```go
package filter

import (
	"fmt"
	"regexp"
	"strings"
)

// Regexes for reference-style link processing.
var (
	// Matches pandoc reference definitions: "  [label]: url"
	reRefDef = regexp.MustCompile(`(?m)^ {0,3}\[([^\]]+)\]:\s+(.+)$`)
	// Matches shortcut reference [text] in body (not followed by : which is a def)
	reRefShortcut = regexp.MustCompile(`\[([^\]]+)\](?:\[(\d+)\])?`)
	// Matches autolinks <https://...>
	reAutolink = regexp.MustCompile(`<(https?://[^>]+)>`)
)

// refDef holds a parsed reference definition.
type refDef struct {
	label string
	url   string
}

// convertToFootnotes transforms pandoc reference-style links into footnote
// syntax. Returns the transformed body text and a slice of "[^N]: url" strings.
// Self-referencing links (where label looks like a URL) become plain URLs.
func convertToFootnotes(text string) (string, []string) {
	// Split into body and reference definitions.
	// Ref defs are indented lines at the end matching [label]: url.
	lines := strings.Split(text, "\n")
	var bodyLines []string
	var defs []refDef

	// Scan from bottom to find ref def block.
	i := len(lines) - 1
	for i >= 0 && strings.TrimSpace(lines[i]) == "" {
		i--
	}
	for i >= 0 {
		groups := reRefDef.FindStringSubmatch(lines[i])
		if groups == nil {
			break
		}
		defs = append(defs, refDef{label: groups[1], url: groups[2]})
		i--
	}
	bodyLines = lines[:i+1]

	// Reverse defs (collected bottom-up).
	for a, b := 0, len(defs)-1; a < b; a, b = a+1, b-1 {
		defs[a], defs[b] = defs[b], defs[a]
	}

	body := strings.Join(bodyLines, "\n")

	// Build label-to-def mapping and assign footnote numbers.
	// Self-referencing links (label is a URL) are excluded.
	type numberedRef struct {
		num int
		url string
	}
	labelMap := make(map[string]numberedRef)
	var refs []string
	n := 0
	for _, d := range defs {
		if isSelfRef(d.label, d.url) {
			continue
		}
		n++
		labelMap[d.label] = numberedRef{num: n, url: d.url}
		refs = append(refs, fmt.Sprintf("[^%d]: %s", n, d.url))
	}

	// Replace body references with footnote markers.
	body = reRefShortcut.ReplaceAllStringFunc(body, func(m string) string {
		groups := reRefShortcut.FindStringSubmatch(m)
		if groups == nil {
			return m
		}
		label := groups[1]
		numericLabel := groups[2] // non-empty for [text][1] form

		// Try the text label first, then numeric label.
		if ref, ok := labelMap[label]; ok {
			return label + fmt.Sprintf("[^%d]", ref.num)
		}
		if numericLabel != "" {
			if ref, ok := labelMap[numericLabel]; ok {
				return label + fmt.Sprintf("[^%d]", ref.num)
			}
		}

		// Self-referencing: strip brackets.
		if isURL(label) {
			return label
		}

		return m
	})

	// Convert autolinks to plain URLs.
	body = reAutolink.ReplaceAllString(body, "$1")

	return body, refs
}

// isSelfRef returns true when a reference label is effectively its own URL.
func isSelfRef(label, url string) bool {
	return strings.TrimPrefix(strings.TrimPrefix(label, "https://"), "http://") ==
		strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
}

// isURL returns true if s looks like a URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/filter/ -run TestConvertToFootnotes -v`
Expected: PASS (all 7 subtests)

- [ ] **Step 5: Commit**

```bash
git add internal/filter/footnotes.go internal/filter/footnotes_test.go
git commit -m "Add convertToFootnotes for reference-to-footnote conversion"
```

---

### Task 2: Create styleFootnotes

**Files:**
- Modify: `internal/filter/footnotes.go`
- Modify: `internal/filter/footnotes_test.go`

- [ ] **Step 1: Write failing tests for styleFootnotes**

Append to `internal/filter/footnotes_test.go`:

```go
func TestStyleFootnotes(t *testing.T) {
	colors := &footnoteColors{
		LinkText: "38;2;136;192;208",
		Dim:      "38;2;97;110;136",
		LinkURL:  "38;2;97;110;136",
		Reset:    "0",
	}

	t.Run("body link text colored", func(t *testing.T) {
		body := "click here[^1] to go"
		refs := []string{"[^1]: https://example.com"}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, "\033[38;2;136;192;208mclick here\033[0m") {
			t.Errorf("link text not colored: %q", got)
		}
	})

	t.Run("footnote marker dimmed", func(t *testing.T) {
		body := "click here[^1] to go"
		refs := []string{"[^1]: https://example.com"}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, "\033[38;2;97;110;136m[^1]\033[0m") {
			t.Errorf("marker not dimmed: %q", got)
		}
	})

	t.Run("separator line present", func(t *testing.T) {
		body := "text[^1]"
		refs := []string{"[^1]: https://example.com"}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, strings.Repeat("─", 40)) {
			t.Errorf("separator missing: %q", got)
		}
	})

	t.Run("reference URL colored", func(t *testing.T) {
		body := "text[^1]"
		refs := []string{"[^1]: https://example.com"}
		got := styleFootnotes(body, refs, 40, colors)
		if !strings.Contains(got, "\033[38;2;97;110;136mhttps://example.com\033[0m") {
			t.Errorf("URL not colored: %q", got)
		}
	})

	t.Run("no refs no separator", func(t *testing.T) {
		body := "just text"
		got := styleFootnotes(body, nil, 40, colors)
		if strings.Contains(got, "─") {
			t.Errorf("separator should not appear with no refs: %q", got)
		}
		if got != "just text" {
			t.Errorf("body changed: %q", got)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/filter/ -run TestStyleFootnotes -v`
Expected: FAIL - `footnoteColors` and `styleFootnotes` undefined

- [ ] **Step 3: Write styleFootnotes implementation**

Add to `internal/filter/footnotes.go`:

```go
// footnoteColors holds ANSI parameter strings for footnote styling.
type footnoteColors struct {
	LinkText string // body link text color
	Dim      string // footnote markers and ref labels
	LinkURL  string // reference section URLs
	Reset    string
}

// reFootnoteInBody matches "linktext[^N]" in body text for coloring.
var reFootnoteInBody = regexp.MustCompile(`(\S[^\[]*?)\[\^(\d+)\]`)

// styleFootnotes applies ANSI colors to footnote-annotated text.
// Body link text gets link color, [^N] markers get dim color.
// A separator and colored reference section are appended.
func styleFootnotes(body string, refs []string, cols int, colors *footnoteColors) string {
	if len(refs) == 0 {
		return body
	}

	lt := ""
	dim := ""
	lu := ""
	r := ""
	if colors.LinkText != "" {
		lt = "\033[" + colors.LinkText + "m"
	}
	if colors.Dim != "" {
		dim = "\033[" + colors.Dim + "m"
	}
	if colors.LinkURL != "" {
		lu = "\033[" + colors.LinkURL + "m"
	}
	if colors.Reset != "" {
		r = "\033[" + colors.Reset + "m"
	}

	// Color body: link text + dimmed marker.
	body = reFootnoteInBody.ReplaceAllStringFunc(body, func(m string) string {
		groups := reFootnoteInBody.FindStringSubmatch(m)
		if groups == nil {
			return m
		}
		text := groups[1]
		num := groups[2]
		return lt + text + r + dim + "[^" + num + "]" + r
	})

	// Build reference section.
	var sb strings.Builder
	sb.WriteString(body)
	sb.WriteString("\n" + dim + strings.Repeat("─", cols) + r + "\n")
	for _, ref := range refs {
		// Split "[^N]: url" into label and URL parts.
		colonIdx := strings.Index(ref, ": ")
		if colonIdx < 0 {
			sb.WriteString(ref + "\n")
			continue
		}
		label := ref[:colonIdx]
		url := ref[colonIdx+2:]
		sb.WriteString(dim + label + ":" + r + " " + lu + url + r + "\n")
	}
	return sb.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/filter/ -run TestStyleFootnotes -v`
Expected: PASS (all 5 subtests)

- [ ] **Step 5: Commit**

```bash
git add internal/filter/footnotes.go internal/filter/footnotes_test.go
git commit -m "Add styleFootnotes for ANSI coloring of footnote references"
```

---

### Task 3: Wire footnotes into the HTML pipeline

**Files:**
- Modify: `internal/filter/html.go:26-30` (linkColors struct)
- Modify: `internal/filter/html.go:221-250` (styleLinks function)
- Modify: `internal/filter/html.go:254-260` (runPandoc args)
- Modify: `internal/filter/html.go:330-387` (HTML function)

- [ ] **Step 1: Add `--reference-links` to pandoc args**

In `internal/filter/html.go`, change the `runPandoc` function args:

```go
// runPandoc pipes input through pandoc for HTML-to-markdown conversion.
func runPandoc(input io.Reader, luaFilter string, cols int) (string, error) {
	args := []string{
		"-f", "html",
		"-t", "markdown-raw_html-native_divs-native_spans-header_attributes-bracketed_spans-fenced_divs-inline_code_attributes-link_attributes",
		"-L", luaFilter,
		"--wrap=auto",
		fmt.Sprintf("--columns=%d", cols),
		"--reference-links",
	}
```

- [ ] **Step 2: Replace styleLinks with footnotes in HTML()**

In `internal/filter/html.go`, change the `HTML` function. Remove the `cleanLinks` parameter. Replace the link styling block with footnote conversion and styling:

```go
func HTML(r io.Reader, w io.Writer, p *palette.Palette, cols int) error {
	if cols < 1 {
		cols = 72
	}

	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	// sed stage: strip Mozilla-specific HTML attributes
	cleaned := cleanMozAttributes(string(raw))

	// Find lua filter
	luaFilter, err := findLuaFilter()
	if err != nil {
		return fmt.Errorf("finding lua filter: %w", err)
	}

	// Run pandoc with reference-style links
	md, err := runPandoc(strings.NewReader(cleaned), luaFilter, cols)
	if err != nil {
		return fmt.Errorf("pandoc conversion: %w", err)
	}

	// Post-pandoc cleanup
	md = html.UnescapeString(md)
	md = cleanPandocArtifacts(md)
	md = cleanImages(md)
	md = joinMultilineLinks(md)
	md = normalizeListIndent(md)
	md = normalizeWhitespace(md)

	// Footnote conversion and styling (before markdown highlighting so
	// ANSI codes from heading/bold wrapping don't confuse the link regex)
	body, refs := convertToFootnotes(md)
	fc := &footnoteColors{
		LinkText: p.Get("C_LINK_TEXT"),
		Dim:      p.Get("FG_DIM"),
		LinkURL:  p.Get("C_LINK_URL"),
		Reset:    "0",
	}
	md = styleFootnotes(body, refs, cols, fc)

	// Markdown syntax highlighting
	mc := &markdownColors{
		Heading: p.Get("C_HEADING"),
		Bold:    p.Get("C_BOLD"),
		Italic:  p.Get("C_ITALIC"),
		Rule:    p.Get("C_RULE"),
		Reset:   "0",
	}
	md = highlightMarkdown(md, mc)

	// Write leading newline + result
	if _, err := fmt.Fprint(w, "\n"+md); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Remove the old styleLinks function and linkColors type**

In `internal/filter/html.go`:
- Remove the `linkColors` struct (lines 26-30)
- Remove the `styleLinks` function (lines 221-250)
- Remove `reLink` from the regex var block (line 57) - it's no longer used in the HTML path

Note: Keep `reLink` if it's used elsewhere. Check with `grep -r reLink internal/`. If only used in `styleLinks`, remove it.

- [ ] **Step 4: Update Plain() to remove cleanLinks parameter**

In `internal/filter/plain.go`, change the `Plain` function signature:

```go
func Plain(r io.Reader, w io.Writer, p *palette.Palette, cols int) error {
```

And update the call to `HTML` inside `Plain()`:

```go
	if detectHTML(text) {
		return HTML(strings.NewReader(text), w, p, cols)
	}
```

- [ ] **Step 5: Run vet to check for compilation errors**

Run: `go vet ./...`
Expected: PASS (no errors)

- [ ] **Step 6: Commit**

```bash
git add internal/filter/html.go internal/filter/plain.go
git commit -m "Wire footnote conversion into HTML pipeline, remove styleLinks"
```

---

### Task 4: Remove --clean-links flag from CLI

**Files:**
- Modify: `cmd/beautiful-aerc/html.go`
- Modify: `cmd/beautiful-aerc/plain.go`

- [ ] **Step 1: Remove --clean-links from html subcommand**

Replace `cmd/beautiful-aerc/html.go` with:

```go
package main

import (
	"os"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/spf13/cobra"
)

func newHTMLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "html",
		Short: "Convert HTML email to styled markdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadPalette()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.HTML(os.Stdin, os.Stdout, p, cols)
		},
	}
	return cmd
}
```

- [ ] **Step 2: Remove --clean-links from plain subcommand**

Replace `cmd/beautiful-aerc/plain.go` with:

```go
package main

import (
	"os"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/spf13/cobra"
)

func newPlainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plain",
		Short: "Format plain text email (reflow and colorize)",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadPalette()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.Plain(os.Stdin, os.Stdout, p, cols)
		},
	}
	return cmd
}
```

- [ ] **Step 3: Run vet and unit tests**

Run: `make check`
Expected: vet passes; unit tests pass; e2e tests may fail (golden files need updating)

- [ ] **Step 4: Commit**

```bash
git add cmd/beautiful-aerc/html.go cmd/beautiful-aerc/plain.go
git commit -m "Remove --clean-links flag, footnotes replace both link modes"
```

---

### Task 5: Update tests and golden files

**Files:**
- Modify: `internal/filter/html_test.go:151-196` (TestStyleLinks)
- Modify: `e2e/e2e_test.go` (no code changes, just regenerate golden files)
- Modify: `e2e/testdata/golden/*.txt` (regenerated)

- [ ] **Step 1: Remove TestStyleLinks from html_test.go**

Delete the `TestStyleLinks` function (lines 151-196) from `internal/filter/html_test.go`. The footnote tests in `footnotes_test.go` replace this coverage.

- [ ] **Step 2: Run unit tests**

Run: `go test ./internal/filter/ -v`
Expected: PASS (all tests in both html_test.go and footnotes_test.go)

- [ ] **Step 3: Update e2e golden files**

Run: `go test ./e2e/... -count=1 -update-golden`
Expected: Golden files regenerated with footnote-style links.

- [ ] **Step 4: Review the golden file diffs**

Run: `git diff e2e/testdata/golden/`
Expected: Links changed from `[text](url)` inline format to `text[^N]` body format with reference sections at the bottom.

- [ ] **Step 5: Run full test suite**

Run: `make check`
Expected: PASS (all vet, unit, and e2e tests)

- [ ] **Step 6: Commit**

```bash
git add internal/filter/html_test.go e2e/testdata/golden/
git commit -m "Update tests for footnote-style links, regenerate golden files"
```

---

### Task 6: Create pick-link subcommand

**Files:**
- Create: `internal/picker/picker.go`
- Create: `internal/picker/picker_test.go`

- [ ] **Step 1: Write failing tests for URL extraction**

Create `internal/picker/picker_test.go`:

```go
package picker

import (
	"testing"
)

func TestExtractURLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			"single URL",
			"Visit https://example.com today",
			[]string{"https://example.com"},
		},
		{
			"multiple URLs",
			"See https://a.com and https://b.com/path",
			[]string{"https://a.com", "https://b.com/path"},
		},
		{
			"deduplicates",
			"https://a.com then https://a.com again",
			[]string{"https://a.com"},
		},
		{
			"strips trailing punctuation",
			"Visit https://example.com.",
			[]string{"https://example.com"},
		},
		{
			"http and https",
			"http://old.com and https://new.com",
			[]string{"http://old.com", "https://new.com"},
		},
		{
			"no URLs",
			"just plain text",
			nil,
		},
		{
			"URL with query params",
			"Go to https://example.com/page?foo=bar&baz=1",
			[]string{"https://example.com/page?foo=bar&baz=1"},
		},
		{
			"strips ANSI codes from URLs",
			"Visit \033[38;2;97;110;136mhttps://example.com\033[0m today",
			[]string{"https://example.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractURLs(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("count: got %d, want %d\ngot: %v", len(got), len(tt.want), got)
				return
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("[%d]: got %q, want %q", i, got[i], want)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/picker/ -run TestExtractURLs -v`
Expected: FAIL - package does not exist

- [ ] **Step 3: Write ExtractURLs implementation**

Create `internal/picker/picker.go`:

```go
package picker

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/glw907/beautiful-aerc/internal/palette"
)

var (
	reURL  = regexp.MustCompile(`https?://[^\s>)\]]+`)
	reANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// ExtractURLs finds all unique URLs in text, preserving order.
// Strips ANSI escape codes and trailing punctuation.
func ExtractURLs(text string) []string {
	clean := reANSI.ReplaceAllString(text, "")
	matches := reURL.FindAllString(clean, -1)
	seen := make(map[string]bool)
	var urls []string
	for _, u := range matches {
		u = strings.TrimRight(u, ".,;:!?")
		if seen[u] {
			continue
		}
		seen[u] = true
		urls = append(urls, u)
	}
	return urls
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/picker/ -run TestExtractURLs -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/picker/picker.go internal/picker/picker_test.go
git commit -m "Add URL extraction for pick-link subcommand"
```

---

### Task 7: Implement the picker UI

**Files:**
- Modify: `internal/picker/picker.go`
- Modify: `internal/picker/picker_test.go`

- [ ] **Step 1: Write test for FormatLine**

Append to `internal/picker/picker_test.go`:

```go
func TestFormatLine(t *testing.T) {
	colors := &Colors{
		Number:   "\033[38;2;129;161;193m",
		URL:      "\033[38;2;97;110;136m",
		Selected: "\033[48;2;57;67;83m\033[38;2;229;233;240m",
		Reset:    "\033[0m",
	}

	t.Run("unselected", func(t *testing.T) {
		got := FormatLine(1, "https://example.com", false, colors)
		if !strings.Contains(got, "1") || !strings.Contains(got, "https://example.com") {
			t.Errorf("missing number or URL: %q", got)
		}
	})

	t.Run("selected", func(t *testing.T) {
		got := FormatLine(1, "https://example.com", true, colors)
		if !strings.Contains(got, colors.Selected) {
			t.Errorf("missing selected color: %q", got)
		}
	})

	t.Run("number 10 shows 0", func(t *testing.T) {
		got := FormatLine(10, "https://example.com", false, colors)
		if !strings.Contains(got, "0") {
			t.Errorf("10th item should show 0: %q", got)
		}
	})

	t.Run("number beyond 10 shows dash", func(t *testing.T) {
		got := FormatLine(11, "https://example.com", false, colors)
		if !strings.Contains(got, " ") && strings.Contains(got, "11") {
			t.Errorf("items beyond 10 should not show shortcut number: %q", got)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/picker/ -run TestFormatLine -v`
Expected: FAIL - `Colors` and `FormatLine` undefined

- [ ] **Step 3: Implement Colors and FormatLine**

Add to `internal/picker/picker.go`:

```go
// Colors holds ANSI escape sequences for the picker UI.
type Colors struct {
	Number   string // shortcut number (1-9, 0)
	URL      string // URL text
	Selected string // highlighted line (bg + fg)
	Reset    string
}

// ColorsFromPalette builds picker colors from a loaded palette.
func ColorsFromPalette(p *palette.Palette) *Colors {
	numColor, _ := palette.HexToANSI(p.Get("ACCENT_PRIMARY"))
	urlColor, _ := palette.HexToANSI(p.Get("FG_DIM"))
	selBG, _ := palette.HexToANSI(p.Get("BG_SELECTION"))
	selFG, _ := palette.HexToANSI(p.Get("FG_BRIGHT"))

	c := &Colors{Reset: "\033[0m"}
	if numColor != "" {
		c.Number = "\033[" + numColor + "m"
	}
	if urlColor != "" {
		c.URL = "\033[" + urlColor + "m"
	}
	// Selected: background color needs 48;2;r;g;b instead of 38;2;r;g;b
	if selBG != "" && selFG != "" {
		bgParam := strings.Replace(selBG, "38;2;", "48;2;", 1)
		c.Selected = "\033[" + bgParam + "m\033[" + selFG + "m"
	}
	return c
}

// FormatLine renders a single picker line with number, URL, and selection state.
func FormatLine(index int, url string, selected bool, colors *Colors) string {
	// Shortcut key: 1-9 for items 1-9, 0 for item 10, space for 11+
	shortcut := " "
	if index >= 1 && index <= 9 {
		shortcut = fmt.Sprintf("%d", index)
	} else if index == 10 {
		shortcut = "0"
	}

	if selected {
		return fmt.Sprintf("%s %s  %s%s", colors.Selected, shortcut, url, colors.Reset)
	}
	return fmt.Sprintf(" %s%s%s  %s%s%s", colors.Number, shortcut, colors.Reset, colors.URL, url, colors.Reset)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/picker/ -run TestFormatLine -v`
Expected: PASS

- [ ] **Step 5: Implement Run function**

Add to `internal/picker/picker.go`:

```go
// Run reads stdin, extracts URLs, and runs the interactive picker.
// Returns the selected URL or empty string if cancelled.
func Run(r io.Reader, w io.Writer, colors *Colors) (string, error) {
	input, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("reading input: %w", err)
	}

	urls := ExtractURLs(string(input))
	if len(urls) == 0 {
		return "", nil
	}

	// Set terminal to raw mode for single-keypress reading.
	oldState, err := makeRaw(os.Stdin.Fd())
	if err != nil {
		return "", fmt.Errorf("setting raw mode: %w", err)
	}
	defer restore(os.Stdin.Fd(), oldState)

	selected := 0
	render(w, urls, selected, colors)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "", nil
		}

		key := buf[:n]

		// 1-9: instant select
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			idx := int(key[0] - '0' - 1)
			if idx < len(urls) {
				return urls[idx], nil
			}
			continue
		}

		// 0: select 10th
		if len(key) == 1 && key[0] == '0' {
			if len(urls) >= 10 {
				return urls[9], nil
			}
			continue
		}

		// Enter: select current
		if len(key) == 1 && (key[0] == '\r' || key[0] == '\n') {
			return urls[selected], nil
		}

		// q or Escape: cancel
		if len(key) == 1 && (key[0] == 'q' || key[0] == 27) {
			return "", nil
		}

		// j or down arrow: next
		if (len(key) == 1 && key[0] == 'j') || (len(key) == 3 && key[0] == 27 && key[1] == '[' && key[2] == 'B') {
			if selected < len(urls)-1 {
				selected++
				render(w, urls, selected, colors)
			}
			continue
		}

		// k or up arrow: prev
		if (len(key) == 1 && key[0] == 'k') || (len(key) == 3 && key[0] == 27 && key[1] == '[' && key[2] == 'A') {
			if selected > 0 {
				selected--
				render(w, urls, selected, colors)
			}
			continue
		}
	}
}

// render clears and redraws the picker list to stderr.
func render(w io.Writer, urls []string, selected int, colors *Colors) {
	// Move cursor to top of list and clear.
	fmt.Fprint(w, "\033[2J\033[H")
	for i, u := range urls {
		fmt.Fprintln(w, FormatLine(i+1, u, i == selected, colors))
	}
}
```

- [ ] **Step 6: Add terminal raw mode helpers**

Add to `internal/picker/picker.go`:

```go
import "golang.org/x/sys/unix"

func makeRaw(fd uintptr) (*unix.Termios, error) {
	old, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	if err != nil {
		return nil, err
	}
	raw := *old
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(fd), unix.TCSETS, &raw); err != nil {
		return nil, err
	}
	return old, nil
}

func restore(fd uintptr, state *unix.Termios) {
	unix.IoctlSetTermios(int(fd), unix.TCSETS, state)
}
```

Run: `go get golang.org/x/sys` to add the dependency.

- [ ] **Step 7: Run vet**

Run: `go vet ./...`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/picker/picker.go internal/picker/picker_test.go go.mod go.sum
git commit -m "Add interactive link picker with themed UI and vim navigation"
```

---

### Task 8: Wire pick-link subcommand into CLI

**Files:**
- Create: `cmd/beautiful-aerc/picklink.go`
- Modify: `cmd/beautiful-aerc/root.go`

- [ ] **Step 1: Create picklink.go**

Create `cmd/beautiful-aerc/picklink.go`:

```go
package main

import (
	"os"

	"github.com/glw907/beautiful-aerc/internal/picker"
	"github.com/spf13/cobra"
)

func newPickLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pick-link",
		Short: "Interactive URL picker for email messages",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadPalette()
			if err != nil {
				return err
			}
			colors := picker.ColorsFromPalette(p)
			url, err := picker.Run(os.Stdin, os.Stderr, colors)
			if err != nil {
				return err
			}
			if url != "" {
				os.Stdout.WriteString(url + "\n")
			}
			return nil
		},
	}
	return cmd
}
```

- [ ] **Step 2: Register in root command**

In `cmd/beautiful-aerc/root.go`, add the subcommand:

```go
func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "beautiful-aerc",
		Short:        "Themeable filters for the aerc email client",
		SilenceUsage: true,
	}
	cmd.AddCommand(newHeadersCmd())
	cmd.AddCommand(newHTMLCmd())
	cmd.AddCommand(newPlainCmd())
	cmd.AddCommand(newPickLinkCmd())
	return cmd
}
```

- [ ] **Step 3: Run vet and build**

Run: `go vet ./... && go build -o /dev/null ./cmd/beautiful-aerc`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/beautiful-aerc/picklink.go cmd/beautiful-aerc/root.go
git commit -m "Add pick-link subcommand to CLI"
```

---

### Task 9: Add Tab keybinding to binds.conf

**Files:**
- Modify: `.config/aerc/binds.conf:92-104` ([view] section)

- [ ] **Step 1: Add Tab keybinding**

In `.config/aerc/binds.conf`, in the `[view]` section (after the existing `<C-l> = :open-link <space>` on line 104), add:

```ini
<Tab> = :menu -dc 'beautiful-aerc pick-link' :open-link 
```

The `[view]` section should now look like:

```ini
[view]
/ = :toggle-key-passthrough<Enter>/
q = :close<Enter>
O = :open<Enter>
o = :open<Enter>
S = :save<space>
| = :pipe<space>
d = :delete<Enter>:next<Enter>
D = :delete<Enter>
a = :archive flat<Enter>:next<Enter>
A = :archive flat<Enter>

<C-l> = :open-link <space>
<Tab> = :menu -dc 'beautiful-aerc pick-link' :open-link 
```

- [ ] **Step 2: Commit**

```bash
git add .config/aerc/binds.conf
git commit -m "Add Tab keybinding for pick-link URL opener"
```

---

### Task 10: Update documentation

**Files:**
- Modify: `docs/filters.md`
- Modify: `CLAUDE.md`
- Modify: `README.md`

- [ ] **Step 1: Update filters.md**

Update the link display modes section in `docs/filters.md` to describe footnote-style links instead of inline/clean modes. Document the `pick-link` subcommand and Tab keybinding.

- [ ] **Step 2: Update README.md**

Replace the "Link display modes" section (lines 106-126) with footnote-style link description. Remove `--clean-links` references. Add pick-link description.

- [ ] **Step 3: Update CLAUDE.md**

Remove any `--clean-links` references in the aerc filter protocol section if present.

- [ ] **Step 4: Commit**

```bash
git add docs/filters.md README.md CLAUDE.md
git commit -m "Update docs for footnote-style links and pick-link"
```

---

### Task 11: Final integration test

**Files:**
- No file changes - verification only

- [ ] **Step 1: Run full test suite**

Run: `make check`
Expected: PASS

- [ ] **Step 2: Install and test manually**

Run: `make install`

Test with a sample HTML email:

```bash
echo '<html><body><p>If you dont recognize this account, <a href="https://accounts.google.com/AccountDisavow?adt=AOX8kiq4Deg5aiOFNwPNb3evrPh8y8OcrJxrSet-oBnt">remove</a> it.</p><p><a href="https://accounts.google.com/AccountChooser?Email=geoff@test.com">Check activity</a></p><p>See <a href="https://myaccount.google.com/notifications">https://myaccount.google.com/notifications</a></p></body></html>' | AERC_CONFIG=~/.config/aerc beautiful-aerc html
```

Expected output:
```
If you dont recognize this account, remove[^1] it.

Check activity[^2]

See https://myaccount.google.com/notifications
────────────────────────────────────────────────────────────────────────────────
[^1]: https://accounts.google.com/AccountDisavow?adt=AOX8kiq4Deg5aiOFNwPNb3evrPh8y8OcrJxrSet-oBnt
[^2]: https://accounts.google.com/AccountChooser?Email=geoff@test.com
```

With link text colored and footnote markers dimmed.

- [ ] **Step 3: Test pick-link**

```bash
echo 'Visit https://example.com and https://google.com' | beautiful-aerc pick-link
```

Expected: Interactive picker appears with numbered URLs. Press 1 to select first URL, prints it to stdout.

- [ ] **Step 4: Test in live aerc**

```bash
tmux kill-session -t test 2>/dev/null
tmux new-session -d -s test -x 140 -y 40 'aerc'
sleep 8
tmux send-keys -t test Enter
sleep 3
tmux capture-pane -t test -p | head -20
```

Verify footnote-style links render in the message viewer. Then:

```bash
tmux send-keys -t test Tab
sleep 2
tmux capture-pane -t test -p
```

Verify the pick-link picker appears with themed colors.

```bash
tmux kill-session -t test
```
