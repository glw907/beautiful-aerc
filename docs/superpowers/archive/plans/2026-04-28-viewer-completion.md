# Pass 2.5b-4b — Viewer Completion: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the viewer with three independent units — long bare URL footnoting, filtered `n`/`N` navigation, and the `Tab` link picker modal.

**Architecture:** Phase 1 is pure content-layer (no UI). Phase 2 adds two key bindings to `AccountTab` and one helper to `MessageList`. Phase 3 introduces a new `LinkPicker` component owned by `App`, mirroring the help-popover overlay pattern from ADR-0082. Each phase ships under `make check` independently.

**Tech Stack:** Go 1.26, `bubbletea`, `bubbles/key`, `lipgloss`, table-driven tests, no assertion libraries. Conventions: `go-conventions` skill for every file, `elm-conventions` for `internal/ui/`, `bubbletea-conventions.md` §10 review at pass end.

**Spec:** `docs/superpowers/specs/2026-04-28-viewer-completion-design.md`

---

## File map

| File | Phase | Action |
|---|---|---|
| `internal/content/url_trim.go` | 1 | create |
| `internal/content/url_trim_test.go` | 1 | create |
| `internal/content/render_footnote.go` | 1 | modify |
| `internal/content/render_footnote_test.go` | 1 | modify |
| `internal/ui/msglist.go` | 2 | modify |
| `internal/ui/msglist_test.go` | 2 | modify |
| `internal/ui/account_tab.go` | 2 | modify |
| `internal/ui/account_tab_test.go` | 2 | modify |
| `internal/ui/cmds.go` | 3 | modify (relocate `launchURLCmd`, add three Msg types + `linkPickerOpenCmd`) |
| `internal/ui/viewer.go` | 3 | modify (Tab handler, drop local `openURL`/`launchURLCmd` body) |
| `internal/ui/linkpicker.go` | 3 | create |
| `internal/ui/linkpicker_test.go` | 3 | create |
| `internal/ui/app.go` | 3 | modify (state, Update routing, View overlay) |
| `internal/ui/app_test.go` | 3 | modify (round-trip) |
| `internal/ui/help.go` | 3 | modify (`Tab` row in viewer vocabulary) |

---

## Phase 1 — Long bare URL handling

### Task 1.1: `trimURL` pure function

**Files:**
- Create: `internal/content/url_trim.go`
- Create: `internal/content/url_trim_test.go`

- [ ] **Step 1: Write the failing test table**

`internal/content/url_trim_test.go`:

```go
// SPDX-License-Identifier: MIT

package content

import "testing"

func TestTrimURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"host only", "https://example.com", "example.com"},
		{"host trailing slash", "https://example.com/", "example.com/"},
		{"single segment", "https://example.com/foo", "example.com/foo"},
		{"single segment trailing slash", "https://example.com/foo/", "example.com/foo/"},
		{"two segments", "https://example.com/foo/bar", "example.com/foo…"},
		{"segment plus query", "https://example.com/foo?q=1", "example.com/foo…"},
		{"segment plus fragment", "https://example.com/foo#frag", "example.com/foo…"},
		{"deep path with query and fragment", "https://example.com/a/b/c?x=1#frag", "example.com/a…"},
		{"http scheme", "http://example.com/foo/bar", "example.com/foo…"},
		{"mailto", "mailto:foo@example.com", "foo@example.com"},
		{"port preserved", "https://example.com:8080/foo/bar", "example.com:8080/foo…"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := trimURL(tc.in)
			if got != tc.want {
				t.Fatalf("trimURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/content/ -run TestTrimURL
```

Expected: FAIL — `trimURL` undefined.

- [ ] **Step 3: Implement `trimURL`**

`internal/content/url_trim.go`:

```go
// SPDX-License-Identifier: MIT

package content

import "strings"

// trimURL produces a compact inline form of a URL for the long-bare-URL
// footnote path. Strips the scheme, keeps the host (with port), and
// optionally appends "/" + the first path segment. A trailing "/" is
// preserved only when it terminates the URL. Appends "…" when anything
// was removed.
//
// The trim cuts on '/', '?', '#', '&'. Userinfo, IPv6 brackets, and
// punycode are pass-through — they do not appear in real bodies poplar
// surfaces.
func trimURL(url string) string {
	if url == "" {
		return ""
	}
	rest := stripScheme(url)
	hostEnd := indexAny(rest, "/?#&")
	if hostEnd < 0 {
		// host-only — no path, no truncation needed.
		return rest
	}
	host := rest[:hostEnd]
	tail := rest[hostEnd:]
	// tail starts with the separator char; only continue building the
	// trim if the separator is '/'. '?' / '#' / '&' on a host-only URL
	// trigger immediate truncation.
	if tail[0] != '/' {
		return host + "…"
	}
	// Find the end of the first path segment.
	segEnd := indexAny(tail[1:], "/?#&")
	if segEnd < 0 {
		// Whole tail is one segment, ends at URL end. Preserve as-is.
		return host + tail
	}
	segEnd++ // re-anchor to tail's index space.
	// If the only thing past the segment is a trailing '/', preserve it.
	if tail[segEnd] == '/' && segEnd == len(tail)-1 {
		return host + tail
	}
	return host + tail[:segEnd] + "…"
}

// stripScheme removes a leading "scheme:" or "scheme://" from url.
// Returns url unchanged when no scheme is present.
func stripScheme(url string) string {
	colon := strings.IndexByte(url, ':')
	if colon <= 0 {
		return url
	}
	rest := url[colon+1:]
	rest = strings.TrimPrefix(rest, "//")
	return rest
}

// indexAny returns the index of the first byte in s that is in chars,
// or -1 when none are found.
func indexAny(s, chars string) int {
	return strings.IndexAny(s, chars)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/content/ -run TestTrimURL -v
```

Expected: all 12 cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/content/url_trim.go internal/content/url_trim_test.go
git commit -m "Pass 2.5b-4b: trimURL pure helper for long-bare-URL footnoting"
```

### Task 1.2: Wire `trimURL` into `harvestFootnotes`

**Files:**
- Modify: `internal/content/render_footnote.go` (extend `footnoteWalker.spans`)
- Modify: `internal/content/render_footnote_test.go` (add three tests)

- [ ] **Step 1: Write the failing tests**

Append to `internal/content/render_footnote_test.go`:

```go
func TestLongBareURLFootnoted(t *testing.T) {
	url := "https://example.com/very/long/path/that/exceeds/thirty/cells?query=1"
	blocks := []Block{Paragraph{Spans: []Span{Link{Text: url, URL: url}}}}
	rewritten, urls := harvestFootnotes(blocks)
	if len(urls) != 1 || urls[0] != url {
		t.Fatalf("expected one harvested url=%q, got %v", url, urls)
	}
	p := rewritten[0].(Paragraph)
	link := p.Spans[0].(Link)
	want := "example.com/very… [^1]"
	if link.Text != want {
		t.Fatalf("link.Text = %q, want %q", link.Text, want)
	}
	if link.URL != url {
		t.Fatalf("link.URL = %q, want %q", link.URL, url)
	}
}

func TestShortBareURLPassThrough(t *testing.T) {
	url := "https://example.com/foo"
	blocks := []Block{Paragraph{Spans: []Span{Link{Text: url, URL: url}}}}
	rewritten, urls := harvestFootnotes(blocks)
	if len(urls) != 0 {
		t.Fatalf("expected no harvested urls, got %v", urls)
	}
	p := rewritten[0].(Paragraph)
	link := p.Spans[0].(Link)
	if link.Text != url {
		t.Fatalf("link.Text = %q, want unchanged %q", link.Text, url)
	}
}

func TestLongBareURLDedupedWithTextLink(t *testing.T) {
	url := "https://example.com/very/long/path/that/exceeds/thirty/cells?q=1"
	blocks := []Block{
		Paragraph{Spans: []Span{Link{Text: url, URL: url}}},
		Paragraph{Spans: []Span{Link{Text: "click here", URL: url}}},
	}
	_, urls := harvestFootnotes(blocks)
	if len(urls) != 1 {
		t.Fatalf("expected one harvested url after dedupe, got %v", urls)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/content/ -run "TestLongBareURLFootnoted|TestShortBareURLPassThrough|TestLongBareURLDedupedWithTextLink" -v
```

Expected: FAIL — current code skips bare URLs entirely, so the harvest list is empty for the first and third tests.

- [ ] **Step 3: Update `footnoteWalker.spans`**

Replace the bare-URL branch in `internal/content/render_footnote.go`. The current `spans` method:

```go
func (w *footnoteWalker) spans(in []Span) []Span {
	if len(in) == 0 {
		return in
	}
	out := make([]Span, len(in))
	for i, s := range in {
		if link, ok := s.(Link); ok && link.Text != link.URL && link.URL != "" {
			n := w.markerFor(link.URL)
			out[i] = Link{Text: link.Text + nbsp + fmt.Sprintf("[^%d]", n), URL: link.URL}
		} else {
			out[i] = s
		}
	}
	return out
}
```

becomes:

```go
// longBareURLThreshold is the display-cell width above which a bare URL
// gets the long-URL footnote treatment instead of inline pass-through.
const longBareURLThreshold = 30

func (w *footnoteWalker) spans(in []Span) []Span {
	if len(in) == 0 {
		return in
	}
	out := make([]Span, len(in))
	for i, s := range in {
		link, ok := s.(Link)
		if !ok || link.URL == "" {
			out[i] = s
			continue
		}
		switch {
		case link.Text != link.URL:
			n := w.markerFor(link.URL)
			out[i] = Link{Text: link.Text + nbsp + fmt.Sprintf("[^%d]", n), URL: link.URL}
		case displayCells(link.URL) > longBareURLThreshold:
			n := w.markerFor(link.URL)
			out[i] = Link{Text: trimURL(link.URL) + nbsp + fmt.Sprintf("[^%d]", n), URL: link.URL}
		default:
			out[i] = s
		}
	}
	return out
}
```

If `displayCells` lives in another package (`internal/ui`), use a content-layer equivalent. Quick check:

```bash
grep -rn "func displayCells" internal/
```

If the helper is UI-package-only, inline a content-layer version using `lipgloss.Width` (URLs are ASCII-only in practice; `len` would also be safe but `lipgloss.Width` is the convention).

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/content/ -v
```

Expected: all content tests PASS, including the three new ones.

- [ ] **Step 5: Commit**

```bash
git add internal/content/render_footnote.go internal/content/render_footnote_test.go
git commit -m "Pass 2.5b-4b: footnote bare URLs > 30 cells with trimmed inline form"
```

### Task 1.3: Phase 1 gate

- [ ] **Step 1: Run `make check`**

```bash
make check
```

Expected: PASS.

---

## Phase 2 — `n`/`N` filtered viewer navigation

### Task 2.1: `MessageList.MoveCursor`

**Files:**
- Modify: `internal/ui/msglist.go` (new method near `MoveDown`/`MoveUp` at ~line 692)
- Modify: `internal/ui/msglist_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/msglist_test.go`:

```go
func TestMessageListMoveCursor(t *testing.T) {
	m := newTestMessageList(t, threeMessageFixture()) // existing helper or inline 3 messages

	uid, moved := m.MoveCursor(1)
	if !moved {
		t.Fatal("MoveCursor(+1) from row 0 should move")
	}
	wantUID := mail.UID("uid-2")
	if uid != wantUID {
		t.Fatalf("MoveCursor(+1) = %q, want %q", uid, wantUID)
	}

	_, moved = m.MoveCursor(1)
	if !moved {
		t.Fatal("MoveCursor(+1) from row 1 should move")
	}
	_, moved = m.MoveCursor(1)
	if moved {
		t.Fatal("MoveCursor(+1) from last row should be inert")
	}

	_, moved = m.MoveCursor(-1)
	if !moved {
		t.Fatal("MoveCursor(-1) from last row should move")
	}
}
```

If `newTestMessageList` and `threeMessageFixture` don't exist, inline three `mail.MessageInfo` values directly with stable UIDs `uid-1`/`uid-2`/`uid-3` and call `m.SetMessages(...)`. Match the construction pattern used by other tests in the same file.

- [ ] **Step 2: Run test to verify failure**

```bash
go test ./internal/ui/ -run TestMessageListMoveCursor
```

Expected: FAIL — `MoveCursor` undefined.

- [ ] **Step 3: Implement `MoveCursor`**

Insert in `internal/ui/msglist.go` near the existing `MoveDown` / `MoveUp` methods:

```go
// MoveCursor shifts the cursor by delta visible rows (negative for up,
// positive for down) and returns the resulting UID and whether the
// cursor actually moved. Boundaries are inert — at first/last visible
// row, calling with the corresponding direction returns ("", false).
// Hidden (folded) rows are skipped.
func (m *MessageList) MoveCursor(delta int) (mail.UID, bool) {
	before := m.selected
	m.moveBy(delta)
	if m.selected == before {
		return "", false
	}
	return m.cursorUID(), true
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/ui/ -run TestMessageListMoveCursor -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "Pass 2.5b-4b: MessageList.MoveCursor for viewer n/N nav"
```

### Task 2.2: `n`/`N` dispatch in `AccountTab`

**Files:**
- Modify: `internal/ui/account_tab.go`
- Modify: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Refactor `openSelectedMessage` to take a `MessageInfo`**

Read `internal/ui/account_tab.go:303-321`. Extract the open-by-message logic so both `Enter` and `n`/`N` reuse it:

```go
// openMessage opens msg in the viewer, fires the body-fetch Cmd, and
// (for unread messages) flips the seen flag locally + fires a backend
// MarkRead. Shared by Enter, n, and N.
func (m AccountTab) openMessage(msg mail.MessageInfo) (AccountTab, tea.Cmd) {
	m.viewer = m.viewer.Open(msg)
	cmds := []tea.Cmd{
		loadBodyCmd(m.backend, msg.UID),
		viewerOpenedCmd(),
		m.viewer.SpinnerTick(),
	}
	if msg.Flags&mail.FlagSeen == 0 {
		m.msglist.MarkSeen(msg.UID)
		cmds = append(cmds, markReadCmd(m.backend, msg.UID))
	}
	return m, tea.Batch(cmds...)
}

func (m AccountTab) openSelectedMessage() (AccountTab, tea.Cmd) {
	msg, ok := m.msglist.SelectedMessage()
	if !ok {
		return m, nil
	}
	return m.openMessage(msg)
}
```

- [ ] **Step 2: Add a `messageByUID` lookup**

If `MessageList` doesn't already expose a UID lookup (grep `MessageByUID|byUID`), add this minimal helper. In `internal/ui/msglist.go` near `SelectedMessage`:

```go
// MessageByUID returns the message info for uid, or ok=false when not
// found in the source set.
func (m MessageList) MessageByUID(uid mail.UID) (mail.MessageInfo, bool) {
	for i := range m.source {
		if m.source[i].UID == uid {
			return m.source[i], true
		}
	}
	return mail.MessageInfo{}, false
}
```

- [ ] **Step 3: Write the failing test for `n` advancing**

Append to `internal/ui/account_tab_test.go` (use the existing test scaffold patterns — model construction, Update dispatch, Cmd inspection):

```go
func TestViewerNAdvancesCursorAndFetchesBody(t *testing.T) {
	m := newAccountTabWithMessages(t, threeMessages())
	// Enter on row 0 to open viewer.
	m, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	// Simulate the loading→ready transition so n is not inert.
	m.viewer = m.viewer.SetBody(nil)

	m2, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if got := m2.viewer.CurrentUID(); got != mail.UID("uid-2") {
		t.Fatalf("after n, viewer UID = %q, want uid-2", got)
	}
	if cmd == nil {
		t.Fatal("expected fetch Cmd batch after n")
	}
}

func TestViewerNAtBoundaryInert(t *testing.T) {
	m := newAccountTabWithMessages(t, threeMessages())
	m, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m.viewer = m.viewer.SetBody(nil)
	// Walk to last row.
	m, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	prevUID := m.viewer.CurrentUID()

	m2, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if cmd != nil {
		t.Fatal("expected no Cmd at boundary")
	}
	if m2.viewer.CurrentUID() != prevUID {
		t.Fatal("expected viewer UID unchanged at boundary")
	}
}

func TestViewerNDuringLoadInert(t *testing.T) {
	m := newAccountTabWithMessages(t, threeMessages())
	m, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	// viewer is in viewerLoading; do NOT call SetBody.

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if cmd != nil {
		t.Fatal("expected no Cmd while viewer is loading")
	}
}
```

If `newAccountTabWithMessages` and `threeMessages` helpers don't exist, define them at the top of the test file (or inline in TestMain). Use the existing `internal/ui/account_tab_test.go` scaffold patterns; they exist for the prior viewer tests.

- [ ] **Step 4: Run tests to verify failure**

```bash
go test ./internal/ui/ -run "TestViewerN" -v
```

Expected: FAIL — `n`/`N` dispatch not yet wired.

- [ ] **Step 5: Wire `n`/`N` in `handleKey`**

In `internal/ui/account_tab.go`, locate the viewer-open key dispatch branch (where the existing `1`-`9`, `q`, `esc` viewer keys flow). Add cases for `n` and `N` **before** delegating to `viewer.Update`:

```go
case "n", "N":
	if !m.viewer.IsOpen() || m.viewer.Phase() != viewerReady {
		return m, nil
	}
	delta := 1
	if s == "N" {
		delta = -1
	}
	uid, moved := m.msglist.MoveCursor(delta)
	if !moved {
		return m, nil
	}
	info, ok := m.msglist.MessageByUID(uid)
	if !ok {
		return m, nil
	}
	return m.openMessage(info)
```

If `Viewer.Phase()` is not currently exported, expose it with a thin accessor on the viewer:

```go
// Phase reports the viewer's current load phase. Used by AccountTab
// to gate n/N during loading so a second fetch isn't queued.
func (v Viewer) Phase() viewerPhase { return v.phase }
```

If `viewerPhase` is unexported and AccountTab cannot reference it from another file in the same package, it can — same package. No export needed.

- [ ] **Step 6: Run tests to verify pass**

```bash
go test ./internal/ui/ -run "TestViewerN" -v
go test ./internal/ui/ -v   # full UI package — make sure nothing regressed
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/msglist.go internal/ui/account_tab.go internal/ui/account_tab_test.go internal/ui/msglist_test.go internal/ui/viewer.go
git commit -m "Pass 2.5b-4b: viewer n/N walks visible row set"
```

### Task 2.3: Phase 2 gate

- [ ] **Step 1: Run `make check`**

```bash
make check
```

Expected: PASS.

---

## Phase 3 — Link picker overlay

### Task 3.1: Msg types and shared `launchURLCmd`

**Files:**
- Modify: `internal/ui/cmds.go` (relocate `launchURLCmd` from `viewer.go`, add three Msg types + `linkPickerOpenCmd`)
- Modify: `internal/ui/viewer.go` (drop the local `launchURLCmd` and `openURL` definitions, keep call sites)

- [ ] **Step 1: Move `openURL` and `launchURLCmd` to `cmds.go`**

In `internal/ui/cmds.go`, append:

```go
// openURL is the URL launcher hook. Tests swap it to capture the URL
// instead of executing xdg-open. Shared by viewer numeric quick-launch
// and the link picker.
var openURL = func(url string) error {
	return exec.Command("xdg-open", url).Start()
}

// launchURLCmd opens url via the openURL hook. Errors are swallowed —
// xdg-open detaches and its exit status is unreliable.
func launchURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		_ = openURL(url)
		return nil
	}
}
```

Delete the matching declarations from `internal/ui/viewer.go`. Add `"os/exec"` to `cmds.go` imports if not already present; remove from `viewer.go` if no other uses remain.

- [ ] **Step 2: Add three Msg types and the open Cmd**

Append to `internal/ui/cmds.go`:

```go
// LinkPickerOpenMsg requests the link picker overlay open with the
// given URL list. Emitted by Viewer when Tab is pressed and at least
// one URL is harvested. Handled at the App level (App owns the picker
// state, mirrors the help-popover pattern from ADR-0082).
type LinkPickerOpenMsg struct {
	Links []string
}

// LinkPickerClosedMsg signals the picker has closed (Esc, Tab, Enter,
// or numeric launch). Handled at the App level to flip linkPicker.open.
type LinkPickerClosedMsg struct{}

// LaunchURLMsg requests App fire launchURLCmd for the given URL.
// Emitted by the link picker on Enter or 1-9 in-range.
type LaunchURLMsg struct {
	URL string
}

// linkPickerOpenCmd wraps a LinkPickerOpenMsg in a tea.Cmd so callers
// can return it from Update.
func linkPickerOpenCmd(links []string) tea.Cmd {
	return func() tea.Msg { return LinkPickerOpenMsg{Links: links} }
}
```

- [ ] **Step 3: Run existing tests**

```bash
go test ./internal/ui/ -v
```

Expected: PASS — the relocations are pure refactors. The viewer's existing `1`-`9` test (which mocks `openURL`) still works because `openURL` is now in `cmds.go` but in the same package.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/cmds.go internal/ui/viewer.go
git commit -m "Pass 2.5b-4b: relocate launchURLCmd, add link-picker Msg types"
```

### Task 3.2: `LinkPicker` model — Open/Close/Update/cursor

**Files:**
- Create: `internal/ui/linkpicker.go`
- Create: `internal/ui/linkpicker_test.go`

- [ ] **Step 1: Write the failing tests for state transitions**

`internal/ui/linkpicker_test.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
)

func newTestLinkPicker(t *testing.T) LinkPicker {
	t.Helper()
	th := theme.OneDark()
	styles := NewStyles(th)
	p := NewLinkPicker(styles, th)
	p = p.SetSize(80, 24)
	return p
}

func TestLinkPickerOpenSetsCursor(t *testing.T) {
	p := newTestLinkPicker(t)
	links := []string{"https://a.com", "https://b.com", "https://c.com"}
	p = p.Open(links)
	if !p.IsOpen() {
		t.Fatal("picker should be open after Open()")
	}
	if p.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", p.Cursor())
	}
}

func TestLinkPickerCursorBounds(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com", "https://b.com"})

	// k from row 0 stays at 0.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if p.Cursor() != 0 {
		t.Fatalf("k from row 0: cursor = %d, want 0", p.Cursor())
	}

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if p.Cursor() != 1 {
		t.Fatalf("j from row 0: cursor = %d, want 1", p.Cursor())
	}

	// j past last row stays at last.
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if p.Cursor() != 1 {
		t.Fatalf("j from last row: cursor = %d, want 1", p.Cursor())
	}
}

func TestLinkPickerEnterEmitsLaunchAndClose(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com", "https://b.com"})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	got := collectMsgs(cmd)
	if !containsLaunchURL(got, "https://b.com") {
		t.Fatalf("expected LaunchURLMsg{https://b.com}, got %v", got)
	}
	if !containsClosed(got) {
		t.Fatalf("expected LinkPickerClosedMsg, got %v", got)
	}
}

func TestLinkPickerNumericLaunchInRange(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com", "https://b.com", "https://c.com"})

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})

	got := collectMsgs(cmd)
	if !containsLaunchURL(got, "https://b.com") {
		t.Fatalf("expected LaunchURLMsg{https://b.com}, got %v", got)
	}
}

func TestLinkPickerNumericOutOfRangeInert(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})

	if cmd != nil {
		t.Fatalf("out-of-range numeric should be inert, got cmd=%v", cmd)
	}
}

func TestLinkPickerEscCloses(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := collectMsgs(cmd)
	if !containsClosed(got) {
		t.Fatalf("expected LinkPickerClosedMsg from Esc, got %v", got)
	}
}

func TestLinkPickerTabCloses(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := collectMsgs(cmd)
	if !containsClosed(got) {
		t.Fatalf("expected LinkPickerClosedMsg from Tab, got %v", got)
	}
}

func TestLinkPickerQSwallowed(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})
	p2, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Fatalf("q should be swallowed, got cmd=%v", cmd)
	}
	if !p2.IsOpen() {
		t.Fatal("q should not close picker")
	}
}

// collectMsgs runs cmd and returns the resulting messages. Handles
// tea.Batch by walking the batch tree.
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

func containsLaunchURL(msgs []tea.Msg, url string) bool {
	for _, m := range msgs {
		if l, ok := m.(LaunchURLMsg); ok && l.URL == url {
			return true
		}
	}
	return false
}

func containsClosed(msgs []tea.Msg) bool {
	for _, m := range msgs {
		if _, ok := m.(LinkPickerClosedMsg); ok {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/ui/ -run TestLinkPicker -v
```

Expected: FAIL — `LinkPicker` and friends undefined.

- [ ] **Step 3: Implement the model — state transitions only (no rendering)**

`internal/ui/linkpicker.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
)

// LinkPicker is the modal overlay launched by Tab while the viewer is
// open and ready. Single-column list of harvested URLs, cursor +
// Enter, 1-9 quick launch, Esc/Tab close. App owns the open state and
// the overlay composition (mirrors help popover, ADR-0082).
type LinkPicker struct {
	open   bool
	links  []string
	cursor int
	offset int
	width  int
	height int
	styles Styles
	theme  *theme.CompiledTheme
	keys   linkPickerKeys
}

type linkPickerKeys struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Close key.Binding
}

// NewLinkPicker returns a closed picker.
func NewLinkPicker(styles Styles, t *theme.CompiledTheme) LinkPicker {
	return LinkPicker{
		styles: styles,
		theme:  t,
		keys: linkPickerKeys{
			Up:    key.NewBinding(key.WithKeys("k", "up")),
			Down:  key.NewBinding(key.WithKeys("j", "down")),
			Enter: key.NewBinding(key.WithKeys("enter")),
			Close: key.NewBinding(key.WithKeys("esc", "tab")),
		},
	}
}

// IsOpen reports whether the picker is visible.
func (p LinkPicker) IsOpen() bool { return p.open }

// Cursor returns the highlighted row index. Exposed for tests.
func (p LinkPicker) Cursor() int { return p.cursor }

// Open transitions the picker into the open state with the given URL
// list. Cursor and offset reset to 0.
func (p LinkPicker) Open(links []string) LinkPicker {
	p.open = true
	p.links = links
	p.cursor = 0
	p.offset = 0
	return p
}

// Close transitions the picker out of view. Caller is responsible for
// any chrome-revert side effects (App handles this via Msg flow).
func (p LinkPicker) Close() LinkPicker {
	p.open = false
	return p
}

// SetSize updates the picker's box dimensions. App threads
// WindowSizeMsg here.
func (p LinkPicker) SetSize(width, height int) LinkPicker {
	p.width = width
	p.height = height
	return p
}

// Update dispatches a tea.Msg while the picker is open. Returns the
// updated picker and any Cmds (launch + close on Enter / numeric;
// close on Esc/Tab; nil otherwise).
func (p LinkPicker) Update(msg tea.Msg) (LinkPicker, tea.Cmd) {
	if !p.open {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Down):
		if p.cursor < len(p.links)-1 {
			p.cursor++
		}
		return p, nil
	case key.Matches(keyMsg, p.keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case key.Matches(keyMsg, p.keys.Enter):
		if p.cursor < 0 || p.cursor >= len(p.links) {
			return p, nil
		}
		return p, tea.Batch(
			func() tea.Msg { return LaunchURLMsg{URL: p.links[p.cursor]} },
			func() tea.Msg { return LinkPickerClosedMsg{} },
		)
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return LinkPickerClosedMsg{} }
	}
	// Numeric quick launch.
	if s := keyMsg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		idx := int(s[0] - '1')
		if idx >= len(p.links) {
			return p, nil
		}
		return p, tea.Batch(
			func() tea.Msg { return LaunchURLMsg{URL: p.links[idx]} },
			func() tea.Msg { return LinkPickerClosedMsg{} },
		)
	}
	// q and any other key: swallowed, no Cmd.
	return p, nil
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/ui/ -run TestLinkPicker -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/linkpicker.go internal/ui/linkpicker_test.go
git commit -m "Pass 2.5b-4b: LinkPicker model with cursor + numeric + close"
```

### Task 3.3: `LinkPicker.View` — layout, truncation, preview, scroll

**Files:**
- Modify: `internal/ui/linkpicker.go` (add `Box`, `Position`, `View`)
- Modify: `internal/ui/linkpicker_test.go` (visual tests for row format, scroll, preview)

- [ ] **Step 1: Write failing layout tests**

Append to `internal/ui/linkpicker_test.go`:

```go
func TestLinkPickerRowFormatLeadingSpacePad(t *testing.T) {
	// 12 links → max index "12", single-digit indices need a leading
	// space before "[" so closing "]" aligns.
	links := make([]string, 12)
	for i := range links {
		links[i] = "https://a.com"
	}
	p := newTestLinkPicker(t)
	p = p.SetSize(80, 24).Open(links)
	out := p.View()
	if !strings.Contains(out, " [1]") {
		t.Fatalf("expected ' [1]' (leading-space pad) in output, got:\n%s", out)
	}
	if !strings.Contains(out, "[12]") {
		t.Fatalf("expected '[12]' in output, got:\n%s", out)
	}
}

func TestLinkPickerRowFormatNoPad(t *testing.T) {
	// 9 links → max index "9", no padding needed.
	links := make([]string, 9)
	for i := range links {
		links[i] = "https://a.com"
	}
	p := newTestLinkPicker(t)
	p = p.SetSize(80, 24).Open(links)
	out := p.View()
	if strings.Contains(out, " [1]") {
		t.Fatalf("expected no leading-space pad in 9-link picker, got:\n%s", out)
	}
	if !strings.Contains(out, "[1]") {
		t.Fatalf("expected '[1]' in output, got:\n%s", out)
	}
}

func TestLinkPickerPreviewShowsFullURL(t *testing.T) {
	long := "https://example.com/some/very/long/path/that/wraps?query=value"
	p := newTestLinkPicker(t)
	p = p.SetSize(80, 24).Open([]string{"https://a.com", long})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	out := p.View()
	if !strings.Contains(out, "example.com/some/very/long") {
		t.Fatalf("preview should expose full URL prefix, got:\n%s", out)
	}
}
```

Add `import "strings"` to the test file if missing.

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/ui/ -run "TestLinkPickerRowFormat|TestLinkPickerPreview" -v
```

Expected: FAIL — `View` not yet implemented.

- [ ] **Step 3: Implement `Box`, `Position`, `View`**

Append to `internal/ui/linkpicker.go`:

```go
import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// boxMaxWidth caps the picker's natural width.
const linkPickerMaxWidth = 70

// linkPickerInlineCap caps the inline URL display length per row,
// independent of box width — keeps the visual tight even on very wide
// terminals.
const linkPickerInlineCap = 50

// View renders the picker as a standalone string. App composes via
// Box + Position + PlaceOverlay; this method is the fallback used by
// tests and when the box doesn't fit.
func (p LinkPicker) View() string {
	if !p.open {
		return ""
	}
	return p.Box(p.width, p.height)
}

// Box returns the rendered modal at the size derived from (w, h).
func (p LinkPicker) Box(w, h int) string {
	boxW := linkPickerMaxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < 20 {
		boxW = 20
	}
	contentW := boxW - 2 // left/right border
	indexW := 2 + len(strconv.Itoa(len(p.links)))
	urlW := contentW - indexW - 1 // 1 space between index and URL
	if urlW > linkPickerInlineCap {
		urlW = linkPickerInlineCap
	}

	visibleRows := len(p.links)
	maxListRows := h - 7 // top + bottom border + rule + 2 preview + 1 title slack
	if maxListRows < 1 {
		maxListRows = 1
	}
	if visibleRows > maxListRows {
		visibleRows = maxListRows
	}

	// Scroll: keep cursor in the [offset, offset+visibleRows) window.
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+visibleRows {
		p.offset = p.cursor - visibleRows + 1
	}

	var b strings.Builder
	// Top border with title.
	title := " Links "
	rest := boxW - 2 - len(title)
	if rest < 0 {
		rest = 0
	}
	b.WriteString("┌─" + title + strings.Repeat("─", rest) + "┐\n")

	// List rows.
	maxIndexDigits := len(strconv.Itoa(len(p.links)))
	for i := 0; i < visibleRows; i++ {
		row := p.offset + i
		if row >= len(p.links) {
			b.WriteString("│" + strings.Repeat(" ", contentW) + "│\n")
			continue
		}
		b.WriteString("│")
		b.WriteString(p.formatRow(row, maxIndexDigits, urlW, contentW))
		b.WriteString("│\n")
	}

	// Rule.
	b.WriteString("├" + strings.Repeat("─", contentW) + "┤\n")

	// Preview footer (2 rows). Wrap full URL of cursor row to contentW.
	previewLines := p.previewLines(contentW)
	for i := 0; i < 2; i++ {
		line := ""
		if i < len(previewLines) {
			line = previewLines[i]
		}
		w := lipgloss.Width(line)
		if w < contentW {
			line += strings.Repeat(" ", contentW-w)
		}
		b.WriteString("│" + line + "│\n")
	}

	// Bottom border.
	b.WriteString("└" + strings.Repeat("─", contentW) + "┘")

	return b.String()
}

// formatRow renders one list row: leading-space-pad + [N] + space + URL.
// Painted with cursor background when row == p.cursor.
func (p LinkPicker) formatRow(row, maxIndexDigits, urlW, contentW int) string {
	idxStr := strconv.Itoa(row + 1)
	pad := strings.Repeat(" ", maxIndexDigits-len(idxStr))
	url := p.links[row]
	if displayCells(url) > urlW {
		url = displayTruncate(url, urlW)
	}
	body := fmt.Sprintf("%s[%d] %s", pad, row+1, url)
	w := lipgloss.Width(body)
	if w < contentW {
		body += strings.Repeat(" ", contentW-w)
	}
	if row == p.cursor {
		return p.styles.Cursor.Render(body)
	}
	return body
}

// previewLines returns up to 2 wrapped lines of the cursor row's full
// URL. The 2nd line is truncated with "…" when the URL exceeds 2
// rows worth of cells.
func (p LinkPicker) previewLines(width int) []string {
	if p.cursor < 0 || p.cursor >= len(p.links) {
		return nil
	}
	full := p.links[p.cursor]
	wrapped := strings.Split(wrap(full, width), "\n")
	if len(wrapped) <= 2 {
		return wrapped
	}
	// Truncate row 2 with "…": take everything that fit on rows 1..2,
	// then ellipsize the tail.
	row2 := wrapped[1]
	if displayCells(row2) >= width {
		row2 = displayTruncate(row2, width-1) + "…"
	} else {
		row2 += "…"
	}
	return []string{wrapped[0], row2}
}

// Position returns the centered top-left for the rendered box at
// (totalW, totalH). Used by App to feed PlaceOverlay.
func (p LinkPicker) Position(box string, totalW, totalH int) (int, int) {
	bw := lipgloss.Width(box)
	bh := lipgloss.Height(box)
	x := (totalW - bw) / 2
	y := (totalH - bh) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}
```

If `Styles` doesn't have a `Cursor` field, use the existing message-list cursor style — grep `styles.go` for the field name and substitute. If `wrap` is the package-internal wrap helper from `render_footnote.go` etc., it lives in `internal/content/`; mirror the equivalent UI helper or use `lipgloss`-based wrapping. (Quick check: `grep -rn "func wrap" internal/ui/`. If absent, port a thin wrap that calls `wordwrap.String(s, w)` then hard-wraps via `ansi.Wrap`.)

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/ui/ -run TestLinkPicker -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/linkpicker.go internal/ui/linkpicker_test.go
git commit -m "Pass 2.5b-4b: LinkPicker layout — rows, truncation, preview"
```

### Task 3.4: Viewer `Tab` triggers picker

**Files:**
- Modify: `internal/ui/viewer.go` (replace the `tab` no-op)
- Modify: `internal/ui/viewer_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/ui/viewer_test.go`:

```go
func TestViewerTabEmitsLinkPickerOpenWhenLinks(t *testing.T) {
	v := newTestViewer(t)
	v = v.Open(mail.MessageInfo{UID: "uid-1"})
	v = v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{
			content.Link{Text: "click", URL: "https://a.com"},
		}},
	})

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyTab})

	got := collectMsgs(cmd)
	var found bool
	for _, m := range got {
		if op, ok := m.(LinkPickerOpenMsg); ok && len(op.Links) == 1 && op.Links[0] == "https://a.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected LinkPickerOpenMsg with [a.com], got %v", got)
	}
}

func TestViewerTabNoLinksInert(t *testing.T) {
	v := newTestViewer(t)
	v = v.Open(mail.MessageInfo{UID: "uid-1"})
	v = v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{content.Text{Text: "no links"}}},
	})

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyTab})

	if cmd != nil {
		t.Fatalf("expected no Cmd when zero links, got %v", cmd)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/ui/ -run "TestViewerTab" -v
```

Expected: FAIL — `tab` is currently a no-op in viewer.

- [ ] **Step 3: Update `Viewer.handleKey`**

In `internal/ui/viewer.go:165`, replace:

```go
case "tab":
    return v, nil
```

with:

```go
case "tab":
    if len(v.links) == 0 {
        return v, nil
    }
    return v, linkPickerOpenCmd(v.links)
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/ui/ -run "TestViewerTab" -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/viewer.go internal/ui/viewer_test.go
git commit -m "Pass 2.5b-4b: viewer Tab opens link picker when links present"
```

### Task 3.5: App owns picker state, routes Update, composes View

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`

- [ ] **Step 1: Write failing integration test**

Append to `internal/ui/app_test.go`:

```go
func TestAppLinkPickerRoundTrip(t *testing.T) {
	captured := ""
	prev := openURL
	openURL = func(url string) error { captured = url; return nil }
	defer func() { openURL = prev }()

	app := newTestApp(t) // existing helper or inline
	// Open viewer on a message with one link.
	app, _ = app.handleEnter(t)
	app = app.setViewerBody(t, []string{"https://example.com"})

	// Tab → LinkPickerOpenMsg → App opens picker.
	app, cmd := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = drainAndApply(t, app, cmd)
	if !app.IsLinkPickerOpen() {
		t.Fatal("expected picker open after Tab")
	}

	// Enter → launch + close.
	app, cmd = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = drainAndApply(t, app, cmd)

	if captured != "https://example.com" {
		t.Fatalf("expected openURL called with example.com, got %q", captured)
	}
	if app.IsLinkPickerOpen() {
		t.Fatal("expected picker closed after Enter launch")
	}
}
```

The `newTestApp`, `handleEnter`, `setViewerBody`, `drainAndApply`, and `IsLinkPickerOpen` helpers are minimal harness code. If existing test scaffolding doesn't cover them, define them in the same test file. `drainAndApply` runs the Cmd, feeds the resulting Msgs back into `app.Update`, and repeats until no Cmd remains (one or two iterations max here).

- [ ] **Step 2: Run test to verify failure**

```bash
go test ./internal/ui/ -run TestAppLinkPickerRoundTrip -v
```

Expected: FAIL — App has no picker state yet.

- [ ] **Step 3: Add `linkPicker` field to `App`**

In `internal/ui/app.go`, add a field to the App struct:

```go
linkPicker LinkPicker
```

In the App constructor, initialize:

```go
linkPicker: NewLinkPicker(styles, t),
```

Add an accessor for tests:

```go
// IsLinkPickerOpen reports whether the link picker overlay is visible.
func (m App) IsLinkPickerOpen() bool { return m.linkPicker.IsOpen() }
```

- [ ] **Step 4: Wire the three Msgs in `App.Update`**

Add cases (place them with the other Msg-type handlers, near the help-popover handlers):

```go
case LinkPickerOpenMsg:
	m.linkPicker = m.linkPicker.Open(msg.Links)
	return m, nil
case LinkPickerClosedMsg:
	m.linkPicker = m.linkPicker.Close()
	return m, nil
case LaunchURLMsg:
	return m, launchURLCmd(msg.URL)
```

In the key dispatch path, **before** delegating to children, short-circuit when picker is open (mirror the `helpOpen` short-circuit):

```go
if m.linkPicker.IsOpen() {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		var cmd tea.Cmd
		m.linkPicker, cmd = m.linkPicker.Update(keyMsg)
		return m, cmd
	}
}
```

In the `WindowSizeMsg` handler, thread size to the picker:

```go
m.linkPicker = m.linkPicker.SetSize(m.width, m.height)
```

- [ ] **Step 5: Compose overlay in `App.View`**

In `App.View`, after the standard frame is composed and **before** returning, check the picker and overlay (mirror the help-popover composition):

```go
if m.linkPicker.IsOpen() {
	box := m.linkPicker.Box(m.width, m.height)
	x, y := m.linkPicker.Position(box, m.width, m.height)
	dimmed := DimANSI(frame)
	return PlaceOverlay(x, y, box, dimmed)
}
```

Use whatever the existing help overlay block does — `PlaceOverlay` and `DimANSI` are already imported per ADR-0082. Match its argument order and return shape exactly.

- [ ] **Step 6: Run tests to verify pass**

```bash
go test ./internal/ui/ -v
```

Expected: all PASS, including `TestAppLinkPickerRoundTrip`.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "Pass 2.5b-4b: App owns link picker — state, Update, overlay"
```

### Task 3.6: Help vocabulary — `Tab` row

**Files:**
- Modify: `internal/ui/help.go`
- Modify: `internal/ui/help_test.go` (if existing tests assert row counts)

- [ ] **Step 1: Locate `viewerGroups`**

```bash
grep -n "viewerGroups\|viewerBottomHints" internal/ui/help.go
```

- [ ] **Step 2: Add the `Tab` row**

In the appropriate viewer group (likely the navigation/links group — match by reading the surrounding rows), insert:

```go
{Key: "Tab", Desc: "open link picker", Wired: true},
```

- [ ] **Step 3: If `help_test.go` asserts row counts, bump them**

```bash
go test ./internal/ui/ -run TestHelp -v
```

Update any expected-count assertions to account for the +1 row.

- [ ] **Step 4: Run full UI tests**

```bash
go test ./internal/ui/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/help.go internal/ui/help_test.go
git commit -m "Pass 2.5b-4b: help — wire Tab row in viewer vocabulary"
```

### Task 3.7: Phase 3 gate + live verification

- [ ] **Step 1: Run `make check`**

```bash
make check
```

Expected: PASS.

- [ ] **Step 2: Install and verify in tmux**

```bash
make install
```

In a tmux pane at 120×40:

1. Open poplar against the live Fastmail account.
2. Open a message with multiple URLs (use `internal/mail/mock.go` if needed — start poplar with the mock backend by setting whatever env or flag the existing tests use).
3. Press `Tab` — confirm the picker overlay appears, dimmed background, list rendered.
4. Press `j`/`k` — cursor moves, preview footer updates with full URL.
5. Press `2` — `xdg-open` fires for URL #2, picker closes.
6. Re-open, press `Esc` — picker closes without launch.
7. Test at narrow width (resize pane to ~50 cols) — picker layout remains coherent.

- [ ] **Step 3: Commit a `STATUS.md` audit link** (optional, only if a UI audit doc was produced)

Skip if no separate audit was written.

---

## Pass-end ritual

(Per `poplar-pass` skill — list here for the executor's reference; the skill drives the actual sequence.)

- [ ] **Step 1: Run `simplify`**

```bash
/simplify
```

Apply genuine wins. Skip cosmetic churn.

- [ ] **Step 2: Run conventions §10 checklist**

Open `docs/poplar/bubbletea-conventions.md` §10. For each item in the new `LinkPicker` and modified Viewer/AccountTab files, confirm:

- View() never wider than width / never more rows than height (clipPane equivalent or hand-enforced).
- No state mutation in View() / tea.Cmd closures.
- All blocking I/O in tea.Cmd (none here — picker is pure).
- Width math via `displayCells` / `displayTruncate` / `lipgloss.Width`, no `len()`.
- Renderers honor width via wordwrap+hardwrap (preview path: `wrap()` + 2-row truncate).
- No defensive parent-side clipping.
- Children → parents via Msg types (LaunchURLMsg, LinkPickerClosedMsg, LinkPickerOpenMsg).
- WindowSizeMsg propagated to picker.
- Keys via `key.Binding` + `key.Matches`.
- No deprecated APIs.

- [ ] **Step 3: Write three ADRs**

In `docs/poplar/decisions/` (next available numbers — check the directory):

1. **Long bare URL footnoting** — > 30 cell threshold, `trimURL` rule, `…` glyph only on actual trim, dedupe with text-bearing links.
2. **`n`/`N` viewer navigation semantics** — visible-row coupling via `MoveCursor`, boundary inert, `viewerLoading` inert, optimistic mark-seen reuse via `openMessage`.
3. **Link picker overlay** — modal launched by `Tab`, hand-rolled (deviation from `bubbles/list` justified per §"Bubbles analogue + deviation" of the spec), key vocabulary (`j/k/Enter/Esc/Tab/1-9/q-swallow`), index-column right-alignment with leading-space pad, 50-cell inline URL truncation, 2-row preview footer with full-URL wrap+truncate.

- [ ] **Step 4: Update `docs/poplar/invariants.md`**

Edit in place. Add or rewrite binding facts touched by the three ADRs:

- Viewer `Tab` is wired to the link picker (not reserved/no-op).
- Bare URLs > 30 cells get the long-URL footnote treatment.
- Viewer `n`/`N` advances the message list cursor and re-fetches the body; inert at boundaries and during loading.
- Link picker is App-owned overlay, viewer-context-only.

Update the decision-index table at the bottom with the three new ADR numbers.

- [ ] **Step 5: Update `docs/poplar/STATUS.md`**

- Mark `2.5b-4b` `done` in the pass table.
- Replace the starter prompt with the next pass's (Pass 5 — `key.Matches` migration per BACKLOG #17, App.View trust per #19, intra-model Cmd → delegation per #18).
- Keep STATUS ≤60 lines.

- [ ] **Step 6: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-04-28-viewer-completion.md \
       docs/superpowers/archive/plans/2026-04-28-viewer-completion.md
git mv docs/superpowers/specs/2026-04-28-viewer-completion-design.md \
       docs/superpowers/archive/specs/2026-04-28-viewer-completion-design.md
```

- [ ] **Step 7: Final `make check`, commit, push, install**

```bash
make check
git add -A
git commit -m "Pass 2.5b-4b: ship — viewer completion (long URLs, n/N, link picker)"
git push
make install
```

---

## Self-review summary

**Spec coverage:** Each spec section maps to a phase: §"Long bare URL handling" → Phase 1 (Tasks 1.1–1.3); §"`n`/`N` filtered navigation" → Phase 2 (Tasks 2.1–2.3); §"Link picker overlay" → Phase 3 (Tasks 3.1–3.7). Spec's ADR list maps to pass-end Step 3. Conventions checklist maps to Step 2.

**Type consistency:** `LinkPicker` exported, `linkPickerKeys` unexported, fields lowercase. `LinkPickerOpenMsg`/`LinkPickerClosedMsg`/`LaunchURLMsg` all exported. `linkPickerOpenCmd`/`launchURLCmd` unexported (package-internal Cmds). `MessageByUID(uid mail.UID) (mail.MessageInfo, bool)` and `MoveCursor(delta int) (mail.UID, bool)` consistent with existing msglist signatures. `Phase()` accessor on `Viewer` returns `viewerPhase` (package-internal type, fine within `internal/ui/`).

**Placeholder scan:** No "TBD"/"TODO"/"similar to". Every step shows code. Helpers that may not exist (`newTestApp`, `drainAndApply`, etc.) are flagged inline with a "define if absent" instruction.
