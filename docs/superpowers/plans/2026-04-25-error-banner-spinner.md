# Error banner + spinner consolidation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface previously-silent backend errors via a one-line banner above the status bar, owned by `App`; standardize the spinner placeholder behind a single `styles.NewSpinner` helper.

**Architecture:** Promote the existing private `backendErrMsg` to an exported `ErrorMsg` carrying an `Op` string. Hoist the banner state to `App` (mirroring the help-popover pattern from Pass 2.5b-5). Render a single tinted-text row above the status bar when `lastErr.Err != nil`, last-write-wins. Centralize the spinner constructor so future Pass 3 / Pass 9 consumers inherit one style.

**Tech Stack:** Go 1.26, bubbletea, lipgloss, bubbles/spinner. Existing repo conventions in `go-conventions` and `elm-conventions` skills.

**Spec:** `docs/superpowers/specs/2026-04-25-error-banner-spinner-design.md`

---

## File map

| File                                     | Role                                       |
|------------------------------------------|--------------------------------------------|
| `internal/ui/cmds.go`                    | rename type → `ErrorMsg`, add `Op`         |
| `internal/ui/account_tab.go`             | delete `backendErrMsg` case                |
| `internal/ui/styles.go`                  | add `ErrorBanner` style + `NewSpinner`     |
| `internal/ui/viewer.go`                  | use `NewSpinner`                           |
| `internal/ui/error_banner.go` (new)      | `renderErrorBanner` formatter              |
| `internal/ui/error_banner_test.go` (new) | banner tests                               |
| `internal/ui/app.go`                     | `lastErr`, `ErrorMsg` intercept, banner    |
| `internal/ui/app_test.go`                | banner integration tests                   |
| `docs/poplar/styling.md`                 | add banner row to surface map              |

---

## Task 1 — Rename `backendErrMsg` to exported `ErrorMsg` with `Op`

**Files:**
- Modify: `internal/ui/cmds.go` (lines 23-27, plus all call sites)
- Modify: `internal/ui/account_tab.go:118`

Pure refactor — no behavior change yet. Existing `AccountTab` handler still drops the message; this task only changes the type's name, exports it, and populates `Op` at each Cmd. Tests must continue to pass after this task.

- [ ] **Step 1.1: Replace the type definition**

In `internal/ui/cmds.go`, replace lines 23-27 with:

```go
// ErrorMsg carries a failure from any tea.Cmd. App captures the most
// recent ErrorMsg into lastErr; the banner renders "⚠ <Op>: <Err>".
// Last-write-wins: a subsequent ErrorMsg replaces the prior one.
type ErrorMsg struct {
	Op  string
	Err error
}
```

- [ ] **Step 1.2: Populate Op at every Cmd call site**

In `internal/ui/cmds.go`, update each return site:

`loadFoldersCmd` (line 45):
```go
return ErrorMsg{Op: "list folders", Err: err}
```

`loadFolderCmd` (lines 61, 65):
```go
if err := b.OpenFolder(name); err != nil {
    return ErrorMsg{Op: "open folder", Err: err}
}
msgs, err := b.FetchHeaders(nil)
if err != nil {
    return ErrorMsg{Op: "fetch headers", Err: err}
}
```

`loadBodyCmd` (lines 144, 148):
```go
r, err := b.FetchBody(uid)
if err != nil {
    return ErrorMsg{Op: "fetch body", Err: err}
}
buf, err := io.ReadAll(r)
if err != nil {
    return ErrorMsg{Op: "read body", Err: err}
}
```

`markReadCmd` (line 161):
```go
if err := b.MarkRead([]mail.UID{uid}); err != nil {
    return ErrorMsg{Op: "mark read", Err: err}
}
```

Update the doc comments mentioning `backendErrMsg` (lines 40, 52, 139, 155) to say `ErrorMsg` instead.

- [ ] **Step 1.3: Update the AccountTab handler**

In `internal/ui/account_tab.go:118`, replace:

```go
	case backendErrMsg:
		// Surfacing waits on the toast/status overlay.
		return m, nil
```

with:

```go
	case ErrorMsg:
		// App owns the banner; AccountTab ignores. Returning a nil
		// Cmd lets the message also be intercepted at App.Update.
		return m, nil
```

(App's `Update` runs *before* delegation in our flow — see `app.go:54-126` — so App captures the message; the AccountTab case stays as a safety no-op. Confirmed in Task 4.)

- [ ] **Step 1.4: Verify build + tests still pass**

Run: `make check`
Expected: all green; the rename is internal-package-scoped so nothing external breaks.

- [ ] **Step 1.5: Commit**

```bash
git add internal/ui/cmds.go internal/ui/account_tab.go
git commit -m "Pass 2.5b-6 step 1: promote backendErrMsg to exported ErrorMsg with Op"
```

---

## Task 2 — Spinner helper + ErrorBanner style

**Files:**
- Modify: `internal/ui/styles.go`
- Modify: `internal/ui/viewer.go:54-64`
- Test: `internal/ui/styles_test.go` (existing — extend)

Centralize the spinner constructor so the viewer (and future consumers) share one style. Add the `ErrorBanner` lipgloss style for Task 3 to consume.

- [ ] **Step 2.1: Write a failing test for `NewSpinner`**

Append to `internal/ui/styles_test.go` (or add a new `TestNewSpinner` if not present):

```go
func TestNewSpinner(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	sp := NewSpinner(th)
	if sp.Spinner.FPS == 0 {
		t.Error("NewSpinner returned an unconfigured spinner.Model")
	}
	// Verify the dot variant: spinner.Dot has 7 frames.
	if got := len(sp.Spinner.Frames); got != len(spinner.Dot.Frames) {
		t.Errorf("frames: got %d, want %d (spinner.Dot)", got, len(spinner.Dot.Frames))
	}
	// Style should render with the dim foreground.
	rendered := sp.Style.Render("x")
	if !strings.Contains(rendered, "x") {
		t.Errorf("Style.Render produced empty output: %q", rendered)
	}
	if rendered == "x" {
		t.Errorf("Style.Render produced unstyled output: %q", rendered)
	}
}
```

If the file lacks the `spinner` and `strings` imports, add them: `"github.com/charmbracelet/bubbles/spinner"` and `"strings"`.

- [ ] **Step 2.2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestNewSpinner`
Expected: FAIL with `undefined: NewSpinner`.

- [ ] **Step 2.3: Add `NewSpinner` to styles.go**

Add the import (top of file): `"github.com/charmbracelet/bubbles/spinner"`.

Add this function after `NewStyles` (end of file):

```go
// NewSpinner returns a configured bubbles/spinner.Model with poplar's
// shared style: Dot variant, FgDim foreground. Callers embed the
// returned model directly. Centralized so Pass 3 folder-load and
// Pass 9 send placeholders inherit the same look.
func NewSpinner(t *theme.CompiledTheme) spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(t.FgDim)
	return sp
}
```

- [ ] **Step 2.4: Run test to verify it passes**

Run: `go test ./internal/ui/ -run TestNewSpinner`
Expected: PASS.

- [ ] **Step 2.5: Add `ErrorBanner` style to the `Styles` struct**

In `internal/ui/styles.go`, add a new field grouped with the chrome styles. Locate the comment `// Top line frame edge` (around line 82) and update the trailing block:

```go
	// Top line frame edge
	TopLine   lipgloss.Style
	ToastText lipgloss.Style

	// ErrorBanner is the one-line surface above the status bar that
	// renders the most recent ErrorMsg. Foreground only; no fill.
	ErrorBanner lipgloss.Style
```

In `NewStyles`, append before the closing brace of the returned struct (after `ToastText`):

```go
		ErrorBanner: lipgloss.NewStyle().
			Foreground(t.ColorError),
```

- [ ] **Step 2.6: Switch the viewer to use `NewSpinner`**

In `internal/ui/viewer.go:54-64`, replace:

```go
func NewViewer(styles Styles, t *theme.CompiledTheme, accountName string) Viewer {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Dim
	return Viewer{
		styles:      styles,
		theme:       t,
		accountName: accountName,
		spinner:     sp,
	}
}
```

with:

```go
func NewViewer(styles Styles, t *theme.CompiledTheme, accountName string) Viewer {
	return Viewer{
		styles:      styles,
		theme:       t,
		accountName: accountName,
		spinner:     NewSpinner(t),
	}
}
```

Then remove the now-unused `"github.com/charmbracelet/bubbles/spinner"` import from `viewer.go` only if it was solely used for `spinner.New()` and `spinner.Dot`. Check: `spinner.Model` and `spinner.TickMsg` are still referenced (lines 45, 142), so keep the import.

- [ ] **Step 2.7: Run all tests**

Run: `make check`
Expected: PASS. The viewer's existing tests must still verify the loading-phase placeholder renders.

- [ ] **Step 2.8: Commit**

```bash
git add internal/ui/styles.go internal/ui/styles_test.go internal/ui/viewer.go
git commit -m "Pass 2.5b-6 step 2: add NewSpinner helper and ErrorBanner style"
```

---

## Task 3 — `renderErrorBanner` function + tests

**Files:**
- Create: `internal/ui/error_banner.go`
- Create: `internal/ui/error_banner_test.go`

Pure render function. No state. Returns `""` for nil error; otherwise `"⚠ <Op>: <Err>"` truncated to width.

- [ ] **Step 3.1: Write the failing tests**

Create `internal/ui/error_banner_test.go`:

```go
package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/theme"
)

func TestRenderErrorBannerNil(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	if got := renderErrorBanner(ErrorMsg{}, 80, styles); got != "" {
		t.Errorf("nil err: got %q, want empty string", got)
	}
}

func TestRenderErrorBannerBasic(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	msg := ErrorMsg{Op: "mark read", Err: errors.New("timeout")}
	got := renderErrorBanner(msg, 80, styles)
	if !strings.Contains(got, "⚠") {
		t.Errorf("missing warning glyph: %q", got)
	}
	if !strings.Contains(got, "mark read") {
		t.Errorf("missing op: %q", got)
	}
	if !strings.Contains(got, "timeout") {
		t.Errorf("missing err message: %q", got)
	}
	// Style should apply ColorError foreground (ANSI escape present).
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("expected ANSI styling, got plain text: %q", got)
	}
}

func TestRenderErrorBannerWithoutOp(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	msg := ErrorMsg{Err: errors.New("connection refused")}
	got := renderErrorBanner(msg, 80, styles)
	if !strings.Contains(got, "connection refused") {
		t.Errorf("missing err message: %q", got)
	}
	// No "Op:" prefix when Op is empty — just "⚠ <err>".
	if strings.Contains(got, ": connection refused") {
		t.Errorf("unexpected colon prefix when Op is empty: %q", got)
	}
}

func TestRenderErrorBannerTruncates(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	long := strings.Repeat("x", 200)
	msg := ErrorMsg{Op: "fetch body", Err: errors.New(long)}
	got := renderErrorBanner(msg, 40, styles)
	// Visible width must be ≤ 40 cells.
	if w := lipglossWidth(got); w > 40 {
		t.Errorf("width = %d, want ≤ 40", w)
	}
	// Must end with the ellipsis when truncated.
	if !strings.Contains(got, "…") {
		t.Errorf("missing truncation ellipsis: %q", got)
	}
}

func TestRenderErrorBannerMultibyte(t *testing.T) {
	// Truncation must split on rune boundaries, never inside a glyph.
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	msg := ErrorMsg{Op: "open", Err: errors.New("日本語日本語日本語日本語日本語")}
	got := renderErrorBanner(msg, 20, styles)
	if w := lipglossWidth(got); w > 20 {
		t.Errorf("width = %d, want ≤ 20", w)
	}
	// Verify the result is valid UTF-8 (no split runes).
	for _, r := range got {
		if r == '�' {
			t.Errorf("found replacement rune in output: %q", got)
		}
	}
}

// lipglossWidth is a thin alias so the test reads naturally; defined
// here rather than imported to avoid a per-file import churn.
func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}
```

Add the import for `"github.com/charmbracelet/lipgloss"` at the top of the test file.

- [ ] **Step 3.2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run TestRenderErrorBanner -v`
Expected: FAIL with `undefined: renderErrorBanner`.

- [ ] **Step 3.3: Implement `renderErrorBanner`**

Create `internal/ui/error_banner.go`:

```go
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// renderErrorBanner formats an ErrorMsg for the single banner row
// above the status bar. Returns "" when msg.Err is nil. Output is
// at most width display cells wide; longer text is truncated with
// "…". The leading "⚠ " glyph is part of the rendered string.
//
// When msg.Op is empty, the format is "⚠ <err>"; otherwise
// "⚠ <op>: <err>".
func renderErrorBanner(msg ErrorMsg, width int, styles Styles) string {
	if msg.Err == nil {
		return ""
	}
	text := "⚠ "
	if msg.Op != "" {
		text += msg.Op + ": "
	}
	text += msg.Err.Error()
	text = truncateToWidth(text, width)
	return styles.ErrorBanner.Render(text)
}

// truncateToWidth shortens s to at most width display cells, adding
// "…" when truncation occurs. Splits on rune boundaries — never
// inside a multi-byte glyph. Counts cells via lipgloss.Width so
// double-width CJK characters are accounted for correctly.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	// Build prefix runewise until adding the next rune + "…" would
	// exceed width. lipgloss.Width("…") == 1.
	const ellipsis = "…"
	limit := width - lipgloss.Width(ellipsis)
	out := make([]rune, 0, len(s))
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > limit {
			break
		}
		out = append(out, r)
		w += rw
	}
	return string(out) + ellipsis
}
```

- [ ] **Step 3.4: Run tests to verify they pass**

Run: `go test ./internal/ui/ -run TestRenderErrorBanner -v`
Expected: all five subtests PASS.

- [ ] **Step 3.5: Run the full UI package tests**

Run: `make check`
Expected: PASS.

- [ ] **Step 3.6: Commit**

```bash
git add internal/ui/error_banner.go internal/ui/error_banner_test.go
git commit -m "Pass 2.5b-6 step 3: add renderErrorBanner formatter with truncation"
```

---

## Task 4 — App owns `lastErr`; renders banner; integration tests

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`

Hoist banner state to root. App.Update intercepts `ErrorMsg` and stores it before delegating. App.View slots the banner row between content and status bar. Account region shrinks by one row when present.

- [ ] **Step 4.1: Write failing tests for App banner integration**

Append to `internal/ui/app_test.go` (do not duplicate existing test helpers — check what exists first; the file already has `setupTestApp` or similar). If the file uses a different helper name, adapt these tests to match.

```go
func TestAppCapturesErrorMsg(t *testing.T) {
	app := newTestApp(t) // existing helper; adapt name if different
	app, _ = app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	app, _ = app.Update(ErrorMsg{Op: "mark read", Err: errors.New("timeout")})

	if app.lastErr.Err == nil {
		t.Fatal("App.lastErr.Err is nil after ErrorMsg")
	}
	if app.lastErr.Op != "mark read" {
		t.Errorf("Op: got %q, want %q", app.lastErr.Op, "mark read")
	}
}

func TestAppBannerRendersAboveStatus(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app, _ = app.Update(ErrorMsg{Op: "fetch body", Err: errors.New("EOF")})

	view := app.View()
	if !strings.Contains(view, "⚠") {
		t.Error("View missing warning glyph")
	}
	if !strings.Contains(view, "fetch body") {
		t.Error("View missing op")
	}
	// The banner row must come above the status bar in the joined view.
	bannerIdx := strings.Index(view, "fetch body")
	statusIdx := strings.Index(view, "─────") // status bar uses box-drawing rules
	if bannerIdx == -1 || statusIdx == -1 {
		t.Skip("could not locate banner or status bar in rendered view")
	}
	if bannerIdx > statusIdx {
		t.Errorf("banner appears below status bar (banner idx=%d, status idx=%d)", bannerIdx, statusIdx)
	}
}

func TestAppBannerLastWriteWins(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app, _ = app.Update(ErrorMsg{Op: "first", Err: errors.New("a")})
	app, _ = app.Update(ErrorMsg{Op: "second", Err: errors.New("b")})

	if app.lastErr.Op != "second" {
		t.Errorf("Op: got %q, want %q (last-write-wins)", app.lastErr.Op, "second")
	}
	view := app.View()
	if strings.Contains(view, "first") {
		t.Errorf("View still contains the first error after replacement: %q", view)
	}
	if !strings.Contains(view, "second") {
		t.Errorf("View missing the second (current) error: %q", view)
	}
}

func TestAppBannerHiddenWhileHelpOpen(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app, _ = app.Update(ErrorMsg{Op: "fetch body", Err: errors.New("EOF")})
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})

	view := app.View()
	// Help popover takes the full screen; banner must not render inside it.
	if strings.Contains(view, "fetch body") {
		t.Errorf("banner rendered while help popover open: %q", view)
	}
}

func TestAppContentShrinksWhenBannerPresent(t *testing.T) {
	app := newTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	withoutBanner := strings.Count(app.View(), "\n")

	app, _ = app.Update(ErrorMsg{Op: "x", Err: errors.New("y")})
	withBanner := strings.Count(app.View(), "\n")

	if withBanner != withoutBanner {
		// The banner adds one row but the content shrinks by one row,
		// so the total line count is unchanged.
		t.Errorf("total view height changed: without=%d, with=%d", withoutBanner, withBanner)
	}
}
```

If `newTestApp` does not exist in `app_test.go`, inspect the file and use whatever constructor pattern existing tests use. Add `"errors"` and `"strings"` imports if missing.

- [ ] **Step 4.2: Run tests to verify they fail**

Run: `go test ./internal/ui/ -run TestApp -v`
Expected: FAIL — the new tests reference `app.lastErr` which doesn't exist yet, and `App.Update` does not handle `ErrorMsg`.

- [ ] **Step 4.3: Add `lastErr` field to `App`**

In `internal/ui/app.go`, modify the `App` struct (lines 14-26):

```go
type App struct {
	acct       AccountTab
	styles     Styles
	topLine    TopLine
	statusBar  StatusBar
	footer     Footer
	keys       GlobalKeys
	viewerOpen bool
	helpOpen   bool
	help       HelpPopover
	lastErr    ErrorMsg
	width      int
	height     int
}
```

- [ ] **Step 4.4: Intercept `ErrorMsg` in `App.Update`**

In `internal/ui/app.go`, add a case to the type switch in `Update` (place it between `ViewerScrollMsg` and `tea.KeyMsg`, around line 82):

```go
	case ErrorMsg:
		m.lastErr = msg
		// Continue delegating so child models still progress on
		// other state. AccountTab's ErrorMsg case is a no-op.
		var cmd tea.Cmd
		m.acct, cmd = m.acct.Update(msg)
		return m, cmd
```

- [ ] **Step 4.5: Account for the banner row in `contentHeight`**

Modify `contentHeight` (lines 159-167):

```go
func (m App) contentHeight() int {
	chrome := 3 // top line + status bar + footer
	if m.lastErr.Err != nil {
		chrome++ // banner row above the status bar
	}
	h := m.height - chrome
	if h < 1 {
		return 1
	}
	return h
}
```

Also propagate the recomputation when an error first arrives. In the new `case ErrorMsg:` block above, before delegating, send a resize down to the child if the banner state actually toggled the chrome height. The cleanest shape:

```go
	case ErrorMsg:
		hadErr := m.lastErr.Err != nil
		m.lastErr = msg
		var cmd tea.Cmd
		if hadErr != (m.lastErr.Err != nil) {
			contentMsg := tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}
			m.acct, cmd = m.acct.Update(contentMsg)
		}
		// Always also forward the original message.
		var cmd2 tea.Cmd
		m.acct, cmd2 = m.acct.Update(msg)
		return m, tea.Batch(cmd, cmd2)
```

(The `hadErr` toggle covers both nil → set and set → nil. Last-write-wins between two non-nil states does not change chrome height, so the resize is skipped.)

- [ ] **Step 4.6: Render the banner in `App.View`**

In `internal/ui/app.go`, modify `View` (lines 129-157) to insert a banner row between content and status:

```go
func (m App) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.helpOpen {
		return m.help.View(m.width, m.height)
	}

	rawContent := m.acct.View()
	rightBorder := m.styles.FrameBorder.Render("│")
	contentLines := strings.Split(rawContent, "\n")
	for i, line := range contentLines {
		pad := max(0, m.width-1-lipgloss.Width(line))
		contentLines[i] = line + strings.Repeat(" ", pad) + rightBorder
	}
	content := strings.Join(contentLines, "\n")

	dividerCol := sidebarWidth
	topLine := m.topLine.View(m.width, dividerCol)
	status := m.statusBar.View(m.width, sidebarWidth)
	foot := m.footer.View(m.width)

	parts := []string{topLine, content}
	if banner := renderErrorBanner(m.lastErr, m.width, m.styles); banner != "" {
		parts = append(parts, banner)
	}
	parts = append(parts, status, foot)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
```

- [ ] **Step 4.7: Run the new tests to verify they pass**

Run: `go test ./internal/ui/ -run TestApp -v`
Expected: all PASS, including the new banner tests.

- [ ] **Step 4.8: Run the full check**

Run: `make check`
Expected: PASS.

- [ ] **Step 4.9: Live verification (tmux)**

Per `.claude/docs/tmux-testing.md`, install and exercise the binary against a backend that produces a real error (e.g., temporarily point `accounts.toml` at an invalid host or use the existing demo backend's error path). Confirm visually:
- Banner appears as one row above the status bar.
- Text is `⚠ <op>: <err>` in `ColorError` foreground, no fill.
- Truncates with `…` when too long.
- Vanishes when help popover (`?`) is opened.
- A second error replaces the first.

If no easy real-error path exists, this step can be deferred to the pass-end live verification — note it in the commit body.

- [ ] **Step 4.10: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "Pass 2.5b-6 step 4: hoist error banner state to App"
```

---

## Task 5 — Update `docs/poplar/styling.md`

**Files:**
- Modify: `docs/poplar/styling.md`

Add the banner surface to the palette → surface map. Per the `Conventions` rule in CLAUDE.md, this doc is updated **before** committing color changes; we treat the banner as a new color surface even though `ColorError` is reused.

- [ ] **Step 5.1: Read the current styling doc**

Read `docs/poplar/styling.md` end-to-end so the banner row matches the existing format.

- [ ] **Step 5.2: Add the banner row**

Add a row to the surface table mapping `ColorError` → `ErrorBanner` (foreground, no background). Place it near the existing `StatusOffline` row (which also uses `ColorError`). Also add a paragraph noting that the banner is the only chrome surface that conditionally takes a row, shrinking the account region by one cell when present.

- [ ] **Step 5.3: Commit**

```bash
git add docs/poplar/styling.md
git commit -m "Pass 2.5b-6 step 5: document ErrorBanner surface in styling map"
```

---

## Pass-end checklist (handled by `poplar-pass`, not this plan)

After all five tasks land, the `poplar-pass` skill runs:

1. `simplify` — review all touched files.
2. ADRs:
   - `00NN-error-msg-app-owned.md` (banner ownership at root + ErrorMsg type)
   - `00NN-spinner-shared-style.md` (NewSpinner helper, no wrapper)
3. `invariants.md` — add facts about `ErrorMsg` (the canonical Cmd error type, App-owned `lastErr`, banner above status bar, last-write-wins, no dismiss key in v1) and the spinner constructor. Update the decision-index table with the new ADR numbers.
4. STATUS.md — mark Pass 2.5b-6 done; write the Pass 3 starter prompt.
5. Archive plan + spec under `docs/superpowers/archive/`.
6. `make check`, commit, push, `make install`.

---

## Self-review notes

- All spec sections covered: ErrorMsg type (Task 1), banner rendering (Task 3), App ownership (Task 4), spinner helper (Task 2), styling-doc update (Task 5), tests at every layer.
- No placeholders. Every step shows the actual code or command.
- Type names consistent: `ErrorMsg`, `Op`, `Err`, `lastErr`, `renderErrorBanner`, `truncateToWidth`, `NewSpinner`, `ErrorBanner` (style field). No drift between tasks.
- Out-of-scope items (multi-error queue, dismiss key, severity level) explicitly noted in the spec; no tasks created for them.
