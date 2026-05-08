# Claude Tidy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the existing `internal/tidy/` package into the compose body as an explicit `Ctrl+T` action that rewrites the body in place and highlights the changed character ranges.

**Architecture:** Compose intercepts `Ctrl+T` when the body is focused, runs `tidy.Tidy` in a `tea.Cmd`, replaces the catkin buffer on success, and feeds a character-range diff to a new `tidyAnnotator` in catkin that paints changed spans. Tidy is never on the send path.

**Tech Stack:** Go, bubbletea, lipgloss, internal/tidy (existing), internal/catkin (existing), Anthropic messages API.

**Spec:** `docs/superpowers/specs/2026-05-08-claude-tidy-design.md`.

---

## File map

**Create**
- `internal/tidy/diff.go` — rune-level LCS, returns `[]ByteRange`.
- `internal/tidy/diff_test.go`
- `internal/ui/compose/tidy.go` — `tidyCmd`, `tidyResultMsg`, `handleTidyKey`, model fields.
- `internal/ui/compose/tidy_test.go`

**Modify**
- `internal/tidy/config.go` — expose `Validate(Config) error`.
- `internal/tidy/config_test.go` — add `Validate` cases.
- `internal/catkin/style.go` — add `Styles.TidyChange`.
- `internal/catkin/annotate.go` — add `KindTidyChange`, `tidyAnnotator`, `Model.SetTidyHighlights`. Register annotator in `New`.
- `internal/catkin/annotate_test.go` — annotator tests.
- `internal/catkin/catkin.go` — register tidyAnnotator in `New`; add `SetTidyHighlights` forwarder.
- `internal/compose/editor.go` — add `SetCatkinStyles` + `SetTidyHighlights` to `Editor` interface and `CatkinEditor`.
- `internal/config/ui.go` — add `Tidy TidyConfig` (a poplar-side wrapper around `tidy.Config` with `Enabled bool`); wire into `LoadUI`.
- `internal/config/ui_test.go` — `[ui.tidy]` decode + validation cases.
- `internal/ui/compose/styles.go` — add `TidyChange` style; expose a helper that builds a `catkin.Styles` for the editor.
- `internal/ui/compose/model.go` — add `tidyFn`, `tidyInFlight`, `tidyCfg`, `tidyAPIKey`, `info`, `infoStyle` fields; constructor wires defaults; Init pushes catkin styles into the editor.
- `internal/ui/compose/bind.go` (or wherever the key dispatch sits) — route `Ctrl+T` through `handleTidyKey` when body focused.
- `internal/ui/app.go` — retire `TidyFn`, `WithTidy`, `tidy TidyFn`, `identityTidy`, `tidy: identityTidy` init, `WithTidy` callsites; thread `[ui.tidy]` config + resolved API key into compose at construction.
- `internal/ui/cmds.go` — drop `tidy TidyFn` parameter from `composeSendCmd`; revert to straight assemble→queue.
- `internal/ui/help_keys.go` (or compose section of help vocabulary) — add `Ctrl+T` row, `wired = true`.
- `internal/ui/footer.go` (or wherever compose footer hints live) — add `^T tidy` hint, drop rank ~6, visible only when body focused and `[ui.tidy] enabled`.
- `docs/poplar/styling.md` — add row binding `catkin.Styles.TidyChange` → `AccentPrimary` (underline).

---

## Task 1: `tidy.DiffRanges` — character-level rune diff

**Files:**
- Create: `internal/tidy/diff.go`
- Test: `internal/tidy/diff_test.go`

LCS over runes; coalesces adjacent change runs into one byte range in `newText`. Pure, no I/O, no third-party deps.

- [ ] **Step 1: Write failing tests**

```go
// internal/tidy/diff_test.go
package tidy

import (
	"reflect"
	"testing"
)

func TestDiffRanges(t *testing.T) {
	cases := []struct {
		name     string
		oldText  string
		newText  string
		want     []ByteRange
	}{
		{"empty/empty", "", "", nil},
		{"identical", "hello world", "hello world", nil},
		{"empty old, all new", "", "hello", []ByteRange{{0, 5}}},
		{"single rune insertion", "hello world", "hello, world", []ByteRange{{5, 6}}},
		{"single rune deletion", "hello, world", "hello world", nil}, // deletions show as nothing in new text
		{"contiguous run", "teh quick brown", "the quick brown", []ByteRange{{1, 3}}},
		{"two non-adjacent edits", "teh quick fox", "the quick foxes", []ByteRange{{1, 3}, {13, 15}}},
		{"multibyte rune change", "café", "cafe", []ByteRange{{3, 4}}},
		{"all-different short", "abc", "xyz", []ByteRange{{0, 3}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DiffRanges(tc.oldText, tc.newText)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("DiffRanges(%q, %q) = %v, want %v",
					tc.oldText, tc.newText, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test, confirm fail**

```bash
go test ./internal/tidy/ -run TestDiffRanges -v
```
Expected: `undefined: DiffRanges` / `undefined: ByteRange`.

- [ ] **Step 3: Implement `DiffRanges`**

```go
// internal/tidy/diff.go
package tidy

// ByteRange is a half-open byte span [Start, End) in newText where
// the rune sequence diverges from oldText. Ranges are sorted by Start
// and never overlap.
type ByteRange struct{ Start, End int }

// DiffRanges returns the byte ranges in newText whose runes were not
// matched against oldText by a longest common subsequence walk.
// Adjacent change positions coalesce into one range. Pure deletions
// (runes in oldText that disappear from newText) produce no range —
// there is no place in newText to underline.
func DiffRanges(oldText, newText string) []ByteRange {
	if newText == "" {
		return nil
	}
	if oldText == newText {
		return nil
	}

	oldRunes := []rune(oldText)
	newRunes := []rune(newText)

	// LCS table: lcs[i][j] = length of LCS of oldRunes[:i] and newRunes[:j].
	n, m := len(oldRunes), len(newRunes)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if oldRunes[i-1] == newRunes[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else if lcs[i-1][j] >= lcs[i][j-1] {
				lcs[i][j] = lcs[i-1][j]
			} else {
				lcs[i][j] = lcs[i][j-1]
			}
		}
	}

	// Walk back. Mark which runes in newRunes are matched (kept) vs
	// inserted. Runes in oldRunes that get dropped don't appear in
	// newText so we ignore them.
	matched := make([]bool, m)
	i, j := n, m
	for i > 0 && j > 0 {
		if oldRunes[i-1] == newRunes[j-1] {
			matched[j-1] = true
			i--
			j--
		} else if lcs[i-1][j] >= lcs[i][j-1] {
			i--
		} else {
			j--
		}
	}

	// Convert unmatched newRunes positions to byte ranges in newText.
	// Coalesce adjacent runs.
	var out []ByteRange
	bytePos := 0
	runStart, inRun := -1, false
	for k, r := range newRunes {
		size := utf8RuneLen(r)
		if !matched[k] {
			if !inRun {
				runStart = bytePos
				inRun = true
			}
		} else if inRun {
			out = append(out, ByteRange{runStart, bytePos})
			inRun = false
		}
		bytePos += size
	}
	if inRun {
		out = append(out, ByteRange{runStart, bytePos})
	}
	return out
}

// utf8RuneLen returns the byte length of r in UTF-8.
func utf8RuneLen(r rune) int {
	switch {
	case r < 0x80:
		return 1
	case r < 0x800:
		return 2
	case r < 0x10000:
		return 3
	default:
		return 4
	}
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
go test ./internal/tidy/ -run TestDiffRanges -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tidy/diff.go internal/tidy/diff_test.go
git commit -m "Pass 9o.1: tidy.DiffRanges — rune LCS character diff

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Expose `tidy.Validate(Config) error`

Poplar's `LoadUI` needs to validate `[ui.tidy]` enums in-memory without going through the package's path-based `LoadConfig`. Cleanest fix: extract the existing per-field validators into one public function. The package already has `validateOxfordComma` / `validateEllipsis` / `validateTimeFormat`.

**Files:**
- Modify: `internal/tidy/config.go`
- Test: `internal/tidy/config_test.go`

- [ ] **Step 1: Add failing test**

Append to `internal/tidy/config_test.go`:

```go
func TestValidate(t *testing.T) {
	good := DefaultConfig()
	if err := Validate(good); err != nil {
		t.Fatalf("Validate(default) = %v, want nil", err)
	}

	bad := DefaultConfig()
	bad.Rules.OxfordComma = "yes"
	if err := Validate(bad); err == nil {
		t.Errorf("Validate(bad oxford_comma) = nil, want error")
	}

	bad = DefaultConfig()
	bad.Style.Ellipsis = "stars"
	if err := Validate(bad); err == nil {
		t.Errorf("Validate(bad ellipsis) = nil, want error")
	}

	bad = DefaultConfig()
	bad.Style.TimeFormat = "weird"
	if err := Validate(bad); err == nil {
		t.Errorf("Validate(bad time_format) = nil, want error")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/tidy/ -run TestValidate -v
```
Expected: `undefined: Validate`.

- [ ] **Step 3: Add `Validate` to `internal/tidy/config.go`**

Append after `validateTimeFormat`:

```go
// Validate reports whether c's string-enum fields hold legal values.
// Returns the first violation as an error matching the LoadConfig
// error shape.
func Validate(c Config) error {
	if err := validateOxfordComma(c.Rules.OxfordComma); err != nil {
		return err
	}
	if err := validateEllipsis(c.Style.Ellipsis); err != nil {
		return err
	}
	if err := validateTimeFormat(c.Style.TimeFormat); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/tidy/ -v
```
Expected: PASS (all package tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tidy/config.go internal/tidy/config_test.go
git commit -m "Pass 9o.2: tidy.Validate — in-memory config check

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Catkin tidy annotator + style slot

**Files:**
- Modify: `internal/catkin/style.go`
- Modify: `internal/catkin/annotate.go`
- Modify: `internal/catkin/catkin.go`
- Test: `internal/catkin/annotate_test.go`

The annotator returns annotations only when its stored `src` matches the input src exactly. Any buffer mutation invalidates the match; on the next debounced annotate tick, no annotations come back, so highlights vanish.

- [ ] **Step 1: Add failing tests**

Append to `internal/catkin/annotate_test.go`:

```go
func TestTidyAnnotator(t *testing.T) {
	a := newTidyAnnotator()

	// No state set — no annotations.
	if got := a.Annotate("anything"); len(got) != 0 {
		t.Errorf("empty state: got %d annotations, want 0", len(got))
	}

	a.Set("hello world", []Range{{0, 5}, {6, 11}})

	// Matching src returns the stored ranges.
	got := a.Annotate("hello world")
	if len(got) != 2 {
		t.Fatalf("matching src: got %d, want 2", len(got))
	}
	if got[0].Range != (Range{0, 5}) || got[1].Range != (Range{6, 11}) {
		t.Errorf("ranges = %v, want [{0,5},{6,11}]", got)
	}
	if got[0].Kind != KindTidyChange {
		t.Errorf("kind = %v, want KindTidyChange", got[0].Kind)
	}

	// Any divergence returns empty.
	if got := a.Annotate("hello world!"); len(got) != 0 {
		t.Errorf("divergent src: got %d, want 0", len(got))
	}
	if got := a.Annotate("hello"); len(got) != 0 {
		t.Errorf("shorter src: got %d, want 0", len(got))
	}
}

func TestTidyAnnotatorClearedByMutation(t *testing.T) {
	m := New()
	m.SetValue("teh quick brown")
	m.SetTidyHighlights("the quick brown", []Range{{1, 3}})
	if got := runAnnotators(m.annotators, "the quick brown"); len(got) == 0 {
		t.Fatalf("post-set: want at least one tidy annotation")
	}
	if got := runAnnotators(m.annotators, "the quick brown!"); len(got) != 0 {
		t.Errorf("post-mutation: got %d, want 0", len(got))
	}
}
```

(`runAnnotators` is the existing private fn at `annotate.go:37`.)

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/catkin/ -run TestTidyAnnotator -v
```
Expected: undefined symbols.

- [ ] **Step 3: Add `Styles.TidyChange`**

Edit `internal/catkin/style.go`, in the `Styles` struct:

```go
type Styles struct {
	Heading         [6]lipgloss.Style
	// ... existing fields ...
	Squiggle        lipgloss.Style
	TidyChange      lipgloss.Style   // NEW: changed spans after tidy run
	Popover         lipgloss.Style
	PopoverSelected lipgloss.Style
}
```

- [ ] **Step 4: Add `KindTidyChange` and `tidyAnnotator`**

Edit `internal/catkin/annotate.go`:

```go
// In the existing AnnotationKind block:
const (
	KindMisspelling AnnotationKind = iota
	KindTidyChange
)
```

Add at the bottom of `annotate.go`:

```go
// tidyAnnotator paints changed character ranges after a Tidy rewrite.
// State is set via Model.SetTidyHighlights and cleared by any buffer
// mutation: the next Annotate(src) call sees a divergent src and
// returns no annotations, so highlights vanish on first keystroke.
type tidyAnnotator struct {
	src    string
	ranges []Range
	style  lipgloss.Style
}

func newTidyAnnotator() *tidyAnnotator { return &tidyAnnotator{} }

// Set replaces the stored src and ranges. Style is configured separately
// via SetStyle so the annotator picks up theme changes.
func (a *tidyAnnotator) Set(src string, ranges []Range) {
	a.src = src
	a.ranges = ranges
}

// SetStyle replaces the per-annotation style. Called by Model.SetStyles.
func (a *tidyAnnotator) SetStyle(s lipgloss.Style) { a.style = s }

func (a *tidyAnnotator) Annotate(src string) []Annotation {
	if a.src == "" || src != a.src || len(a.ranges) == 0 {
		return nil
	}
	out := make([]Annotation, 0, len(a.ranges))
	for _, r := range a.ranges {
		out = append(out, Annotation{
			Range: r,
			Kind:  KindTidyChange,
			Style: a.style,
		})
	}
	return out
}
```

- [ ] **Step 5: Wire annotator into Model**

Edit `internal/catkin/catkin.go`:

In the `Model` struct add:

```go
type Model struct {
	// ... existing fields ...
	tidyA       *tidyAnnotator
	annotators  []Annotator
	// ... existing fields ...
}
```

In `New` (or wherever annotators are initialized), register the tidy annotator:

```go
func New() Model {
	// ... existing initialization ...
	tidy := newTidyAnnotator()
	m.tidyA = tidy
	m.annotators = append(m.annotators, tidy)
	return m
}
```

(If `annotators` already has a slice in `Model`, append; otherwise locate the existing `RegisterAnnotator` slot and call it once on `tidy`.)

Add the public method:

```go
// SetTidyHighlights configures the post-Tidy character-range highlights.
// The annotator returns annotations only while the buffer matches src.
// Any subsequent buffer mutation invalidates the match and clears the
// highlights on the next annotate tick.
func (m *Model) SetTidyHighlights(src string, ranges []Range) {
	if m.tidyA == nil {
		return
	}
	m.tidyA.Set(src, ranges)
}
```

In `SetStyles` (which already exists at `catkin.go:171`), forward the new style to the annotator:

```go
func (m *Model) SetStyles(s Styles) {
	m.styles = s
	if m.tidyA != nil {
		m.tidyA.SetStyle(s.TidyChange)
	}
}
```

- [ ] **Step 6: Run, confirm pass**

```bash
go test ./internal/catkin/ -v
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/catkin/
git commit -m "Pass 9o.3: catkin.tidyAnnotator — character-range highlights

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Editor seam — `SetCatkinStyles` + `SetTidyHighlights`

**Files:**
- Modify: `internal/compose/editor.go`

The `compose.Editor` interface is the abstraction over catkin (and the future v1.1 nvim adapter). Both new operations ride through it.

- [ ] **Step 1: Add methods to interface and implementation**

Edit `internal/compose/editor.go`:

```go
type Editor interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Editor, tea.Cmd)
	View() string

	SetSize(w, h int)
	SetWidth(w int)

	Focus() tea.Cmd
	Blur()
	Focused() bool

	Value() string
	SetValue(s string)

	SetStyles(s catkin.Styles)                        // NEW
	SetTidyHighlights(src string, ranges []catkin.Range) // NEW
	RegisterAnnotator(a catkin.Annotator)

	WordCount() int
	CharCount() int
}
```

Add the implementations on `CatkinEditor`:

```go
func (e *CatkinEditor) SetStyles(s catkin.Styles) { e.inner.SetStyles(s) }

func (e *CatkinEditor) SetTidyHighlights(src string, ranges []catkin.Range) {
	e.inner.SetTidyHighlights(src, ranges)
}
```

- [ ] **Step 2: Build to confirm interface satisfied**

```bash
go build ./...
```
Expected: clean build.

- [ ] **Step 3: Commit**

```bash
git add internal/compose/editor.go
git commit -m "Pass 9o.4: Editor seam carries catkin Styles + tidy highlights

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: `[ui.tidy]` config decode

**Files:**
- Modify: `internal/config/ui.go`
- Test: `internal/config/ui_test.go`

`UIConfig.Tidy` carries a poplar-side `TidyConfig` that wraps `tidy.Config` plus the `Enabled bool` gate. `LoadUI` decodes the sub-tables one-to-one onto `tidy.APIConfig` / `tidy.RulesConfig` / `tidy.StyleConfig`, then runs `tidy.Validate`.

- [ ] **Step 1: Add failing tests**

Add to `internal/config/ui_test.go`:

```go
func TestLoadUI_TidyDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[ui]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUI(path)
	if err != nil {
		t.Fatalf("LoadUI: %v", err)
	}
	if cfg.Tidy.Enabled {
		t.Errorf("Tidy.Enabled default = true, want false")
	}
	if cfg.Tidy.Config.API.Model == "" {
		t.Errorf("Tidy.Config.API.Model default empty, want package default")
	}
	if !cfg.Tidy.Config.Rules.Spelling {
		t.Errorf("Tidy.Config.Rules.Spelling default = false, want true")
	}
}

func TestLoadUI_TidyPopulated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[ui.tidy]
enabled = true

[ui.tidy.api]
model   = "claude-sonnet-4-6"
api_key = "sk-test"

[ui.tidy.rules]
spelling     = false
oxford_comma = "insert"

[ui.tidy.style]
ellipsis = "dots"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUI(path)
	if err != nil {
		t.Fatalf("LoadUI: %v", err)
	}
	if !cfg.Tidy.Enabled {
		t.Errorf("Tidy.Enabled = false, want true")
	}
	if cfg.Tidy.Config.API.Model != "claude-sonnet-4-6" {
		t.Errorf("API.Model = %q, want claude-sonnet-4-6", cfg.Tidy.Config.API.Model)
	}
	if cfg.Tidy.Config.API.APIKey != "sk-test" {
		t.Errorf("API.APIKey = %q, want sk-test", cfg.Tidy.Config.API.APIKey)
	}
	if cfg.Tidy.Config.Rules.Spelling {
		t.Errorf("Rules.Spelling = true, want false")
	}
	if cfg.Tidy.Config.Rules.OxfordComma != "insert" {
		t.Errorf("OxfordComma = %q, want insert", cfg.Tidy.Config.Rules.OxfordComma)
	}
	if cfg.Tidy.Config.Style.Ellipsis != "dots" {
		t.Errorf("Ellipsis = %q, want dots", cfg.Tidy.Config.Style.Ellipsis)
	}
}

func TestLoadUI_TidyBadEnum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[ui.tidy.rules]
oxford_comma = "yes"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUI(path); err == nil {
		t.Error("LoadUI accepted bad oxford_comma, want error")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/config/ -run TestLoadUI_Tidy -v
```
Expected: `cfg.Tidy undefined`.

- [ ] **Step 3: Add `TidyConfig` and wire into UIConfig + LoadUI**

Edit `internal/config/ui.go`:

Add the import at the top:

```go
import (
	// ... existing ...
	"github.com/glw907/poplar/internal/tidy"
)
```

Add the type:

```go
// TidyConfig carries the [ui.tidy] block. Enabled gates the Ctrl+T
// binding; Config is the package-native settings struct.
type TidyConfig struct {
	Enabled bool
	Config  tidy.Config
}
```

Add field to `UIConfig`:

```go
type UIConfig struct {
	// ... existing fields ...
	DownloadDir string
	Tidy        TidyConfig // NEW
}
```

Update `DefaultUIConfig`:

```go
func DefaultUIConfig() UIConfig {
	return UIConfig{
		Threading:   true,
		Folders:     map[string]FolderConfig{},
		Icons:       "auto",
		UndoSeconds: 6,
		DownloadDir: defaultDownloadDir(),
		Tidy:        TidyConfig{Enabled: false, Config: tidy.DefaultConfig()},
	}
}
```

Add raw-side decoding. In `rawUI`:

```go
type rawUI struct {
	// ... existing fields ...
	DownloadDir string         `toml:"download_dir"`
	Tidy        *rawTidy       `toml:"tidy"`
}

type rawTidy struct {
	Enabled *bool       `toml:"enabled"`
	API     *tidy.APIConfig   `toml:"api"`
	Rules   *tidy.RulesConfig `toml:"rules"`
	Style   *tidy.StyleConfig `toml:"style"`
}
```

In `LoadUI`, after the `DownloadDir` block and before `for name, fc := range raw.UI.Folders`:

```go
if raw.UI.Tidy != nil {
	tcfg := tidy.DefaultConfig()
	if raw.UI.Tidy.API != nil {
		tcfg.API = *raw.UI.Tidy.API
	}
	if raw.UI.Tidy.Rules != nil {
		tcfg.Rules = *raw.UI.Tidy.Rules
	}
	if raw.UI.Tidy.Style != nil {
		tcfg.Style = *raw.UI.Tidy.Style
	}
	if err := tidy.Validate(tcfg); err != nil {
		return UIConfig{}, fmt.Errorf("ui.tidy: %w", err)
	}
	out.Tidy.Config = tcfg
	if raw.UI.Tidy.Enabled != nil {
		out.Tidy.Enabled = *raw.UI.Tidy.Enabled
	}
}
```

- [ ] **Step 4: Run, confirm pass**

```bash
go test ./internal/config/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "Pass 9o.5: [ui.tidy] config decode

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Compose styles → catkin Styles bridge

**Files:**
- Modify: `internal/ui/compose/styles.go`
- Modify: `internal/ui/compose/model.go`

Compose builds a `catkin.Styles` from its theme reference and pushes it into the editor in `New`/`Open`. v1 only sets `TidyChange`; the existing markdown styling stays at zero (catkin renders plain). When future passes turn on full markdown styling in compose, they extend this same builder.

- [ ] **Step 1: Add compose-side helper**

Edit `internal/ui/compose/styles.go`:

```go
import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/catkin"
	"github.com/glw907/poplar/internal/theme"
)
```

```go
type Styles struct {
	ErrorBanner         lipgloss.Style
	InfoBanner          lipgloss.Style       // NEW: tidy "no changes" + similar transient info
	DropdownRow         lipgloss.Style
	DropdownRowSelected lipgloss.Style
	DropdownOrg         lipgloss.Style
	FromChip            lipgloss.Style
	TidyChange          lipgloss.Style       // NEW: catkin TidyChange slot, palette accent + underline
}

func NewStyles(t *theme.CompiledTheme) Styles {
	chip := lipgloss.NewStyle().Foreground(t.FgDim).Background(t.BgBase)
	return Styles{
		ErrorBanner:         lipgloss.NewStyle().Foreground(t.ColorError),
		InfoBanner:          lipgloss.NewStyle().Foreground(t.FgDim),
		DropdownRow:         lipgloss.NewStyle().Foreground(t.FgBase),
		DropdownRowSelected: lipgloss.NewStyle().Foreground(t.FgBright).Background(t.AccentPrimary),
		DropdownOrg:         lipgloss.NewStyle().Foreground(t.FgDim),
		FromChip:            chip,
		TidyChange:          lipgloss.NewStyle().Foreground(t.AccentPrimary).Underline(true),
	}
}

// CatkinStyles projects the compose Styles onto catkin's Styles struct.
// v1 only fills TidyChange; markdown render styling stays zero so
// catkin emits plain text (matches current compose behavior).
func (s Styles) CatkinStyles() catkin.Styles {
	return catkin.Styles{TidyChange: s.TidyChange}
}
```

- [ ] **Step 2: Push styles into editor at construction**

Edit `internal/ui/compose/model.go` `newModel`:

```go
func newModel(styles Styles, self string, suggest SuggestFn) *Model {
	mk := func() textinput.Model { /* unchanged */ }
	c := &Model{
		styles:  styles,
		from:    self,
		// ... unchanged ...
		editor:  mailcompose.NewCatkinEditor(),
		suggest: NewDropdown(suggest).WithStyles(styles),
	}
	c.editor.SetStyles(styles.CatkinStyles())   // NEW
	c.to.Focus()
	c.focus = focusTo
	return c
}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/ui/compose/styles.go internal/ui/compose/model.go
git commit -m "Pass 9o.6: compose builds catkin Styles with TidyChange slot

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Compose tidy command + key intercept

**Files:**
- Create: `internal/ui/compose/tidy.go`
- Test: `internal/ui/compose/tidy_test.go`
- Modify: `internal/ui/compose/model.go` (add tidy state fields, info-line render, tidyResultMsg branch in Update)
- Modify: `internal/ui/compose/bind.go` (route Ctrl+T through handleTidyKey when body is focused — exact filename TBD on inspection; if no `bind.go`, the key dispatch lives in `model.go`'s Update)

`tidyFn` is exposed as a private field on `Model` for tests; defaults to `tidy.Tidy`. The cmd captures the pre-tidy body so the diff can be computed even if the buffer mutates between the cmd kicking off and the result landing.

- [ ] **Step 1: Add failing tests**

Create `internal/ui/compose/tidy_test.go`:

```go
package compose

import (
	"testing"

	"github.com/glw907/poplar/internal/catkin"
	"github.com/glw907/poplar/internal/tidy"
	"github.com/glw907/poplar/internal/theme"
)

func newTestComposeModel() *Model {
	t := theme.OneDark()
	st := NewStyles(t)
	return New(st, "test@example.com", func(string) []contactsSuggestion { return nil })
}

// helper to inject a fake tidyFn
type fakeTidy struct {
	res tidy.Result
	err error
}

func (f fakeTidy) Tidy(input string, _ tidy.Config, _, _ string) (tidy.Result, error) {
	return f.res, f.err
}

func TestHandleTidyKey_Disabled(t *testing.T) {
	m := newTestComposeModel()
	m.focus = focusBody
	m.tidyEnabled = false
	cmd := m.handleTidyKey()
	if cmd != nil {
		t.Errorf("disabled: cmd != nil")
	}
}

func TestHandleTidyKey_NoAPIKey(t *testing.T) {
	m := newTestComposeModel()
	m.focus = focusBody
	m.tidyEnabled = true
	m.tidyAPIKey = ""
	cmd := m.handleTidyKey()
	if cmd != nil {
		t.Errorf("no api key: returned a cmd")
	}
	if m.err == "" {
		t.Errorf("no api key: m.err empty, want explanation")
	}
}

func TestHandleTidyKey_NotBodyFocus(t *testing.T) {
	m := newTestComposeModel()
	m.focus = focusSubject
	m.tidyEnabled = true
	m.tidyAPIKey = "sk-test"
	cmd := m.handleTidyKey()
	if cmd != nil {
		t.Errorf("not body focus: cmd != nil")
	}
}

func TestHandleTidyKey_InFlight(t *testing.T) {
	m := newTestComposeModel()
	m.focus = focusBody
	m.tidyEnabled = true
	m.tidyAPIKey = "sk-test"
	m.tidyInFlight = true
	cmd := m.handleTidyKey()
	if cmd != nil {
		t.Errorf("in flight: cmd != nil (re-entry should be ignored)")
	}
}

func TestUpdate_TidyResult_Corrected(t *testing.T) {
	m := newTestComposeModel()
	m.editor.SetValue("teh quick brown")
	m.tidyInFlight = true

	msg := tidyResultMsg{
		oldBody: "teh quick brown",
		res: tidy.Result{
			Status:  tidy.StatusCorrected,
			Text:    "the quick brown",
			Message: "tidytext: 1 corrections applied",
		},
	}
	next, _ := m.Update(msg)
	c := next.(*Model)
	if c.editor.Value() != "the quick brown" {
		t.Errorf("body not replaced: got %q", c.editor.Value())
	}
	if c.tidyInFlight {
		t.Errorf("inFlight not cleared")
	}
	if c.info == "" {
		t.Errorf("info empty, want toast text")
	}
}

func TestUpdate_TidyResult_NoChanges(t *testing.T) {
	m := newTestComposeModel()
	m.editor.SetValue("perfect text")
	m.tidyInFlight = true
	msg := tidyResultMsg{
		oldBody: "perfect text",
		res: tidy.Result{
			Status:  tidy.StatusNoChanges,
			Text:    "perfect text",
			Message: "tidytext: no changes needed",
		},
	}
	next, _ := m.Update(msg)
	c := next.(*Model)
	if c.editor.Value() != "perfect text" {
		t.Errorf("body changed on no-changes path")
	}
	if c.info == "" {
		t.Errorf("info empty, want 'no changes' toast")
	}
}

func TestUpdate_TidyResult_Error(t *testing.T) {
	m := newTestComposeModel()
	m.editor.SetValue("anything")
	m.tidyInFlight = true
	msg := tidyResultMsg{
		oldBody: "anything",
		res: tidy.Result{
			Status:  tidy.StatusError,
			Text:    "anything",
			Message: "tidytext: API error (500): boom, text unchanged",
		},
	}
	next, _ := m.Update(msg)
	c := next.(*Model)
	if c.editor.Value() != "anything" {
		t.Errorf("body changed on error path")
	}
	if c.err == "" {
		t.Errorf("err empty, want error text")
	}
}

// _ = catkin.Range{} keeps the import alive while wiring matures.
var _ = catkin.Range{}
```

(`contactsSuggestion` is the local alias for `contacts.Suggestion`; if the existing test file already imports the contacts package directly, mirror that.)

- [ ] **Step 2: Run, confirm fail**

```bash
go test ./internal/ui/compose/ -run TestHandleTidyKey -v
```
Expected: undefined symbols.

- [ ] **Step 3: Add Model fields**

Edit `internal/ui/compose/model.go`. Add to `Model`:

```go
type Model struct {
	// ... existing fields ...
	err            string
	info           string                 // NEW: transient info-banner text (cleared on next non-info action)

	tidyEnabled    bool                   // NEW: gate from [ui.tidy] enabled
	tidyAPIKey     string                 // NEW: resolved at App construction via tidy.ResolveAPIKey
	tidyCfg        tidy.Config            // NEW: package config to pass to Tidy
	tidyFn         func(input string, cfg tidy.Config, apiKey, apiURL string) (tidy.Result, error) // NEW: test seam, defaults to tidy.Tidy
	tidyInFlight   bool                   // NEW: single-flight gate
	// ... rest unchanged ...
}
```

In `newModel`, after the existing init:

```go
c.tidyFn = tidy.Tidy
```

Add a setter for App to plumb:

```go
// SetTidy configures Claude Tidy. enabled=false makes Ctrl+T inert.
// apiKey is the resolved key (from config or env). cfg is the package
// configuration block.
func (c *Model) SetTidy(enabled bool, apiKey string, cfg tidy.Config) {
	c.tidyEnabled = enabled
	c.tidyAPIKey = apiKey
	c.tidyCfg = cfg
}
```

Add the `tidy` import.

- [ ] **Step 4: Add `tidy.go`**

Create `internal/ui/compose/tidy.go`:

```go
package compose

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glw907/poplar/internal/catkin"
	"github.com/glw907/poplar/internal/tidy"
)

// tidyResultMsg lands when tidy.Tidy returns. oldBody is the body
// captured at request time so the diff can be computed against the
// pre-tidy text regardless of any buffer mutation in flight.
type tidyResultMsg struct {
	oldBody string
	res     tidy.Result
	err     error
}

// handleTidyKey is called when Ctrl+T fires. Returns the cmd that runs
// tidy.Tidy or nil when invocation is inert. Sets c.err for the
// "missing API key" case so the user sees why nothing happened.
func (c *Model) handleTidyKey() tea.Cmd {
	if !c.tidyEnabled || c.focus != focusBody || c.tidyInFlight {
		return nil
	}
	if c.tidyAPIKey == "" {
		c.err = "Tidy: ANTHROPIC_API_KEY not set"
		return nil
	}
	body := c.editor.Value()
	c.tidyInFlight = true
	c.info = "Tidy: running…"
	cfg, key, fn := c.tidyCfg, c.tidyAPIKey, c.tidyFn
	return func() tea.Msg {
		res, err := fn(body, cfg, key, "")
		return tidyResultMsg{oldBody: body, res: res, err: err}
	}
}

// applyTidyResult routes a tidyResultMsg by Status and updates the
// editor + chrome state. Returns whatever cmd the editor emits when
// SetValue is applied (typically nil).
func (c *Model) applyTidyResult(msg tidyResultMsg) tea.Cmd {
	c.tidyInFlight = false
	c.info = ""

	if msg.err != nil {
		c.err = "Tidy: " + msg.err.Error()
		return nil
	}

	switch msg.res.Status {
	case tidy.StatusCorrected:
		c.editor.SetValue(msg.res.Text)
		ranges := byteRangesToCatkin(tidy.DiffRanges(msg.oldBody, msg.res.Text))
		c.editor.SetTidyHighlights(msg.res.Text, ranges)
		c.info = msg.res.Message
		c.err = ""
	case tidy.StatusNoChanges:
		c.info = msg.res.Message
		c.err = ""
	case tidy.StatusNoAuthorText, tidy.StatusError:
		c.err = msg.res.Message
		c.info = ""
	default:
		c.err = fmt.Sprintf("Tidy: unknown status %d", msg.res.Status)
	}
	return nil
}

// byteRangesToCatkin projects tidy.ByteRange onto catkin.Range.
// They have the same shape; the conversion exists because the two
// types live in different packages.
func byteRangesToCatkin(in []tidy.ByteRange) []catkin.Range {
	if len(in) == 0 {
		return nil
	}
	out := make([]catkin.Range, len(in))
	for i, r := range in {
		out[i] = catkin.Range{Start: r.Start, End: r.End}
	}
	return out
}
```

- [ ] **Step 5: Wire Update + key dispatch**

In `internal/ui/compose/model.go`, `Update`:

Locate the key-dispatch switch. Add a case for `Ctrl+T` BEFORE delegating to the editor (so the editor never sees the keystroke):

```go
case tea.KeyMsg:
	switch msg.String() {
	// ... existing cases (Tab, Esc, Ctrl+X, Ctrl+C) ...
	case "ctrl+t":
		if cmd := c.handleTidyKey(); cmd != nil {
			return c, cmd
		}
		return c, nil
	}
	// fall through to focus dispatch
```

Add the result branch in `Update`:

```go
case tidyResultMsg:
	cmd := c.applyTidyResult(msg)
	return c, cmd
```

Clear `c.info` on any other keystroke that mutates the body. Simplest: in the body-key dispatch path, before forwarding to editor:

```go
if c.focus == focusBody {
	c.info = "" // clear transient info on body input
	// existing forward-to-editor
}
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/ui/compose/ -v
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/compose/
git commit -m "Pass 9o.7: compose Ctrl+T → tidy.Tidy → annotated body

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: App-level wiring + retire `TidyFn`

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/cmds.go`
- Modify: `cmd/poplar/root.go` (or wherever `NewApp` is called) — pass `[ui.tidy]` config + resolved API key into App

The `TidyFn` seam, `WithTidy`, `identityTidy`, `tidy` field, and the `tidy` parameter on `composeSendCmd` all go away. App acquires the tidy config + resolved API key at construction and threads them into compose via `compose.Model.SetTidy`.

- [ ] **Step 1: Drop TidyFn surface from app.go**

Edit `internal/ui/app.go`. Remove:

```go
// TidyFn rewrites the markdown body before MIME assembly. The default
// (identityTidy) is a passthrough; Pass 9i will swap in Claude Tidy.
type TidyFn func(ctx context.Context, body string) (string, error)

func identityTidy(_ context.Context, body string) (string, error) {
	return body, nil
}
```

Remove the `tidy TidyFn` field from `App`. Remove the `WithTidy` method. Remove the `tidy: identityTidy` initialization in `NewApp`. Remove the `tidy := m.tidy` line and pass-through in the `composeSendCmd` callsite.

If `context` is now unused in this file, drop it from imports.

- [ ] **Step 2: Drop tidy parameter from composeSendCmd**

Edit `internal/ui/cmds.go`. The current shape (`internal/ui/cmds.go:253`):

```go
func composeSendCmd(acct *cache.Account, sentFolder string, tidy TidyFn, d compose.Draft, ids []compose.Identity) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		body, err := tidy(ctx, d.Body)
		if err != nil {
			return ErrorMsg{Op: "tidy body", Err: err}
		}
		d.Body = body
		// ... rest of MIME assembly + queue ...
	}
}
```

Replace with:

```go
func composeSendCmd(acct *cache.Account, sentFolder string, d compose.Draft, ids []compose.Identity) tea.Cmd {
	return func() tea.Msg {
		// ... rest of MIME assembly + queue, no tidy step ...
	}
}
```

Update the callsite in `app.go` to drop the `tidy` argument.

- [ ] **Step 3: Thread `[ui.tidy]` config into compose at App construction**

In `internal/ui/app.go` `NewApp`, after building `app := App{...}`:

```go
// Resolve tidy API key once. Empty when neither config nor env supplies one;
// compose surfaces the error inline when Ctrl+T fires.
tidyKey := tidy.ResolveAPIKey(uiCfg.Tidy.Config)
// compose is created lazily on first c/r/R/f. Stash the inputs so each
// new compose.Model gets configured the same way.
app.tidyEnabled = uiCfg.Tidy.Enabled
app.tidyAPIKey  = tidyKey
app.tidyCfg     = uiCfg.Tidy.Config
return app
```

Add the matching fields on `App`:

```go
type App struct {
	// ... existing fields ...
	tidyEnabled bool
	tidyAPIKey  string
	tidyCfg     tidy.Config
}
```

In every place that creates a new `compose.Model` (search for `uicompose.New(` and `uicompose.Open(`), call `SetTidy` on the freshly-built model:

```go
m.compose = uicompose.New(uicompose.NewStyles(m.theme), self, m.suggestAddresses)
m.compose.SetTidy(m.tidyEnabled, m.tidyAPIKey, m.tidyCfg)
```

Add the `tidy` import to `app.go`.

- [ ] **Step 4: Build + run tests**

```bash
go build ./...
go test ./...
```
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/app.go internal/ui/cmds.go
git commit -m "Pass 9o.8: retire TidyFn seam — tidy is no longer on send path

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Footer hint + help-popover row

**Files:**
- Modify: footer hints data structure (`internal/ui/footer.go` or similar — locate via `grep -n "send\|^X\|drop" internal/ui/footer*.go`)
- Modify: help-popover compose section (`grep -lrn "Compose" internal/ui/helppopover/`)

- [ ] **Step 1: Locate footer hint registry**

```bash
grep -rn "ctrl.x.*send\|^X send\|FooterHint" internal/ui/ | head
grep -rn "Compose" internal/ui/helppopover/ | head
```

- [ ] **Step 2: Add `^T tidy` footer hint**

In the compose footer hint table (typical shape: `[]FooterHint{ {Key: "^X", Desc: "send", Rank: 0}, ... }`), append:

```go
{Key: "^T", Desc: "tidy", Rank: 6, Wired: true, Visible: func(c *Model) bool {
	return c.tidyEnabled && c.focus == focusBody
}},
```

Adapt to whatever the existing hint shape is — many hint registries use a static slice + an `if` in the renderer. Match the surrounding code.

- [ ] **Step 3: Add help-popover row in compose section**

Add a row to the compose-context help vocabulary, marked wired:

```go
{Key: "Ctrl+T", Desc: "Run Claude Tidy on the body (when [ui.tidy] enabled)", Wired: true},
```

- [ ] **Step 4: Build + tmux verify**

```bash
go build ./...
make install
# In tmux: open compose, focus body, confirm "^T tidy" appears in footer.
```

- [ ] **Step 5: Commit**

```bash
git add internal/ui/
git commit -m "Pass 9o.9: footer hint + help row for Ctrl+T

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Styling doc

**Files:**
- Modify: `docs/poplar/styling.md`

- [ ] **Step 1: Add row binding**

Append to the appropriate component table in `docs/poplar/styling.md`:

```
| `catkin.Styles.TidyChange` | `AccentPrimary` foreground + `Underline(true)` | Compose body — character ranges changed by Claude Tidy |
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/styling.md
git commit -m "Pass 9o.10: styling.md — TidyChange → AccentPrimary

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: `make check` + live tmux verification + ADR + STATUS

The pass-end consolidation ritual from the `poplar-pass` skill kicks in here.

- [ ] **Step 1: `make check`**

```bash
make check
```
Expected: all gates green (fmt, vet, voice, tests).

- [ ] **Step 2: Live tmux verification**

Set `ANTHROPIC_API_KEY` (or `[ui.tidy.api] api_key`) and `[ui.tidy] enabled = true`. Then through tmux against Fastmail:

1. Compose with intentional typos: `this  is teh  message,it has issues`
2. Press `Ctrl+T`. Confirm:
   - Footer briefly shows tidying state (or the info line shows `Tidy: running…`).
   - On return, body becomes corrected text with character ranges underlined in `AccentPrimary`.
   - Info line shows `tidytext: <N> corrections applied`.
3. Press an arrow key. Confirm highlights vanish on the next debounced tick.
4. Press catkin's undo. Confirm pre-tidy text returns.
5. Send. Confirm Sent copy contains corrected text.
6. Capture at 80×24 and 120×40 — store under `docs/poplar/captures/2026-05-08-tidy/`.

- [ ] **Step 3: Run `/simplify`**

Invoke the `simplify` skill against the diff. Apply genuine wins.

- [ ] **Step 4: Idiomatic-bubbletea checklist**

Run `docs/poplar/bubbletea-conventions.md` §10 checklist over the UI diff.

- [ ] **Step 5: Write ADR**

Next available number under `docs/poplar/decisions/`. Title: "Claude Tidy — user-invoked, in-place, character-highlighted." Frontmatter, Context, Decision, Consequences. Mark ADR-0159's `TidyFn` clause as superseded — append to ADR-0159's Consequences a line pointing at the new ADR.

- [ ] **Step 6: Update `docs/poplar/invariants.md`**

The Compose section currently says:

> App owns the lifecycle: `compose *ComposeTab` (nil when closed) + `tidy TidyFn` (function-pointer seam, default identity, Pass 9i swaps in Claude Tidy). … The send path runs `tidy`, calls `compose.AssembleMIME`, …

Rewrite that paragraph to remove `TidyFn` and the send-path tidy step. Add a new sentence describing the user-invoked tidy: "`Ctrl+T` in the body runs `tidy.Tidy` in a `tea.Cmd`; on return compose replaces the catkin buffer and feeds character-range diffs to a `tidyAnnotator` painted in `Styles.TidyChange`. Highlights clear on first body mutation. Tidy never runs on send."

Add the same to `.claude/rules/ui-invariants.md` Compose section.

Update `docs/poplar/decisions/INDEX.md` with rows for the new ADR.

- [ ] **Step 7: Update STATUS.md**

Mark Pass 9o `done`. Replace the starter prompt with Pass 9p.

- [ ] **Step 8: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-05-08-claude-tidy.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-08-claude-tidy-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 9: Final commit, push, install**

```bash
make check
git add -A
git commit -m "Pass 9o: Claude Tidy — user-invoked body proofread (ADR-NNNN)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
git push
make install
```

---

## Self-review

**Spec coverage**
- User flow + key + body-only → Tasks 7, 9.
- Highlight pipeline (DiffRanges, annotator, clear-on-keystroke) → Tasks 1, 3.
- Config block → Task 5.
- Editor seam (SetCatkinStyles, SetTidyHighlights) → Task 4.
- Compose styles bridge → Task 6.
- Retire TidyFn → Task 8.
- Footer + help → Task 9.
- Styling doc → Task 10.
- ADR + invariants update + supersession → Task 11.
- Live verification → Task 11.

**Type consistency**
- `tidy.ByteRange` vs `catkin.Range`: explicit conversion in `byteRangesToCatkin`.
- `Editor.SetStyles(catkin.Styles)`, `Editor.SetTidyHighlights(string, []catkin.Range)`: matches Task 4 declaration and Task 7 callsite.
- `Model.SetTidy(bool, string, tidy.Config)`: matches Task 7 signature and Task 8 callsite.
- `tidyResultMsg{oldBody, res, err}`: matches Task 7 producer and consumer.

**Placeholder scan**
- Task 9 references "wherever the existing hint registry lives" — that's a deliberate locate-then-edit instruction with the grep command in Step 1. Acceptable because the path varies and I verified during planning that no canonical filename can be cited yet.
- Task 7 Step 5 says "if no `bind.go`, the dispatch lives in `model.go`'s Update" — both alternatives are concrete; either branch leaves no placeholder in the final code.
- ADR number is `NNNN` placeholder until pass-end — that's expected.
