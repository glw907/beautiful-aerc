# Catkin Elm Conformance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert `internal/catkin/` to the Elm all-value path (Pass 27), then delete the `mailcompose.Editor` wrapper so compose holds catkin directly (Pass 28).

**Architecture:** `catkin.Model` and `catkin.Buffer` become value types with `With*` setters returning new values. The wrapped `bubbles/v2/textarea.Model` pointer is sealed inside `Buffer`. Pass 27 keeps `mailcompose.CatkinEditor` as a body-only shim so compose stays untouched; Pass 28 deletes the wrapper and rewires compose to hold `catkin.Model` directly.

**Tech Stack:** Go 1.26.1, `charm.land/bubbletea/v2`, `charm.land/bubbles/v2/textarea`, table-driven tests, `make check` as commit gate.

**Companion spec:** `docs/superpowers/specs/2026-05-11-catkin-elm-design.md`

---

## Working notes

**The conversion pattern.** Every external pointer-mutator (`func (m *Model) SetX(...)`) becomes a value-returning setter (`func (m Model) WithX(...) Model`) whose body is the existing method body with a leading `return m`. Internal helpers that mutate through receivers may stay pointer-receiver where they are still called locally within Update, but no value crosses the package boundary as a pointer.

**Buffer mechanics.** `Buffer` wraps `textarea.Model` (which is itself a struct with pointer-internals). Copying a `Buffer` by value copies the textarea struct, which is shallow-safe for the cursor/value APIs we use — bubbles' textarea exposes its state as scalar fields and slice headers that the textarea owns. The pointer mutators inside `Buffer` operate on `b.ta` through a value receiver in their new form: Go allows pointer methods on `b.ta` field of a value receiver because `b.ta` is addressable within the method.

Wait — that last claim is wrong. A pointer method on a field of a non-addressable value receiver doesn't compile. To call `b.ta.SetValue(...)` from inside a value-receiver method, `b` itself must be addressable. The standard idiom: take `b Buffer`, mutate `b.ta` through a *local pointer* using `(&b.ta).SetValue(...)` — except that's not how Go works either. In practice the right form is:

```go
func (b Buffer) WithValue(s string) Buffer {
    b.ta.SetValue(s)  // b is the local copy; b.ta is addressable
    return b
}
```

This works because `b` is a local variable inside the method, hence addressable. The textarea's internal state is mutated on the local copy, which is then returned. Callers must reassign (`b = b.WithValue(s)`); old aliases of `b` keep the pre-mutation textarea state. Same idiom carries through `Model`.

**The shim trick.** `CatkinEditor` keeps its pointer-receiver methods and same signatures. Bodies change from `e.inner.SetValue(s)` to `e.inner = e.inner.WithValue(s)`. The interface `mailcompose.Editor` is unchanged. Compose is untouched in Pass 27.

**Run `go build ./...` after every conversion task.** The compiler catches every call site that needs reassignment. If it builds, the conversion is mechanically complete.

---

## Pass 27 — Catkin all-value conversion

### Task 1: Buffer value setters

**Files:**
- Modify: `internal/catkin/buffer.go`

- [ ] **Step 1: Rewrite `buffer.go` end-to-end**

Replace the whole file with:

```go
package catkin

import (
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

// Buffer wraps a bubbles/textarea.Model. Catkin uses textarea for
// its buffer storage, cursor management, and edit operations. The
// renderer is Catkin's own (see render.go).
//
// Buffer is a value type. Mutators return a new Buffer; callers
// reassign. The wrapped textarea.Model is sealed — it never leaves
// the package.
type Buffer struct {
	ta textarea.Model
}

func NewBuffer(ta textarea.Model) Buffer { return Buffer{ta: ta} }

func (b Buffer) Update(msg tea.Msg) (Buffer, tea.Cmd) {
	var cmd tea.Cmd
	b.ta, cmd = b.ta.Update(msg)
	return b, cmd
}

func (b Buffer) Value() string  { return b.ta.Value() }
func (b Buffer) Focused() bool  { return b.ta.Focused() }

func (b Buffer) WithValue(s string) Buffer {
	b.ta.SetValue(s)
	return b
}

func (b Buffer) WithWidth(w int) Buffer {
	b.ta.SetWidth(w)
	return b
}

func (b Buffer) WithHeight(h int) Buffer {
	b.ta.SetHeight(h)
	return b
}

func (b Buffer) WithFocus() (Buffer, tea.Cmd) {
	cmd := b.ta.Focus()
	return b, cmd
}

func (b Buffer) WithBlur() Buffer {
	b.ta.Blur()
	return b
}

// RuneOffset returns the cursor's rune offset from the start of the value.
func (b Buffer) RuneOffset() int {
	row := b.ta.Line()
	value := b.ta.Value()
	lines := strings.Split(value, "\n")
	off := 0
	for i := range min(row, len(lines)) {
		off += utf8.RuneCountInString(lines[i]) + 1
	}
	li := b.ta.LineInfo()
	off += li.StartColumn + li.ColumnOffset
	total := utf8.RuneCountInString(value)
	if off > total {
		off = total
	}
	return off
}

// WithRuneOffset positions the cursor at rune offset off.
func (b Buffer) WithRuneOffset(off int) Buffer {
	value := b.ta.Value()
	total := utf8.RuneCountInString(value)
	if off > total {
		off = total
	}
	lines := strings.Split(value, "\n")
	row, col := 0, off
	for i, l := range lines {
		ln := utf8.RuneCountInString(l)
		if col <= ln {
			break
		}
		col -= ln + 1
		row = i + 1
	}
	if row >= len(lines) {
		row = len(lines) - 1
		col = utf8.RuneCountInString(lines[row])
	}
	for b.ta.Line() < row {
		b.ta.CursorDown()
	}
	for b.ta.Line() > row {
		b.ta.CursorUp()
	}
	b.ta.SetCursorColumn(col)
	return b
}
```

- [ ] **Step 2: Update `buffer_test.go` to value form**

Replace with:

```go
package catkin

import (
	"testing"

	"charm.land/bubbles/v2/textarea"
)

func TestBufferRuneOffsetRoundTrip(t *testing.T) {
	b := NewBuffer(newEmptyTextarea()).WithValue("hello\nworld\nfoo").WithRuneOffset(7)
	if got := b.RuneOffset(); got != 7 {
		t.Errorf("RuneOffset round-trip: got %d, want 7", got)
	}
}

func TestBufferRuneOffsetClampsPastEnd(t *testing.T) {
	b := NewBuffer(newEmptyTextarea()).WithValue("abc").WithRuneOffset(100)
	if got := b.RuneOffset(); got > 3 {
		t.Errorf("RuneOffset past-end: got %d, want ≤3", got)
	}
}

func TestBufferRuneOffsetAtNewline(t *testing.T) {
	b := NewBuffer(newEmptyTextarea()).WithValue("hello\nworld\nfoo").WithRuneOffset(5)
	if got := b.RuneOffset(); got != 5 {
		t.Errorf("RuneOffset at newline boundary: got %d, want 5", got)
	}
}

func newEmptyTextarea() textarea.Model {
	t := textarea.New()
	t.SetWidth(40)
	t.SetHeight(10)
	return t
}
```

- [ ] **Step 3: Run tests**

```bash
go test -tags=dev ./internal/catkin/ -run TestBuffer -v
```

Expected: all three pass. **Do not proceed to Task 2 if any fail** — the textarea-via-value-receiver pattern is load-bearing for the rest of the pass and a failure here means the idiom doesn't work as planned.

- [ ] **Step 4: Build the package — many call sites will break**

```bash
go build -tags=dev ./internal/catkin/
```

Expected: compile errors in `autopair.go`, `dispatch.go`, `find.go`, `catkin.go`, `popover.go` for `b.SetValue` / `b.SetRuneOffset` calls. These get fixed in Task 2.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/buffer.go internal/catkin/buffer_test.go
git commit -m "$(cat <<'EOF'
catkin: Buffer to value type with With* setters

First step of the all-value path. Buffer's textarea pointer is
sealed; mutators return a new Buffer. Internal callers in catkin
break and get rewritten in the next commit.
EOF
)"
```

---

### Task 2: Convert Buffer call sites inside catkin

**Files:**
- Modify: `internal/catkin/autopair.go`
- Modify: `internal/catkin/dispatch.go`
- Modify: `internal/catkin/find.go`
- Modify: `internal/catkin/catkin.go` (paste handler + afterEdit)
- Modify: `internal/catkin/popover.go`

- [ ] **Step 1: Convert `autopair.go` and `dispatch.go`**

The pattern: `b.SetValue(s); b.SetRuneOffset(c)` → `b = b.WithValue(s).WithRuneOffset(c)`.

In `autopair.go` lines 29–30 and 45–46, and `dispatch.go` lines 43–44, replace the two-line pointer-mutator sequence with a single chained `With*` reassignment. Example for `dispatch.go`:

```go
	b = b.WithValue(newSrc).WithRuneOffset(newCur)
	return true, b, nil
```

- [ ] **Step 2: Convert `find.go`**

`find.go:153`: `m.buf.SetRuneOffset(...)` → `m.buf = m.buf.WithRuneOffset(...)`.

`find.go:168` and `find.go:192`: there are similar `m.buf.SetValue(out); m.buf.SetRuneOffset(caret)` patterns. Convert each to `m.buf = m.buf.WithValue(out).WithRuneOffset(caret)`.

- [ ] **Step 3: Convert `popover.go` `applyReplacement`**

`popover.go` around lines 125–131 has a sequence that calls `m.buf.SetValue(...)` and `m.buf.SetRuneOffset(...)`. Convert to `m.buf = m.buf.WithValue(...).WithRuneOffset(...)`.

- [ ] **Step 4: Convert `catkin.go` paste handler**

Lines 104–105 of the original `catkin.go`:

```go
			b.SetValue(newVal)
			b.SetRuneOffset(start + utf8.RuneCountInString(replacement))
```

becomes:

```go
			b = b.WithValue(newVal).WithRuneOffset(start + utf8.RuneCountInString(replacement))
```

Also update `pasteInto` (lines 21–28): it currently calls `buf.SetValue(newVal); buf.SetRuneOffset(newCur)` on a value parameter — change to value-returning form:

```go
func pasteInto(buf Buffer, runes []rune, cur int, payload string) (Buffer, int) {
	payloadRunes := []rune(payload)
	newVal := string(runes[:cur]) + payload + string(runes[cur:])
	newCur := cur + len(payloadRunes)
	return buf.WithValue(newVal).WithRuneOffset(newCur), newCur
}
```

- [ ] **Step 5: Build the package**

```bash
go build -tags=dev ./internal/catkin/
```

Expected: package compiles. Tests fail (still using old `Model` mutators) — that's Task 3.

- [ ] **Step 6: Commit**

```bash
git add internal/catkin/autopair.go internal/catkin/dispatch.go internal/catkin/find.go internal/catkin/popover.go internal/catkin/catkin.go
git commit -m "$(cat <<'EOF'
catkin: convert Buffer call sites to value setters

Mechanical sweep — every `b.SetX(...)` in catkin internals becomes
`b = b.WithX(...)`. No behavior change. Model's external surface is
still pointer-shaped; converted in the next commit.
EOF
)"
```

---

### Task 3: Model value setters and builder

**Files:**
- Modify: `internal/catkin/catkin.go`

- [ ] **Step 1: Convert Model's external pointer methods to value setters and add builders**

Replace the public-surface section of `catkin.go` (from `RegisterAnnotator` through `Focused`) with:

```go
// RegisterAnnotator returns a Model with a appended to the annotator
// list. Mount-time configuration: call before the tea loop sees the
// model. Annotators run in registration order.
func (m Model) WithAnnotator(a Annotator) Model {
	m.annotators = append(m.annotators, a)
	return m
}

// WithUserWordlistPath sets the file the popover's add-action appends to.
// An empty path makes the add a session-local addition only.
func (m Model) WithUserWordlistPath(path string) Model {
	m.userWordlistPath = path
	return m
}

// WithStyles replaces the render-time style table. The zero value is
// no-op styles; consumers map their theme onto Styles at the boundary.
func (m Model) WithStyles(s Styles) Model {
	m.styles = s
	if m.tidyA != nil {
		m.tidyA.SetStyle(s.TidyChange)
	}
	return m
}

// WithTidyHighlights configures the post-Tidy character-range highlights.
// The annotator returns annotations only while the buffer matches src.
// Any subsequent buffer mutation invalidates the match and clears the
// highlights on the next annotate tick.
func (m Model) WithTidyHighlights(src string, ranges []Range) Model {
	if m.tidyA != nil {
		m.tidyA.Set(src, ranges)
	}
	return m
}

// WithValue replaces the buffer and re-seeds the undo ring; programmatic
// loads are not user edits.
func (m Model) WithValue(s string) Model {
	m.buf = m.buf.WithValue(s)
	m.undo.seed(snap{s, m.buf.RuneOffset()})
	return m
}

// WithSize sets the viewport dimensions.
func (m Model) WithSize(w, h int) Model {
	m.width, m.height = w, h
	m.buf = m.buf.WithWidth(w).WithHeight(h)
	return m
}

// WithWidth sets the body wrap width and re-runs reflow.
func (m Model) WithWidth(w int) Model {
	if w == m.width {
		return m
	}
	m.width = w
	m.buf = m.buf.WithWidth(w)
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	src, cur = Reflow(src, w, cur)
	m.buf = m.buf.WithValue(src).WithRuneOffset(cur)
	return m
}

func (m Model) WithFocus() (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.buf, cmd = m.buf.WithFocus()
	return m, cmd
}

func (m Model) WithBlur() Model {
	m.buf = m.buf.WithBlur()
	return m
}

func (m Model) Focused() bool { return m.buf.Focused() }
```

- [ ] **Step 2: Inline `recordSnap` into its callers**

`recordSnap` (line 191) was `func (m *Model) recordSnap()`. Its callers (`afterEdit`, `find.go`'s replace handlers, `popover.go`'s `applyReplacement`) are now value-receiver methods, so calling a pointer-receiver helper on a non-addressable value won't compile. Inline the body — it's one line — and delete the standalone method.

In `catkin.go`, rewrite `afterEdit`:

```go
func (m Model) afterEdit(b Buffer, cmd tea.Cmd) (Model, tea.Cmd) {
	prev := m.buf.Value()
	m.buf = b
	m.undo.record(snap{m.buf.Value(), m.buf.RuneOffset()})
	if m.buf.Value() != prev && len(m.annotators) > 0 {
		m.srcGen++
		cmd = tea.Batch(cmd, scheduleAnnotateCmd(m.srcGen))
	}
	m = m.closePopoverIfCursorLeftRange()
	return applyScrollOff(m), cmd
}
```

Delete the standalone `recordSnap` method. In `find.go` and `popover.go`, replace each `m.recordSnap()` with the inlined body `m.undo.record(snap{m.buf.Value(), m.buf.RuneOffset()})`. If a remaining caller is itself a pointer-receiver helper, keep the call as-is — only the value-receiver call sites need inlining.

- [ ] **Step 3: Build and triage**

```bash
go build -tags=dev ./internal/catkin/
```

Expected errors will name every remaining `m.recordSnap()` callsite plus any other pointer-receiver helpers that broke. Fix each by inlining `m.undo.record(snap{m.buf.Value(), m.buf.RuneOffset()})` or by promoting the caller to operate on a local `Model` value.

- [ ] **Step 4: Re-build until catkin package compiles**

```bash
go build -tags=dev ./internal/catkin/
```

Expected: clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/catkin/catkin.go internal/catkin/find.go internal/catkin/popover.go
git commit -m "$(cat <<'EOF'
catkin: Model to value type with With* setters

External surface now value-shaped. RegisterAnnotator, SetStyles,
SetValue, SetSize, SetWidth, SetTidyHighlights, SetUserWordlistPath,
Focus, Blur replaced by With* setters returning Model. Internal
recordSnap inlined into its callers.
EOF
)"
```

---

### Task 4: Convert catkin test files

**Files:**
- Modify: `internal/catkin/annotate_test.go`
- Modify: `internal/catkin/catkin_test.go`
- Modify: `internal/catkin/count_test.go`
- Modify: `internal/catkin/popover_test.go`
- Modify: any other `*_test.go` that calls converted mutators

- [ ] **Step 1: List the failing files**

```bash
go vet -tags=dev ./internal/catkin/ 2>&1 | head -50
```

Expected: errors at each call site of the deleted methods (`SetValue`, `SetSize`, `RegisterAnnotator`, `SetUserWordlistPath`, `SetTidyHighlights`).

- [ ] **Step 2: Apply the mechanical conversion across every test file**

Conversion pattern:
- `m.SetValue(s)` → `m = m.WithValue(s)`
- `m.SetSize(w, h)` → `m = m.WithSize(w, h)`
- `m.RegisterAnnotator(a)` → `m = m.WithAnnotator(a)`
- `m.SetUserWordlistPath(p)` → `m = m.WithUserWordlistPath(p)`
- `m.SetTidyHighlights(src, r)` → `m = m.WithTidyHighlights(src, r)`
- `m.SetStyles(s)` → `m = m.WithStyles(s)`

Where the test does several setups in a row, chain them:

```go
m := New().
    WithAnnotator(a).
    WithStyles(s).
    WithUserWordlistPath(path).
    WithSize(80, 10).
    WithValue("hello world")
```

- [ ] **Step 3: Run the full catkin test suite**

```bash
go test -tags=dev ./internal/catkin/ -v
```

Expected: all green. Behavior is unchanged; only setter syntax moved.

- [ ] **Step 4: Commit**

```bash
git add internal/catkin/*_test.go
git commit -m "$(cat <<'EOF'
catkin: convert tests to value-setter form

Mechanical follow-up; no behavior change.
EOF
)"
```

---

### Task 5: CatkinEditor shim — body-only rewrite

**Files:**
- Modify: `internal/mailcompose/editor.go`

- [ ] **Step 1: Rewrite the mutator bodies, leave the interface alone**

Edit `internal/mailcompose/editor.go`. The `Editor` interface and `CatkinEditor` struct definitions stay byte-identical. Only method bodies change.

```go
func (e *CatkinEditor) Update(msg tea.Msg) (Editor, tea.Cmd) {
	next, cmd := e.inner.Update(msg)
	e.inner = next
	return e, cmd
}

func (e *CatkinEditor) View() string { return e.inner.View() }

func (e *CatkinEditor) SetSize(w, h int) { e.inner = e.inner.WithSize(w, h) }
func (e *CatkinEditor) SetWidth(w int)   { e.inner = e.inner.WithWidth(w) }

func (e *CatkinEditor) Focus() tea.Cmd {
	var cmd tea.Cmd
	e.inner, cmd = e.inner.WithFocus()
	return cmd
}
func (e *CatkinEditor) Blur()         { e.inner = e.inner.WithBlur() }
func (e *CatkinEditor) Focused() bool { return e.inner.Focused() }

func (e *CatkinEditor) Value() string     { return e.inner.Value() }
func (e *CatkinEditor) SetValue(s string) { e.inner = e.inner.WithValue(s) }

func (e *CatkinEditor) SetStyles(s catkin.Styles) { e.inner = e.inner.WithStyles(s) }

func (e *CatkinEditor) SetTidyHighlights(src string, ranges []catkin.Range) {
	e.inner = e.inner.WithTidyHighlights(src, ranges)
}

func (e *CatkinEditor) RegisterAnnotator(a catkin.Annotator) {
	e.inner = e.inner.WithAnnotator(a)
}

func (e *CatkinEditor) WordCount() int { return e.inner.WordCount() }
func (e *CatkinEditor) CharCount() int { return e.inner.CharCount() }
```

`NewCatkinEditor` becomes:

```go
func NewCatkinEditor() *CatkinEditor {
	return &CatkinEditor{inner: catkin.New()}
}
```

(unchanged signature; body identical, since `catkin.New()` already returns a value).

`Init` stays:

```go
func (e *CatkinEditor) Init() tea.Cmd { return e.inner.Init() }
```

- [ ] **Step 2: Build the whole module**

```bash
go build -tags=dev ./...
```

Expected: compose builds clean against the unchanged Editor interface. Wizard breaks (it imports catkin directly) — fixed in Task 6.

- [ ] **Step 3: Commit**

```bash
git add internal/mailcompose/editor.go
git commit -m "$(cat <<'EOF'
mailcompose: CatkinEditor shim against value-shaped catkin

Method signatures preserved so compose stays untouched. Pass 28
deletes this wrapper entirely.
EOF
)"
```

---

### Task 6: Wizard signature section

**Files:**
- Modify: `internal/ui/wizard/section_signature.go`

- [ ] **Step 1: Convert the two mutator call sites**

In `newSignatureSection`, replace:

```go
	ed := catkin.New()
	ed.SetSize(64, 8)
	if parent.State.Signature != "" {
		ed.SetValue(parent.State.Signature)
	}
```

with:

```go
	ed := catkin.New().WithSize(64, 8)
	if parent.State.Signature != "" {
		ed = ed.WithValue(parent.State.Signature)
	}
```

The `s.editor, cmd = s.editor.Update(msg)` line already works (Update is value-shaped now).

- [ ] **Step 2: Build the module**

```bash
go build -tags=dev ./...
```

Expected: clean.

- [ ] **Step 3: Run the wizard test suite**

```bash
go test -tags=dev ./internal/ui/wizard/ -v
```

Expected: green. The signature section's behavior is unchanged.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/wizard/section_signature.go
git commit -m "$(cat <<'EOF'
wizard: signature section to catkin value setters

Wizard imports catkin directly (not via Editor), so it can't ride
the CatkinEditor shim — converts in this pass.
EOF
)"
```

---

### Task 7: ADR-0212, invariants delta, pass-end checklist

**Files:**
- Create: `docs/poplar/decisions/0212-catkin-all-value-path.md`
- Modify: `.claude/rules/catkin-invariants.md`
- Modify: `docs/poplar/decisions/INDEX.md`
- Modify: `docs/poplar/STATUS.md`

- [ ] **Step 1: Write ADR-0212**

Create `docs/poplar/decisions/0212-catkin-all-value-path.md`:

```markdown
---
title: Catkin all-value path
status: accepted
date: 2026-05-11
---

## Context

Catkin's `Model` and `Buffer` carried pointer-receiver mutators
(`SetValue`, `SetSize`, `Focus`, `Blur`, `SetStyles`,
`SetTidyHighlights`, `RegisterAnnotator`, `SetUserWordlistPath`).
The contagion source is `bubbles/v2/textarea.Model`, itself
pointer-shaped. The 2026-05-11 audit flagged this as an
unenforced Elm boundary: any caller with a `*catkin.Model` could
mutate it from a Cmd closure or a View method, and the
`elm-conventions` rule "mutations only in Update" was satisfied
in spirit but not by types. Compose held catkin via
`mailcompose.Editor` purely to get a stable pointer; the wizard
held `catkin.Model` directly, exposing the inconsistency.

## Decision

`catkin.Model` and `catkin.Buffer` are value types. Runtime
mutations route through value-returning `With*` setters called
from the parent's Update; mount-time configuration uses a fluent
builder. The wrapped `textarea.Model` is sealed inside `Buffer`
and never escapes the package.

Surface:

- Mount: `catkin.New().WithAnnotator(a).WithStyles(s).WithUserWordlistPath(p)`
- Runtime: `WithValue`, `WithSize`, `WithWidth`, `WithFocus`,
  `WithBlur`, `WithTidyHighlights`
- Update: `func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)`
  (signature unchanged; handler body mutates a local value)

The brainstorm considered a Msg-vocabulary alternative
(`SetValueMsg` etc.); setters won on four grounds: matches the
bubbles convention; keeps catkin's Msg surface scoped to genuine
external events; allows single-statement paired mutations
(tidy's text + ranges); satisfies the Elm invariant without
ceremony.

## Consequences

- Compose can hold `catkin.Model` directly. `mailcompose.Editor`
  + `CatkinEditor` no longer have a structural reason to exist
  and delete in Pass 28 (ADR-0213).
- Hidden mutation from a Cmd closure or View method is now a
  type error.
- The textarea-via-value-receiver idiom (mutate a local field
  through a pointer method on an addressable copy, then return
  the copy) is load-bearing for the package and is documented in
  `Buffer.WithValue`.
```

- [ ] **Step 2: Update `.claude/rules/catkin-invariants.md`**

Append to the top Catkin paragraph (after the existing `ADR-0144, 0145, 0146, 0147.` line, before the next bullet):

```markdown
- Model and Buffer are value types; runtime mutations are
  value-returning `With*` setters called from the parent's
  Update; mount-time uses a `New().With*` builder. The wrapped
  `textarea.Model` pointer is sealed inside Buffer and never
  escapes the package. ADR-0212.
```

- [ ] **Step 3: Update `docs/poplar/decisions/INDEX.md`**

Add an entry under the relevant Catkin / Elm section. Match the existing index format — one line per ADR.

- [ ] **Step 4: Update `docs/poplar/STATUS.md` pass table**

In the pass table, mark Pass 27 done; rewrite the next-starter-prompt block for Pass 28 (compose holds catkin directly). Keep STATUS.md ≤60 lines.

- [ ] **Step 5: tmux verify**

Follow `.claude/docs/tmux-testing.md` to capture compose body + wizard signature section at 120×40. Both must render identically to pre-pass state.

- [ ] **Step 6: Commit, run check, install**

The spec and plan stay in `docs/superpowers/{specs,plans}/` through Pass 27 — Pass 28 uses the same documents and archives both at its end.

```bash
git add docs/poplar/decisions/0212-catkin-all-value-path.md \
        docs/poplar/decisions/INDEX.md \
        .claude/rules/catkin-invariants.md \
        docs/poplar/STATUS.md
make check
git commit -m "$(cat <<'EOF'
Pass 27: Catkin Elm conformance — all-value path

Catkin.Model and Buffer are value types with With* setters; the
wrapped textarea pointer is sealed inside Buffer. CatkinEditor
shim absorbs the change with body-only edits so compose stays
untouched until Pass 28.

ADR-0212.
EOF
)"
git push
make install
```

---

## Pass 28 — Delete Editor wrapper, compose holds catkin directly

### Task 8: Delete the Editor wrapper

**Files:**
- Delete: `internal/mailcompose/editor.go`
- Modify: `internal/ui/compose/model.go`
- Modify: `internal/ui/compose/tidy.go`

- [ ] **Step 1: Delete editor.go**

```bash
git rm internal/mailcompose/editor.go
```

The build will break across compose. Fix in the next step.

- [ ] **Step 2: Change the compose field type**

In `internal/ui/compose/model.go` around line 46, replace:

```go
	editor  mailcompose.Editor
```

with:

```go
	editor  catkin.Model
```

Add the import: `"github.com/glw907/poplar/internal/catkin"` (mailcompose is still imported for `Draft`, `Identity`, `Signature`, `AssembleMIME` etc.).

In the constructor (around line 125), replace:

```go
		editor:   mailcompose.NewCatkinEditor(),
```

with:

```go
		editor:   catkin.New().WithStyles(styles.CatkinStyles()),
```

Delete the separate `c.editor.SetStyles(styles.CatkinStyles())` line that immediately follows the struct literal — its work folded into the builder.

- [ ] **Step 3: Convert every compose call site**

Walk `internal/ui/compose/model.go` and `internal/ui/compose/tidy.go`. Apply:

- `c.editor.SetSize(w, h)` → `c.editor = c.editor.WithSize(w, h)`
- `c.editor.SetValue(s)` → `c.editor = c.editor.WithValue(s)`
- `c.editor.Focus()` (used for its tea.Cmd return) → `var cmd tea.Cmd; c.editor, cmd = c.editor.WithFocus()`
  - Look at line 738 specifically: `_ = c.editor.Focus()`. That discards the cmd, which is wrong even today — but since the pass goal is no-behavior-change, preserve the discard: `c.editor, _ = c.editor.WithFocus()`.
- `c.editor.Blur()` → `c.editor = c.editor.WithBlur()`
- `c.editor.Init()` (line 231) — `Init` is a value-receiver method on `Model` returning `tea.Cmd`. Stays unchanged: `c.editor.Init()`.
- `c.editor.Update(msg)` (lines 564, 688) — already value-shaped. Stays unchanged.
- `c.editor.View()`, `c.editor.Value()`, `c.editor.WordCount()`, `c.editor.CharCount()` — value-receiver accessors. Stay unchanged.
- `c.editor.SetTidyHighlights(text, ranges)` in `tidy.go:57` — this is the paired mutation with `SetValue` from the tidy result handler. Refactor the two-line sequence into one statement:

  Find the existing tidy result handler (around `tidy.go:50–60`). The sequence currently looks like:

  ```go
		c.editor.SetValue(msg.res.Text)
		c.editor.SetTidyHighlights(msg.res.Text, ranges)
  ```

  Replace with:

  ```go
		c.editor = c.editor.WithValue(msg.res.Text).WithTidyHighlights(msg.res.Text, ranges)
  ```

- [ ] **Step 4: Build the module**

```bash
go build -tags=dev ./...
```

Expected: clean build. Any remaining error is a missed call site — the compiler names it.

- [ ] **Step 5: Commit**

```bash
git add internal/mailcompose/editor.go internal/ui/compose/model.go internal/ui/compose/tidy.go
git commit -m "$(cat <<'EOF'
compose: hold catkin.Model directly, delete Editor wrapper

mailcompose.Editor + CatkinEditor existed solely to insulate
compose from catkin's pointer shape. Pass 27 made catkin value-
shaped; the wrapper is now dead weight. Compose embeds
catkin.Model and routes through With* setters. The tidy result
handler now applies its paired (value, highlights) mutation in a
single statement, closing the ordering hazard a Msg path would
have opened.
EOF
)"
```

---

### Task 9: Convert compose tests

**Files:**
- Modify: `internal/ui/compose/model_test.go`
- Modify: `internal/ui/compose/tidy_test.go`
- Modify: `internal/ui/compose/suggest_test.go` (if it touches the editor)

- [ ] **Step 1: List the failing files**

```bash
go test -tags=dev ./internal/ui/compose/ 2>&1 | head -40
```

- [ ] **Step 2: Apply the same conversion sweep**

Same pattern as Pass 27 Task 4: `c.editor.SetX(...)` → `c.editor = c.editor.WithX(...)`.

For tests that construct a `compose.Model` directly without going through `compose.Open`, they may also need the constructor's `catkin.New().WithStyles(...)` form — match what `Open` does.

- [ ] **Step 3: Run the compose tests**

```bash
go test -tags=dev ./internal/ui/compose/ -v
```

Expected: green.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/compose/*_test.go
git commit -m "$(cat <<'EOF'
compose: convert tests to value-setter form

Mechanical follow-up; no behavior change.
EOF
)"
```

---

### Task 10: ADR-0213, ADR-0033 update, invariants delta

**Files:**
- Create: `docs/poplar/decisions/0213-compose-holds-catkin-directly.md`
- Modify: `docs/poplar/decisions/0033-*.md` (the neovim adapter ADR — find by glob)
- Modify: `docs/poplar/invariants.md`
- Modify: `docs/poplar/decisions/INDEX.md`
- Modify: `docs/poplar/STATUS.md`

- [ ] **Step 1: Find the ADR-0033 filename**

```bash
ls docs/poplar/decisions/0033-*
```

Note the filename for the next step.

- [ ] **Step 2: Write ADR-0213**

Create `docs/poplar/decisions/0213-compose-holds-catkin-directly.md`:

```markdown
---
title: Compose holds catkin directly
status: accepted
date: 2026-05-11
---

## Context

`mailcompose.Editor` (interface) and `mailcompose.CatkinEditor`
(impl) existed because catkin was pointer-shaped — compose needed
a stable pointer to hold across Update. The Editor interface
advertised a future neovim adapter (ADR-0033) but was already
inconsistent: the wizard's signature section imported
`catkin.Model` directly. The interface was a single-impl seam
preserved for a hypothetical post-v1 consumer — the exact
anti-pattern `go-conventions` flags.

Pass 27 (ADR-0212) converted catkin to a value type. The
structural reason for the wrapper is now gone.

## Decision

`mailcompose.Editor` interface and `mailcompose.CatkinEditor`
adapter delete. `compose.Model.editor` is `catkin.Model` (value).
The compose package mutates the embedded editor from its own
Update via `c.editor = c.editor.WithX(...)`, mirroring the
wizard's shape.

The tidy result handler applies its paired mutation in a single
statement:

```go
c.editor = c.editor.WithValue(text).WithTidyHighlights(text, ranges)
```

This closes the ordering hazard a Msg-routed alternative would
have opened (two Cmds, no guarantee of arrival order).

## Consequences

- ADR-0033 (neovim editor adapter, post-v1) is not superseded —
  its rationale survives. Only the v1-era *implementation
  strategy* via the Editor interface is dropped. The adapter
  shape will be designed fresh when concrete v1.1 requirements
  exist, per CLAUDE.md's "don't design for hypothetical future
  requirements" clause.
- The compose package no longer depends on `mailcompose` for the
  body editor type. Other `mailcompose` exports (Draft, Identity,
  Signature, AssembleMIME) remain in use.
- One fewer single-impl interface in the tree.
```

- [ ] **Step 3: Update ADR-0033 Consequences**

Open the ADR-0033 file (filename found in Step 1). Add a paragraph to its Consequences section:

```markdown
- Superseded — implementation strategy. ADR-0213 deleted the
  `mailcompose.Editor` interface that this ADR proposed as the
  v1.1 adapter seam. The neovim adapter remains a post-1.0 goal;
  its shape will be designed fresh against value-typed
  `catkin.Model` when concrete requirements exist.
```

Status stays `accepted` — only the implementation strategy is updated; the goal survives.

- [ ] **Step 4: Update `docs/poplar/invariants.md` Compose section**

Find the Compose section (around line 280). Currently begins:

```
- `internal/mailcompose/` is the UI-free outbound-mail surface: the
  `Editor` seam (CatkinEditor wraps `catkin.Model`; v1.1 will add a
  neovim adapter), the `Draft` value type, …
```

Rewrite the bullet head (preserving the rest of the bullet's body about Draft, AssembleMIME, SeedReply etc.):

```
- `internal/mailcompose/` is the UI-free outbound-mail surface:
  the `Draft` value type, pure `AssembleMIME(d, now)` (multipart/
  alternative text+html via `filter.MarkdownToHTML`; multipart/
  mixed when attachments are present), and `SeedReply` /
  `SeedReplyAll` / `SeedForward` parsing parent Message-Id and
  References from raw RFC 5322 bytes with depth-preserving `>`
  quoting. `gomail.Address` is the Draft address type;
  `content.ParseAddressList` is the shared list parser.
  `internal/filter` exposes `MarkdownBody` / `MarkdownToHTML` as
  the shared goldmark entries (Linkify + Table). `compose.Model`
  (`internal/ui/compose/`) embeds `catkin.Model` directly. ADRs
  0212, 0213.
```

In the `internal/ui/compose/` bullet immediately below, update the body-editor description: drop the `mailcompose.Editor (CatkinEditor in v1; v1.1 will add a neovim adapter behind the same interface)` clause and replace with `catkin.Model`.

- [ ] **Step 5: Update INDEX.md**

Add ADR-0213 to the relevant section in `docs/poplar/decisions/INDEX.md`.

- [ ] **Step 6: Update STATUS.md**

Mark Pass 28 done; rewrite next-starter-prompt for Pass 29 (`app.go` decomposition) using the format in `poplar-pass` skill. Pull source language from `STATUS.md`'s existing description of Pass 29.

- [ ] **Step 7: tmux verify**

Capture compose body at 120×40. Output must be byte-identical (modulo timestamps) to the Pass 27 capture.

- [ ] **Step 8: Archive the spec and plan**

```bash
git mv docs/superpowers/specs/2026-05-11-catkin-elm-design.md docs/superpowers/archive/specs/
git mv docs/superpowers/plans/2026-05-11-catkin-elm.md docs/superpowers/archive/plans/
```

- [ ] **Step 9: Commit, check, push, install**

```bash
git add docs/poplar/decisions/0213-compose-holds-catkin-directly.md \
        docs/poplar/decisions/0033-*.md \
        docs/poplar/invariants.md \
        docs/poplar/decisions/INDEX.md \
        docs/poplar/STATUS.md \
        docs/superpowers/archive/specs/2026-05-11-catkin-elm-design.md \
        docs/superpowers/archive/plans/2026-05-11-catkin-elm.md
make check
git commit -m "$(cat <<'EOF'
Pass 28: Delete Editor wrapper, compose holds catkin directly

mailcompose.Editor + CatkinEditor delete entirely. compose.Model
embeds catkin.Model. ADR-0033 (neovim adapter) consequences
updated — implementation strategy superseded; goal survives.

ADR-0213.
EOF
)"
git push
make install
```

---

## Definition of done

Both passes ship when:

- `make check` green at the end of each pass
- Catkin's existing test suite passes unchanged in intent
- `compose.Model.editor` is `catkin.Model` (not an interface)
- `internal/mailcompose/editor.go` does not exist on master
- ADR-0212 + ADR-0213 written; ADR-0033 Consequences updated
- `docs/poplar/invariants.md` + `.claude/rules/catkin-invariants.md` reflect the new shape
- tmux capture at 120×40 shows compose body + wizard signature section rendering identically to pre-pass behavior
- Plan + spec archived under `docs/superpowers/archive/`
