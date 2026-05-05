# Catkin Core (Pass 9) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land Catkin's core editing primitive — package skeleton, block-aware reflow, basic plain-text rendering, word-level navigation, scroll-off — wrapping `bubbles/textarea` as the buffer + cursor + edit-op primitive. No styling, no markdown commands beyond what textarea provides natively, no compose wiring. Subsequent sub-passes (9a–9i per the spec) layer styling, commands, QoL, spellcheck, compose, SMTP, and tidy on top.

**Architecture:** Catkin is a `tea.Model` in `internal/catkin/`, library-pure (depends only on `bubbletea`, `bubbles`, `lipgloss`, `muesli/reflow`). Composed of small focused files: `catkin.go` (Model + tea.Model contract), `buffer.go` (textarea wrapper), `blocks.go` (block classifier — pure function), `reflow.go` (block-aware paragraph reflow — pure function), `render.go` (Catkin's own plain renderer; textarea's `View()` is **not** used), `wordnav.go` (Ctrl-arrow word motion supplement), `scrolloff.go` (viewport clamp). Each file ≤ ~150 LOC.

**Tech Stack:** Go 1.26.1, `charmbracelet/bubbletea`, `charmbracelet/bubbles/textarea`, `charmbracelet/lipgloss`, `muesli/reflow`. No new direct dependencies.

**Spec reference:** `docs/superpowers/specs/2026-05-04-compose-design.md` §Architecture, §Catkin internals, §Right-sized passes (Pass 9 row).

**Out of scope for this plan (later sub-passes):** live markdown styling (9a), markdown commands `Ctrl+B/I/K/L/Q` + smart Enter + Tab indent (9b), undo/redo + find/replace + auto-pair + smart paste + bracket match + typewriter + focus mode (9c), annotations + spellcheck (9d), compose package + AssembleMIME (9e), mail backend Send/Append (9f), cache outbox (9g), ComposeTab UI (9h), Claude Tidy impl (9i).

---

## File map

```
internal/catkin/
  catkin.go         Model struct + tea.Model contract (Init/Update/View) + accessors
  buffer.go         Buffer wraps textarea.Model with rune-offset cursor helpers
  blocks.go         BlockKind enum + LineContext + Classify(lines) []LineContext
  reflow.go         Reflow(src string, width int, oldCursor int) (string, int)
  render.go         Render(src, ctx, viewport, width, cursor) string  -- plain text + cursor
  wordnav.go        Ctrl-Left/Right/Backspace/Delete word-motion supplement
  scrolloff.go      ClampViewport(top, height, cursorLine, total) int

  catkin_test.go    Smoke + Model contract tests
  blocks_test.go    Classifier table tests
  reflow_test.go    Round-trip + idempotency + cursor-tracking tests
  render_test.go    Plain-render snapshot tests
  wordnav_test.go   Word-motion table tests
  scrolloff_test.go Clamp tests
```

No edits to other packages in this pass — Catkin stands alone. ComposeTab wiring lands in 9h.

---

## Task 1: Package skeleton + types

**Files:**
- Create: `internal/catkin/catkin.go`
- Create: `internal/catkin/catkin_test.go`

- [ ] **Step 1: Write the failing smoke test**

`internal/catkin/catkin_test.go`:

```go
package catkin

import "testing"

func TestNewReturnsUsableModel(t *testing.T) {
	m := New()
	if got := m.Value(); got != "" {
		t.Errorf("new model: Value() = %q, want empty", got)
	}
	m.SetValue("hello")
	if got := m.Value(); got != "hello" {
		t.Errorf("after SetValue: Value() = %q, want %q", got, "hello")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/catkin/ -run TestNewReturnsUsableModel -v
```

Expected: FAIL — `package catkin` doesn't exist yet.

- [ ] **Step 3: Write minimal Model + constructor**

`internal/catkin/catkin.go`:

```go
// Package catkin is poplar's markdown-first bubbletea editor.
//
// Catkin wraps bubbles/textarea as its buffer + cursor + edit-op
// primitive, but owns its own renderer so live markdown styling
// and block-aware reflow can drive the display directly from the
// raw source without parsing textarea's ANSI output.
//
// This package depends only on bubbletea, bubbles, lipgloss, and
// muesli/reflow. It has no poplar-specific imports and is
// extractable as github.com/glw907/catkin.
package catkin

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Model is Catkin's tea.Model. Construct with New.
type Model struct {
	buf     Buffer
	width   int
	height  int
	focused bool
}

// New returns a Model with default settings.
func New() Model {
	return Model{
		buf: NewBuffer(textarea.New()),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.buf, cmd = m.buf.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	// Replaced in Task 5. For now defer to buffer's View() so the
	// smoke test compiles.
	return m.buf.View()
}

// Value returns the raw markdown source.
func (m Model) Value() string { return m.buf.Value() }

// SetValue replaces the buffer with s.
func (m *Model) SetValue(s string) { m.buf.SetValue(s) }

// SetSize sets the editor's display dimensions.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.buf.SetWidth(w)
	m.buf.SetHeight(h)
}

// SetWidth sets the body wrap width and re-runs reflow.
func (m *Model) SetWidth(w int) {
	m.width = w
	m.buf.SetWidth(w)
	// Reflow wiring added in Task 3.
}

// Focus focuses the editor.
func (m Model) Focus() tea.Cmd { return m.buf.Focus() }

// Blur blurs the editor.
func (m *Model) Blur() { m.buf.Blur() }

// Focused reports whether the editor has focus.
func (m Model) Focused() bool { return m.buf.Focused() }
```

- [ ] **Step 4: Stub `Buffer` with the minimal surface so the package compiles**

`internal/catkin/buffer.go`:

```go
package catkin

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Buffer wraps a bubbles/textarea.Model. Catkin uses textarea for
// its buffer storage, cursor management, and edit operations; the
// renderer is Catkin's own (see render.go).
type Buffer struct {
	ta textarea.Model
}

// NewBuffer wraps an existing textarea.Model.
func NewBuffer(ta textarea.Model) Buffer { return Buffer{ta: ta} }

func (b Buffer) Update(msg tea.Msg) (Buffer, tea.Cmd) {
	var cmd tea.Cmd
	b.ta, cmd = b.ta.Update(msg)
	return b, cmd
}

func (b Buffer) View() string  { return b.ta.View() }
func (b Buffer) Value() string { return b.ta.Value() }

func (b *Buffer) SetValue(s string) { b.ta.SetValue(s) }
func (b *Buffer) SetWidth(w int)    { b.ta.SetWidth(w) }
func (b *Buffer) SetHeight(h int)   { b.ta.SetHeight(h) }

func (b Buffer) Focus() tea.Cmd { return b.ta.Focus() }
func (b *Buffer) Blur()         { b.ta.Blur() }
func (b Buffer) Focused() bool  { return b.ta.Focused() }
```

- [ ] **Step 5: Run test to verify it passes**

```
go test ./internal/catkin/ -run TestNewReturnsUsableModel -v
```

Expected: PASS.

- [ ] **Step 6: Run full vet + test gate**

```
make check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```
git add internal/catkin/catkin.go internal/catkin/buffer.go internal/catkin/catkin_test.go
git commit -m "Pass 9 task 1: catkin package skeleton + Model/Buffer types"
```

---

## Task 2: Block classifier — types + table tests

**Files:**
- Create: `internal/catkin/blocks.go`
- Create: `internal/catkin/blocks_test.go`

The classifier is a pure function: given a slice of source lines, return a parallel slice of `LineContext` describing each line's block role. Single-pass with a small state machine for fenced-code regions; everything else (quote depth, list marker, heading) reads from the line itself.

- [ ] **Step 1: Write the failing test table**

`internal/catkin/blocks_test.go`:

```go
package catkin

import (
	"reflect"
	"testing"
)

func TestClassifyTable(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []LineContext
	}{
		{
			name:  "blank line",
			lines: []string{""},
			want:  []LineContext{{Kind: BlockBlank}},
		},
		{
			name:  "plain paragraph",
			lines: []string{"hello world"},
			want:  []LineContext{{Kind: BlockParagraph, PostPrefix: "hello world"}},
		},
		{
			name:  "ATX heading h1",
			lines: []string{"# Title"},
			want:  []LineContext{{Kind: BlockHeading, HeadingLevel: 1, PostPrefix: "Title"}},
		},
		{
			name:  "ATX heading h3",
			lines: []string{"### Sub"},
			want:  []LineContext{{Kind: BlockHeading, HeadingLevel: 3, PostPrefix: "Sub"}},
		},
		{
			name:  "single quote prefix",
			lines: []string{"> hi"},
			want:  []LineContext{{Kind: BlockQuote, QuoteDepth: 1, PrefixWidth: 2, PostPrefix: "hi"}},
		},
		{
			name:  "nested quote depth 2",
			lines: []string{"> > nested"},
			want:  []LineContext{{Kind: BlockQuote, QuoteDepth: 2, PrefixWidth: 4, PostPrefix: "nested"}},
		},
		{
			name:  "dash list",
			lines: []string{"- item one"},
			want:  []LineContext{{Kind: BlockListItem, ListMarker: "-", PrefixWidth: 2, PostPrefix: "item one"}},
		},
		{
			name:  "ordered list",
			lines: []string{"1. first"},
			want:  []LineContext{{Kind: BlockListItem, ListMarker: "1.", PrefixWidth: 3, PostPrefix: "first"}},
		},
		{
			name:  "task list unchecked",
			lines: []string{"- [ ] todo"},
			want:  []LineContext{{Kind: BlockTaskItem, ListMarker: "- [ ]", PrefixWidth: 6, PostPrefix: "todo"}},
		},
		{
			name:  "task list checked",
			lines: []string{"- [x] done"},
			want:  []LineContext{{Kind: BlockTaskItem, ListMarker: "- [x]", PrefixWidth: 6, PostPrefix: "done"}},
		},
		{
			name:  "fenced code open and close",
			lines: []string{"```", "code", "```"},
			want: []LineContext{
				{Kind: BlockCodeFence, InsideFence: false, PostPrefix: "```"},
				{Kind: BlockCodeFence, InsideFence: true, PostPrefix: "code"},
				{Kind: BlockCodeFence, InsideFence: false, PostPrefix: "```"},
			},
		},
		{
			name:  "indented code (4-space)",
			lines: []string{"    indented"},
			want:  []LineContext{{Kind: BlockCodeIndent, PrefixWidth: 4, PostPrefix: "indented"}},
		},
		{
			name:  "table header + separator",
			lines: []string{"| a | b |", "| --- | --- |"},
			want: []LineContext{
				{Kind: BlockTable, PostPrefix: "| a | b |"},
				{Kind: BlockTable, PostPrefix: "| --- | --- |"},
			},
		},
		{
			name:  "quoted heading",
			lines: []string{"> # heading in quote"},
			want:  []LineContext{{Kind: BlockHeading, QuoteDepth: 1, HeadingLevel: 1, PrefixWidth: 2, PostPrefix: "heading in quote"}},
		},
		{
			name:  "quoted list",
			lines: []string{"> - item"},
			want:  []LineContext{{Kind: BlockListItem, QuoteDepth: 1, ListMarker: "-", PrefixWidth: 4, PostPrefix: "item"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.lines)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Classify(%q):\ngot  %#v\nwant %#v", tt.lines, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/catkin/ -run TestClassifyTable -v
```

Expected: FAIL — `BlockKind`, `LineContext`, `Classify` undefined.

- [ ] **Step 3: Implement types and classifier**

`internal/catkin/blocks.go`:

```go
package catkin

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// BlockKind identifies the markdown block role of a single line.
type BlockKind int

const (
	BlockBlank BlockKind = iota
	BlockParagraph
	BlockHeading
	BlockQuote
	BlockListItem
	BlockTaskItem
	BlockCodeFence  // line is a fence marker (``` etc.)
	BlockCodeIndent // 4-space indented code line
	BlockTable
)

// LineContext describes a single line's block role.
type LineContext struct {
	Kind         BlockKind
	QuoteDepth   int    // count of leading '>' levels (0 = not quoted)
	ListMarker   string // "-", "*", "+", "1.", "- [ ]", etc.; "" if not a list
	HeadingLevel int    // 1..6 for ATX; 0 if not a heading
	InsideFence  bool   // line is between (not on) two fence markers
	PrefixWidth  int    // width in cells of quote+list prefix; 0 if no prefix
	PostPrefix   string // line content after quote+list prefix
}

var (
	atxHeadingRE   = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	taskItemRE     = regexp.MustCompile(`^([-*+])\s+(\[[ xX]\])\s+(.*)$`)
	dashListRE     = regexp.MustCompile(`^([-*+])\s+(.*)$`)
	orderedListRE  = regexp.MustCompile(`^(\d+\.)\s+(.*)$`)
	tableSeparator = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)+\|?\s*$`)
	tableRowRE     = regexp.MustCompile(`^\s*\|.*\|\s*$`)
)

// Classify returns a LineContext for each line in lines, in order.
func Classify(lines []string) []LineContext {
	out := make([]LineContext, len(lines))
	insideFence := false

	for i, raw := range lines {
		line := raw
		ctx := LineContext{}

		// Quote prefix walk.
		for {
			s := strings.TrimPrefix(line, ">")
			if s == line {
				break
			}
			ctx.QuoteDepth++
			ctx.PrefixWidth++
			s = strings.TrimPrefix(s, " ")
			if len(s) != len(strings.TrimPrefix(line, ">")) {
				ctx.PrefixWidth++
			}
			line = s
		}

		// Inside a fenced code region: everything is code.
		if insideFence {
			if isFenceMarker(line) {
				ctx.Kind = BlockCodeFence
				ctx.InsideFence = false
				ctx.PostPrefix = line
				insideFence = false
			} else {
				ctx.Kind = BlockCodeFence
				ctx.InsideFence = true
				ctx.PostPrefix = line
			}
			out[i] = ctx
			continue
		}

		// Fence opener.
		if isFenceMarker(line) {
			ctx.Kind = BlockCodeFence
			ctx.PostPrefix = line
			insideFence = true
			out[i] = ctx
			continue
		}

		// Blank.
		if strings.TrimSpace(line) == "" {
			ctx.Kind = BlockBlank
			out[i] = ctx
			continue
		}

		// Indented code (only when not in a list/quote — keep simple).
		if ctx.QuoteDepth == 0 && strings.HasPrefix(line, "    ") {
			ctx.Kind = BlockCodeIndent
			ctx.PrefixWidth = 4
			ctx.PostPrefix = strings.TrimPrefix(line, "    ")
			out[i] = ctx
			continue
		}

		// ATX heading.
		if m := atxHeadingRE.FindStringSubmatch(line); m != nil {
			ctx.Kind = BlockHeading
			ctx.HeadingLevel = len(m[1])
			ctx.PostPrefix = m[2]
			out[i] = ctx
			continue
		}

		// Task list (must come before plain dash list).
		if m := taskItemRE.FindStringSubmatch(line); m != nil {
			ctx.Kind = BlockTaskItem
			ctx.ListMarker = m[1] + " " + m[2]
			ctx.PrefixWidth += utf8.RuneCountInString(ctx.ListMarker) + 1
			ctx.PostPrefix = m[3]
			out[i] = ctx
			continue
		}

		// Dash/star/plus list.
		if m := dashListRE.FindStringSubmatch(line); m != nil {
			ctx.Kind = BlockListItem
			ctx.ListMarker = m[1]
			ctx.PrefixWidth += utf8.RuneCountInString(m[1]) + 1
			ctx.PostPrefix = m[2]
			out[i] = ctx
			continue
		}

		// Ordered list.
		if m := orderedListRE.FindStringSubmatch(line); m != nil {
			ctx.Kind = BlockListItem
			ctx.ListMarker = m[1]
			ctx.PrefixWidth += utf8.RuneCountInString(m[1]) + 1
			ctx.PostPrefix = m[2]
			out[i] = ctx
			continue
		}

		// Table row (header + separator detected as a unit; classifier
		// flags any line that matches the row shape — paragraph context
		// distinguishes false positives from real tables).
		if tableRowRE.MatchString(line) {
			ctx.Kind = BlockTable
			ctx.PostPrefix = line
			out[i] = ctx
			continue
		}

		// Default: paragraph.
		ctx.Kind = BlockParagraph
		ctx.PostPrefix = line
		out[i] = ctx
	}

	return out
}

func isFenceMarker(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}
```

- [ ] **Step 4: Run test, fix failures iteratively until all cases pass**

```
go test ./internal/catkin/ -run TestClassifyTable -v
```

Expected: PASS for all 16 sub-tests. The quote-prefix walk is the trickiest part — verify `> > nested` produces `QuoteDepth: 2, PrefixWidth: 4`.

- [ ] **Step 5: Run full check**

```
make check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```
git add internal/catkin/blocks.go internal/catkin/blocks_test.go
git commit -m "Pass 9 task 2: catkin block classifier + table tests"
```

---

## Task 3: Reflow engine — block-aware paragraph reflow

**Files:**
- Create: `internal/catkin/reflow.go`
- Create: `internal/catkin/reflow_test.go`

Reflow is a pure function. Given a source string, a wrap width, and a cursor rune offset, return a new source with paragraphs/quotes rewrapped, plus an adjusted cursor offset that points at the same logical character.

Algorithm:
1. Split into lines.
2. Classify each line.
3. Group consecutive lines with the same `(QuoteDepth, BlockKind, ListMarker)` into paragraph blocks.
4. For each paragraph block:
   - If `Kind` is `BlockHeading`, `BlockTable`, `BlockCodeFence` (any), `BlockCodeIndent`, or `BlockBlank`: copy verbatim.
   - Else (paragraph, list item, quote): tokenize post-prefix content by whitespace; never split a token; re-emit with the prefix and `width - PrefixWidth` budget.
5. Reassemble lines. Track cursor position by counting characters into the original buffer and emitting a matching offset in the new buffer.

- [ ] **Step 1: Write failing tests**

`internal/catkin/reflow_test.go`:

```go
package catkin

import "testing"

func TestReflowParagraphSimple(t *testing.T) {
	src := "the quick brown fox jumps over the lazy dog"
	got, _ := Reflow(src, 20, 0)
	want := "the quick brown fox\njumps over the lazy\ndog"
	if got != want {
		t.Errorf("Reflow:\ngot  %q\nwant %q", got, want)
	}
}

func TestReflowPreservesQuotePrefix(t *testing.T) {
	src := "> the quick brown fox jumps over the lazy dog"
	got, _ := Reflow(src, 20, 0)
	want := "> the quick brown\n> fox jumps over the\n> lazy dog"
	if got != want {
		t.Errorf("Reflow quoted:\ngot  %q\nwant %q", got, want)
	}
}

func TestReflowSkipsCodeFence(t *testing.T) {
	src := "```\nthis is code with very long lines that should not wrap\n```"
	got, _ := Reflow(src, 20, 0)
	if got != src {
		t.Errorf("Reflow should preserve code fence:\ngot  %q\nwant %q", got, src)
	}
}

func TestReflowSkipsHeading(t *testing.T) {
	src := "# A long heading that exceeds the wrap width"
	got, _ := Reflow(src, 20, 0)
	if got != src {
		t.Errorf("Reflow should preserve heading:\ngot  %q\nwant %q", got, src)
	}
}

func TestReflowNeverBreaksLongToken(t *testing.T) {
	src := "see https://example.com/very/long/path/that/exceeds/wrap/width here"
	got, _ := Reflow(src, 20, 0)
	// "see" (3) + " " + URL (>20) — URL on its own line, longer than 20.
	want := "see\nhttps://example.com/very/long/path/that/exceeds/wrap/width\nhere"
	if got != want {
		t.Errorf("Reflow long-token:\ngot  %q\nwant %q", got, want)
	}
}

func TestReflowIdempotent(t *testing.T) {
	src := "the quick brown fox jumps over the lazy dog"
	once, _ := Reflow(src, 20, 0)
	twice, _ := Reflow(once, 20, 0)
	if once != twice {
		t.Errorf("Reflow not idempotent:\nonce  %q\ntwice %q", once, twice)
	}
}

func TestReflowCursorTracking(t *testing.T) {
	// Cursor at offset 10 ("brown" begins at 10 in the source).
	src := "the quick brown fox"
	got, cur := Reflow(src, 10, 10)
	// Source wraps to "the quick\nbrown fox"; "brown" still at offset 10
	// (newline counts as 1 char, replacing the space).
	want := "the quick\nbrown fox"
	if got != want {
		t.Errorf("Reflow cursor: got %q, want %q", got, want)
	}
	if cur != 10 {
		t.Errorf("Reflow cursor offset: got %d, want 10", cur)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./internal/catkin/ -run TestReflow -v
```

Expected: FAIL — `Reflow` undefined.

- [ ] **Step 3: Implement Reflow**

`internal/catkin/reflow.go`:

```go
package catkin

import (
	"strings"
	"unicode/utf8"
)

// Reflow rewraps paragraphs and quoted blocks to width while
// preserving headings, code fences, indented code, tables, and
// blank lines. Long single tokens (URLs longer than width) are
// emitted on their own line and exceed width — display-level wrap
// in render.go handles them visually.
//
// Reflow returns the new source plus an adjusted cursor offset
// (rune offset into the buffer) that points at the same logical
// character. If oldCursor is past the end of src, the new cursor
// snaps to the new end.
func Reflow(src string, width int, oldCursor int) (string, int) {
	if width <= 0 {
		return src, oldCursor
	}
	lines := strings.Split(src, "\n")
	ctx := Classify(lines)

	var out []string
	// Track absolute char offsets so we can map oldCursor → newCursor.
	srcOffset := 0
	newOffset := 0
	newCursor := -1

	i := 0
	for i < len(lines) {
		end := groupEnd(lines, ctx, i)
		group := lines[i:end]
		groupCtx := ctx[i:end]

		// Compute the source range covered by this group.
		groupStart := srcOffset
		groupChars := 0
		for j, l := range group {
			groupChars += utf8.RuneCountInString(l)
			if j < len(group)-1 || end < len(lines) {
				groupChars++ // newline
			}
		}

		var emitted []string
		if isPreservedBlock(groupCtx[0]) {
			emitted = group
		} else {
			emitted = reflowGroup(group, groupCtx, width)
		}

		// Map cursor if it falls in this group.
		if newCursor < 0 && oldCursor >= groupStart && oldCursor <= groupStart+groupChars {
			rel := oldCursor - groupStart
			rel = remapCursor(group, emitted, rel)
			newCursor = newOffset + rel
		}

		// Append to output, count chars.
		for _, l := range emitted {
			out = append(out, l)
			newOffset += utf8.RuneCountInString(l) + 1 // +1 for newline
		}
		// Last group has no trailing newline.
		if end >= len(lines) {
			newOffset-- // remove the imagined trailing newline
		}

		srcOffset = groupStart + groupChars
		i = end
	}

	if newCursor < 0 {
		newCursor = newOffset
	}
	return strings.Join(out, "\n"), newCursor
}

// groupEnd returns the exclusive end index of the paragraph group
// starting at i. Lines belong to the same group if they share
// (Kind, QuoteDepth, ListMarker) — a blank line ends the group.
func groupEnd(lines []string, ctx []LineContext, i int) int {
	first := ctx[i]
	if first.Kind == BlockBlank {
		return i + 1
	}
	for j := i + 1; j < len(lines); j++ {
		if ctx[j].Kind == BlockBlank {
			return j
		}
		if ctx[j].Kind != first.Kind ||
			ctx[j].QuoteDepth != first.QuoteDepth ||
			ctx[j].ListMarker != first.ListMarker {
			return j
		}
	}
	return len(lines)
}

func isPreservedBlock(c LineContext) bool {
	switch c.Kind {
	case BlockHeading, BlockTable, BlockCodeFence, BlockCodeIndent, BlockBlank:
		return true
	}
	return false
}

// reflowGroup re-wraps a group of lines that share a prefix.
func reflowGroup(lines []string, ctx []LineContext, width int) []string {
	prefix := buildPrefix(ctx[0])
	budget := width - utf8.RuneCountInString(prefix)
	if budget < 1 {
		budget = 1
	}

	// Concatenate post-prefix content of all lines.
	var words []string
	for i, l := range lines {
		_ = l
		words = append(words, strings.Fields(ctx[i].PostPrefix)...)
	}

	if len(words) == 0 {
		return []string{prefix}
	}

	var out []string
	current := prefix
	currentLen := utf8.RuneCountInString(prefix)
	for _, w := range words {
		wLen := utf8.RuneCountInString(w)
		if current == prefix {
			// First token on a new line — always accept, even if too long.
			current += w
			currentLen += wLen
			continue
		}
		if currentLen+1+wLen <= utf8.RuneCountInString(prefix)+budget {
			current += " " + w
			currentLen += 1 + wLen
		} else {
			out = append(out, current)
			current = prefix + w
			currentLen = utf8.RuneCountInString(prefix) + wLen
		}
	}
	out = append(out, current)
	return out
}

// buildPrefix reconstructs the leading prefix (quote levels + list
// marker + space) for emission.
func buildPrefix(c LineContext) string {
	var sb strings.Builder
	for d := 0; d < c.QuoteDepth; d++ {
		sb.WriteString("> ")
	}
	if c.ListMarker != "" {
		sb.WriteString(c.ListMarker)
		sb.WriteString(" ")
	}
	return sb.String()
}

// remapCursor takes a relative offset into the original group's
// concatenated text and returns the equivalent offset into the
// emitted group's concatenated text. Approximation: counts only
// non-whitespace chars; whitespace boundaries shift but content
// chars stay aligned.
func remapCursor(orig, emitted []string, rel int) int {
	if len(orig) == 0 || len(emitted) == 0 {
		return rel
	}
	origJoined := strings.Join(orig, "\n")
	emitJoined := strings.Join(emitted, "\n")
	// Count non-whitespace chars up to rel in origJoined.
	nonWS := 0
	for i, r := range origJoined {
		if i >= rel {
			break
		}
		if !isReflowWS(r) {
			nonWS++
		}
	}
	// Find the offset in emitJoined that has the same non-WS count.
	count := 0
	for i, r := range emitJoined {
		if count >= nonWS {
			return i
		}
		if !isReflowWS(r) {
			count++
		}
	}
	return utf8.RuneCountInString(emitJoined)
}

func isReflowWS(r rune) bool { return r == ' ' || r == '\n' || r == '\t' }
```

- [ ] **Step 4: Run tests, iterate until pass**

```
go test ./internal/catkin/ -run TestReflow -v
```

Expected: PASS for all seven sub-tests. Cursor tracking is the trickiest — debug `TestReflowCursorTracking` first if it fails.

- [ ] **Step 5: Wire reflow into Model.SetWidth**

Edit `internal/catkin/catkin.go`, replace the `SetWidth` body with:

```go
func (m *Model) SetWidth(w int) {
	if w == m.width {
		return
	}
	m.width = w
	m.buf.SetWidth(w)
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	src, cur = Reflow(src, w, cur)
	m.buf.SetValue(src)
	m.buf.SetRuneOffset(cur)
}
```

This requires `Buffer.RuneOffset` and `Buffer.SetRuneOffset` — add to `buffer.go`:

```go
// RuneOffset returns the cursor's absolute rune offset into the
// buffer.
func (b Buffer) RuneOffset() int {
	row, col := b.ta.Line(), b.ta.LineInfo().ColumnOffset
	value := b.ta.Value()
	lines := strings.SplitN(value, "\n", row+2)
	off := 0
	for i := 0; i < row && i < len(lines); i++ {
		off += utf8.RuneCountInString(lines[i]) + 1 // +1 for newline
	}
	off += col
	return off
}

// SetRuneOffset moves the cursor to the given absolute rune offset.
func (b *Buffer) SetRuneOffset(off int) {
	value := b.ta.Value()
	row := 0
	col := off
	for _, l := range strings.Split(value, "\n") {
		ln := utf8.RuneCountInString(l)
		if col <= ln {
			break
		}
		col -= ln + 1
		row++
	}
	b.ta.SetCursor(col)
	for b.ta.Line() < row {
		b.ta.CursorDown()
	}
	for b.ta.Line() > row {
		b.ta.CursorUp()
	}
}
```

Add `import "strings"` and `import "unicode/utf8"` to `buffer.go` if not already present.

- [ ] **Step 6: Run full check**

```
make check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```
git add internal/catkin/reflow.go internal/catkin/reflow_test.go internal/catkin/catkin.go internal/catkin/buffer.go
git commit -m "Pass 9 task 3: catkin block-aware reflow + cursor tracking"
```

---

## Task 4: Buffer paragraph helpers

**Files:**
- Modify: `internal/catkin/buffer.go`
- Create: `internal/catkin/buffer_test.go`

The renderer (Task 5) needs `(line, col)` cursor coordinates. Textarea exposes `Line()` and `LineInfo()` already. Add a thin helper for the paragraph the cursor sits in (used by 9c focus mode and 9b smart Enter; defining it here keeps it owned by the buffer).

- [ ] **Step 1: Write failing test**

`internal/catkin/buffer_test.go`:

```go
package catkin

import "testing"

func TestBufferRuneOffsetRoundTrip(t *testing.T) {
	b := NewBuffer(newEmptyTextarea())
	b.SetValue("hello\nworld\nfoo")
	b.SetRuneOffset(7) // "world"[1] == 'o'
	if got := b.RuneOffset(); got != 7 {
		t.Errorf("RuneOffset round-trip: got %d, want 7", got)
	}
}

func TestBufferRuneOffsetClampsPastEnd(t *testing.T) {
	b := NewBuffer(newEmptyTextarea())
	b.SetValue("abc")
	b.SetRuneOffset(100)
	if got := b.RuneOffset(); got > 3 {
		t.Errorf("RuneOffset past-end: got %d, want ≤3", got)
	}
}
```

Helper at the bottom of `buffer_test.go`:

```go
import "github.com/charmbracelet/bubbles/textarea"

func newEmptyTextarea() textarea.Model {
	t := textarea.New()
	t.SetWidth(40)
	t.SetHeight(10)
	return t
}
```

- [ ] **Step 2: Run test, verify failure**

```
go test ./internal/catkin/ -run TestBuffer -v
```

Expected: depends on Task 3's `RuneOffset` impl quality. If it failed, fix it now (the row/col → offset math is fragile).

- [ ] **Step 3: Iterate the impl until tests pass**

Likely fixes:
- Use `b.ta.LineInfo().ColumnOffset` carefully — it's the **column in the wrapped display line**, not the source line. For Catkin we need source-line column. May need to track via `b.ta.Line()` plus walking the value manually.

Replace `RuneOffset` with a more robust impl:

```go
func (b Buffer) RuneOffset() int {
	row := b.ta.Line()
	value := b.ta.Value()
	lines := strings.Split(value, "\n")
	off := 0
	for i := 0; i < row && i < len(lines); i++ {
		off += utf8.RuneCountInString(lines[i]) + 1
	}
	// Column within the source line: textarea exposes via LineInfo,
	// but if that's wrapped-column we need the wrap-aware mapping.
	// Pass 9 simplification: use SetCursor's column accessor.
	col := b.ta.LineInfo().ColumnOffset
	off += col
	if off > utf8.RuneCountInString(value) {
		off = utf8.RuneCountInString(value)
	}
	return off
}
```

If `ColumnOffset` proves to be wrap-aware in practice, fall back to walking textarea's internal cursor with `CursorStart` + counting `CursorRight` calls — but expect this to land cleanly with the simple form for plain wrapping.

- [ ] **Step 4: Run check**

```
make check
```

Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/catkin/buffer.go internal/catkin/buffer_test.go
git commit -m "Pass 9 task 4: catkin buffer rune-offset helpers + tests"
```

---

## Task 5: Plain renderer (no styling)

**Files:**
- Create: `internal/catkin/render.go`
- Create: `internal/catkin/render_test.go`

The renderer is Catkin's own `View()` content producer. Pass 9 ships **no styling** — just text + cursor + display-level wrap for long lines. Styling lands in 9a.

- [ ] **Step 1: Write failing test**

`internal/catkin/render_test.go`:

```go
package catkin

import "testing"

func TestRenderPlainSingleLine(t *testing.T) {
	got := Render("hello", 20, 5, 0, 5)
	// 5 chars + cursor block at col 5 (end of line) — represented
	// by ` `-block in the smoke test.
	want := "hello█" // cursor at end
	if got != want {
		t.Errorf("Render plain:\ngot  %q\nwant %q", got, want)
	}
}

func TestRenderDisplayWrapsLongLine(t *testing.T) {
	// Source single line longer than display width — render must
	// soft-wrap visually; raw source unchanged. Display width=10.
	src := "abcdefghijklmnopqrst" // 20 chars
	got := Render(src, 10, 5, 0, 0)
	// Expect 2 visual rows: "abcdefghij" and "klmnopqrst" with
	// cursor at row 0, col 0 (somewhere inside "abcdefghij").
	if !strings.Contains(got, "abcdefghij") || !strings.Contains(got, "klmnopqrst") {
		t.Errorf("Render display-wrap missing rows: %q", got)
	}
}
```

(Add `import "strings"` to the test file.)

- [ ] **Step 2: Run test to verify failure**

```
go test ./internal/catkin/ -run TestRender -v
```

Expected: FAIL — `Render` undefined.

- [ ] **Step 3: Implement Render**

`internal/catkin/render.go`:

```go
package catkin

import (
	"strings"
	"unicode/utf8"
)

// Render produces Catkin's view content: plain text + cursor +
// display-level soft-wrap for long source lines. Pass 9 applies no
// styling — that lands in 9a.
//
// width:  display width in cells.
// height: viewport height in rows.
// top:    rune offset of the first source line in the viewport (the
//         scroll position; row index, not absolute rune offset).
// cursor: absolute rune offset of the cursor in src.
func Render(src string, width, height, top, cursor int) string {
	lines := strings.Split(src, "\n")
	cursorRow, cursorCol := offsetToRowCol(src, cursor)

	var visual []string
	for i := top; i < len(lines) && len(visual) < height; i++ {
		wrapped := softWrap(lines[i], width)
		for _, w := range wrapped {
			if len(visual) >= height {
				break
			}
			visual = append(visual, w)
		}
	}

	// Place cursor block.
	if cursorRow >= top && cursorRow-top < len(visual) {
		visualRow, visualCol := mapToVisual(lines[cursorRow], cursorCol, width)
		row := (cursorRow - top) + visualRow
		if row < len(visual) {
			visual[row] = insertCursorBlock(visual[row], visualCol)
		}
	}

	// Pad to height.
	for len(visual) < height {
		visual = append(visual, "")
	}

	return strings.Join(visual, "\n")
}

// softWrap breaks a single source line into display-width chunks,
// allowing mid-token break only as a last resort.
func softWrap(line string, width int) []string {
	if width <= 0 || utf8.RuneCountInString(line) <= width {
		return []string{line}
	}
	var out []string
	runes := []rune(line)
	for len(runes) > width {
		out = append(out, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}

func offsetToRowCol(src string, off int) (row, col int) {
	for i, r := range src {
		if i >= off {
			return row, col
		}
		if r == '\n' {
			row++
			col = 0
		} else {
			col++
		}
	}
	return row, col
}

func mapToVisual(line string, srcCol, width int) (row, col int) {
	if width <= 0 {
		return 0, srcCol
	}
	return srcCol / width, srcCol % width
}

func insertCursorBlock(line string, col int) string {
	runes := []rune(line)
	if col >= len(runes) {
		return line + "█"
	}
	return string(runes[:col]) + "█" + string(runes[col+1:])
}
```

- [ ] **Step 4: Wire renderer into Model.View**

Edit `internal/catkin/catkin.go`, replace `View()`:

```go
func (m Model) View() string {
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	return Render(src, m.width, m.height, m.viewportTop, cur)
}
```

Add `viewportTop int` field to `Model` struct. Default 0.

- [ ] **Step 5: Run tests, iterate**

```
go test ./internal/catkin/ -run TestRender -v
```

Expected: PASS.

- [ ] **Step 6: Run full check**

```
make check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```
git add internal/catkin/render.go internal/catkin/render_test.go internal/catkin/catkin.go
git commit -m "Pass 9 task 5: catkin plain renderer + cursor + display wrap"
```

---

## Task 6: Word-level navigation supplement

**Files:**
- Create: `internal/catkin/wordnav.go`
- Create: `internal/catkin/wordnav_test.go`

Bubbles textarea ships some word navigation (`Alt+Left/Right` historically, or `Ctrl+Left/Right` in newer versions — verify at impl time). Catkin must guarantee `Ctrl+Left/Right` (jump by word) and `Ctrl+Backspace` / `Ctrl+Delete` (delete-word) regardless of textarea version.

Strategy: intercept these keys in `Model.Update` **before** delegating to the buffer. If textarea already handles them correctly, the intercept is a no-op pass-through. If not, the supplement implements them directly via `Buffer.RuneOffset` + `Buffer.SetRuneOffset` + value mutation.

- [ ] **Step 1: Write failing test**

`internal/catkin/wordnav_test.go`:

```go
package catkin

import "testing"

func TestWordBoundaryForward(t *testing.T) {
	// "the quick brown" — from offset 0, next boundary is 4 (start of "quick").
	got := nextWordBoundary("the quick brown", 0)
	if got != 4 {
		t.Errorf("nextWordBoundary(0): got %d, want 4", got)
	}
	// From inside "quick" (offset 5), next boundary is 10 (start of "brown").
	got = nextWordBoundary("the quick brown", 5)
	if got != 10 {
		t.Errorf("nextWordBoundary(5): got %d, want 10", got)
	}
	// From end, returns end.
	got = nextWordBoundary("the quick brown", 15)
	if got != 15 {
		t.Errorf("nextWordBoundary(end): got %d, want 15", got)
	}
}

func TestWordBoundaryBackward(t *testing.T) {
	got := prevWordBoundary("the quick brown", 15)
	if got != 10 {
		t.Errorf("prevWordBoundary(15): got %d, want 10", got)
	}
	got = prevWordBoundary("the quick brown", 10)
	if got != 4 {
		t.Errorf("prevWordBoundary(10): got %d, want 4", got)
	}
	got = prevWordBoundary("the quick brown", 0)
	if got != 0 {
		t.Errorf("prevWordBoundary(0): got %d, want 0", got)
	}
}
```

- [ ] **Step 2: Run, verify failure**

```
go test ./internal/catkin/ -run TestWordBoundary -v
```

Expected: FAIL.

- [ ] **Step 3: Implement word boundary helpers**

`internal/catkin/wordnav.go`:

```go
package catkin

import (
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// nextWordBoundary returns the rune offset of the start of the next
// word at or after off. If off is already past the last word,
// returns the rune count of src.
func nextWordBoundary(src string, off int) int {
	runes := []rune(src)
	i := off
	// Skip current word characters.
	for i < len(runes) && isWordRune(runes[i]) {
		i++
	}
	// Skip whitespace and punctuation runs.
	for i < len(runes) && !isWordRune(runes[i]) {
		i++
	}
	return i
}

// prevWordBoundary returns the rune offset of the start of the
// previous word strictly before off.
func prevWordBoundary(src string, off int) int {
	runes := []rune(src)
	if off <= 0 {
		return 0
	}
	i := off - 1
	// Skip non-word runs.
	for i > 0 && !isWordRune(runes[i]) {
		i--
	}
	// Skip current word.
	for i > 0 && isWordRune(runes[i-1]) {
		i--
	}
	return i
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// handleWordNav intercepts Ctrl+Left/Right/Backspace/Delete; returns
// (handled, updated buffer, cmd). Caller passes through to the
// buffer's normal Update if not handled.
func handleWordNav(b Buffer, msg tea.KeyMsg) (bool, Buffer, tea.Cmd) {
	if msg.Alt {
		return false, b, nil
	}
	switch msg.String() {
	case "ctrl+right":
		off := b.RuneOffset()
		next := nextWordBoundary(b.Value(), off)
		b.SetRuneOffset(next)
		return true, b, nil
	case "ctrl+left":
		off := b.RuneOffset()
		prev := prevWordBoundary(b.Value(), off)
		b.SetRuneOffset(prev)
		return true, b, nil
	case "ctrl+backspace":
		off := b.RuneOffset()
		prev := prevWordBoundary(b.Value(), off)
		val := b.Value()
		runes := []rune(val)
		newVal := string(runes[:prev]) + string(runes[off:])
		b.SetValue(newVal)
		b.SetRuneOffset(prev)
		return true, b, nil
	case "ctrl+delete":
		off := b.RuneOffset()
		next := nextWordBoundary(b.Value(), off)
		val := b.Value()
		runes := []rune(val)
		newVal := string(runes[:off]) + string(runes[next:])
		b.SetValue(newVal)
		b.SetRuneOffset(off)
		return true, b, nil
	}
	return false, b, nil
}
```

- [ ] **Step 4: Wire into Model.Update**

Edit `internal/catkin/catkin.go`:

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if handled, b, cmd := handleWordNav(m.buf, k); handled {
			m.buf = b
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.buf, cmd = m.buf.Update(msg)
	return m, cmd
}
```

- [ ] **Step 5: Run tests**

```
go test ./internal/catkin/ -run TestWordBoundary -v
make check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```
git add internal/catkin/wordnav.go internal/catkin/wordnav_test.go internal/catkin/catkin.go
git commit -m "Pass 9 task 6: catkin Ctrl+arrow word nav + Ctrl+Backspace/Delete"
```

---

## Task 7: Scroll-off

**Files:**
- Create: `internal/catkin/scrolloff.go`
- Create: `internal/catkin/scrolloff_test.go`

When the cursor moves, the viewport adjusts so the cursor is never within 3 lines of the top or bottom edge.

- [ ] **Step 1: Write failing test**

`internal/catkin/scrolloff_test.go`:

```go
package catkin

import "testing"

func TestClampViewportNoChange(t *testing.T) {
	// Cursor at line 10, viewport top=5, height=20, total=30 — cursor
	// at visual row 5 (10-5), well within the 3..16 safe band.
	got := ClampViewport(5, 20, 10, 30)
	if got != 5 {
		t.Errorf("ClampViewport stable: got %d, want 5", got)
	}
}

func TestClampViewportScrollsDown(t *testing.T) {
	// Cursor at line 18, top=0, height=20 — visual row 18, within
	// 3 of bottom (height-3=17). Should scroll so cursor is at row 16.
	got := ClampViewport(0, 20, 18, 100)
	if got != 2 {
		t.Errorf("ClampViewport scroll-down: got %d, want 2", got)
	}
}

func TestClampViewportScrollsUp(t *testing.T) {
	// Cursor at line 5, top=10, height=20 — cursor above viewport.
	// Should scroll so cursor at visual row 3.
	got := ClampViewport(10, 20, 5, 100)
	if got != 2 {
		t.Errorf("ClampViewport scroll-up: got %d, want 2", got)
	}
}

func TestClampViewportRespectsTotal(t *testing.T) {
	// Total only 10 lines; height 20. Top should clamp to 0.
	got := ClampViewport(5, 20, 5, 10)
	if got != 0 {
		t.Errorf("ClampViewport short doc: got %d, want 0", got)
	}
}
```

- [ ] **Step 2: Run, verify failure**

```
go test ./internal/catkin/ -run TestClampViewport -v
```

Expected: FAIL.

- [ ] **Step 3: Implement**

`internal/catkin/scrolloff.go`:

```go
package catkin

const scrollOff = 3

// ClampViewport returns a new viewport top so the cursor is never
// within scrollOff lines of the top or bottom edge of the visible
// region. If total < height, top clamps to 0.
func ClampViewport(top, height, cursorLine, total int) int {
	if total <= height {
		return 0
	}
	// Cursor visual row within the viewport.
	rel := cursorLine - top
	switch {
	case rel < scrollOff:
		top = cursorLine - scrollOff
	case rel >= height-scrollOff:
		top = cursorLine - (height - scrollOff - 1)
	}
	if top < 0 {
		top = 0
	}
	if top > total-height {
		top = total - height
	}
	return top
}
```

- [ ] **Step 4: Wire into Model.Update**

Edit `internal/catkin/catkin.go` `Update`:

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if handled, b, cmd := handleWordNav(m.buf, k); handled {
			m.buf = b
			m.applyScrollOff()
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.buf, cmd = m.buf.Update(msg)
	m.applyScrollOff()
	return m, cmd
}

func (m *Model) applyScrollOff() {
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	cursorLine, _ := offsetToRowCol(src, cur)
	total := lineCount(src)
	m.viewportTop = ClampViewport(m.viewportTop, m.height, cursorLine, total)
}

func lineCount(s string) int {
	if s == "" {
		return 1
	}
	n := 1
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}
```

- [ ] **Step 5: Run check**

```
make check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```
git add internal/catkin/scrolloff.go internal/catkin/scrolloff_test.go internal/catkin/catkin.go
git commit -m "Pass 9 task 7: catkin scroll-off (3 lines top + bottom)"
```

---

## Task 8: Pass-end ritual

**Files:**
- Edit: `docs/poplar/STATUS.md`
- Edit: `docs/poplar/invariants.md`
- Create: `docs/poplar/decisions/0144-catkin-renderer-ownership.md`
- Move: `docs/superpowers/plans/2026-05-04-catkin-core.md` → `docs/superpowers/archive/plans/2026-05-04-catkin-core.md`

Per the `poplar-pass` skill consolidation ritual.

- [ ] **Step 1: Run /simplify on the pass diff**

Invoke the `simplify` skill against the Pass 9 commits. Apply genuine wins.

- [ ] **Step 2: Write ADR-0144 — Catkin renderer ownership**

`docs/poplar/decisions/0144-catkin-renderer-ownership.md`:

```markdown
---
title: Catkin owns its renderer; bubbles/textarea is the buffer primitive
status: accepted
date: 2026-05-04
---

## Context

ADR-0076 chose `bubbles/textarea` as Catkin's foundation library
for buffer + cursor + edit operations. It did not specify whether
Catkin would also use textarea's `View()` for rendering. Pass 9
needed to decide: render through textarea (overlay style on its
ANSI output) vs. render directly from the raw buffer.

The compose spec commits Catkin to live markdown styling
(iA-Writer-shaped: `**bold**` displays bold with asterisks
visible) and block-aware reflow + display-level soft-wrap for
long URLs. Both require the renderer to know about block context
and inline span boundaries — context that exists in the raw
source, not in textarea's wrapped output. Parsing textarea's
ANSI escape stream to overlay styling proved fragile and
version-coupled in early prototyping.

## Decision

Catkin owns its `View()`. The package uses `textarea.Model` as
the buffer + cursor + edit-op primitive (storage, edit commands,
cursor positioning by row/col), but Catkin's `Render()` reads
the raw buffer string, runs the block classifier, applies
styling (in 9a), and produces the displayed lines independently.
Textarea's own `View()` is not called.

## Consequences

- Catkin is now a renderer, not just a dispatch + buffer wrapper.
  All future styling, annotation overlays (9d), and display
  modes (9c typewriter / focus) live in `render.go` and consume
  the same block classifier output.
- Cursor positioning math reads the raw buffer via
  `Buffer.RuneOffset` — no escape-sequence parsing.
- Lipgloss styles add zero display cells, so `displayCells`
  invariants hold across styled and unstyled output.
- Catkin's dependency on textarea is internal-only; if textarea
  changes its `View()` ANSI shape in a future bubbles release,
  Catkin is unaffected.
```

- [ ] **Step 3: Update invariants.md**

Add a Catkin section after the existing "Compose (planned)" stub in `internal/ui/` rules — or, better, add a top-level Catkin section under "Architecture" in `docs/poplar/invariants.md`. Add the binding fact:

```markdown
### Catkin

- Catkin (`internal/catkin/`) is poplar's markdown-first
  bubbletea editor, library-pure (`bubbletea`, `bubbles`,
  `lipgloss`, `muesli/reflow` only — no poplar imports).
  Catkin uses `bubbles/textarea` as the buffer + cursor +
  edit-op primitive but owns its own `View()` (renderer reads
  the raw buffer). Block classifier (`Classify`) and reflow
  engine (`Reflow`) are pure functions over the raw source.
  Display-level wrap soft-breaks long source lines mid-token
  for the editor view only; the buffer is unchanged. ADR-0144.
```

Update the decision index table at the bottom of `invariants.md` to include ADR-0144.

- [ ] **Step 4: Update STATUS.md**

Mark Pass 9 done, add the next starter prompt for Pass 9a (live markdown rendering). Per the spec's right-sized passes table.

- [ ] **Step 5: Archive plan**

```
git mv docs/superpowers/plans/2026-05-04-catkin-core.md docs/superpowers/archive/plans/
```

- [ ] **Step 6: Run final check**

```
make check
```

Expected: PASS.

- [ ] **Step 7: Commit, push, install**

```
git add -A
git commit -m "Pass 9: Catkin core — package, classifier, reflow, plain render, word-nav, scroll-off"
git push
make install
```

---

## Self-review checklist

Run through this once after writing the plan. Fix issues inline.

**Spec coverage** — every Pass 9 item in the right-sized passes table:
- [x] Package skeleton (Task 1)
- [x] textarea wrap (Task 1, refined Task 4)
- [x] Block classifier (Task 2)
- [x] Reflow engine (Task 3)
- [x] Basic cursor render, no styling (Task 5)
- [x] Word-level navigation (Task 6)
- [x] Scroll-off 3 lines (Task 7)
- [x] Unit tests for classifier table + reflow round-trip (Tasks 2, 3)

**Placeholder scan** — no TBD/TODO/"add validation"/"similar to Task N" — confirmed.

**Type consistency** — `LineContext`, `BlockKind`, `Buffer`, `Model`, `Reflow`, `Render`, `Classify` defined once and referenced consistently — confirmed. `RuneOffset` / `SetRuneOffset` introduced in Task 3 and used in Tasks 5, 6, 7 — consistent.

**Out-of-scope guard** — no styling, no Ctrl+B/I/K/L/Q, no smart Enter, no auto-pair, no undo, no compose wiring — those land in 9a–9h.
