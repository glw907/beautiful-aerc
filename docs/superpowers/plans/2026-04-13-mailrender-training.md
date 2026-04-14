# Mailrender Training System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a first-class training capture system for poplar's
rendering pipeline. Provides a `poplar train` subcommand (interactive
TUI + headless harness), a per-user capture store under
`xdg.StatePath`, a rewritten `fix-corpus` directory skill with a
normative `well-crafted-markdown.md` reference, and the
sanitize-then-submit flow that turns real captures into committable
`testdata/` fixtures.

**Architecture:** Four pieces, each with one clear job. (1)
`internal/train/` package owns the capture store, the TUI model,
migration, HTML minimization, and submit helpers. (2)
`internal/content/markdown.go` adds `RenderMarkdown` as an audit
sibling to `RenderBody` — markdown is **not** a pipeline
intermediate; the lipgloss display path is unchanged. (3)
`cmd/poplar/train.go` mounts the cobra subcommand tree (one TUI
entry + 10 named child commands). (4) `.claude/skills/fix-corpus/`
becomes a directory skill with `SKILL.md` plus a normative
`well-crafted-markdown.md` reference the skill loads at every
triage pass.

**Tech Stack:** Go 1.26.1, cobra (already vendored), bubbletea
(already vendored), bubbles/textarea + textinput, lipgloss
(through theme), `golang.org/x/net/html` (new dep — only for the
HTML minimizer; small, stdlib-adjacent), TOML via the existing
`github.com/BurntSushi/toml` already pulled by config.

**Spec:** `docs/superpowers/specs/2026-04-12-mailrender-training-design.md`

**Required reading before starting:**
- Invoke `go-conventions` skill before writing any Go file.
- Invoke `elm-conventions` skill before writing any TUI model code.
- Read `docs/poplar/invariants.md` once at the start.
- Read the spec once at the start.
- Read `internal/content/blocks.go` and `internal/content/render.go`
  before Phase 1 — `RenderMarkdown` mirrors `RenderBody`'s shape
  and walks the same Block types.

---

## Phase 1 — Pure library foundations

Two pieces of pure code with no dependencies on the capture store.
Both are short, testable in isolation, and unblock everything in
Phase 2.

### Task 1: Add `RenderMarkdown` to `internal/content`

**Files:**
- Create: `internal/content/markdown.go`
- Create: `internal/content/markdown_test.go`

`RenderMarkdown` walks a `[]Block` and emits canonical markdown.
It mirrors `RenderBody`'s structure but emits markdown syntax
instead of styled lipgloss output. It is pure (no theme, no
width, no I/O).

- [ ] **Step 1: Write the failing test for an empty input**

```go
// internal/content/markdown_test.go
package content

import "testing"

func TestRenderMarkdown(t *testing.T) {
	tests := []struct {
		name   string
		blocks []Block
		want   string
	}{
		{
			name:   "empty",
			blocks: nil,
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderMarkdown(tc.blocks)
			if got != tc.want {
				t.Fatalf("RenderMarkdown() = %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd ~/Projects/poplar && go test ./internal/content/ -run TestRenderMarkdown -v
```

Expected: `undefined: RenderMarkdown`.

- [ ] **Step 3: Write the minimal implementation**

```go
// internal/content/markdown.go
package content

import (
	"fmt"
	"strings"
)

// RenderMarkdown emits canonical markdown for a block tree. It
// mirrors RenderBody but produces markdown syntax instead of
// styled output. Used by the training system as an audit
// artifact — it is NOT part of the display pipeline.
func RenderMarkdown(blocks []Block) string {
	if len(blocks) == 0 {
		return ""
	}
	var b strings.Builder
	for i, block := range blocks {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(renderBlockMarkdown(block, 0))
	}
	return b.String()
}

func renderBlockMarkdown(block Block, depth int) string {
	switch b := block.(type) {
	case Paragraph:
		return renderSpansMarkdown(b.Spans)
	case Heading:
		level := b.Level
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		return strings.Repeat("#", level) + " " + renderSpansMarkdown(b.Spans)
	case Blockquote:
		return renderBlockquoteMarkdown(b, depth)
	case QuoteAttribution:
		return "> " + renderSpansMarkdown(b.Spans)
	case Signature:
		var lines []string
		lines = append(lines, "---")
		for _, line := range b.Lines {
			lines = append(lines, renderSpansMarkdown(line))
		}
		return strings.Join(lines, "\n")
	case Rule:
		return "---"
	case CodeBlock:
		fence := "```"
		if b.Lang != "" {
			return fence + b.Lang + "\n" + b.Text + "\n" + fence
		}
		return fence + "\n" + b.Text + "\n" + fence
	case Table:
		return renderTableMarkdown(b)
	case ListItem:
		prefix := "- "
		if b.Ordered {
			prefix = fmt.Sprintf("%d. ", b.Index)
		}
		return prefix + renderSpansMarkdown(b.Spans)
	default:
		return ""
	}
}

func renderBlockquoteMarkdown(q Blockquote, depth int) string {
	prefix := strings.Repeat("> ", q.Level)
	var lines []string
	for _, child := range q.Blocks {
		inner := renderBlockMarkdown(child, depth+1)
		for _, line := range strings.Split(inner, "\n") {
			lines = append(lines, prefix+line)
		}
	}
	return strings.Join(lines, "\n")
}

func renderSpansMarkdown(spans []Span) string {
	var b strings.Builder
	for _, span := range spans {
		switch s := span.(type) {
		case Text:
			b.WriteString(s.Content)
		case Bold:
			b.WriteString("**" + s.Content + "**")
		case Italic:
			b.WriteString("_" + s.Content + "_")
		case Code:
			b.WriteString("`" + s.Content + "`")
		case Link:
			if s.Text == "" || s.Text == s.URL {
				b.WriteString("<" + s.URL + ">")
			} else {
				b.WriteString("[" + s.Text + "](" + s.URL + ")")
			}
		}
	}
	return b.String()
}

func renderTableMarkdown(t Table) string {
	var rows []string
	if len(t.Headers) > 0 {
		var cells []string
		for _, hdr := range t.Headers {
			cells = append(cells, renderSpansMarkdown(hdr))
		}
		rows = append(rows, "| "+strings.Join(cells, " | ")+" |")
		var seps []string
		for range t.Headers {
			seps = append(seps, "---")
		}
		rows = append(rows, "| "+strings.Join(seps, " | ")+" |")
	}
	for _, row := range t.Rows {
		var cells []string
		for _, cell := range row {
			cells = append(cells, renderSpansMarkdown(cell))
		}
		rows = append(rows, "| "+strings.Join(cells, " | ")+" |")
	}
	return strings.Join(rows, "\n")
}
```

- [ ] **Step 4: Run the test to verify the empty case passes**

```bash
go test ./internal/content/ -run TestRenderMarkdown -v
```

Expected: PASS.

- [ ] **Step 5: Expand the test table to cover every block type**

Append these cases to the `tests` slice in
`markdown_test.go`:

```go
{
	name: "single paragraph",
	blocks: []Block{
		Paragraph{Spans: []Span{Text{Content: "hello world"}}},
	},
	want: "hello world",
},
{
	name: "two paragraphs separated by blank line",
	blocks: []Block{
		Paragraph{Spans: []Span{Text{Content: "first"}}},
		Paragraph{Spans: []Span{Text{Content: "second"}}},
	},
	want: "first\n\nsecond",
},
{
	name: "heading level 2",
	blocks: []Block{
		Heading{Level: 2, Spans: []Span{Text{Content: "Section"}}},
	},
	want: "## Section",
},
{
	name: "heading clamps to 6",
	blocks: []Block{
		Heading{Level: 99, Spans: []Span{Text{Content: "Deep"}}},
	},
	want: "###### Deep",
},
{
	name: "blockquote level 1",
	blocks: []Block{
		Blockquote{Level: 1, Blocks: []Block{
			Paragraph{Spans: []Span{Text{Content: "quoted"}}},
		}},
	},
	want: "> quoted",
},
{
	name: "blockquote level 2",
	blocks: []Block{
		Blockquote{Level: 2, Blocks: []Block{
			Paragraph{Spans: []Span{Text{Content: "deep"}}},
		}},
	},
	want: "> > deep",
},
{
	name: "quote attribution",
	blocks: []Block{
		QuoteAttribution{Spans: []Span{Text{Content: "On Mon, Alice wrote:"}}},
	},
	want: "> On Mon, Alice wrote:",
},
{
	name: "horizontal rule",
	blocks: []Block{Rule{}},
	want:   "---",
},
{
	name: "code block with language",
	blocks: []Block{
		CodeBlock{Lang: "go", Text: "fmt.Println(\"hi\")"},
	},
	want: "```go\nfmt.Println(\"hi\")\n```",
},
{
	name: "code block without language",
	blocks: []Block{
		CodeBlock{Text: "plain"},
	},
	want: "```\nplain\n```",
},
{
	name: "unordered list item",
	blocks: []Block{
		ListItem{Spans: []Span{Text{Content: "first"}}},
	},
	want: "- first",
},
{
	name: "ordered list item",
	blocks: []Block{
		ListItem{Ordered: true, Index: 3, Spans: []Span{Text{Content: "third"}}},
	},
	want: "3. third",
},
{
	name: "bold span",
	blocks: []Block{
		Paragraph{Spans: []Span{Bold{Content: "loud"}}},
	},
	want: "**loud**",
},
{
	name: "italic span",
	blocks: []Block{
		Paragraph{Spans: []Span{Italic{Content: "soft"}}},
	},
	want: "_soft_",
},
{
	name: "inline code span",
	blocks: []Block{
		Paragraph{Spans: []Span{Code{Content: "x := 1"}}},
	},
	want: "`x := 1`",
},
{
	name: "link with text",
	blocks: []Block{
		Paragraph{Spans: []Span{Link{Text: "click", URL: "https://example.com"}}},
	},
	want: "[click](https://example.com)",
},
{
	name: "link bare URL",
	blocks: []Block{
		Paragraph{Spans: []Span{Link{Text: "https://example.com", URL: "https://example.com"}}},
	},
	want: "<https://example.com>",
},
{
	name: "table with headers",
	blocks: []Block{
		Table{
			Headers: [][]Span{
				{Text{Content: "A"}}, {Text{Content: "B"}},
			},
			Rows: [][][]Span{
				{{Text{Content: "1"}}, {Text{Content: "2"}}},
			},
		},
	},
	want: "| A | B |\n| --- | --- |\n| 1 | 2 |",
},
{
	name: "signature",
	blocks: []Block{
		Signature{Lines: [][]Span{
			{Text{Content: "Alice"}},
			{Text{Content: "alice@example.com"}},
		}},
	},
	want: "---\nAlice\nalice@example.com",
},
```

- [ ] **Step 6: Run the full test table**

```bash
go test ./internal/content/ -run TestRenderMarkdown -v
```

Expected: all cases PASS. If any fail, fix the implementation in
`markdown.go` until they all pass.

- [ ] **Step 7: Run the whole content package test suite**

```bash
go test ./internal/content/ -v
```

Expected: PASS (no regressions in `RenderBody` or parser tests).

- [ ] **Step 8: Run vet**

```bash
go vet ./internal/content/
```

Expected: no output.

- [ ] **Step 9: Commit**

```bash
git add internal/content/markdown.go internal/content/markdown_test.go
git commit -m "$(cat <<'EOF'
Add RenderMarkdown audit sibling to RenderBody

Pure function that walks a block tree and emits canonical markdown.
Used by the training system as an audit artifact — not part of the
display pipeline. Mirrors RenderBody's switch-on-block-kind shape
and reuses the same Block and Span types.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Add HTML subtree minimizer

**Files:**
- Create: `internal/train/minimize/minimize.go`
- Create: `internal/train/minimize/minimize_test.go`

The minimizer walks an HTML tree and removes content that doesn't
affect rendered output: scripts, styles, head/meta, hidden nodes,
and wrapper containers that pass through their children unchanged.
The output is the smallest HTML that the production pipeline (in
`internal/filter` + `internal/content`) would render to the same
text. **It is not a sanitizer** — output may still contain PII.
Sanitization happens in a later step under human review.

- [ ] **Step 1: Add the html dependency to go.mod**

```bash
cd ~/Projects/poplar && go get golang.org/x/net/html
```

Expected: `go.mod` updated with `golang.org/x/net` (likely
already transitively present; this makes it direct).

- [ ] **Step 2: Write the failing test**

```go
// internal/train/minimize/minimize_test.go
package minimize

import (
	"strings"
	"testing"
)

func TestMinimize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantHas []string  // substrings that MUST appear in output
		wantNo  []string  // substrings that MUST NOT appear
	}{
		{
			name:    "drops script tag",
			input:   `<html><body><p>hi</p><script>alert(1)</script></body></html>`,
			wantHas: []string{"<p>hi</p>"},
			wantNo:  []string{"script", "alert"},
		},
		{
			name:    "drops style tag",
			input:   `<html><body><p>hi</p><style>p{color:red}</style></body></html>`,
			wantHas: []string{"<p>hi</p>"},
			wantNo:  []string{"style", "color:red"},
		},
		{
			name:    "drops head section",
			input:   `<html><head><meta charset="utf-8"><title>x</title></head><body><p>hi</p></body></html>`,
			wantHas: []string{"<p>hi</p>"},
			wantNo:  []string{"<head>", "<title>", "<meta"},
		},
		{
			name:    "drops display:none",
			input:   `<html><body><p>visible</p><div style="display:none">hidden</div></body></html>`,
			wantHas: []string{"visible"},
			wantNo:  []string{"hidden"},
		},
		{
			name:    "preserves nested structure",
			input:   `<html><body><blockquote><p>quoted</p></blockquote></body></html>`,
			wantHas: []string{"<blockquote>", "<p>quoted</p>"},
		},
		{
			name:    "preserves links",
			input:   `<html><body><p><a href="https://example.com">click</a></p></body></html>`,
			wantHas: []string{`href="https://example.com"`, "click"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Minimize([]byte(tc.input))
			if err != nil {
				t.Fatalf("Minimize() error: %v", err)
			}
			s := string(out)
			for _, want := range tc.wantHas {
				if !strings.Contains(s, want) {
					t.Errorf("output missing %q\nfull output:\n%s", want, s)
				}
			}
			for _, no := range tc.wantNo {
				if strings.Contains(s, no) {
					t.Errorf("output should not contain %q\nfull output:\n%s", no, s)
				}
			}
		})
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

```bash
go test ./internal/train/minimize/ -v
```

Expected: `package internal/train/minimize is not in std`.

- [ ] **Step 4: Write the implementation**

```go
// internal/train/minimize/minimize.go
package minimize

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Minimize strips scripts, styles, head/meta, hidden nodes, and
// other content that does not affect the rendered output of the
// poplar filter+content pipeline. It is NOT a PII sanitizer — the
// output may still contain personal information. Use this as the
// first step before manual sanitization.
func Minimize(in []byte) ([]byte, error) {
	doc, err := html.Parse(bytes.NewReader(in))
	if err != nil {
		return nil, fmt.Errorf("minimize: parse: %w", err)
	}
	prune(doc)
	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return nil, fmt.Errorf("minimize: render: %w", err)
	}
	return buf.Bytes(), nil
}

// prune removes nodes from the tree that do not affect rendered
// output. It walks depth-first and rewrites Children in place.
func prune(n *html.Node) {
	if n == nil {
		return
	}
	var kept []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if dropNode(c) {
			continue
		}
		prune(c)
		kept = append(kept, c)
	}
	rewireChildren(n, kept)
}

func rewireChildren(parent *html.Node, kept []*html.Node) {
	parent.FirstChild = nil
	parent.LastChild = nil
	for i, c := range kept {
		c.Parent = parent
		if i == 0 {
			c.PrevSibling = nil
			parent.FirstChild = c
		} else {
			c.PrevSibling = kept[i-1]
			kept[i-1].NextSibling = c
		}
		c.NextSibling = nil
	}
	if n := len(kept); n > 0 {
		parent.LastChild = kept[n-1]
	}
}

// dropNode reports whether a node should be removed entirely
// (along with its children).
func dropNode(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Head, atom.Meta,
		atom.Link, atom.Noscript, atom.Iframe, atom.Object,
		atom.Embed, atom.Svg, atom.Canvas:
		return true
	}
	if isHidden(n) {
		return true
	}
	return false
}

func isHidden(n *html.Node) bool {
	for _, a := range n.Attr {
		switch strings.ToLower(a.Key) {
		case "hidden":
			return true
		case "style":
			s := strings.ToLower(strings.ReplaceAll(a.Val, " ", ""))
			if strings.Contains(s, "display:none") || strings.Contains(s, "visibility:hidden") {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/train/minimize/ -v
```

Expected: all cases PASS. If any fail, examine the html.Render
output and adjust either `dropNode` or test expectations.

- [ ] **Step 6: Add a roundtrip-fidelity test**

Append to `minimize_test.go`:

```go
func TestMinimizeRoundtripFidelity(t *testing.T) {
	// Minimize must not change the rendered output. We feed both
	// the original and the minimized HTML through filter.CleanHTML
	// + content.ParseBlocks + content.RenderMarkdown and assert
	// the markdown output is identical.
	t.Skip("filled in once filter+content are wired through Phase 2 helper")
}
```

The skip is intentional — the wiring isn't ready yet. Phase 2 fills it in.

- [ ] **Step 7: Run vet**

```bash
go vet ./internal/train/minimize/
```

Expected: no output.

- [ ] **Step 8: Commit**

```bash
git add internal/train/minimize/ go.mod go.sum
git commit -m "$(cat <<'EOF'
Add HTML subtree minimizer for training fixture extraction

Strips scripts, styles, head, meta, link, hidden nodes, and other
elements that don't affect rendered output. Uses x/net/html for
parsing and rendering. NOT a sanitizer — output may still contain
PII; sanitization is a separate manual step downstream.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2 — Capture store

The capture store is plain files under `xdg.StatePath("poplar","captures")`.
One directory per capture, fixed file names inside, no index. All
mutations go through `internal/train` functions; no other package
touches the store directly.

### Task 3: Create the `internal/train` package skeleton with the Meta type

**Files:**
- Create: `internal/train/meta.go`
- Create: `internal/train/meta_test.go`

`Meta` is the per-capture sidecar. It serializes to TOML for
human-friendly editing.

- [ ] **Step 1: Write the failing test**

```go
// internal/train/meta_test.go
package train

import (
	"strings"
	"testing"
	"time"
)

func TestMetaRoundtrip(t *testing.T) {
	in := Meta{
		CreatedAt:    time.Date(2026, 4, 13, 9, 0, 0, 0, time.UTC),
		SourceKind:   "html",
		SourceHash:   "deadbeef",
		Platform:     "gmail",
		Status:       "new",
		FixtureRef:   "",
		Notes:        "",
		RenderedOnly: false,
	}
	data, err := in.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `source_kind = "html"`) {
		t.Errorf("expected source_kind in TOML, got: %s", data)
	}
	out, err := UnmarshalMeta(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.SourceHash != "deadbeef" {
		t.Errorf("SourceHash roundtrip failed: %s", out.SourceHash)
	}
	if out.Status != "new" {
		t.Errorf("Status roundtrip failed: %s", out.Status)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/train/ -v
```

Expected: `package internal/train is not in std` or
`undefined: Meta`.

- [ ] **Step 3: Write the Meta type**

```go
// internal/train/meta.go
package train

import (
	"bytes"
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
)

// Meta is the per-capture sidecar persisted as meta.toml. All
// fields are documented in the design spec
// (docs/superpowers/specs/2026-04-12-mailrender-training-design.md).
type Meta struct {
	CreatedAt    time.Time `toml:"created_at"`
	SourceKind   string    `toml:"source_kind"` // "html" | "plain" | "rendered-only"
	SourceHash   string    `toml:"source_hash"` // sha256(raw)[:8]
	Platform     string    `toml:"platform"`    // "gmail" | "outlook" | ...
	Status       string    `toml:"status"`      // "new"|"triaged"|"fixed"|"wontfix"|"broken"|"unscored"
	FixtureRef   string    `toml:"fixture_ref,omitempty"`
	Notes        string    `toml:"notes,omitempty"`
	RenderedOnly bool      `toml:"rendered_only,omitempty"`
}

// Marshal serializes Meta to TOML bytes.
func (m Meta) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return nil, fmt.Errorf("meta marshal: %w", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalMeta parses TOML bytes into a Meta.
func UnmarshalMeta(data []byte) (Meta, error) {
	var m Meta
	if _, err := toml.Decode(string(data), &m); err != nil {
		return Meta{}, fmt.Errorf("meta unmarshal: %w", err)
	}
	return m, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/train/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/train/meta.go internal/train/meta_test.go
git commit -m "$(cat <<'EOF'
Add Meta type for training capture sidecar

TOML-serialized per-capture metadata: timestamps, source kind/hash,
platform, status, optional fixture ref and notes. Roundtrip tested.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Add `Capture` type, `Root`, and `Save`

**Files:**
- Create: `internal/train/store.go`
- Create: `internal/train/store_test.go`

`Save` is the entry point for writing a new capture. It computes
the id, creates the directory, writes `raw.*`, computes the
`rendered.ansi` snapshot via the production pipeline, and writes
`meta.toml` and `comment.md`.

- [ ] **Step 1: Write the failing test**

```go
// internal/train/store_test.go
package train

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadCapture(t *testing.T) {
	root := t.TempDir()
	raw := []byte("<html><body><p>hello</p></body></html>")

	c, err := Save(root, raw, "html", "list wraps badly")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if c.ID == "" {
		t.Fatal("Save returned empty ID")
	}
	if !strings.HasPrefix(c.ID, time.Now().UTC().Format("20060102")) {
		t.Errorf("ID does not start with today's date: %s", c.ID)
	}

	// raw.html written
	if _, err := os.Stat(filepath.Join(c.Dir, "raw.html")); err != nil {
		t.Errorf("raw.html missing: %v", err)
	}
	// meta.toml written
	if _, err := os.Stat(filepath.Join(c.Dir, "meta.toml")); err != nil {
		t.Errorf("meta.toml missing: %v", err)
	}
	// comment.md written with the supplied comment
	commentBytes, err := os.ReadFile(filepath.Join(c.Dir, "comment.md"))
	if err != nil {
		t.Fatalf("comment.md missing: %v", err)
	}
	if string(commentBytes) != "list wraps badly\n" {
		t.Errorf("comment mismatch: %q", commentBytes)
	}

	// Load returns the same capture
	loaded, err := Load(root, c.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Meta.SourceKind != "html" {
		t.Errorf("source_kind roundtrip failed: %s", loaded.Meta.SourceKind)
	}
	if loaded.Comment != "list wraps badly\n" {
		t.Errorf("comment roundtrip failed: %q", loaded.Comment)
	}
}
```

Add `import "time"` to the test file.

- [ ] **Step 2: Run to verify failure**

Expected: `undefined: Save`.

- [ ] **Step 3: Write the store implementation**

```go
// internal/train/store.go
package train

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/glw907/poplar/internal/mailworker/xdg"
)

// Capture represents one captured email rendering. Comment is
// loaded lazily — call Load to populate it.
type Capture struct {
	ID      string
	Dir     string
	RawPath string
	Meta    Meta
	Comment string
}

// Root returns the canonical captures directory:
// $XDG_STATE_HOME/poplar/captures, falling back to
// ~/.local/state/poplar/captures.
func Root() string {
	return xdg.StatePath("poplar", "captures")
}

// Save writes a new capture to root. raw is the email source;
// srcKind is "html" or "plain". comment may be empty (the user
// can fill it in later via the TUI). Returns the created Capture.
func Save(root string, raw []byte, srcKind, comment string) (Capture, error) {
	if srcKind != "html" && srcKind != "plain" {
		return Capture{}, fmt.Errorf("save: invalid srcKind %q", srcKind)
	}
	hash := shortHash(raw)
	id := time.Now().UTC().Format("20060102") + "-" + hash
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Capture{}, fmt.Errorf("save: mkdir: %w", err)
	}

	rawName := "raw.html"
	if srcKind == "plain" {
		rawName = "raw.txt"
	}
	rawPath := filepath.Join(dir, rawName)
	if err := os.WriteFile(rawPath, raw, 0o600); err != nil {
		return Capture{}, fmt.Errorf("save: write raw: %w", err)
	}

	if comment != "" && !strings.HasSuffix(comment, "\n") {
		comment += "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "comment.md"), []byte(comment), 0o600); err != nil {
		return Capture{}, fmt.Errorf("save: write comment: %w", err)
	}

	meta := Meta{
		CreatedAt:  time.Now().UTC(),
		SourceKind: srcKind,
		SourceHash: hash,
		Platform:   detectPlatform(raw),
		Status:     "new",
	}
	metaBytes, err := meta.Marshal()
	if err != nil {
		return Capture{}, fmt.Errorf("save: marshal meta: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.toml"), metaBytes, 0o600); err != nil {
		return Capture{}, fmt.Errorf("save: write meta: %w", err)
	}

	// rendered.ansi snapshot is written by Render() at first read,
	// not at Save time, to keep Save dependency-free for tests.

	return Capture{
		ID:      id,
		Dir:     dir,
		RawPath: rawPath,
		Meta:    meta,
		Comment: comment,
	}, nil
}

// Load reads a capture by id from root. Includes comment text.
func Load(root, id string) (Capture, error) {
	dir := filepath.Join(root, id)
	metaBytes, err := os.ReadFile(filepath.Join(dir, "meta.toml"))
	if err != nil {
		return Capture{}, fmt.Errorf("load: read meta: %w", err)
	}
	meta, err := UnmarshalMeta(metaBytes)
	if err != nil {
		return Capture{}, fmt.Errorf("load: parse meta: %w", err)
	}
	rawName := "raw.html"
	if meta.SourceKind == "plain" || meta.SourceKind == "rendered-only" {
		rawName = "raw.txt"
	}
	commentBytes, err := os.ReadFile(filepath.Join(dir, "comment.md"))
	if err != nil && !os.IsNotExist(err) {
		return Capture{}, fmt.Errorf("load: read comment: %w", err)
	}
	return Capture{
		ID:      id,
		Dir:     dir,
		RawPath: filepath.Join(dir, rawName),
		Meta:    meta,
		Comment: string(commentBytes),
	}, nil
}

// List walks root and returns all captures, sorted by id ascending.
// Captures with corrupt meta.toml are skipped; their paths are
// returned as Skipped so the caller can surface them.
func List(root string) ([]Capture, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("list: mkdir root: %w", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("list: read root: %w", err)
	}
	var out []Capture
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := Load(root, e.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "train: skipping %s: %v\n", e.Name(), err)
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func shortHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:8]
}

var (
	platformGmail   = regexp.MustCompile(`(?i)gmail|google[. ]?mail`)
	platformOutlook = regexp.MustCompile(`(?i)outlook|microsoft[. ]?mail`)
	platformYahoo   = regexp.MustCompile(`(?i)yahoo`)
	platformFastmail = regexp.MustCompile(`(?i)fastmail`)
)

func detectPlatform(raw []byte) string {
	head := raw
	if len(head) > 4096 {
		head = head[:4096]
	}
	switch {
	case platformGmail.Match(head):
		return "gmail"
	case platformOutlook.Match(head):
		return "outlook"
	case platformYahoo.Match(head):
		return "yahoo"
	case platformFastmail.Match(head):
		return "fastmail"
	}
	return "unknown"
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/train/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/train/store.go internal/train/store_test.go
git commit -m "$(cat <<'EOF'
Add training capture store: Save, Load, List, Root

Save writes raw, comment, and meta files into a per-capture dir
under xdg.StatePath. List walks the root and returns captures
sorted by id; corrupt meta.toml entries are skipped with a stderr
notice. Platform detection is a regex over the first 4KB.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Add `UpdateStatus` and a `List`-skips-corrupt test

**Files:**
- Modify: `internal/train/store.go` — add `UpdateStatus`
- Modify: `internal/train/store_test.go` — add tests

- [ ] **Step 1: Write the tests**

Append to `store_test.go`:

```go
func TestUpdateStatus(t *testing.T) {
	root := t.TempDir()
	c, err := Save(root, []byte("<html><body><p>x</p></body></html>"), "html", "")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := UpdateStatus(root, c.ID, "triaged"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	loaded, err := Load(root, c.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Meta.Status != "triaged" {
		t.Errorf("Status not updated: %s", loaded.Meta.Status)
	}
}

func TestUpdateStatusInvalid(t *testing.T) {
	root := t.TempDir()
	c, _ := Save(root, []byte("<p>x</p>"), "html", "")
	if err := UpdateStatus(root, c.ID, "bogus"); err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestListSkipsCorruptMeta(t *testing.T) {
	root := t.TempDir()
	good, _ := Save(root, []byte("<p>good</p>"), "html", "")
	bad := filepath.Join(root, "20260413-bad00000")
	if err := os.MkdirAll(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "meta.toml"), []byte("not valid toml ::::"), 0o600); err != nil {
		t.Fatal(err)
	}
	captures, err := List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(captures) != 1 {
		t.Errorf("expected 1 capture, got %d", len(captures))
	}
	if captures[0].ID != good.ID {
		t.Errorf("wrong capture: %s", captures[0].ID)
	}
}
```

- [ ] **Step 2: Implement UpdateStatus**

Append to `store.go`:

```go
// UpdateStatus sets the status field on a capture's meta.toml.
func UpdateStatus(root, id, status string) error {
	switch status {
	case "new", "triaged", "fixed", "wontfix", "broken", "unscored":
	default:
		return fmt.Errorf("update status: invalid status %q", status)
	}
	c, err := Load(root, id)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	c.Meta.Status = status
	data, err := c.Meta.Marshal()
	if err != nil {
		return fmt.Errorf("update status: marshal: %w", err)
	}
	return os.WriteFile(filepath.Join(c.Dir, "meta.toml"), data, 0o600)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/train/ -v
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/train/store.go internal/train/store_test.go
git commit -m "$(cat <<'EOF'
Add UpdateStatus and corrupt-meta skip test for training store

UpdateStatus validates the status string against the allowed set,
loads the capture, mutates Meta.Status, and writes meta.toml back.
List now has explicit test coverage for skipping corrupt entries.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Add `Render`, `Markdown`, and `Diff` helpers

**Files:**
- Create: `internal/train/render.go`
- Create: `internal/train/render_test.go`
- Modify: `internal/train/minimize/minimize_test.go` — fill in roundtrip test

These bridge a `Capture` to the production pipeline.

- [ ] **Step 1: Write the failing tests**

```go
// internal/train/render_test.go
package train

import (
	"strings"
	"testing"
)

func TestRenderAndMarkdownAndDiff(t *testing.T) {
	root := t.TempDir()
	raw := []byte("<html><body><p>hello world</p></body></html>")
	c, err := Save(root, raw, "html", "")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	rendered, err := Render(c)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(rendered, "hello world") {
		t.Errorf("rendered missing content: %q", rendered)
	}

	md, err := Markdown(c)
	if err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if !strings.Contains(md, "hello world") {
		t.Errorf("markdown missing content: %q", md)
	}

	diff, err := Diff(c)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	// First Diff after Save snapshots and returns "no changes".
	if !strings.Contains(diff, "no changes") {
		t.Errorf("expected no-changes diff, got: %q", diff)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Expected: `undefined: Render`.

- [ ] **Step 3: Write the implementation**

```go
// internal/train/render.go
package train

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/filter"
	"github.com/glw907/poplar/internal/theme"
)

// Render runs the production pipeline on a capture's raw source
// and returns the styled terminal output. Side effect: writes
// rendered.ansi to the capture dir on first call (snapshot for
// future Diff).
func Render(c Capture) (string, error) {
	raw, err := os.ReadFile(c.RawPath)
	if err != nil {
		return "", fmt.Errorf("render: read raw: %w", err)
	}
	cleaned, err := cleanForKind(raw, c.Meta.SourceKind)
	if err != nil {
		return "", fmt.Errorf("render: clean: %w", err)
	}
	blocks := content.ParseBlocks(cleaned)
	t := theme.Default()
	out := content.RenderBody(blocks, t, 80)

	snapPath := filepath.Join(c.Dir, "rendered.ansi")
	if _, err := os.Stat(snapPath); os.IsNotExist(err) {
		_ = os.WriteFile(snapPath, []byte(out), 0o600)
	}
	return out, nil
}

// Markdown runs the production pipeline on a capture's raw source
// and returns canonical markdown. Pure — no side effects.
func Markdown(c Capture) (string, error) {
	raw, err := os.ReadFile(c.RawPath)
	if err != nil {
		return "", fmt.Errorf("markdown: read raw: %w", err)
	}
	cleaned, err := cleanForKind(raw, c.Meta.SourceKind)
	if err != nil {
		return "", fmt.Errorf("markdown: clean: %w", err)
	}
	blocks := content.ParseBlocks(cleaned)
	return content.RenderMarkdown(blocks), nil
}

// Diff compares the current Render output to the rendered.ansi
// snapshot stored at capture time. Returns "no changes" if
// identical, otherwise a unified-diff-style string.
func Diff(c Capture) (string, error) {
	current, err := Render(c)
	if err != nil {
		return "", err
	}
	snap, err := os.ReadFile(filepath.Join(c.Dir, "rendered.ansi"))
	if err != nil {
		return "", fmt.Errorf("diff: read snapshot: %w", err)
	}
	if string(snap) == current {
		return "no changes\n", nil
	}
	return simpleDiff(string(snap), current), nil
}

func cleanForKind(raw []byte, kind string) (string, error) {
	switch kind {
	case "html":
		return filter.CleanHTML(string(raw))
	case "plain":
		return filter.CleanPlain(string(raw)), nil
	case "rendered-only":
		return "", fmt.Errorf("cannot render rendered-only capture")
	default:
		return "", fmt.Errorf("unknown source kind: %s", kind)
	}
}

// simpleDiff is a minimal line-oriented diff. Not a unified-diff;
// just side-by-side line markers. Adequate for the training
// inspection use case.
func simpleDiff(a, b string) string {
	var out []byte
	la := splitLines(a)
	lb := splitLines(b)
	n := len(la)
	if len(lb) > n {
		n = len(lb)
	}
	for i := 0; i < n; i++ {
		var av, bv string
		if i < len(la) {
			av = la[i]
		}
		if i < len(lb) {
			bv = lb[i]
		}
		if av == bv {
			out = append(out, ' ', ' ')
			out = append(out, av...)
		} else {
			if av != "" {
				out = append(out, '-', ' ')
				out = append(out, av...)
				out = append(out, '\n')
			}
			if bv != "" {
				out = append(out, '+', ' ')
				out = append(out, bv...)
			}
		}
		out = append(out, '\n')
	}
	return string(out)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
```

- [ ] **Step 4: Verify `theme.Default()` exists**

```bash
grep -n "func Default" internal/theme/*.go
```

If it doesn't exist, use the actual name (likely `theme.New()`,
`theme.OneDark()`, or similar). Adjust the import in `render.go`
accordingly. Run:

```bash
go build ./internal/train/
```

Fix until it compiles.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/train/ -v
```

Expected: all PASS.

- [ ] **Step 6: Fill in the minimize roundtrip-fidelity test**

Replace the skipped `TestMinimizeRoundtripFidelity` in
`internal/train/minimize/minimize_test.go`:

```go
func TestMinimizeRoundtripFidelity(t *testing.T) {
	cases := []string{
		`<html><body><p>plain</p></body></html>`,
		`<html><head><title>x</title></head><body><h1>Hi</h1><p>body</p></body></html>`,
		`<html><body><blockquote><p>quoted</p></blockquote><p>after</p></body></html>`,
	}
	for _, in := range cases {
		t.Run(in[:20], func(t *testing.T) {
			origMd := pipelineMarkdown(t, []byte(in))
			min, err := Minimize([]byte(in))
			if err != nil {
				t.Fatalf("Minimize: %v", err)
			}
			minMd := pipelineMarkdown(t, min)
			if origMd != minMd {
				t.Errorf("minimize changed rendered markdown\n--- orig:\n%s\n--- min:\n%s", origMd, minMd)
			}
		})
	}
}

// pipelineMarkdown is a test helper that runs the production
// pipeline on raw HTML and returns the canonical markdown. It is
// intentionally local to the test file (avoiding a public
// dependency from minimize on filter+content) so the minimizer
// stays pure.
func pipelineMarkdown(t *testing.T, raw []byte) string {
	t.Helper()
	cleaned, err := filter.CleanHTML(string(raw))
	if err != nil {
		t.Fatalf("CleanHTML: %v", err)
	}
	return content.RenderMarkdown(content.ParseBlocks(cleaned))
}
```

Add the imports:

```go
import (
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/filter"
	"strings"
	"testing"
)
```

- [ ] **Step 7: Run the minimize tests**

```bash
go test ./internal/train/minimize/ -v
```

Expected: PASS. If a case fails, the minimizer's `dropNode` rules
need adjustment — examine the failing diff and decide whether to
relax `dropNode` or accept the mismatch as expected-and-update the
test.

- [ ] **Step 8: Commit**

```bash
git add internal/train/render.go internal/train/render_test.go internal/train/minimize/minimize_test.go
git commit -m "$(cat <<'EOF'
Add training Render/Markdown/Diff helpers and minimize fidelity test

Render runs the filter+content pipeline and snapshots rendered.ansi
on first call. Markdown is pure — same pipeline, RenderMarkdown
output. Diff compares current render to the snapshot via a simple
line-oriented diff. The minimize roundtrip test now actually runs
the pipeline both before and after minimization and asserts
rendered markdown is unchanged.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Add `Migrate` for legacy `corpus/` and `audit-output/`

**Files:**
- Create: `internal/train/migrate.go`
- Create: `internal/train/migrate_test.go`

`Migrate` is idempotent and supports a dry-run mode (no `--confirm`).

- [ ] **Step 1: Write the failing test**

```go
// internal/train/migrate_test.go
package train

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateDryRun(t *testing.T) {
	root := t.TempDir()
	legacyCorpus := t.TempDir()
	legacyAudit := t.TempDir()

	if err := os.WriteFile(filepath.Join(legacyCorpus, "salmon.html"), []byte("<p>x</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyAudit, "Gabc.txt"), []byte("rendered"), 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := Migrate(root, legacyAudit, legacyCorpus, false)
	if err != nil {
		t.Fatalf("Migrate dry: %v", err)
	}
	if rep.Planned != 2 || rep.Executed != 0 {
		t.Errorf("dry-run report wrong: %+v", rep)
	}
	// captures dir should still be empty
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Errorf("dry-run wrote captures: %d entries", len(entries))
	}
}

func TestMigrateConfirmAndIdempotent(t *testing.T) {
	root := t.TempDir()
	legacyCorpus := t.TempDir()
	legacyAudit := t.TempDir()

	os.WriteFile(filepath.Join(legacyCorpus, "salmon.html"), []byte("<p>full fidelity</p>"), 0o600)
	os.WriteFile(filepath.Join(legacyAudit, "Gabc.txt"), []byte("rendered only"), 0o600)

	rep, err := Migrate(root, legacyAudit, legacyCorpus, true)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if rep.Executed != 2 {
		t.Errorf("expected 2 executed, got %+v", rep)
	}
	captures, _ := List(root)
	if len(captures) != 2 {
		t.Errorf("expected 2 captures, got %d", len(captures))
	}

	// Re-run is idempotent — same hash → same id → no-op.
	rep2, err := Migrate(root, legacyAudit, legacyCorpus, true)
	if err != nil {
		t.Fatalf("Migrate 2: %v", err)
	}
	if rep2.Skipped != 2 {
		t.Errorf("expected 2 skipped on idempotent re-run, got %+v", rep2)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Expected: `undefined: Migrate`.

- [ ] **Step 3: Write the implementation**

```go
// internal/train/migrate.go
package train

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MigrateReport summarizes a Migrate run.
type MigrateReport struct {
	Planned  int // total candidates found
	Executed int // captures actually written
	Skipped  int // captures that already existed
}

// Migrate ingests legacy corpus/ and audit-output/ files into the
// training capture store. Without confirm, returns a plan only.
// Idempotent: re-running with the same legacy files is a no-op.
func Migrate(root, auditDir, corpusDir string, confirm bool) (MigrateReport, error) {
	var rep MigrateReport
	if err := os.MkdirAll(root, 0o700); err != nil {
		return rep, fmt.Errorf("migrate: mkdir root: %w", err)
	}

	// audit-output: rendered-only captures
	if auditDir != "" {
		entries, err := os.ReadDir(auditDir)
		if err != nil && !os.IsNotExist(err) {
			return rep, fmt.Errorf("migrate: read audit: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") || e.Name() == "index.txt" || e.Name() == "errors.txt" {
				continue
			}
			rep.Planned++
			path := filepath.Join(auditDir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return rep, fmt.Errorf("migrate: read %s: %w", path, err)
			}
			id := time.Now().UTC().Format("20060102") + "-" + shortHash(data)
			if _, err := os.Stat(filepath.Join(root, id)); err == nil {
				rep.Skipped++
				continue
			}
			if !confirm {
				continue
			}
			if err := writeRenderedOnly(root, id, data, e.Name()); err != nil {
				return rep, err
			}
			rep.Executed++
		}
	}

	// corpus: full-fidelity captures
	if corpusDir != "" {
		entries, err := os.ReadDir(corpusDir)
		if err != nil && !os.IsNotExist(err) {
			return rep, fmt.Errorf("migrate: read corpus: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
				continue
			}
			rep.Planned++
			path := filepath.Join(corpusDir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return rep, fmt.Errorf("migrate: read %s: %w", path, err)
			}
			id := time.Now().UTC().Format("20060102") + "-" + shortHash(data)
			if _, err := os.Stat(filepath.Join(root, id)); err == nil {
				rep.Skipped++
				continue
			}
			if !confirm {
				continue
			}
			c, err := Save(root, data, "html", "Migrated from legacy corpus/. No original comment.")
			if err != nil {
				return rep, fmt.Errorf("migrate: save corpus %s: %w", e.Name(), err)
			}
			c.Meta.Status = "triaged"
			c.Meta.Notes = "Migrated " + time.Now().UTC().Format("2006-01-02") + " from pre-pivot corpus/"
			data, _ := c.Meta.Marshal()
			os.WriteFile(filepath.Join(c.Dir, "meta.toml"), data, 0o600)
			rep.Executed++
		}
	}

	return rep, nil
}

func writeRenderedOnly(root, id string, rendered []byte, origName string) error {
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("rendered-only: mkdir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "raw.txt"), rendered, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "rendered.ansi"), rendered, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "comment.md"), []byte(""), 0o600); err != nil {
		return err
	}
	meta := Meta{
		CreatedAt:    time.Now().UTC(),
		SourceKind:   "rendered-only",
		SourceHash:   shortHash(rendered),
		Platform:     "unknown",
		Status:       "unscored",
		Notes:        "Migrated from audit-output/" + origName,
		RenderedOnly: true,
	}
	data, err := meta.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.toml"), data, 0o600)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/train/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/train/migrate.go internal/train/migrate_test.go
git commit -m "$(cat <<'EOF'
Add training Migrate for legacy corpus/ and audit-output/

Idempotent migration with dry-run as default. Audit-output entries
become rendered-only captures (no raw source available); corpus
entries become full-fidelity captures with status=triaged. Same
hash → same id → no-op on re-run.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Add `ExtractFixture` for sanitizable testdata

**Files:**
- Create: `internal/train/extract.go`
- Create: `internal/train/extract_test.go`

`ExtractFixture` runs the minimizer on a capture's raw HTML and
writes the result to a `testdata/` path. It refuses to overwrite
existing files unless `force` is true. **It does not sanitize PII**
— that's a downstream manual step.

- [ ] **Step 1: Write the failing test**

```go
// internal/train/extract_test.go
package train

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractFixture(t *testing.T) {
	root := t.TempDir()
	c, err := Save(root, []byte(`<html><body><p>hi</p><script>x</script></body></html>`), "html", "")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	out := filepath.Join(t.TempDir(), "fixture.html")

	written, err := ExtractFixture(c, out, true, false)
	if err != nil {
		t.Fatalf("ExtractFixture: %v", err)
	}
	if written != out {
		t.Errorf("written path mismatch: %s", written)
	}
	data, _ := os.ReadFile(out)
	if strings.Contains(string(data), "script") {
		t.Errorf("script not stripped: %s", data)
	}

	// Refuses to overwrite without force
	if _, err := ExtractFixture(c, out, true, false); err == nil {
		t.Error("expected overwrite refusal")
	}
	// Force allows overwrite
	if _, err := ExtractFixture(c, out, true, true); err != nil {
		t.Errorf("force overwrite failed: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Expected: `undefined: ExtractFixture`.

- [ ] **Step 3: Write the implementation**

```go
// internal/train/extract.go
package train

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glw907/poplar/internal/train/minimize"
)

// ExtractFixture runs the minimizer on a capture's raw HTML and
// writes the result to outPath. If minimize is false, copies the
// raw bytes as-is. Refuses to overwrite an existing file unless
// force is true. Returns the absolute path written.
//
// NOT a sanitizer — PII removal is a separate manual step. The
// caller (the fix-corpus skill) must review the output and scrub
// remaining identifiers before committing.
func ExtractFixture(c Capture, outPath string, doMinimize bool, force bool) (string, error) {
	if c.Meta.SourceKind != "html" {
		return "", fmt.Errorf("extract-fixture: only html captures supported, got %s", c.Meta.SourceKind)
	}
	raw, err := os.ReadFile(c.RawPath)
	if err != nil {
		return "", fmt.Errorf("extract-fixture: read raw: %w", err)
	}
	out := raw
	if doMinimize {
		out, err = minimize.Minimize(raw)
		if err != nil {
			return "", fmt.Errorf("extract-fixture: minimize: %w", err)
		}
	}
	if !force {
		if _, err := os.Stat(outPath); err == nil {
			return "", fmt.Errorf("extract-fixture: %s already exists (use --force to overwrite)", outPath)
		}
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", fmt.Errorf("extract-fixture: mkdir: %w", err)
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return "", fmt.Errorf("extract-fixture: write: %w", err)
	}
	abs, _ := filepath.Abs(outPath)
	return abs, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/train/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/train/extract.go internal/train/extract_test.go
git commit -m "$(cat <<'EOF'
Add ExtractFixture for sanitizable testdata extraction

Runs the minimizer on a capture's raw HTML and writes to a
testdata path. Refuses to overwrite without --force. Not a PII
sanitizer — that's a downstream manual step under human review.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3 — TUI

The TUI uses bubbletea. Before writing any code in this phase,
**invoke the `elm-conventions` skill** even though `internal/train/tui/`
lives outside `internal/ui/` — the discipline still applies by
project convention.

### Task 9: Build the TUI model skeleton (list pane only)

**Files:**
- Create: `internal/train/tui/model.go`
- Create: `internal/train/tui/model_test.go`

- [ ] **Step 1: Invoke `elm-conventions` skill**

Read it; do not skip.

- [ ] **Step 2: Write the failing test**

```go
// internal/train/tui/model_test.go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelInitAndJK(t *testing.T) {
	captures := fakeCaptures(3)
	m := New(captures, "")
	if cmd := m.Init(); cmd != nil {
		t.Logf("Init returned cmd (ok): %T", cmd)
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm := updated.(Model)
	if mm.cursor != 1 {
		t.Errorf("j should advance cursor to 1, got %d", mm.cursor)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	mm = updated.(Model)
	if mm.cursor != 0 {
		t.Errorf("k should retreat cursor to 0, got %d", mm.cursor)
	}
}
```

- [ ] **Step 3: Write the model**

```go
// internal/train/tui/model.go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/train"
)

// Model is the poplar train TUI root.
type Model struct {
	captures []train.Capture
	cursor   int
	root     string
	width    int
	height   int
	err      error
}

// New constructs a Model over the given captures. root is the
// captures directory (passed to Save/UpdateStatus calls).
func New(captures []train.Capture, root string) Model {
	return Model{captures: captures, root: root}
}

// Init satisfies tea.Model. No initial command.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update satisfies tea.Model. Single-key bindings only.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j":
			if m.cursor < len(m.captures)-1 {
				m.cursor++
			}
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}

// View satisfies tea.Model.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	for i, c := range m.captures {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%s  %-9s  %-8s  %s\n",
			marker, c.ID, c.Meta.Status, c.Meta.Platform,
			commentPreview(c.Comment))
	}
	b.WriteString("\nj/k move · q quit\n")
	return b.String()
}

func (m Model) header() string {
	return fmt.Sprintf("poplar train — %d captures", len(m.captures))
}

func commentPreview(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i != -1 {
		s = s[:i]
	}
	if len(s) > 50 {
		s = s[:47] + "..."
	}
	return s
}

// fakeCaptures is a test helper used by model_test.go.
func fakeCaptures(n int) []train.Capture {
	out := make([]train.Capture, n)
	for i := range out {
		out[i] = train.Capture{
			ID: fmt.Sprintf("20260413-fake%04d", i),
			Meta: train.Meta{
				Status:   "new",
				Platform: "gmail",
			},
			Comment: fmt.Sprintf("issue %d", i),
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/train/tui/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/train/tui/
git commit -m "$(cat <<'EOF'
Add poplar train TUI model skeleton with j/k navigation

bubbletea Model with a list pane, j/k cursor navigation, q to
quit. Render pane and capture flow added in subsequent commits.
Single-key bindings — no Ctrl modifiers, matching poplar
vim-first convention.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Add render pane and Enter cycling

**Files:**
- Modify: `internal/train/tui/model.go`
- Modify: `internal/train/tui/model_test.go`

- [ ] **Step 1: Add the test**

Append to `model_test.go`:

```go
func TestEnterCyclesPane(t *testing.T) {
	m := New(fakeCaptures(2), "")
	if m.pane != paneRender {
		t.Errorf("default pane should be render, got %v", m.pane)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(Model)
	if mm.pane != paneMarkdown {
		t.Errorf("after Enter pane should be markdown, got %v", mm.pane)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)
	if mm.pane != paneRaw {
		t.Errorf("after second Enter pane should be raw, got %v", mm.pane)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)
	if mm.pane != paneComment {
		t.Errorf("after third Enter pane should be comment, got %v", mm.pane)
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(Model)
	if mm.pane != paneRender {
		t.Errorf("after fourth Enter pane should wrap to render, got %v", mm.pane)
	}
}
```

- [ ] **Step 2: Add pane state to the model**

In `model.go`, add at the top of the file:

```go
type pane int

const (
	paneRender pane = iota
	paneMarkdown
	paneRaw
	paneComment
)
```

Add `pane pane` to the `Model` struct:

```go
type Model struct {
	captures []train.Capture
	cursor   int
	pane     pane
	root     string
	width    int
	height   int
	err      error
}
```

In `Update`, add the Enter case inside the `KeyMsg` switch:

```go
case "enter":
	m.pane = (m.pane + 1) % 4
```

- [ ] **Step 3: Render the bottom pane in View**

Replace the `View` method:

```go
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	for i, c := range m.captures {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%s  %-9s  %-8s  %s\n",
			marker, c.ID, c.Meta.Status, c.Meta.Platform,
			commentPreview(c.Comment))
	}
	b.WriteString("\n")
	b.WriteString(m.bottomPane())
	b.WriteString("\nj/k move · Enter cycle pane · q quit\n")
	return b.String()
}

func (m Model) bottomPane() string {
	if len(m.captures) == 0 {
		return "(no captures)"
	}
	c := m.captures[m.cursor]
	switch m.pane {
	case paneRender:
		out, err := train.Render(c)
		if err != nil {
			return fmt.Sprintf("RENDER (error): %v", err)
		}
		return "RENDER\n\n" + out
	case paneMarkdown:
		out, err := train.Markdown(c)
		if err != nil {
			return fmt.Sprintf("MARKDOWN (error): %v", err)
		}
		return "MARKDOWN\n\n" + out
	case paneRaw:
		raw, err := os.ReadFile(c.RawPath)
		if err != nil {
			return fmt.Sprintf("RAW (error): %v", err)
		}
		return "RAW\n\n" + string(raw)
	case paneComment:
		return "COMMENT\n\n" + c.Comment
	}
	return ""
}
```

Add `"os"` to the imports.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/train/tui/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/train/tui/
git commit -m "$(cat <<'EOF'
Add render/markdown/raw/comment pane cycling to train TUI

Enter cycles through four bottom panes. Render and Markdown
delegate to internal/train helpers (which run the production
pipeline). Raw reads the capture's source file. Comment shows
the user note. All errors surface in the pane content rather
than crashing the model.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Add status cycling (`f` key) and capture flow (`b` key)

**Files:**
- Modify: `internal/train/tui/model.go`
- Modify: `internal/train/tui/model_test.go`

The `b` key opens a textinput for a file path, then a textarea
for a comment, then calls `train.Save` and refreshes the list.
For Phase 3 we wire the keys; the textinput/textarea modal is
implemented in Task 12.

- [ ] **Step 1: Add tests**

```go
func TestFCyclesStatus(t *testing.T) {
	captures := fakeCaptures(1)
	captures[0].Meta.Status = "new"
	m := New(captures, "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	mm := updated.(Model)
	if mm.captures[0].Meta.Status != "triaged" {
		t.Errorf("expected triaged, got %s", mm.captures[0].Meta.Status)
	}
}

func TestBEntersCaptureMode(t *testing.T) {
	m := New(fakeCaptures(1), "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	mm := updated.(Model)
	if mm.mode != modeCapturePath {
		t.Errorf("expected capture-path mode, got %v", mm.mode)
	}
}
```

- [ ] **Step 2: Add mode state**

In `model.go`, add at the top:

```go
type mode int

const (
	modeBrowse mode = iota
	modeCapturePath
	modeCaptureComment
)
```

Add `mode mode` to the `Model` struct.

- [ ] **Step 3: Add the f and b cases in Update**

In the `KeyMsg` switch, add:

```go
case "f":
	if len(m.captures) == 0 {
		break
	}
	next := nextStatus(m.captures[m.cursor].Meta.Status)
	m.captures[m.cursor].Meta.Status = next
	if m.root != "" {
		_ = train.UpdateStatus(m.root, m.captures[m.cursor].ID, next)
	}
case "b":
	m.mode = modeCapturePath
```

Add the helper at the bottom of the file:

```go
func nextStatus(s string) string {
	order := []string{"new", "triaged", "fixed", "wontfix"}
	for i, v := range order {
		if s == v {
			return order[(i+1)%len(order)]
		}
	}
	return "new"
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/train/tui/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/train/tui/
git commit -m "$(cat <<'EOF'
Add f (cycle status) and b (enter capture mode) to train TUI

f cycles status new→triaged→fixed→wontfix→new and persists via
train.UpdateStatus when a real root is set. b transitions the
model to capture-path mode (textinput modal added in next commit).

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: Add file-path textinput and comment textarea modals

**Files:**
- Modify: `internal/train/tui/model.go`
- Modify: `internal/train/tui/model_test.go`

Wire bubbles/textinput for the file path and bubbles/textarea
for the comment. On submit of both, call `train.Save` and refresh
the capture list.

- [ ] **Step 1: Add the test**

```go
func TestCaptureFlowEndToEnd(t *testing.T) {
	root := t.TempDir()
	srcPath := filepath.Join(t.TempDir(), "msg.html")
	os.WriteFile(srcPath, []byte("<html><body><p>hi</p></body></html>"), 0o600)

	m := New(nil, root)

	// b → enter capture mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)
	if m.mode != modeCapturePath {
		t.Fatalf("expected modeCapturePath")
	}

	// type the path one rune at a time
	for _, r := range srcPath {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != modeCaptureComment {
		t.Fatalf("expected modeCaptureComment after path enter, got %v", m.mode)
	}

	// type a comment + enter
	for _, r := range "list wraps badly" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != modeBrowse {
		t.Errorf("expected modeBrowse after save, got %v", m.mode)
	}
	if len(m.captures) != 1 {
		t.Errorf("expected 1 capture after save, got %d", len(m.captures))
	}
}
```

Add imports: `"path/filepath"`, `"os"`.

- [ ] **Step 2: Wire bubbles/textinput and bubbles/textarea**

In `model.go`, add imports:

```go
import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
)
```

Add fields to `Model`:

```go
pathInput textinput.Model
comment   textarea.Model
```

Update `New` to initialize them:

```go
func New(captures []train.Capture, root string) Model {
	ti := textinput.New()
	ti.Placeholder = "path to email source"
	ti.CharLimit = 1024
	ta := textarea.New()
	ta.Placeholder = "describe the issue"
	ta.SetWidth(80)
	ta.SetHeight(5)
	return Model{
		captures:  captures,
		root:      root,
		pathInput: ti,
		comment:   ta,
	}
}
```

In `Update`, branch on `m.mode` BEFORE the key switch:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch m.mode {
		case modeCapturePath:
			return m.updateCapturePath(msg)
		case modeCaptureComment:
			return m.updateCaptureComment(msg)
		}
		// browse mode falls through to existing key switch below
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		// ... (existing cases: j, k, enter, f, b) ...
		}
	}
	return m, nil
}

func (m Model) updateCapturePath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.mode = modeBrowse
		m.pathInput.Reset()
		return m, nil
	}
	if msg.String() == "enter" {
		path := m.pathInput.Value()
		if path == "" {
			return m, nil
		}
		m.pathInput.Reset()
		m.pendingPath = path
		m.mode = modeCaptureComment
		m.comment.Focus()
		return m, nil
	}
	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

func (m Model) updateCaptureComment(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.mode = modeBrowse
		m.comment.Reset()
		m.pendingPath = ""
		return m, nil
	}
	if msg.String() == "enter" {
		// Submit the capture
		raw, err := os.ReadFile(m.pendingPath)
		if err != nil {
			m.err = err
			return m, nil
		}
		kind := "html"
		if !strings.HasSuffix(strings.ToLower(m.pendingPath), ".html") &&
			!strings.HasSuffix(strings.ToLower(m.pendingPath), ".htm") {
			kind = "plain"
		}
		c, err := train.Save(m.root, raw, kind, m.comment.Value())
		if err != nil {
			m.err = err
			return m, nil
		}
		m.captures = append(m.captures, c)
		m.cursor = len(m.captures) - 1
		m.comment.Reset()
		m.pendingPath = ""
		m.mode = modeBrowse
		return m, nil
	}
	var cmd tea.Cmd
	m.comment, cmd = m.comment.Update(msg)
	return m, cmd
}
```

Add `pendingPath string` to the Model struct.

In `View`, render the appropriate modal when `m.mode != modeBrowse`:

```go
if m.mode == modeCapturePath {
	return "Capture: enter source file path\n\n" + m.pathInput.View() + "\n\nEnter to continue · Esc to cancel\n"
}
if m.mode == modeCaptureComment {
	return "Capture: write a comment\n\n" + m.comment.View() + "\n\nEnter to save · Esc to cancel\n"
}
// ... rest of existing browse view ...
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/train/tui/ -v
```

Expected: PASS. The textinput/textarea handle their own key
buffering, so typing one rune at a time should accumulate into
`Value()`.

- [ ] **Step 4: Commit**

```bash
git add internal/train/tui/
git commit -m "$(cat <<'EOF'
Add capture path + comment textinput/textarea modals to train TUI

b key flow: enter capture-path mode, type path + Enter, transition
to capture-comment mode, type comment + Enter, train.Save, append
to captures, return to browse. Esc cancels at any modal step. The
comment buffer is bubbles/textarea so multi-line notes work.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4 — cobra subcommands

The `train` subcommand mounts under the existing `poplar` root.
Each child subcommand is thin: parse flags, call into
`internal/train`, format output, exit.

### Task 13: Mount `train` subcommand and TUI entry point

**Files:**
- Create: `cmd/poplar/train.go`
- Modify: `cmd/poplar/root.go` — register the train command

- [ ] **Step 1: Read the existing root command**

```bash
cat cmd/poplar/root.go
```

Note how `themes` and `config init` are registered. Use the same
pattern.

- [ ] **Step 2: Create `cmd/poplar/train.go`**

```go
// cmd/poplar/train.go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/train"
	"github.com/glw907/poplar/internal/train/tui"
	"github.com/spf13/cobra"
)

var trainCmd = &cobra.Command{
	Use:   "train",
	Short: "Mailrender training capture system",
	Long: `poplar train manages the rendering bug capture loop.

Run with no arguments to launch the interactive TUI. Use the named
subcommands (list, show, render, markdown, diff, capture, status,
migrate, extract-fixture, submit) for headless operation — these
are what the fix-corpus Claude skill consumes.

Captures live in $XDG_STATE_HOME/poplar/captures (default
~/.local/state/poplar/captures). They are private by design and
must never enter the repo.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root := train.Root()
		captures, err := train.List(root)
		if err != nil {
			return fmt.Errorf("train: list captures: %w", err)
		}
		m := tui.New(captures, root)
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(trainCmd)
}

// trainExit is a shared exit helper for the headless subcommands:
// non-zero exit on error with the error written to stderr.
func trainExit(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "train:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build poplar**

```bash
cd ~/Projects/poplar && make build
```

Expected: builds cleanly. Fix any compile errors in the train
package or this command file.

- [ ] **Step 4: Manual smoke test**

```bash
./poplar train --help
```

Expected: prints the `Long` description.

- [ ] **Step 5: Commit**

```bash
git add cmd/poplar/train.go
git commit -m "$(cat <<'EOF'
Add train cobra subcommand and TUI entry point

Mounts under the poplar root. Default RunE launches the interactive
training TUI via bubbletea with alt-screen. Named child subcommands
land in subsequent commits.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 14: Implement read-only headless subcommands

**Files:**
- Create: `cmd/poplar/train_inspect.go`

Adds: `list`, `show`, `render`, `markdown`, `diff`. All read-only,
all delegate to `internal/train`.

- [ ] **Step 1: Create the file**

```go
// cmd/poplar/train_inspect.go
package main

import (
	"fmt"
	"os"

	"github.com/glw907/poplar/internal/train"
	"github.com/spf13/cobra"
)

var (
	listStatusFilter string
)

var trainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List training captures (TSV: id status platform comment-preview)",
	Run: func(cmd *cobra.Command, args []string) {
		root := train.Root()
		captures, err := train.List(root)
		trainExit(err)
		for _, c := range captures {
			if listStatusFilter != "" && c.Meta.Status != listStatusFilter {
				continue
			}
			fmt.Printf("%s\t%s\t%s\t%s\n",
				c.ID, c.Meta.Status, c.Meta.Platform,
				commentPreview1Line(c.Comment))
		}
	},
}

var trainShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Print full details of a capture",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		root := train.Root()
		c, err := train.Load(root, args[0])
		trainExit(err)
		fmt.Printf("ID:        %s\n", c.ID)
		fmt.Printf("Status:    %s\n", c.Meta.Status)
		fmt.Printf("Platform:  %s\n", c.Meta.Platform)
		fmt.Printf("Source:    %s\n", c.Meta.SourceKind)
		fmt.Printf("Hash:      %s\n", c.Meta.SourceHash)
		fmt.Printf("Created:   %s\n", c.Meta.CreatedAt)
		if c.Meta.FixtureRef != "" {
			fmt.Printf("Fixture:   %s\n", c.Meta.FixtureRef)
		}
		if c.Meta.Notes != "" {
			fmt.Printf("Notes:     %s\n", c.Meta.Notes)
		}
		fmt.Println()
		fmt.Println("--- COMMENT ---")
		fmt.Println(c.Comment)
		fmt.Println("--- MARKDOWN ---")
		md, err := train.Markdown(c)
		if err != nil {
			fmt.Fprintln(os.Stderr, "markdown:", err)
		} else {
			fmt.Println(md)
		}
		fmt.Println()
		fmt.Println("--- RENDER ---")
		out, err := train.Render(c)
		if err != nil {
			fmt.Fprintln(os.Stderr, "render:", err)
		} else {
			fmt.Println(out)
		}
	},
}

var trainRenderCmd = &cobra.Command{
	Use:   "render <id>",
	Short: "Print the styled terminal render of a capture",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		root := train.Root()
		c, err := train.Load(root, args[0])
		trainExit(err)
		out, err := train.Render(c)
		trainExit(err)
		fmt.Print(out)
	},
}

var trainMarkdownCmd = &cobra.Command{
	Use:   "markdown <id>",
	Short: "Print the canonical markdown of a capture",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		root := train.Root()
		c, err := train.Load(root, args[0])
		trainExit(err)
		md, err := train.Markdown(c)
		trainExit(err)
		fmt.Print(md)
	},
}

var trainDiffCmd = &cobra.Command{
	Use:   "diff <id>",
	Short: "Diff current render against capture-time snapshot",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		root := train.Root()
		c, err := train.Load(root, args[0])
		trainExit(err)
		diff, err := train.Diff(c)
		trainExit(err)
		fmt.Print(diff)
	},
}

func init() {
	trainListCmd.Flags().StringVar(&listStatusFilter, "status", "", "filter by status")
	trainCmd.AddCommand(trainListCmd, trainShowCmd, trainRenderCmd, trainMarkdownCmd, trainDiffCmd)
}

func commentPreview1Line(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}
```

- [ ] **Step 2: Build**

```bash
make build
```

- [ ] **Step 3: Manual smoke test against the empty store**

```bash
./poplar train list
```

Expected: empty output (no captures yet) and exit 0.

- [ ] **Step 4: Commit**

```bash
git add cmd/poplar/train_inspect.go
git commit -m "$(cat <<'EOF'
Add train inspect subcommands: list, show, render, markdown, diff

Read-only headless surface for the fix-corpus skill. list emits
TSV (id, status, platform, comment-preview) for easy parsing. show
prints meta + comment + markdown + render. render and markdown
emit just the named output for piping. diff compares current
render to the capture-time snapshot.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 15: Implement `capture` and `status` subcommands

**Files:**
- Create: `cmd/poplar/train_write.go`

- [ ] **Step 1: Create the file**

```go
// cmd/poplar/train_write.go
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/glw907/poplar/internal/train"
	"github.com/spf13/cobra"
)

var (
	captureComment string
)

var trainCaptureCmd = &cobra.Command{
	Use:   "capture <path>",
	Short: "Import an email file as a new capture",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		raw, err := os.ReadFile(path)
		trainExit(err)
		kind := "html"
		lower := strings.ToLower(path)
		if !strings.HasSuffix(lower, ".html") && !strings.HasSuffix(lower, ".htm") {
			kind = "plain"
		}
		c, err := train.Save(train.Root(), raw, kind, captureComment)
		trainExit(err)
		fmt.Println(c.ID)
	},
}

var trainStatusCmd = &cobra.Command{
	Use:   "status <id> <state>",
	Short: "Set the status of a capture",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		err := train.UpdateStatus(train.Root(), args[0], args[1])
		trainExit(err)
	},
}

func init() {
	trainCaptureCmd.Flags().StringVar(&captureComment, "comment", "", "comment to attach to the new capture")
	trainCmd.AddCommand(trainCaptureCmd, trainStatusCmd)
}
```

- [ ] **Step 2: Build and smoke test**

```bash
make build
echo '<html><body><p>smoke test</p></body></html>' > /tmp/smoke.html
./poplar train capture /tmp/smoke.html --comment "smoke test"
./poplar train list
```

Expected: a single capture id printed, then `list` shows it.

- [ ] **Step 3: Test the status command**

```bash
ID=$(./poplar train list | head -1 | cut -f1)
./poplar train status "$ID" triaged
./poplar train list
```

Expected: status field flips to `triaged`.

- [ ] **Step 4: Clean up the test capture**

```bash
rm -rf "$HOME/.local/state/poplar/captures"
```

- [ ] **Step 5: Commit**

```bash
git add cmd/poplar/train_write.go
git commit -m "$(cat <<'EOF'
Add train capture and status headless subcommands

capture imports an email file as a new capture (kind inferred
from extension). status sets a capture's status field. Both
write to the xdg state captures dir.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 16: Implement `migrate`, `extract-fixture`, and `submit` subcommands

**Files:**
- Create: `cmd/poplar/train_ops.go`

- [ ] **Step 1: Create the file**

```go
// cmd/poplar/train_ops.go
package main

import (
	"fmt"
	"os/exec"

	"github.com/glw907/poplar/internal/train"
	"github.com/spf13/cobra"
)

var (
	migrateConfirm bool
	migrateAudit   string
	migrateCorpus  string

	extractOut       string
	extractNoMin     bool
	extractForce     bool

	submitGhPath string
)

var trainMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate legacy corpus/ and audit-output/ into the capture store",
	Long: `Idempotent migration. Without --confirm, prints a dry-run
report and exits 0. With --confirm, executes the moves.`,
	Run: func(cmd *cobra.Command, args []string) {
		rep, err := train.Migrate(train.Root(), migrateAudit, migrateCorpus, migrateConfirm)
		trainExit(err)
		fmt.Printf("planned:  %d\n", rep.Planned)
		fmt.Printf("executed: %d\n", rep.Executed)
		fmt.Printf("skipped:  %d\n", rep.Skipped)
		if !migrateConfirm {
			fmt.Println("\n(dry run; pass --confirm to execute)")
		}
	},
}

var trainExtractFixtureCmd = &cobra.Command{
	Use:   "extract-fixture <id>",
	Short: "Extract a minimized HTML fixture from a capture",
	Long: `Runs the HTML minimizer on the capture's raw source and writes
the result to --out. NOT a sanitizer — manually scrub any remaining
PII before committing.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c, err := train.Load(train.Root(), args[0])
		trainExit(err)
		if extractOut == "" {
			trainExit(fmt.Errorf("--out is required"))
		}
		written, err := train.ExtractFixture(c, extractOut, !extractNoMin, extractForce)
		trainExit(err)
		fmt.Println(written)
	},
}

var trainSubmitCmd = &cobra.Command{
	Use:   "submit <id>",
	Short: "Open a PR for the fix associated with a capture (gh pr create)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		gh := submitGhPath
		if gh == "" {
			gh = "gh"
		}
		if _, err := exec.LookPath(gh); err != nil {
			trainExit(fmt.Errorf("submit: %s not found in PATH; install from https://cli.github.com/", gh))
		}
		c, err := train.Load(train.Root(), args[0])
		trainExit(err)
		body := fmt.Sprintf("Fix for capture %s.\n\nComment:\n%s\n", c.ID, c.Comment)
		out, err := exec.Command(gh, "pr", "create", "--title",
			"renderer: fix from capture "+c.ID, "--body", body).CombinedOutput()
		fmt.Print(string(out))
		trainExit(err)
	},
}

func init() {
	trainMigrateCmd.Flags().BoolVar(&migrateConfirm, "confirm", false, "execute moves (default is dry run)")
	trainMigrateCmd.Flags().StringVar(&migrateAudit, "audit", "audit-output", "legacy audit-output dir")
	trainMigrateCmd.Flags().StringVar(&migrateCorpus, "corpus", "corpus", "legacy corpus dir")

	trainExtractFixtureCmd.Flags().StringVar(&extractOut, "out", "", "output testdata path (required)")
	trainExtractFixtureCmd.Flags().BoolVar(&extractNoMin, "no-minimize", false, "skip HTML minimization")
	trainExtractFixtureCmd.Flags().BoolVar(&extractForce, "force", false, "overwrite existing file")

	trainSubmitCmd.Flags().StringVar(&submitGhPath, "gh", "", "path to gh binary (default: gh in PATH)")

	trainCmd.AddCommand(trainMigrateCmd, trainExtractFixtureCmd, trainSubmitCmd)
}
```

- [ ] **Step 2: Build**

```bash
make build
```

- [ ] **Step 3: Smoke test migrate dry-run**

```bash
./poplar train migrate
```

Expected: prints `planned: ~46`, `executed: 0`, `skipped: 0`,
plus the "dry run" notice. Does NOT modify the captures dir.

- [ ] **Step 4: Run make check**

```bash
make check
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/poplar/train_ops.go
git commit -m "$(cat <<'EOF'
Add train migrate, extract-fixture, and submit subcommands

migrate takes --confirm; default is a dry-run plan. extract-fixture
requires --out and refuses to overwrite without --force. submit
shells out to gh pr create with a body referencing the capture.
gh-not-installed is a hard error with an install hint.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5 — Skill rewrite

The `fix-corpus` skill becomes a directory. The new normative
reference `well-crafted-markdown.md` is a load-bearing artifact —
write it with the same care as code.

### Task 17: Restructure `fix-corpus` to a directory skill with new SKILL.md

**Files:**
- Delete: `.claude/skills/fix-corpus` (the old single file)
- Create: `.claude/skills/fix-corpus/SKILL.md`

- [ ] **Step 1: Remove the old skill file**

```bash
git rm .claude/skills/fix-corpus
mkdir -p .claude/skills/fix-corpus
```

- [ ] **Step 2: Write the new SKILL.md**

```markdown
---
name: fix-corpus
description: >
  Triage and fix rendering bugs from training captures. Loads the
  normative well-crafted-markdown.md reference at the start of
  every triage pass and evaluates each capture against it. Trigger
  on "fix rendering bug", "review captures", "mailrender issue",
  "email rendering broken", "triage corpus", or explicit invocation.
---

# Fix Corpus

Drive renderer bug fixes through the poplar training capture loop.

## Required reading at start

1. **`well-crafted-markdown.md`** (sibling file in this directory).
   The normative definition of "good markdown." Load it before
   touching any capture — it defines the standard you score
   against.
2. `docs/poplar/invariants.md` — for binding facts about the
   pipeline and the training system.
3. `docs/poplar/training.md` — for the capture system overview.

## Prerequisites

- `poplar train` is installed (`make install` in the repo root).
- At least one capture exists in `~/.local/state/poplar/captures/`.
  If none, run `poplar train capture <path>` against a problematic
  email file first and add a comment via the TUI.

## Workflow

### 1. Pull the triage queue

```bash
poplar train list --status new
```

TSV format: `id<TAB>status<TAB>platform<TAB>comment-preview`. If
the queue is empty, also try `--status unscored` for migrated
audit-output entries that need first-pass annotation.

### 2. Inspect each capture

For every id in the queue:

```bash
poplar train show <id>
```

Read the developer comment. Read the markdown output. Read the
render output. **Do not skip the comment** — it captures
information the automated checks cannot recover.

### 3. Score against the reference

For each capture, walk through the five sections of
`well-crafted-markdown.md`:

1. **§4 metrics first** — compute density signals (lines per
   paragraph, chars per line, blank-line ratio, vertical extent,
   orphan rate, heading density). Note any outliers.
2. **§3 syntactic rules** — apply each MUST/SHOULD rule against
   the markdown output. Record violations as `{rule, location,
   probable fix layer}`.
3. **§2 inference rules** — walk the raw source and check whether
   the pipeline made each expected inference (heading from bold,
   list from prefix patterns, etc.). Record misses.
4. **§1 principles** — apply when the rules are silent or in
   conflict. Principles win over rules in ambiguous cases; flag
   the rule for revision.
5. Cross-reference the developer comment with the violations. If
   the comment mentions an issue and a rule confirms it, that's
   high confidence. Disagreement = manual inspection.

### 4. Group by root cause

After all captures are scored, group violations across the queue
by root cause. **Holistic fixes beat one-at-a-time fixes.** A
single change to `internal/filter/` may resolve issues in eight
captures at once.

Present the grouped issues to the developer for confirmation
before editing any code.

### 5. Apply fixes

For each grouped fix:

1. Edit the appropriate file in `internal/filter/` or
   `internal/content/`. Invoke `go-conventions` skill before
   writing Go code.
2. Add a unit test in the existing table-driven style
   (`*_test.go` in the same package, no assertion libraries).
3. Run `poplar train diff <id>` for each affected capture to
   confirm the fix is regression-neutral or improved.
4. Run `make check` after each fix.

### 6. Sanitize and extract test fixtures

For each capture that drove a fix:

```bash
poplar train extract-fixture <id> --out testdata/filter/<slug>.html
```

(Or `testdata/content/<slug>.html` if the fix landed in
`internal/content/`.)

Then **manually review the extracted fixture for PII**:

- Replace real names with Alice, Bob, Carol.
- Replace real emails with `@example.com` addresses.
- Replace real URLs with `example.com` paths.
- Strip tracking parameters (`utm_*`, `fbclid`, `gclid`, `mc_*`,
  `_hsenc`, `_hsmi`).
- Strip account numbers, order numbers, postal addresses.
- Strip any unique identifiers that could re-identify the sender.

When the user has trusted the auto-scrub (e.g., `--trust` was set
in the workflow), the manual review is still recommended — never
fully bypass it for outside contributions.

Add a golden file alongside the fixture:

```go
// testdata/filter/<slug>_golden.txt
<expected RenderMarkdown output for the sanitized fixture>
```

Add a test case in the matching `_test.go` that loads the fixture
and golden, runs the pipeline, and asserts equality.

### 7. Mark fixed

```bash
poplar train status <id> fixed
```

For each capture the fix resolved.

### 8. Ship

For the primary maintainer:

```bash
/ship
```

For an outside contributor without push access:

```bash
poplar train submit <id>
```

This wraps `gh pr create` with a body referencing the capture.

## Notes

- **Captures are PII.** Never paste capture content into chat
  outputs, never commit raw HTML from a capture, never share
  captures across machines. Only sanitized `testdata/` fixtures
  ever leave the local capture dir.
- **The reference is the standard.** When in doubt, edit
  `well-crafted-markdown.md` to clarify the rule. Don't make
  ad-hoc judgments; codify them.
- **Use `poplar train`, not the filesystem.** Never read
  `~/.local/state/poplar/captures/` directly. The skill is
  decoupled from the on-disk layout for a reason.
```

- [ ] **Step 3: Verify the skill is discoverable**

The skill should be registered automatically because Claude Code
walks `.claude/skills/` looking for `SKILL.md` files. No registry
edit is needed.

- [ ] **Step 4: Commit**

```bash
git add .claude/skills/fix-corpus/
git commit -m "$(cat <<'EOF'
Restructure fix-corpus as directory skill with new SKILL.md

Old single-file fix-corpus removed. New SKILL.md drives the
training capture loop end to end: load reference, pull queue,
score against the well-crafted-markdown spec, group by root
cause, fix, sanitize, extract fixture, mark fixed, ship.
Reference doc lands in the next commit.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 18: Write `well-crafted-markdown.md` reference

**Files:**
- Create: `.claude/skills/fix-corpus/well-crafted-markdown.md`

This is the load-bearing artifact. Write it in full — no stubs.
The structure follows the spec's "Components → well-crafted-markdown"
section.

- [ ] **Step 1: Write the reference**

Use the spec at `docs/superpowers/specs/2026-04-12-mailrender-training-design.md`
section "`.claude/skills/fix-corpus/well-crafted-markdown.md` (new
— normative reference)" as the source of truth. Reproduce the
full content of that section (§1 Principles, §2 Structural
inference rules, §3 Syntactic rules, §4 Density signals, §5
Evaluation procedure) into the new file. The spec already
contains the actual rules and inference patterns; copy them
verbatim.

The file should start with:

```markdown
# Well-Crafted Markdown — Normative Reference

This document defines what "well-crafted markdown" means for the
poplar rendering pipeline. It is the standard the `fix-corpus`
skill scores every capture against. It is versioned with the
skill and updated in place as the understanding of good output
evolves.

This document is normative. When the rules and the principles
disagree, principles win and the rule is flagged as a candidate
for revision.

---

## §1 Principles

[Reproduce the principles from the spec — density is signal,
structure is inferred not copied, output is for narrow terminal
columns, consistency beats cleverness, failures are diagnosable.]

## §2 Structural Inference Rules

[Reproduce the inference rules from the spec — heading inference,
paragraph reconstruction, list detection, signature separation.]

## §3 Syntactic Rules

[Reproduce the MUST/SHOULD/MAY rules from the spec — entities,
whitespace hygiene, orphaned punctuation, heading hygiene, list
formatting, blockquote wrapping, link handling, code, horizontal
rules, emphasis. Each rule needs a probable-fix-layer hint.]

## §4 Density Signals

[Reproduce the density metrics from the spec — lines per
paragraph, chars per line, blank-line ratio, block count per 100
chars, vertical extent, orphan rate, heading density.]

## §5 Evaluation Procedure

[Reproduce the 10-step procedure from the spec.]
```

When transcribing from the spec, **do not paraphrase**. The whole
point of the reference is reproducibility — exact wording matters.

- [ ] **Step 2: Verify the file is non-stub**

```bash
wc -l .claude/skills/fix-corpus/well-crafted-markdown.md
```

Expected: at least 250 lines (the spec section is substantial).
If it's under 100, you stubbed it. Go back and reproduce the spec
content fully.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/fix-corpus/well-crafted-markdown.md
git commit -m "$(cat <<'EOF'
Add well-crafted-markdown.md normative reference for fix-corpus

Five-section reference: principles, structural inference rules,
syntactic rules (MUST/SHOULD/MAY), density signals, evaluation
procedure. Normative — fix-corpus loads it at the start of every
triage pass and scores captures against it. Versioned with the
skill; updates require a commit.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 19: Add pass-end capture status checklist item to `poplar-pass`

**Files:**
- Modify: `.claude/skills/poplar-pass/SKILL.md`

- [ ] **Step 1: Read the existing skill**

```bash
cat .claude/skills/poplar-pass/SKILL.md
```

Locate the pass-end checklist section.

- [ ] **Step 2: Add one line**

Insert this bullet near the end of the pass-end checklist:

```markdown
- If any training captures were touched this pass, update their
  status via `poplar train status <id> <state>`. Captures that
  were used to drive a fix should move from `new` or `triaged`
  to `fixed`.
```

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/poplar-pass/SKILL.md
git commit -m "$(cat <<'EOF'
Add capture status update to poplar-pass end checklist

Pass-end ritual now reminds the skill to update training capture
status for any captures used during the pass. Keeps the capture
store status field honest.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 6 — Migration

### Task 20: Migrate legacy files and delete in-repo dirs

**Files:**
- Delete: `corpus/`
- Delete: `audit-output/`
- Modify: `.gitignore`

This is a manual operational task that uses the code from prior
phases.

- [ ] **Step 1: Build and install the latest poplar**

```bash
make install
```

Expected: `~/.local/bin/poplar` updated.

- [ ] **Step 2: Dry-run the migration**

```bash
poplar train migrate
```

Expected: planned ~46, executed 0, skipped 0, plus dry-run notice.

- [ ] **Step 3: Execute the migration**

```bash
poplar train migrate --confirm
```

Expected: planned 46, executed 46, skipped 0.

- [ ] **Step 4: Verify captures landed**

```bash
poplar train list | wc -l
poplar train list | head -5
```

Expected: 46 lines, mix of `triaged` (the 1 corpus file) and
`unscored` (the 45 audit entries).

- [ ] **Step 5: Verify nothing in the repo references the legacy dirs**

```bash
grep -rn "corpus/" --include="*.go" --include="*.md" .
grep -rn "audit-output" --include="*.go" --include="*.md" .
```

If any results show up beyond the spec/plan/skill files (which
reference them historically), update those references first.

- [ ] **Step 6: Delete the legacy dirs**

```bash
rm -rf corpus/ audit-output/
```

- [ ] **Step 7: Update `.gitignore`**

Add to `.gitignore`:

```
# Legacy training dirs — captures live in $XDG_STATE_HOME/poplar/captures
corpus/
audit-output/
```

- [ ] **Step 8: Run make check**

```bash
make check
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add -u corpus/ audit-output/ .gitignore
git commit -m "$(cat <<'EOF'
Migrate legacy corpus/ and audit-output/ to training capture store

46 legacy files migrated into ~/.local/state/poplar/captures via
poplar train migrate --confirm. Salmon-selvedge corpus entry
landed as a full-fidelity capture (status: triaged). 45
audit-output entries landed as rendered-only captures (status:
unscored, no raw source). Both directories deleted from the repo
and gitignored to prevent regrowth.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 7 — Infrastructure documentation

These docs make the system discoverable to future Claude sessions.

### Task 21: Write `docs/poplar/training.md`

**Files:**
- Create: `docs/poplar/training.md`

- [ ] **Step 1: Write the doc**

Target length: 150-250 lines. Sections:

```markdown
# Poplar Training Capture System

On-demand reference. Load when fixing a rendering bug, reviewing
captures, or contributing a renderer fix. The fix-corpus skill is
the day-to-day driver; this doc is the human-facing overview.

## Overview

The training system replaces the pre-pivot `corpus/` +
`mailrender save` workflow. It captures problematic email renders
together with a developer comment and a rendering snapshot, stores
them privately under XDG state, and exposes a headless API the
fix-corpus Claude skill consumes.

Captures are PII by design. They never enter the repo. Only
sanitized `testdata/` fixtures derived from captures are
committable.

## Where captures live

`xdg.StatePath("poplar", "captures")`, which resolves to:

- Linux / macOS: `~/.local/state/poplar/captures/`
- Any OS with `$XDG_STATE_HOME` set: `$XDG_STATE_HOME/poplar/captures/`

Each capture is a directory:

```
20260413-a3b1c4/
├── raw.html        # or raw.txt for plaintext sources
├── rendered.ansi   # styled output at capture time (snapshot)
├── comment.md      # developer note
└── meta.toml       # status, platform, hash, timestamp
```

## Capturing a problem render

Two paths:

**Interactive:** `poplar train` opens the TUI. Press `b`, type the
path to the problematic email source, type a comment, Enter to
save. The capture appears in the list.

**Headless:** `poplar train capture <path> --comment "what's wrong"`
imports an email file and prints the new capture id.

## Inspecting captures

- `poplar train list` — TSV of all captures.
- `poplar train list --status new` — only new captures.
- `poplar train show <id>` — full details (meta, comment, markdown,
  render).
- `poplar train render <id>` — styled render only (stdout).
- `poplar train markdown <id>` — canonical markdown only (stdout).
- `poplar train diff <id>` — current render vs capture-time snapshot
  (regression detection after a fix).

## PII policy

Captures are private and stay private. Forever.

- Never paste capture content into chat outputs.
- Never commit raw HTML from a capture.
- Never share captures across machines.
- Only sanitized `testdata/` fixtures derived via `extract-fixture`
  ever enter the repo, and even those go through manual review.

## Submitting a fix as a contributor

1. Use `poplar train` to capture the problem and write a comment.
2. Invoke the `fix-corpus` skill — it runs the triage loop and
   proposes a code fix.
3. Once `make check` passes, run:
   ```
   poplar train extract-fixture <id> --out testdata/filter/<slug>.html
   ```
4. Manually scrub PII from the extracted fixture (see the
   sanitization checklist in `.claude/skills/fix-corpus/SKILL.md`).
5. Add a golden test that loads the sanitized fixture and asserts
   the expected `RenderMarkdown` output.
6. For the primary maintainer: `/ship`. For outside contributors:
   `poplar train submit <id>` (wraps `gh pr create`).

## Migrating legacy files

One-shot for the pre-pivot `corpus/` and `audit-output/` dirs:

```
poplar train migrate           # dry run
poplar train migrate --confirm # execute
```

Idempotent — same hash → same id → no-op on re-run.

## The "well-crafted markdown" standard

The fix-corpus skill scores captures against a normative
reference at:

```
.claude/skills/fix-corpus/well-crafted-markdown.md
```

That file is the single source of truth for what good output
looks like. Five sections: principles, structural inference rules,
syntactic rules (MUST/SHOULD/MAY), density signals, evaluation
procedure. To propose a change to the standard, edit that file
and commit — the next triage pass picks it up automatically.

This doc never duplicates the standard. The reference lives in
the skill.
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/training.md
git commit -m "$(cat <<'EOF'
Add docs/poplar/training.md on-demand reference

Human-facing overview of the training capture system: where
captures live, how to capture, how to inspect, PII policy, the
contributor submit flow, and a pointer to the well-crafted
markdown reference. Loaded on demand when working on rendering
issues.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 22: Write ADR 0059

**Files:**
- Create: `docs/poplar/decisions/0059-training-capture-system.md`

- [ ] **Step 1: Verify the next ADR number**

```bash
ls docs/poplar/decisions/ | tail -5
```

Confirm 0058 is the highest. If a 0059 exists already, bump.

- [ ] **Step 2: Write the ADR**

Use the spec section "`docs/poplar/decisions/0059-training-capture-system.md`
(new)" as the source of truth. The ADR contains: Context,
Decision, Consequences, Alternatives considered, Cross-refs.
Reproduce the full content from the spec.

- [ ] **Step 3: Commit**

```bash
git add docs/poplar/decisions/0059-training-capture-system.md
git commit -m "$(cat <<'EOF'
Add ADR 0059: training capture system

Codifies the design decisions: capture store at xdg state,
markdown as audit artifact (not pipeline intermediate), fix-corpus
as the authoritative loop, PII contained outside the repo. Cross-
refs ADR 0001, 0046, 0058 and BACKLOG #7.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 23: Update `docs/poplar/invariants.md`

**Files:**
- Modify: `docs/poplar/invariants.md`

- [ ] **Step 1: Add four new bullets**

Use the spec section "`docs/poplar/invariants.md`" under
"Infrastructure updates" as the source. Add the four bullets to
the appropriate sections (Architecture × 2, UX × 1, Build &
verification × 1).

- [ ] **Step 2: Add the decision-index row**

Append to the decision-index table:

```markdown
| Training capture system, markdown as audit artifact | 0059 |
```

- [ ] **Step 3: Commit**

```bash
git add docs/poplar/invariants.md
git commit -m "$(cat <<'EOF'
Update invariants for training capture system

Four new binding facts: capture store location (xdg state, never
in-repo), internal/train ownership and markdown-as-audit, train
TUI exemption from user-facing UX rules, fix-corpus skill as
authoritative loop. Decision index gains row for ADR 0059.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 24: Update `docs/poplar/system-map.md`

**Files:**
- Modify: `docs/poplar/system-map.md`

- [ ] **Step 1: Add the package row**

In the package layout table, add:

```markdown
| `internal/train/` | Capture store (xdg state), `poplar train` TUI, migration, HTML minimizer. Consumers: `cmd/poplar/train.go`, `.claude/skills/fix-corpus`. |
```

- [ ] **Step 2: Update the `internal/content/` row**

Append to its description: "`RenderMarkdown` is an audit sibling
to `RenderBody`, consumed by `poplar train markdown` and
`fix-corpus`."

- [ ] **Step 3: Update the Binary section**

Note that `cmd/poplar/` now includes the `train` subcommand.

- [ ] **Step 4: Add the docs line**

In the Docs section, add:

```markdown
- `docs/poplar/training.md` — capture store, poplar train workflow, PII policy, fix-corpus loop.
```

- [ ] **Step 5: Commit**

```bash
git add docs/poplar/system-map.md
git commit -m "$(cat <<'EOF'
Update system-map for training capture system

Adds internal/train/ package row, notes RenderMarkdown audit
sibling on internal/content/, mentions train subcommand under
cmd/poplar/, and adds training.md to the docs list.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 25: Update `docs/poplar/STATUS.md` and `CLAUDE.md`

**Files:**
- Modify: `docs/poplar/STATUS.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add Tooling section to STATUS.md**

Append below the pass table:

```markdown
## Tooling

- **Training capture system** — `poplar train` + `internal/train/` +
  `fix-corpus` skill. Built out-of-band from the pass sequence.
  Provides the authoritative loop for renderer bug-fixing. See
  `docs/poplar/training.md`.
```

- [ ] **Step 2: Add on-demand-reading line to CLAUDE.md**

In the "On-demand reading" section of `CLAUDE.md`, add:

```markdown
- `docs/poplar/training.md` — capture store + fix-corpus loop.
  **Load when a user reports a rendering bug or asks to review
  captures.**
```

Verify CLAUDE.md is still under the 200-line limit:

```bash
wc -l CLAUDE.md
```

If it's at or above 200, condense an existing line first.

- [ ] **Step 3: Commit**

```bash
git add docs/poplar/STATUS.md CLAUDE.md
git commit -m "$(cat <<'EOF'
Document training tooling in STATUS.md and CLAUDE.md

STATUS gains a Tooling section flagging the training capture
system as out-of-band of the pass sequence. CLAUDE.md gains an
on-demand-reading line for training.md so future sessions discover
it when working on rendering issues.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 8 — Ship

### Task 26: Final quality gates and ship

- [ ] **Step 1: Run full test suite**

```bash
make check
```

Expected: PASS. Fix any failures before proceeding.

- [ ] **Step 2: Run `/simplify`**

This launches the three-agent simplification review (reuse,
quality, efficiency). Aggregate findings; apply genuine wins;
commit any cleanups as their own commit.

- [ ] **Step 3: Verify the binary works end to end**

```bash
make install
poplar train list
```

Expected: lists migrated captures. If you wiped the state dir
during testing, re-run migrate first.

- [ ] **Step 4: Smoke test the skill**

In a Claude Code session in this repo, type "review captures" or
"fix rendering bug." The fix-corpus skill should activate and
load `well-crafted-markdown.md`.

- [ ] **Step 5: Run `/ship`**

This commits any straggling docs, pushes to remote, and reinstalls
the binary.

- [ ] **Step 6: Update STATUS.md to mark the training pass complete**

Edit the Tooling section to add a status indicator:

```markdown
## Tooling

- **Training capture system** ✅ shipped 2026-04-13 — `poplar train`
  + `internal/train/` + `fix-corpus` skill. Provides the
  authoritative loop for renderer bug-fixing. See
  `docs/poplar/training.md`.
```

Commit:

```bash
git add docs/poplar/STATUS.md
git commit -m "Mark training capture system as shipped in STATUS"
git push
```

---

## Self-review checklist (run after writing the plan)

- ✅ Every spec deliverable mapped to a task: 15 deliverables → 26 tasks across 8 phases.
- ✅ No placeholders, TBDs, or "implement later" markers.
- ✅ Type/function names consistent across tasks (`Save`, `Load`, `List`, `UpdateStatus`, `Migrate`, `ExtractFixture`, `Render`, `Markdown`, `Diff` all match the spec).
- ✅ Tests precede implementation in every code task (TDD).
- ✅ Each task ends with a commit step.
- ✅ Phase 6 (migration) explicitly depends on Phase 4 (cobra subcommands) being shipped via `make install`.
- ✅ Phase 5 (skill rewrite) does not depend on any code phase — can be done in parallel if desired.

## Cross-references

- **Spec:** `docs/superpowers/specs/2026-04-12-mailrender-training-design.md`
- **ADR (created in Task 22):** `docs/poplar/decisions/0059-training-capture-system.md`
- **Reference doc (created in Task 18):** `.claude/skills/fix-corpus/well-crafted-markdown.md`
- **Existing pipeline:** `internal/filter/`, `internal/content/`
- **xdg helper:** `internal/mailworker/xdg/xdg.go`
