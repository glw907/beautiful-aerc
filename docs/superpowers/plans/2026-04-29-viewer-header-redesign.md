# Viewer Header Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Re-shape the message viewer's header so it reads as a clear region distinct from the body, using indentation as the primary cue (no tint, no rule, no overline). Subject sits at the very top of the viewer pane; metadata block indents inward; body starts after a double blank.

**Architecture:** The change spans `internal/content/render.go` (the `RenderHeaders` rewrite + label helpers), `internal/theme/palette.go` (drop the now-unused `SubjectOverline` style), `internal/ui/viewer.go` (one extra blank between header and body + body-height accounting), test updates in `internal/content/render_test.go` and `internal/ui/viewer_test.go`, and a docs refresh in `docs/poplar/styling.md`. There is uncommitted partial progress in the working tree (overline already removed from `RenderHeaders` and from the `CompiledTheme` struct field) — Task 1 finishes that cleanup before structural changes start.

**Tech Stack:** Go 1.26.x, lipgloss, bubbletea (existing toolchain). No new dependencies.

**Spec:** [`docs/superpowers/specs/2026-04-29-viewer-header-redesign-design.md`](../specs/2026-04-29-viewer-header-redesign-design.md)

---

## File Structure

| File | Role |
|---|---|
| `internal/theme/palette.go` | Compiled theme styles. Drop the unused `SubjectOverline` assignment. |
| `internal/content/render.go` | `RenderHeaders` and label helpers. Drop trailing rule; lowercase + uncolon labels; add +2-cell indent on metadata rows. |
| `internal/ui/viewer.go` | `View` and `layout`. Emit a second blank between header and body; reserve 3 rows in body-height accounting. |
| `internal/content/render_test.go` | Header rendering assertions. Update `"From:"` → `"from "` etc; remove the rule-presence test; add an indent assertion. |
| `internal/ui/viewer_test.go` | Viewer assertions. Update `"From:"` → `"from "`. |
| `docs/poplar/styling.md` | Surface map. Drop the `SubjectOverline` row; refresh the "Message viewer" prose. |

---

## Pre-flight

The working tree currently has uncommitted partial changes: `internal/content/render.go` has the overline rendering already removed from `RenderHeaders`, and `internal/theme/palette.go` has the `SubjectOverline` field removed from the struct. The remaining inconsistency is that `NewCompiledTheme` still assigns to the (now-nonexistent) `t.SubjectOverline`, so the package will not compile until Task 1 runs.

- [ ] **Step 0: Confirm uncommitted state matches expectations**

```bash
git status
git diff --stat
```

Expected:

```
modified:   internal/content/render.go
modified:   internal/theme/palette.go
```

If the diff includes anything else, stop and inspect — don't proceed until the working tree is the expected partial state.

---

## Task 1: Drop the unused SubjectOverline style

**Files:**
- Modify: `internal/theme/palette.go` (remove the assignment block)

- [ ] **Step 1: Remove the SubjectOverline assignment**

Edit `internal/theme/palette.go` and delete the two-line assignment. The current text is:

```go
	t.SubjectTitle = lipgloss.NewStyle().
		Foreground(t.FgBright).Bold(true)
	t.SubjectOverline = lipgloss.NewStyle().
		Foreground(t.AccentSecondary)
	t.Paragraph = lipgloss.NewStyle().
```

Delete only the `t.SubjectOverline = ...` block so it becomes:

```go
	t.SubjectTitle = lipgloss.NewStyle().
		Foreground(t.FgBright).Bold(true)
	t.Paragraph = lipgloss.NewStyle().
```

- [ ] **Step 2: Verify build passes**

Run: `go build ./...`
Expected: clean exit, no output.

- [ ] **Step 3: Run full check**

Run: `make check`
Expected: all packages pass. Tests for `render.go` may still pass at this point because the spec only removed the overline rendering from `RenderHeaders` — that change was already in the working tree.

- [ ] **Step 4: Commit Task 1**

```bash
git add internal/content/render.go internal/theme/palette.go
git commit -m "Remove SubjectOverline style and rendering

The overline was sized to the rendered subject's width and looked
awkward when the subject was short. The new header design uses
indentation as the primary boundary cue, so the overline is unused.
Drop both the rendering call site (already done in working tree)
and the no-longer-referenced palette style assignment."
```

---

## Task 2: Lowercase header labels and drop the colon

**Files:**
- Modify: `internal/content/render.go` (`renderHeaderKey`)
- Modify: `internal/content/render_test.go` (assertions)
- Modify: `internal/ui/viewer_test.go` (assertions)

**Why this comes before the indent change:** the label change is the smallest unit; doing it alone keeps the diff focused and lets the test updates land in the same commit as the rendering change they cover.

- [ ] **Step 1: Update `renderHeaderKey` in `internal/content/render.go`**

Replace the existing function (around lines 226–239):

```go
// headerKeyColWidth is the cell width of the key+colon column. The
// longest key ("Subject:") is 8 cells; padding every key to this
// width aligns header values into a single column.
const headerKeyColWidth = 8

// renderHeaderKey renders "Key:" right-padded to headerKeyColWidth.
func renderHeaderKey(key string, t *theme.CompiledTheme) string {
	label := key + ":"
	pad := headerKeyColWidth - len(label)
	if pad < 0 {
		pad = 0
	}
	return t.HeaderKey.Render(label) + strings.Repeat(" ", pad)
}
```

with:

```go
// headerKeyColWidth is the cell width of the label column. The
// longest visible label after the Subject hoist is "date" (4 cells);
// the column stays at 8 cells for alignment headroom and to keep
// values landing where the prior layout placed them.
const headerKeyColWidth = 8

// renderHeaderKey renders the lowercase, colon-less header label
// right-padded to headerKeyColWidth. The HeaderDim style (FgDim) is
// applied so the label reads as a quiet margin annotation.
func renderHeaderKey(key string, t *theme.CompiledTheme) string {
	label := strings.ToLower(key)
	pad := headerKeyColWidth - len(label)
	if pad < 0 {
		pad = 0
	}
	return t.HeaderDim.Render(label) + strings.Repeat(" ", pad)
}
```

Note three things changed:

1. `label := strings.ToLower(key)` instead of `key + ":"` — lowercase, no colon.
2. The render style is `t.HeaderDim` (FgDim) instead of `t.HeaderKey` (AccentPrimary bold). This matches the spec's "lowercase dim labels" decision.
3. Padding math now operates on a 7-char-max label rather than an 8-char-max label-with-colon, but the column width stays 8 so values still align.

- [ ] **Step 2: Update `TestRenderHeaders` in `internal/content/render_test.go`**

Find the test (around line 97) that asserts `"From:"` is in the output. Replace those checks:

```go
	if !strings.Contains(visible, "From:") {
		t.Error("missing From header")
	}
```

with:

```go
	if !strings.Contains(visible, "from ") {
		t.Error("missing From header")
	}
```

(The trailing space disambiguates the label from the word "from" if it appeared in a value.)

- [ ] **Step 3: Update `TestRenderHeadersOrder`**

Find the assertions (around lines 130–141) and update them to match the new label form:

```go
	fromIdx := strings.Index(visible, "from ")
	toIdx := strings.Index(visible, "to ")
	dateIdx := strings.Index(visible, "date ")
```

The Subject-before-From assertion stays:

```go
	if subjectIdx > fromIdx {
		t.Error("Subject title should appear before From")
	}
```

- [ ] **Step 4: Update `TestRenderHeadersSkipsEmpty`**

Replace the body of the test:

```go
func TestRenderHeadersSkipsEmpty(t *testing.T) {
	h := ParsedHeaders{
		From:    []Address{{Name: "Alice", Email: "alice@example.com"}},
		Subject: "Test",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	visible := stripANSITest(result)
	if strings.Contains(visible, "to ") {
		t.Error("should not render empty To header")
	}
	if strings.Contains(visible, "cc ") {
		t.Error("should not render empty Cc header")
	}
}
```

- [ ] **Step 5: Update viewer assertion in `internal/ui/viewer_test.go`**

Find (around line 53):

```go
	if !strings.Contains(out, "From:") {
		t.Errorf("ready view missing From label: %q", out)
	}
```

Replace with:

```go
	if !strings.Contains(out, "from ") {
		t.Errorf("ready view missing From label: %q", out)
	}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/content/ ./internal/ui/ -run "Headers|Viewer" -v`
Expected: all pass.

- [ ] **Step 7: Run full check**

Run: `make check`
Expected: all packages pass.

- [ ] **Step 8: Commit Task 2**

```bash
git add internal/content/render.go internal/content/render_test.go internal/ui/viewer_test.go
git commit -m "Lowercase header labels, drop the colon, render in FgDim

Header labels (From/To/Cc/Bcc/Date) become 'from'/'to'/'cc'/'bcc'/
'date' in FgDim with no colon. The colored-bold AccentPrimary key
treatment competed visually with the subject title; FgDim labels
read as quiet margin annotations and let the subject be the only
loud element above the body. The 8-cell column width stays so
values continue to align."
```

---

## Task 3: Indent metadata rows by +2 cells

**Files:**
- Modify: `internal/content/render.go` (`renderHeaderScalar`, `renderHeaderAddresses`, new `metadataIndent` constant)
- Modify: `internal/content/render_test.go` (add an indent assertion)

- [ ] **Step 1: Add the indent constant in `internal/content/render.go`**

Find the `headerKeyColWidth` constant block (around line 226 after Task 2). Add a sibling constant right below it:

```go
// metadataIndent is the leading whitespace prefix on every metadata
// row (From/To/Cc/Bcc/Date). The two-cell inset reads as a margin
// annotation, distinct from the subject and body which sit flush
// at the pane's existing 1-cell padding.
const metadataIndent = "  "
```

- [ ] **Step 2: Apply `metadataIndent` in `renderHeaderScalar`**

Replace:

```go
func renderHeaderScalar(key, value string, t *theme.CompiledTheme) string {
	return renderHeaderKey(key, t) + " " + t.HeaderValue.Render(value)
}
```

with:

```go
func renderHeaderScalar(key, value string, t *theme.CompiledTheme) string {
	return metadataIndent + renderHeaderKey(key, t) + " " + t.HeaderValue.Render(value)
}
```

- [ ] **Step 3: Apply `metadataIndent` in `renderHeaderAddresses`**

Find the function (around line 259). Update the two places that produce row text:

Replace:

```go
	keyStr := renderHeaderKey(key, t)
	indent := strings.Repeat(" ", headerKeyColWidth+1)
```

with:

```go
	keyStr := metadataIndent + renderHeaderKey(key, t)
	indent := metadataIndent + strings.Repeat(" ", headerKeyColWidth+1)
```

And replace:

```go
	current := keyStr + " "
	currentVisible := headerKeyColWidth + 1
```

with:

```go
	current := keyStr + " "
	currentVisible := len(metadataIndent) + headerKeyColWidth + 1
```

The width-budget math now correctly accounts for the leading indent so the wrap accumulator doesn't overflow when a long address list spills to a continuation line.

- [ ] **Step 4: Add an indent-assertion test in `internal/content/render_test.go`**

Add a new test below `TestRenderHeadersAddressWrap`:

```go
func TestRenderHeadersMetadataIndented(t *testing.T) {
	h := ParsedHeaders{
		From:    []Address{{Name: "Alice", Email: "alice@example.com"}},
		To:      []Address{{Email: "bob@example.com"}},
		Date:    "Mon, 5 Jan 2026",
		Subject: "Hello",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	visible := stripANSITest(result)
	for _, label := range []string{"from ", "to ", "date "} {
		idx := strings.Index(visible, label)
		if idx < 0 {
			t.Fatalf("missing label %q in render", label)
		}
		// The label must be preceded by a newline (or start-of-string)
		// followed by exactly the metadataIndent (2 spaces).
		var prefixStart int
		if nl := strings.LastIndex(visible[:idx], "\n"); nl >= 0 {
			prefixStart = nl + 1
		}
		prefix := visible[prefixStart:idx]
		if prefix != "  " {
			t.Errorf("label %q prefix = %q, want two spaces", label, prefix)
		}
	}
}
```

- [ ] **Step 5: Run the new test**

Run: `go test ./internal/content/ -run TestRenderHeadersMetadataIndented -v`
Expected: PASS.

- [ ] **Step 6: Run full check**

Run: `make check`
Expected: all packages pass. The wrap-accumulator change in Step 3 is a width-math fix; `TestRenderHeadersAddressWrap` should still pass because the indent only shrinks effective wrap width by 2 cells and the existing fixture is far over the wrap threshold.

- [ ] **Step 7: Commit Task 3**

```bash
git add internal/content/render.go internal/content/render_test.go
git commit -m "Indent metadata rows by 2 cells in RenderHeaders

Each From/To/Cc/Bcc/Date row now has a 2-cell leading indent so
the metadata block reads as inset relative to the subject and body
(both flush at column 1). The address-list wrap accumulator
correctly accounts for the indent so continuation lines don't
overflow the pane width."
```

---

## Task 4: Drop the full-width header rule

**Files:**
- Modify: `internal/content/render.go` (`RenderHeaders`)
- Modify: `internal/content/render_test.go` (delete `TestRenderHeadersSeparator`)

- [ ] **Step 1: Remove the trailing rule in `RenderHeaders`**

Find the bottom of `RenderHeaders` (around lines 220–223 after Task 2/3 edits):

```go
	sep := t.HeaderDim.Render(strings.Repeat("─", width))
	lines = append(lines, "", sep)

	return strings.Join(lines, "\n")
}
```

Replace with:

```go
	return strings.Join(lines, "\n")
}
```

The header now ends with the last metadata row; the visual boundary between header and body is supplied by the double blank line emitted in `Viewer.View` (Task 5).

- [ ] **Step 2: Delete `TestRenderHeadersSeparator`**

Find the test (around line 160) and delete it entirely. It looks like:

```go
func TestRenderHeadersSeparator(t *testing.T) {
	h := ParsedHeaders{
		From:    []Address{{Email: "alice@example.com"}},
		Subject: "Test",
	}
	result := RenderHeaders(h, theme.Nord, 80)
	if !strings.Contains(result, "─") {
		t.Error("missing separator line")
	}
}
```

Delete the whole function and the blank line above it.

- [ ] **Step 3: Run full check**

Run: `make check`
Expected: all packages pass. No test should be looking for a `─` separator after the deletion.

- [ ] **Step 4: Commit Task 4**

```bash
git add internal/content/render.go internal/content/render_test.go
git commit -m "Drop the full-width header rule

The dim '─' rule under the metadata block was a third-tier boundary
cue that no longer earned its row — the +2 metadata indent + the
subject title in FgBright bold make the header region distinct on
their own. The double blank line View() emits between header and
body is the only remaining boundary marker, and that's enough."
```

---

## Task 5: Second blank between header and body in the viewer

**Files:**
- Modify: `internal/ui/viewer.go` (`View`, `layout`)

- [ ] **Step 1: Add a second blank in `Viewer.View`**

Find the `View` method (around lines 202–221). Replace the join near line 219:

```go
	out := lipgloss.JoinVertical(lipgloss.Left, headers, blank, body, blank)
```

with:

```go
	out := lipgloss.JoinVertical(lipgloss.Left, headers, blank, blank, body, blank)
```

The first `blank` is the existing trailing-of-header blank; the second is the new "extra breathing room before body" blank.

- [ ] **Step 2: Bump `bodyHeight` reservation in `Viewer.layout`**

Find the `layout` method (around line 257). Replace the body-height calculation (around line 271):

```go
	bodyHeight := max(1, v.height-headerHeight-2)
```

with:

```go
	bodyHeight := max(1, v.height-headerHeight-3)
```

Three rows reserved: two blanks between header and body + one bottom-of-pane blank.

- [ ] **Step 3: Update the `layout` doc comment**

Find the comment block immediately above `func (v *Viewer) layout()` (around lines 248–256) and replace:

```go
// contentWidth is one cell narrower than v.width. padLeftLinesBg adds
// the leading space back in View(), so the total per-line cell count
// equals v.width after clipPaneBg pads the remainder. The body height
// reserves two rows for the blank padding rows View() emits: one
// between headers and body, and one at the bottom.
func (v *Viewer) layout() {
```

with:

```go
// contentWidth is one cell narrower than v.width. padLeftLinesBg adds
// the leading space back in View(), so the total per-line cell count
// equals v.width after clipPaneBg pads the remainder. The body height
// reserves three rows for the blank padding rows View() emits: two
// between headers and body (the trailing-of-header blank plus an
// extra breathing-room blank), and one at the bottom of the pane.
func (v *Viewer) layout() {
```

- [ ] **Step 4: Run full check**

Run: `make check`
Expected: all packages pass.

- [ ] **Step 5: Commit Task 5**

```bash
git add internal/ui/viewer.go
git commit -m "Emit a second blank between viewer headers and body

The layout now reserves three blank rows in body-height accounting:
two between header and body (the existing trailing-header blank +
a new breathing-room blank before the body's first paragraph), and
one at the bottom of the pane. Combined with the dropped header
rule, this is the only boundary cue between the header region and
the body content."
```

---

## Task 6: Documentation refresh

**Files:**
- Modify: `docs/poplar/styling.md` (drop `SubjectOverline` row, refresh "Message viewer" prose)

- [ ] **Step 1: Drop the SubjectOverline row from the surface table**

Open `docs/poplar/styling.md` and find the "Message viewer" section. The current surface table looks like:

```markdown
| Field | fg | bg | Role |
|-------|----|----|------|
| `ViewerBg` | — | `BgBase` | Base pane background (all padding rows + leading column + right-edge fill) |
| `SubjectOverline` (theme) | `AccentSecondary` | — | Short `─` accent above the Subject title, sized to the rendered subject's display width |
| `SubjectTitle` (theme) | `FgBright` bold | — | Subject rendered as a standalone title above the structured header block |
| `HeaderKey` (theme) | `AccentPrimary` bold | — | `From:`/`To:`/`Cc:`/`Bcc:`/`Date:` label |
| `HeaderValue` (theme) | `FgBase` | — | Address name and scalar value |
| `HeaderDim` (theme) | `FgDim` | — | `<email>` brackets and the full-width `─` separator under the headers |
```

Replace with:

```markdown
| Field | fg | bg | Role |
|-------|----|----|------|
| `ViewerBg` | — | `BgBase` | Base pane background (all padding rows + leading column + right-edge fill) |
| `SubjectTitle` (theme) | `FgBright` bold | — | Subject rendered as a standalone title at the top of the viewer pane (column 1, no leading blank) |
| `HeaderValue` (theme) | `FgBase` | — | Address name and scalar value in the metadata block |
| `HeaderDim` (theme) | `FgDim` | — | Lowercase `from`/`to`/`cc`/`bcc`/`date` labels and `<email>` brackets in the metadata block |
```

`HeaderKey` is no longer used by the viewer (labels render in `HeaderDim` now), so drop its row from this section. The `HeaderKey` style still exists in the theme for any future consumer; this table documents only what the viewer surface uses.

- [ ] **Step 2: Refresh the "Message viewer" prose**

Find the introductory paragraph for the section. Replace:

```markdown
The viewer shares `BgBase` with the message list — the right panel is
a single surface, so the viewer fills its full pane (header rows,
body rows, and the blank padding rows at the top, between headers
and body, and at the bottom) with `BgBase`. Header keys (From/To/Cc/
Bcc/Date) are right-padded to a common 8-cell column so values align.
```

with:

```markdown
The viewer shares `BgBase` with the message list — the right panel is
a single surface. The Subject sits at column 1 on the very first row
of the pane (vertically aligned with the sidebar's account label).
A blank row separates the Subject from the metadata block (From/To/
Cc/Bcc/Date), which is indented two cells inward and rendered with
lowercase labels in `FgDim` and no colons. Two blank rows follow the
metadata before the body content begins, and a final blank row closes
the pane at the bottom. The header has no rule, no overline, and no
background tint — indentation alone delineates the header region.
```

Also find the "Background composition" paragraph below the table and replace:

```markdown
Background composition: `clipPaneBg` and `padLeftLinesBg` use
bg-styled spaces so the right-edge fill, left column, and top/bottom
blank padding rows all carry `BgBase`. Each rendered content line is
then run through `bgFillLine` (in `styles.go`), which prepends the
bg ANSI prefix and re-emits it after every embedded `\x1b[0m` reset
so cells under styled content don't fall back to the terminal default.
```

with:

```markdown
Background composition: `clipPaneBg` and `padLeftLinesBg` use
bg-styled spaces so the right-edge fill, left column, and the blank
padding rows all carry `BgBase`. Each rendered content line is then
run through `bgFillLine` (in `styles.go`), which prepends the bg ANSI
prefix and re-emits it after every embedded `\x1b[0m` reset so cells
under styled content don't fall back to the terminal default.
```

(The only change is dropping "top/" before "bottom blank padding rows" since the top blank no longer exists.)

- [ ] **Step 3: Verify the file still parses cleanly**

Run: `head -200 docs/poplar/styling.md | tail -60`
Expected: the "Message viewer" section reads coherently with no orphaned table rows or stray prose.

- [ ] **Step 4: Commit Task 6**

```bash
git add docs/poplar/styling.md
git commit -m "Update styling.md for the viewer header redesign

Drop the SubjectOverline row from the viewer surface table (the
style is gone). Drop the HeaderKey row (the viewer's metadata
labels now render in HeaderDim, not HeaderKey). Rewrite the
'Message viewer' prose to describe the indentation-led layout:
subject at column 1 on row 1, metadata indented +2 with lowercase
no-colon labels, double blank before body, no rule/overline/tint."
```

---

## Task 7: Live verification

**Files:** none modified

- [ ] **Step 1: Install the binary**

Run: `make install`
Expected: `go install ./cmd/poplar` runs without error.

- [ ] **Step 2: Capture the viewer in tmux**

Run:

```bash
tmux kill-session -t poplar 2>/dev/null
tmux new-session -d -s poplar -x 120 -y 40 'poplar'
sleep 2.0
tmux send-keys -t poplar 'Enter'
sleep 2.0
tmux capture-pane -t poplar -p | sed -n '1,12p'
tmux kill-session -t poplar
```

Expected output shape (the actual subject and sender will differ depending on which message lands at the cursor, but the structure must match):

```
──────────────────────────────┬────────────────────────────────────────────────────────────────────────────────────────╮
 geoff@907.life               │ <Subject text on the same row as geoff@907.life>                                       │
                              │                                                                                        │
┃ <icon>  Inbox            <N>│   from    <Sender name>                                                                │
  <icon>  Drafts              │   to      geoff@907.life                                                               │
  <icon>  Sent                │   date    <weekday, mon dd yyyy hh:mm AM>                                              │
…
```

Verify by inspection:

- The `geoff@907.life` row in the sidebar and the Subject row in the viewer are on the **same line** of the captured output.
- The first metadata row (`from`) is preceded by exactly 3 cells of leading whitespace inside the viewer column (1 cell of pane padding + 2 cells of metadata indent).
- The labels are lowercase with no colon: `from `, `to `, `date `.
- There is no `─` rule line between the metadata block and the body content.
- Two blank rows separate the last metadata row from the body's first paragraph.

- [ ] **Step 3: Capture again with a multi-recipient message to verify Cc rendering**

Open a message that has Cc recipients (any group/list email). Verify:

- The `cc` row appears between `to` and `date` (or omitted when no Cc is present).
- The `cc` row uses the same `  cc      ` indent + label form as `from` and `to`.

If you don't have a Cc-bearing message in the inbox at test time, this step can be skipped; the unit test in `TestRenderHeadersOrder` already covers the structural ordering.

- [ ] **Step 4: Commit any final adjustments**

If Steps 2–3 surface any visual issues that the unit tests didn't catch (e.g., a misaligned label column, a stray blank, a wrong indent), fix the offending file directly and commit with a `Fix viewer header: <description>` message. If everything looks right, no commit is needed.

- [ ] **Step 5: Push**

```bash
git push
```

Expected: push succeeds; the CI gate (if any) is `make check` which has been passing throughout.

---

## Acceptance verification

The spec lists three acceptance criteria. Confirm each:

| Criterion | How to verify |
|---|---|
| Subject sits on the same screen row as `geoff@907.life` in 120×40 | Task 7 Step 2 captures the pane and you eyeball that row 2 has both. |
| Metadata indented +2 cells with lowercase no-colon labels | Task 3's `TestRenderHeadersMetadataIndented` enforces the +2 indent; Task 2's lowercase changes are covered by `TestRenderHeaders` and `TestRenderHeadersOrder`. |
| No tint, no rule, no overline | Tint never landed (we abandoned D in brainstorming). Rule removed in Task 4 (`TestRenderHeadersSeparator` deleted). Overline removed before this plan started + Task 1 finishes the cleanup. |

`make check` passes after each task.
