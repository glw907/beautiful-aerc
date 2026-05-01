# Pass 6 — Triage Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire delete/archive/star/read-toggle on the message list with optimistic mutation, a 6s toast/undo bar above the status row, visual-mode multi-select (`v`+`Space`), and `u`-undo while toast is visible.

**Architecture:** `MessageList` owns visual-mode state and local mutation helpers (`apply*`). `AccountTab` builds forward + inverse `tea.Cmd`s and emits a typed `triageStartedMsg`. `App` owns the `pendingAction` (toast), drives the timer via `tea.Tick`, and applies the inverse on `u` or on backend failure. The toast renders into the existing one-row banner slot above the status bar; the error banner wins precedence when both could appear.

**Tech Stack:** Go 1.26, bubbletea, lipgloss, `bubbles/key`, BurntSushi/toml.

**Spec:** `docs/superpowers/specs/2026-05-01-triage-actions-design.md`

**Working invariants:** Optimistic mutation (ADR-0086), foreground-only banner shape (ADR-0073), accessor-after-delegation (ADR-0088), modifier-free keys (ADR-0068), bubbletea conventions in `docs/poplar/bubbletea-conventions.md`.

---

## Task 1: Backend `MarkUnread` symmetry

**Files:**
- Modify: `internal/mail/backend.go` (add interface method near line 50, after `MarkRead`)
- Modify: `internal/mail/mock.go` (add at line 232 area, after `MarkRead`)
- Modify: `internal/mailjmap/jmap.go` (add after `MarkRead` at line 695)
- Test: `internal/mailjmap/jmap_test.go` (extend the `MarkRead` test patterns at lines 752–840)

- [ ] **Step 1: Add `MarkUnread` to the `Backend` interface**

In `internal/mail/backend.go`, add directly after the `MarkRead` line:

```go
MarkUnread(uids []UID) error
```

- [ ] **Step 2: Implement on the mock**

In `internal/mail/mock.go`, after the `MarkRead` line:

```go
func (m *MockBackend) MarkUnread(_ []UID) error              { return nil }
```

- [ ] **Step 3: Write the failing JMAP test**

In `internal/mailjmap/jmap_test.go`, after `TestMarkRead_RequestShape`, add a sibling test that asserts `MarkUnread` patches `keywords/$seen` to `nil` (unset) on every UID. Mirror the body of `TestMarkRead_RequestShape` (around line 769) but assert the patch value is `nil` instead of `true`. Also add the empty-input pair (mirror lines 758–763): `MarkUnread(nil)` and `MarkUnread([]mail.UID{})` must both return nil without making a request.

- [ ] **Step 4: Run the test to confirm it fails**

```bash
go test ./internal/mailjmap/ -run TestMarkUnread -v
```

Expected: FAIL — `b.MarkUnread undefined`.

- [ ] **Step 5: Implement `MarkUnread` on the JMAP backend**

In `internal/mailjmap/jmap.go`, after `MarkRead` (line 695):

```go
// MarkUnread satisfies mail.Backend.
func (b *Backend) MarkUnread(uids []mail.UID) error {
	return b.setKeyword(uids, "$seen", false)
}
```

`setKeyword(_, _, false)` already patches `keywords/$seen` to a `nil` value (unset), per the existing implementation at lines 732–747.

- [ ] **Step 6: Run the test to confirm it passes**

```bash
go test ./internal/mailjmap/ -run TestMarkUnread -v
```

Expected: PASS.

- [ ] **Step 7: Run full check**

```bash
make check
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/mail/backend.go internal/mail/mock.go internal/mailjmap/jmap.go internal/mailjmap/jmap_test.go
git commit -m "Add Backend.MarkUnread for read-toggle symmetry"
```

---

## Task 2: Config knob `[ui] undo_seconds`

**Files:**
- Modify: `internal/config/ui.go`
- Test: `internal/config/ui_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/ui_test.go`. If `LoadUI` is exercised via test fixture files, follow the existing pattern; otherwise add a table-driven test against an inline TOML string fed through `toml.Unmarshal` mirroring the existing tests. Cases:

```go
{name: "default when unset",         input: "[ui]\nthreading = true\n",                    want: 6},
{name: "explicit value within range", input: "[ui]\nundo_seconds = 10\n",                  want: 10},
{name: "below floor clamps to 2",     input: "[ui]\nundo_seconds = 0\n",                   want: 2},
{name: "above ceiling clamps to 30",  input: "[ui]\nundo_seconds = 99\n",                  want: 30},
{name: "negative clamps to 2",        input: "[ui]\nundo_seconds = -5\n",                  want: 2},
```

If your tests are file-based (write a temp file then `LoadUI(path)`), follow that pattern instead — same cases, same expectations.

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/config/ -run TestLoadUI_UndoSeconds -v
```

Expected: FAIL — `UndoSeconds` field does not exist.

- [ ] **Step 3: Add `UndoSeconds` to `UIConfig` and `rawUI`**

In `internal/config/ui.go`:

```go
type UIConfig struct {
	// ... existing fields
	// UndoSeconds is the toast/undo timer in seconds. Default 6,
	// clamped to [2, 30] on parse.
	UndoSeconds int
}
```

```go
type rawUI struct {
	// ... existing fields
	UndoSeconds *int `toml:"undo_seconds"`
}
```

Update `DefaultUIConfig`:

```go
return UIConfig{
	Threading:   true,
	Folders:     map[string]FolderConfig{},
	Icons:       "auto",
	UndoSeconds: 6,
}
```

In `LoadUI`, after the `Icons` block (around line 111), add:

```go
if raw.UI.UndoSeconds != nil {
	v := *raw.UI.UndoSeconds
	if v < 2 {
		v = 2
	} else if v > 30 {
		v = 30
	}
	out.UndoSeconds = v
}
```

- [ ] **Step 4: Run the test to confirm it passes**

```bash
go test ./internal/config/ -run TestLoadUI_UndoSeconds -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/ui.go internal/config/ui_test.go
git commit -m "Add [ui] undo_seconds config knob (default 6, clamp [2,30])"
```

---

## Task 3: `MessageList` visual-mode state + `actionTargets`

**Files:**
- Modify: `internal/ui/msglist.go`
- Test: `internal/ui/msglist_test.go`

- [ ] **Step 1: Write failing tests for visual-mode and `actionTargets`**

In `internal/ui/msglist_test.go`, add a new test function `TestMessageList_VisualModeAndTargets` (table-driven). Cases must cover:

1. Default: `VisualMode()` is false; `Marked()` is empty.
2. After `EnterVisual()`: `VisualMode()` is true; `Marked()` is empty.
3. After `EnterVisual()` + `ToggleMark(uid)`: `Marked()` contains `uid`.
4. `ToggleMark(uid)` twice removes it.
5. `ExitVisual()`: clears `marked` and sets `visualMode` false.
6. `ActionTargets()` with no marks → returns `[]mail.UID{cursorUID()}`.
7. `ActionTargets()` with two marks → returns those two UIDs (order: insertion order — use a slice-tracked set or sort by source order; pick **source order in `m.source`**, deterministic).
8. `ActionTargets()` from a folded thread root with N children → returns all N+1 UIDs (root + children) — the WYSIWYG expansion.
9. `ActionTargets()` from a non-folded thread root → returns only the root UID.

Use the existing test helpers in `msglist_test.go` for constructing a `MessageList` with thread fixtures (search the file for `newTestMessageList` or similar helper, mirror it).

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/ui/ -run TestMessageList_VisualModeAndTargets -v
```

Expected: FAIL — `EnterVisual`, `ToggleMark`, `ActionTargets` undefined.

- [ ] **Step 3: Add the state fields**

In `internal/ui/msglist.go`, add to the `MessageList` struct (near the other behavior fields, around lines 90–95):

```go
visualMode bool
marked     map[mail.UID]struct{}
```

Initialize in `NewMessageList` (line 105 area):

```go
m := MessageList{
	// ... existing
	marked: map[mail.UID]struct{}{},
}
```

- [ ] **Step 4: Add the methods**

Append to `internal/ui/msglist.go` (after the existing accessor methods around line 600):

```go
// VisualMode reports whether the list is in visual-select mode.
func (m MessageList) VisualMode() bool { return m.visualMode }

// EnterVisual enters visual-select mode. Marked set is unchanged.
func (m *MessageList) EnterVisual() { m.visualMode = true }

// ExitVisual leaves visual-select mode and clears the marked set.
func (m *MessageList) ExitVisual() {
	m.visualMode = false
	m.marked = map[mail.UID]struct{}{}
}

// ToggleMark flips membership of uid in the marked set.
func (m *MessageList) ToggleMark(uid mail.UID) {
	if _, ok := m.marked[uid]; ok {
		delete(m.marked, uid)
		return
	}
	m.marked[uid] = struct{}{}
}

// Marked returns the marked UIDs in source order (the order they
// appear in m.source). Returns an empty slice when nothing is marked.
func (m MessageList) Marked() []mail.UID {
	if len(m.marked) == 0 {
		return nil
	}
	out := make([]mail.UID, 0, len(m.marked))
	for _, msg := range m.source {
		if _, ok := m.marked[msg.UID]; ok {
			out = append(out, msg.UID)
		}
	}
	return out
}

// ActionTargets returns the UIDs a triage action should operate on.
// Marked-set if non-empty; otherwise the cursor UID. For a folded
// thread root, expands to root + all child UIDs (WYSIWYG).
func (m MessageList) ActionTargets() []mail.UID {
	if marked := m.Marked(); len(marked) > 0 {
		return marked
	}
	if m.selected < 0 || m.selected >= len(m.rows) {
		return nil
	}
	row := m.rows[m.selected]
	if row.isThreadRoot && row.threadSize > 1 && m.folded[row.msg.UID] {
		return m.threadUIDs(row.msg.UID)
	}
	return []mail.UID{row.msg.UID}
}

// threadUIDs returns the root UID followed by all child UIDs in
// source order. Used for WYSIWYG expansion on a folded thread root.
func (m MessageList) threadUIDs(root mail.UID) []mail.UID {
	out := []mail.UID{root}
	for _, msg := range m.source {
		if msg.ThreadID != "" && msg.UID != root {
			// Walk the source: any msg whose thread chain leads back
			// to root belongs in the bag. The cheap approximation
			// (and the one MessageList already uses for prefix
			// computation): same ThreadID as root.
			if rootMsg, ok := m.findSourceByUID(root); ok && msg.ThreadID == rootMsg.ThreadID {
				out = append(out, msg.UID)
			}
		}
	}
	return out
}

// findSourceByUID looks up a MessageInfo by UID in m.source.
func (m MessageList) findSourceByUID(uid mail.UID) (mail.MessageInfo, bool) {
	for _, msg := range m.source {
		if msg.UID == uid {
			return msg, true
		}
	}
	return mail.MessageInfo{}, false
}
```

- [ ] **Step 5: Run tests to confirm they pass**

```bash
go test ./internal/ui/ -run TestMessageList_VisualModeAndTargets -v
```

Expected: PASS.

- [ ] **Step 6: Run full vet + test**

```bash
make check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "Add MessageList visual mode + ActionTargets (mode-agnostic)"
```

---

## Task 4: `MessageList` local mutation helpers

These are the optimistic-flip primitives. None of them fires a `tea.Cmd` — pure local state mutation, used for the initial flip and (with reversed direction) for the inverse roll-back.

**Files:**
- Modify: `internal/ui/msglist.go`
- Test: `internal/ui/msglist_test.go`

- [ ] **Step 1: Write failing tests**

Add `TestMessageList_ApplyMutations` (table-driven) covering:

1. `ApplyDelete([uidA])` removes the row whose UID is `uidA` from `m.source` and rebuilds. The display row count drops by 1.
2. `ApplyDelete` on a thread root with children: removes only the root from source — children re-root via the existing fallback (covered by existing rebuild logic).
3. `ApplyDelete` on the cursor row leaves the cursor at the same index (clamped to `len(rows)-1` if it overflowed).
4. `ApplyDelete` on multiple UIDs (bulk) lands the cursor at the index of the first removed display-row before the mutation, clamped.
5. `ApplyInsert(msgs, atSourceIndex)` re-inserts messages — used by the inverse path. Order in `m.source` after insert matches the saved snapshot.
6. `ApplyFlag([uid], FlagFlagged, true)` flips `Flags` on the matching `MessageInfo`. Cursor and row count unchanged.
7. `ApplyFlag([uid], FlagFlagged, false)` clears the flag.
8. `ApplySeen([uid], false)` clears `FlagSeen` on the matching `MessageInfo`. (`ApplySeen([uid], true)` sets it.)

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/ui/ -run TestMessageList_ApplyMutations -v
```

Expected: FAIL — undefined methods.

- [ ] **Step 3: Implement the helpers**

Append to `internal/ui/msglist.go`. The cursor-placement logic captures the first-removed-display-index *before* the rebuild:

```go
// ApplyDelete removes uids from m.source and rebuilds rows. Cursor
// holds at the index of the first display-row that was removed,
// clamped to len(rows)-1; if the resulting list is empty, selected
// becomes 0.
func (m *MessageList) ApplyDelete(uids []mail.UID) {
	if len(uids) == 0 {
		return
	}
	rmSet := make(map[mail.UID]struct{}, len(uids))
	for _, u := range uids {
		rmSet[u] = struct{}{}
	}

	// Find the index of the first display-row that will be removed.
	firstIdx := -1
	for i, r := range m.rows {
		if _, ok := rmSet[r.msg.UID]; ok {
			firstIdx = i
			break
		}
	}

	// Filter source.
	kept := m.source[:0:0]
	for _, msg := range m.source {
		if _, ok := rmSet[msg.UID]; !ok {
			kept = append(kept, msg)
		}
	}
	m.source = kept
	m.rebuild()

	// Place cursor.
	switch {
	case len(m.rows) == 0:
		m.selected = 0
	case firstIdx < 0:
		// Nothing was visible; leave cursor where it was, clamped.
		if m.selected >= len(m.rows) {
			m.selected = len(m.rows) - 1
		}
	default:
		if firstIdx >= len(m.rows) {
			firstIdx = len(m.rows) - 1
		}
		m.selected = firstIdx
	}
}

// ApplyInsert re-inserts msgs into m.source preserving their original
// positions. positions[i] is the source-index where msgs[i] originally
// lived; positions must be sorted ascending. Used for inverse roll-back.
func (m *MessageList) ApplyInsert(msgs []mail.MessageInfo, positions []int) {
	if len(msgs) == 0 {
		return
	}
	// Walk source and msgs together, splicing as we go.
	out := make([]mail.MessageInfo, 0, len(m.source)+len(msgs))
	mi := 0
	for si := 0; si <= len(m.source); si++ {
		for mi < len(msgs) && positions[mi] == si {
			out = append(out, msgs[mi])
			mi++
		}
		if si < len(m.source) {
			out = append(out, m.source[si])
		}
	}
	m.source = out
	m.rebuild()
}

// ApplyFlag flips a flag on every msg in m.source whose UID is in uids.
func (m *MessageList) ApplyFlag(uids []mail.UID, flag mail.Flag, set bool) {
	uidSet := make(map[mail.UID]struct{}, len(uids))
	for _, u := range uids {
		uidSet[u] = struct{}{}
	}
	for i := range m.source {
		if _, ok := uidSet[m.source[i].UID]; !ok {
			continue
		}
		if set {
			m.source[i].Flags |= flag
		} else {
			m.source[i].Flags &^= flag
		}
	}
	m.rebuild()
}

// ApplySeen is the read/unread shorthand. set=true marks read; false
// marks unread.
func (m *MessageList) ApplySeen(uids []mail.UID, seen bool) {
	m.ApplyFlag(uids, mail.FlagSeen, seen)
}

// SnapshotSource returns the messages whose UIDs are in uids, paired
// with their indexes in m.source. Used to build the inverse Cmd
// before an ApplyDelete. positions is sorted ascending.
func (m MessageList) SnapshotSource(uids []mail.UID) (msgs []mail.MessageInfo, positions []int) {
	uidSet := make(map[mail.UID]struct{}, len(uids))
	for _, u := range uids {
		uidSet[u] = struct{}{}
	}
	for i, msg := range m.source {
		if _, ok := uidSet[msg.UID]; ok {
			msgs = append(msgs, msg)
			positions = append(positions, i)
		}
	}
	return msgs, positions
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/ui/ -run TestMessageList_ApplyMutations -v
```

Expected: PASS.

- [ ] **Step 5: Run full check**

```bash
make check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "Add MessageList Apply* mutation helpers (optimistic flip)"
```

---

## Task 5: Toast renderer

**Files:**
- Create: `internal/ui/toast.go`
- Create: `internal/ui/toast_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/toast_test.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderToast(t *testing.T) {
	styles := DefaultStyles(testTheme())
	cases := []struct {
		name     string
		p        pendingAction
		width    int
		wantSubs []string // substrings that must appear in the output
		empty    bool
	}{
		{name: "zero pending → empty", p: pendingAction{}, width: 80, empty: true},
		{name: "delete one", p: pendingAction{op: "delete", n: 1}, width: 80, wantSubs: []string{"Deleted 1 message", "u undo"}},
		{name: "delete many", p: pendingAction{op: "delete", n: 3}, width: 80, wantSubs: []string{"Deleted 3 messages", "u undo"}},
		{name: "archive one", p: pendingAction{op: "archive", n: 1}, width: 80, wantSubs: []string{"Archived 1 message"}},
		{name: "star", p: pendingAction{op: "star", n: 1}, width: 80, wantSubs: []string{"Starred"}},
		{name: "unstar", p: pendingAction{op: "unstar", n: 2}, width: 80, wantSubs: []string{"Unstarred 2"}},
		{name: "read", p: pendingAction{op: "read", n: 1}, width: 80, wantSubs: []string{"Marked read"}},
		{name: "unread", p: pendingAction{op: "unread", n: 1}, width: 80, wantSubs: []string{"Marked unread"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderToast(tc.p, tc.width, styles)
			if tc.empty {
				if got != "" {
					t.Fatalf("want empty, got %q", got)
				}
				return
			}
			for _, sub := range tc.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("output %q missing %q", got, sub)
				}
			}
			if w := lipgloss.Width(got); w > tc.width {
				t.Errorf("width %d exceeds %d", w, tc.width)
			}
		})
	}
}

func TestRenderToast_Truncation(t *testing.T) {
	styles := DefaultStyles(testTheme())
	got := renderToast(pendingAction{op: "delete", n: 999}, 12, styles)
	if w := lipgloss.Width(got); w > 12 {
		t.Errorf("truncated width %d > 12", w)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis in %q", got)
	}
}
```

If `testTheme()` doesn't already exist as a helper, mirror the pattern used in `error_banner_test.go` (look for how it constructs a `Styles`).

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/ui/ -run TestRenderToast -v
```

Expected: FAIL — `pendingAction` undefined, `renderToast` undefined.

- [ ] **Step 3: Create the toast renderer**

Create `internal/ui/toast.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	tea "github.com/charmbracelet/bubbletea"
)

// pendingAction is the App-owned state for an in-flight optimistic
// triage action. The zero value means "no toast active".
type pendingAction struct {
	op       string    // "delete" | "archive" | "star" | "unstar" | "read" | "unread"
	n        int       // affected message count
	inverse  tea.Cmd   // the undo Cmd; nil for unrecoverable ops
	deadline time.Time // monotonic moment at which the toast expires
	// onUndo runs alongside firing inverse: applies the local
	// state roll-back on MessageList. App calls it on `u` before
	// firing inverse, and on ErrorMsg after the optimistic flip.
	onUndo func()
	// uids snapshots the UIDs the action was applied to. Used by
	// commit-on-folder-change and for tests to assert.
	uids []mail.UID
}

// IsZero reports whether p represents "no active toast".
func (p pendingAction) IsZero() bool {
	return p.op == "" && p.n == 0 && p.inverse == nil && p.deadline.IsZero()
}

// renderToast produces the one-row toast string. Returns "" for the
// zero pendingAction. Width-bounded; truncates with ellipsis.
func renderToast(p pendingAction, width int, styles Styles) string {
	if p.IsZero() {
		return ""
	}
	verb := toastVerb(p.op)
	var body string
	switch p.op {
	case "star", "unstar", "read", "unread":
		// "Starred" / "Marked read" — verb already complete; n suffix
		// only shown when > 1.
		if p.n > 1 {
			body = fmt.Sprintf("%s %d", verb, p.n)
		} else {
			body = verb
		}
	default:
		// Delete / archive — "Deleted N message[s]"
		body = fmt.Sprintf("%s %d %s", verb, p.n, pluralize("message", p.n))
	}
	hint := "[u undo]"
	full := "✓ " + body + "   " + hint
	if lipgloss.Width(full) <= width {
		return styles.Toast.Render(full)
	}
	// Truncate body, keep hint visible if at all possible.
	hintW := lipgloss.Width(hint)
	bodyBudget := width - hintW - 4 // "✓ " + "   "
	if bodyBudget < 1 {
		return styles.Toast.Render(truncateToWidth(full, width))
	}
	bodyTrunc := truncateToWidth("✓ "+body, bodyBudget+2)
	return styles.Toast.Render(bodyTrunc + "   " + hint)
}

func toastVerb(op string) string {
	switch op {
	case "delete":
		return "Deleted"
	case "archive":
		return "Archived"
	case "star":
		return "Starred"
	case "unstar":
		return "Unstarred"
	case "read":
		return "Marked read"
	case "unread":
		return "Marked unread"
	}
	return op
}

func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
```

- [ ] **Step 4: Add a `Toast` style to `Styles`**

In `internal/ui/styles.go`, add `Toast lipgloss.Style` to the `Styles` struct, and in `DefaultStyles` define it (use `FgDim` foreground; mirror how `ErrorBanner` is defined). The `[u undo]` substring will inherit the style — fine for v1; richer keystyling can land in a polish pass.

- [ ] **Step 5: Run tests to confirm they pass**

```bash
go test ./internal/ui/ -run TestRenderToast -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/toast.go internal/ui/toast_test.go internal/ui/styles.go
git commit -m "Add toast renderer for triage undo bar"
```

---

## Task 6: New `Msg` types in `cmds.go`

**Files:**
- Modify: `internal/ui/cmds.go`

- [ ] **Step 1: Add the message types**

Append to `internal/ui/cmds.go`:

```go
// triageStartedMsg is emitted by AccountTab after an optimistic
// triage flip. App receives it, sets the toast, and schedules a
// tea.Tick for the undo timer. inverse runs on `u` or on an
// ErrorMsg rollback. uids snapshots the affected message IDs.
type triageStartedMsg struct {
	op      string
	n       int
	uids    []mail.UID
	inverse tea.Cmd
	onUndo  func()
}

// toastExpireMsg fires when the undo timer elapses. App ignores it
// if deadline does not match the active toast (stale tick).
type toastExpireMsg struct {
	deadline time.Time
}

// undoRequestedMsg is emitted when the user presses `u` while a
// toast is active. App applies the local roll-back and fires the
// inverse Cmd.
type undoRequestedMsg struct{}
```

Imports: `time` is already used elsewhere; add it if missing.

- [ ] **Step 2: Run vet to confirm it compiles**

```bash
go vet ./internal/ui/
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/cmds.go
git commit -m "Add Msg types for triage/toast/undo flow"
```

---

## Task 7: `App` toast state and lifecycle handlers

**Files:**
- Modify: `internal/ui/app.go`
- Test: `internal/ui/app_test.go`

- [ ] **Step 1: Write failing tests**

Add `TestApp_ToastLifecycle` covering:

1. `Update(triageStartedMsg{op:"delete", n:1, ...})` sets `app.toast` non-zero, returns a non-nil Cmd (the Tick + the forward Cmd batched).
2. `Update(toastExpireMsg{deadline: app.toast.deadline})` clears the toast.
3. `Update(toastExpireMsg{deadline: somePastTime})` is ignored — toast unchanged.
4. `Update(undoRequestedMsg{})` while toast is set: invokes `onUndo`, returns the inverse Cmd, clears toast.
5. `Update(undoRequestedMsg{})` while toast is zero: no-op.
6. `Update(ErrorMsg{Op:"delete", Err: someErr})` while toast is set: invokes `onUndo` (rollback), clears toast, sets `lastErr`.
7. `Update(ErrorMsg{...})` with no toast: just sets `lastErr`.

Use a fake `tea.Cmd` and a `bool` closure to assert `onUndo` ran. Use a fixed clock by injecting `app.now = func() time.Time { return ... }` if `App` doesn't already have a clock-injection seam — see Step 3.

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/ui/ -run TestApp_ToastLifecycle -v
```

Expected: FAIL — `app.toast` undefined.

- [ ] **Step 3: Add toast state to `App`**

In `internal/ui/app.go`, add to the `App` struct (near `lastErr`):

```go
toast       pendingAction
undoSeconds int
// now returns the wall clock; test seam, defaults to time.Now.
now func() time.Time
```

Update the constructor `NewApp` to wire `undoSeconds` from the config (passed in by `cmd/poplar/root.go`) and initialize `now: time.Now`. If the constructor doesn't currently take a `UIConfig` reference, thread one through (mirror how the icon mode is threaded — see existing `IconSet` parameter).

- [ ] **Step 4: Add the handlers in `App.Update`**

Locate the existing `Update` switch (around line 100) and add:

```go
case triageStartedMsg:
	deadline := a.now().Add(time.Duration(a.undoSeconds) * time.Second)
	a.toast = pendingAction{
		op:       msg.op,
		n:        msg.n,
		uids:     msg.uids,
		inverse:  msg.inverse,
		onUndo:   msg.onUndo,
		deadline: deadline,
	}
	return a, tea.Tick(time.Until(deadline), func(time.Time) tea.Msg {
		return toastExpireMsg{deadline: deadline}
	})

case toastExpireMsg:
	if !a.toast.IsZero() && msg.deadline.Equal(a.toast.deadline) {
		a.toast = pendingAction{}
	}
	return a, nil

case undoRequestedMsg:
	if a.toast.IsZero() {
		return a, nil
	}
	if a.toast.onUndo != nil {
		a.toast.onUndo()
	}
	cmd := a.toast.inverse
	a.toast = pendingAction{}
	return a, cmd
```

In the existing `case ErrorMsg:` block, **before** assigning `lastErr`, roll back any pending toast:

```go
case ErrorMsg:
	if !a.toast.IsZero() && a.toast.onUndo != nil {
		a.toast.onUndo()
	}
	a.toast = pendingAction{}
	a.lastErr = msg
	return a, nil
```

(If the existing `ErrorMsg` handler does more than that, preserve its behavior and add the toast rollback at the top.)

- [ ] **Step 5: Run tests to confirm they pass**

```bash
go test ./internal/ui/ -run TestApp_ToastLifecycle -v
```

Expected: PASS.

- [ ] **Step 6: Run full check**

```bash
make check
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go cmd/poplar/root.go
git commit -m "Add App pendingAction lifecycle (start/expire/undo/error)"
```

---

## Task 8: `App.View` — toast row + banner-wins precedence

**Files:**
- Modify: `internal/ui/app.go`
- Test: `internal/ui/app_test.go`

- [ ] **Step 1: Write failing test**

Add `TestApp_BannerToastPrecedence`. Cases:

1. `lastErr.Err == nil`, `toast.IsZero()` → row is "" (not rendered; account region uses full height).
2. `lastErr.Err == nil`, toast active → row contains "u undo".
3. `lastErr.Err != nil`, toast active → row contains "⚠"; does **not** contain "u undo".
4. `lastErr.Err != nil`, toast zero → row contains "⚠".

A direct way: render `app.View()` and grep, but `App.View()` includes everything. Easier: extract the row composition into a small helper `app.chromeRow(width int) string` and assert against that. If you'd rather not refactor, render the full view and grep for the substrings against tightly-controlled inputs.

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/ui/ -run TestApp_BannerToastPrecedence -v
```

Expected: FAIL — toast row absent or wrong precedence.

- [ ] **Step 3: Add precedence logic to `App.View`**

Locate where the error banner is currently composed inside `App.View` (search for `renderErrorBanner`). Replace the single-banner call with:

```go
// Precedence: error banner wins; otherwise toast; otherwise empty.
var bannerRow string
switch {
case a.lastErr.Err != nil:
	bannerRow = renderErrorBanner(a.lastErr, contentWidth, a.styles)
case !a.toast.IsZero():
	bannerRow = renderToast(a.toast, contentWidth, a.styles)
}
```

Where `bannerRow` is then composed into the frame in the same row slot the error banner occupies today. The "row collapses when empty" behavior already exists for the error banner — reuse it unchanged.

- [ ] **Step 4: Run test to confirm pass**

```bash
go test ./internal/ui/ -run TestApp_BannerToastPrecedence -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "Render toast in chrome row with banner-wins precedence"
```

---

## Task 9: `AccountTab` triage Cmd construction (delete + archive)

**Files:**
- Modify: `internal/ui/account_tab.go`
- Test: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Write failing tests**

Add `TestAccountTab_Triage_DeleteArchive` covering:

1. `dispatchTriage(opDelete)` on cursor row → MessageList row count drops by 1 (optimistic) and a `triageStartedMsg` is emitted with `op:"delete"`, `n:1`, `uids:[cursorUID]`. The forward Cmd, when invoked, calls `mock.Delete([cursorUID])`.
2. The inverse Cmd, when invoked, calls `mock.Move([cursorUID], sourceFolder)`.
3. Calling the returned `onUndo` re-inserts the message into MessageList at the original source-index.
4. `dispatchTriage(opArchive)` on cursor row → forward Cmd calls `mock.Move([uid], "Archive")` (or whatever the classifier returns); inverse Cmd calls `mock.Move` back to the source folder.
5. Archive without an Archive folder classified → no mutation; an `ErrorMsg{Op:"archive"}` is emitted instead of `triageStartedMsg`.
6. Visual mode auto-exits after a triage action.

Use a `mail.MockBackend` and assert via the recorded calls — extend `MockBackend` with simple call-recording slices if it doesn't already have them.

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/ui/ -run TestAccountTab_Triage_DeleteArchive -v
```

Expected: FAIL — `dispatchTriage` undefined.

- [ ] **Step 3: Add `dispatchTriage` to AccountTab**

In `internal/ui/account_tab.go`, add an unexported method `dispatchTriage(op string) (tea.Cmd)`. The op string is one of `"delete"`, `"archive"`, `"star"`, `"unstar"`, `"read"`, `"unread"`.

```go
// dispatchTriage performs an optimistic triage action: snapshots
// state for inverse, mutates MessageList locally, exits visual
// mode, and returns a Cmd that emits triageStartedMsg + the
// forward backend Cmd. The caller composes this with any other
// Cmds (e.g. status bar updates) via tea.Batch.
func (a *AccountTab) dispatchTriage(op string) tea.Cmd {
	uids := a.list.ActionTargets()
	if len(uids) == 0 {
		return nil
	}
	srcFolder := a.currentFolder

	switch op {
	case "delete":
		return a.dispatchRemoval(op, "delete", uids, srcFolder, func() error {
			return a.backend.Delete(uids)
		}, func() error {
			return a.backend.Move(uids, srcFolder)
		})

	case "archive":
		archive, ok := a.classifiedArchiveFolder()
		if !ok {
			return func() tea.Msg {
				return ErrorMsg{Op: "archive", Err: errors.New("no Archive folder configured")}
			}
		}
		return a.dispatchRemoval(op, "archive", uids, srcFolder, func() error {
			return a.backend.Move(uids, archive)
		}, func() error {
			return a.backend.Move(uids, srcFolder)
		})
	}
	return nil
}

// dispatchRemoval factors the optimistic-flip / inverse-snapshot /
// triageStartedMsg emission shared by delete and archive.
func (a *AccountTab) dispatchRemoval(op, errOp string, uids []mail.UID, srcFolder string, fwd, rev func() error) tea.Cmd {
	snapshot, positions := a.list.SnapshotSource(uids)
	a.list.ApplyDelete(uids)
	a.list.ExitVisual()

	onUndo := func() {
		a.list.ApplyInsert(snapshot, positions)
	}
	forward := func() tea.Msg {
		if err := fwd(); err != nil {
			return ErrorMsg{Op: errOp, Err: err}
		}
		return nil
	}
	inverse := func() tea.Msg {
		if err := rev(); err != nil {
			return ErrorMsg{Op: errOp + " undo", Err: err}
		}
		return nil
	}
	start := func() tea.Msg {
		return triageStartedMsg{
			op:      op,
			n:       len(uids),
			uids:    uids,
			inverse: inverse,
			onUndo:  onUndo,
		}
	}
	return tea.Batch(start, forward)
}

// classifiedArchiveFolder returns the canonical Archive folder name
// from the current classification, or ("", false) if absent.
func (a *AccountTab) classifiedArchiveFolder() (string, bool) {
	for _, f := range a.classified {
		if f.Canonical == "Archive" {
			return f.Folder.Name, true
		}
	}
	return "", false
}
```

(If the field/accessor names differ — `a.classified`, `a.currentFolder`, `a.backend` — adjust to match the existing `AccountTab` shape.)

- [ ] **Step 4: Run tests to confirm pass**

```bash
go test ./internal/ui/ -run TestAccountTab_Triage_DeleteArchive -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/account_tab.go internal/ui/account_tab_test.go
git commit -m "Add AccountTab.dispatchTriage for delete/archive"
```

---

## Task 10: `AccountTab` triage Cmd construction (star + read-toggle)

**Files:**
- Modify: `internal/ui/account_tab.go`
- Test: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Write failing tests**

Add `TestAccountTab_Triage_StarReadToggle`. Cases:

1. Cursor on an unstarred row, `dispatchTriage("star")` → row's `FlagFlagged` is set; forward Cmd calls `mock.Flag([uid], FlagFlagged, true)`; inverse calls `mock.Flag([uid], FlagFlagged, false)`. Toast op is `"star"`.
2. Cursor on a starred row, `dispatchTriage("star")` → flag is *cleared*; forward calls `Flag(_, _, false)`; toast op is `"unstar"`.
3. Mixed selection (one starred, one unstarred): the action sets the dominant flag based on the cursor row. (Document this as the policy: cursor row decides direction for bulk star.)
4. `dispatchTriage("read")` on an unread row → `FlagSeen` set; forward calls `MarkRead([uid])`; inverse calls `MarkUnread([uid])`; toast op `"read"`.
5. `dispatchTriage("read")` on a read row → `FlagSeen` cleared; forward calls `MarkUnread([uid])`; inverse `MarkRead([uid])`; toast op `"unread"`.

- [ ] **Step 2: Run tests to confirm failure**

```bash
go test ./internal/ui/ -run TestAccountTab_Triage_StarReadToggle -v
```

Expected: FAIL.

- [ ] **Step 3: Extend `dispatchTriage`**

In `internal/ui/account_tab.go`, add the missing arms to the switch:

```go
case "star":
	cursor, ok := a.list.SelectedMessage()
	if !ok {
		return nil
	}
	set := cursor.Flags&mail.FlagFlagged == 0 // toggle direction from cursor row
	op := "star"
	if !set {
		op = "unstar"
	}
	return a.dispatchFlagToggle(op, uids, mail.FlagFlagged, set)

case "read":
	cursor, ok := a.list.SelectedMessage()
	if !ok {
		return nil
	}
	set := cursor.Flags&mail.FlagSeen == 0 // unread → mark read
	op := "read"
	if !set {
		op = "unread"
	}
	return a.dispatchSeenToggle(op, uids, set)
}
```

And add the two helpers:

```go
func (a *AccountTab) dispatchFlagToggle(op string, uids []mail.UID, flag mail.Flag, set bool) tea.Cmd {
	a.list.ApplyFlag(uids, flag, set)
	a.list.ExitVisual()

	onUndo := func() { a.list.ApplyFlag(uids, flag, !set) }
	forward := func() tea.Msg {
		if err := a.backend.Flag(uids, flag, set); err != nil {
			return ErrorMsg{Op: op, Err: err}
		}
		return nil
	}
	inverse := func() tea.Msg {
		if err := a.backend.Flag(uids, flag, !set); err != nil {
			return ErrorMsg{Op: op + " undo", Err: err}
		}
		return nil
	}
	start := func() tea.Msg {
		return triageStartedMsg{op: op, n: len(uids), uids: uids, inverse: inverse, onUndo: onUndo}
	}
	return tea.Batch(start, forward)
}

func (a *AccountTab) dispatchSeenToggle(op string, uids []mail.UID, seen bool) tea.Cmd {
	a.list.ApplySeen(uids, seen)
	a.list.ExitVisual()

	fwdFn := a.backend.MarkRead
	revFn := a.backend.MarkUnread
	if !seen {
		fwdFn, revFn = a.backend.MarkUnread, a.backend.MarkRead
	}
	onUndo := func() { a.list.ApplySeen(uids, !seen) }
	forward := func() tea.Msg {
		if err := fwdFn(uids); err != nil {
			return ErrorMsg{Op: op, Err: err}
		}
		return nil
	}
	inverse := func() tea.Msg {
		if err := revFn(uids); err != nil {
			return ErrorMsg{Op: op + " undo", Err: err}
		}
		return nil
	}
	start := func() tea.Msg {
		return triageStartedMsg{op: op, n: len(uids), uids: uids, inverse: inverse, onUndo: onUndo}
	}
	return tea.Batch(start, forward)
}
```

- [ ] **Step 4: Run tests to confirm pass**

```bash
go test ./internal/ui/ -run TestAccountTab_Triage_StarReadToggle -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/account_tab.go internal/ui/account_tab_test.go
git commit -m "Add AccountTab dispatchTriage arms for star/read toggles"
```

---

## Task 11: Key bindings + `Update` wiring

**Files:**
- Modify: `internal/ui/keys.go`
- Modify: `internal/ui/account_tab.go`
- Modify: `internal/ui/msglist.go`
- Modify: `internal/ui/app.go`
- Test: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Write failing tests**

Add `TestAccountTab_TriageKeys`. For each key (`d`, `a`, `s`, `.`), assert that pressing it routes through `dispatchTriage` with the right op. Also:

- `v` enters visual mode (state flag flips, no Cmd emitted).
- `Space` while `visualMode == true` toggles the cursor row's mark; while `visualMode == false`, fold-toggle behavior is unchanged.
- `u` while a toast is set emits `undoRequestedMsg`; with no toast, it's a no-op.

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/ui/ -run TestAccountTab_TriageKeys -v
```

Expected: FAIL.

- [ ] **Step 3: Add key bindings**

In `internal/ui/keys.go`, add to the account `KeyMap` (near the other triage-adjacent bindings around line 56):

```go
Delete:       key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
Archive:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
Star:         key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "star")),
ReadToggle:   key.NewBinding(key.WithKeys("."), key.WithHelp(".", "read")),
EnterVisual:  key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "select")),
Undo:         key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
```

Add the corresponding fields to the `KeyMap` struct.

- [ ] **Step 4: Wire `d`/`a`/`s`/`.` in `AccountTab.Update`**

In `account_tab.go`'s key switch, add:

```go
case key.Matches(msg, a.keys.Delete):
	return a, a.dispatchTriage("delete")
case key.Matches(msg, a.keys.Archive):
	return a, a.dispatchTriage("archive")
case key.Matches(msg, a.keys.Star):
	return a, a.dispatchTriage("star")
case key.Matches(msg, a.keys.ReadToggle):
	return a, a.dispatchTriage("read")
case key.Matches(msg, a.keys.EnterVisual):
	a.list.EnterVisual()
	return a, nil
```

- [ ] **Step 5: Wire `Space` dual-meaning in `MessageList.Update`**

In `msglist.go`'s key handling, replace the existing `Space` arm with:

```go
case key.Matches(msg, m.keys.ToggleFold):
	if m.visualMode {
		if uid, ok := m.selectedUID(); ok {
			m.ToggleMark(uid)
		}
		return m, nil
	}
	// existing fold-toggle behavior
	...
```

(`selectedUID` already exists at line 612; use it or `SelectedMessage().UID`.)

- [ ] **Step 6: Wire `u` in `App.Update`**

In `app.go`, before the existing key dispatch into children, add:

```go
case key.Matches(msg, a.keys.Undo):
	if a.toast.IsZero() {
		return a, nil
	}
	return a, func() tea.Msg { return undoRequestedMsg{} }
```

The `Undo` binding lives on the App-level KeyMap (or wherever App's quit/help bindings live — mirror that).

- [ ] **Step 7: Run tests to confirm pass**

```bash
go test ./internal/ui/ -run TestAccountTab_TriageKeys -v
make check
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/keys.go internal/ui/account_tab.go internal/ui/msglist.go internal/ui/app.go internal/ui/account_tab_test.go
git commit -m "Wire triage/visual/undo keys into Update tree"
```

---

## Task 12: WYSIWYG verification on folded threads

This task validates Task 3's WYSIWYG `ActionTargets` expansion end-to-end through the dispatch path.

**Files:**
- Test: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Write the test**

Add `TestAccountTab_TriageOnFoldedThread`:

1. Construct a MessageList with one 3-message thread (root `T1` + 2 children).
2. Fold the thread (`m.ToggleFold(rootUID)` or equivalent — find the existing public method).
3. Cursor is on the folded root.
4. `dispatchTriage("delete")` — assert all 3 UIDs are passed to `mock.Delete` (forward Cmd) and that `triageStartedMsg.n == 3`, `uids == [root, child1, child2]`.
5. Same for `dispatchTriage("star")` — `mock.Flag` called with all 3 UIDs.
6. After undo: all 3 messages reappear in the list.

- [ ] **Step 2: Run the test**

```bash
go test ./internal/ui/ -run TestAccountTab_TriageOnFoldedThread -v
```

Expected: PASS without further code change (Task 3's `ActionTargets` already returns expanded UIDs; Task 9–10 already pass that slice through).

If it fails, the fix is in `ActionTargets` — verify the folded-thread branch.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/account_tab_test.go
git commit -m "Verify triage WYSIWYG on folded threads end-to-end"
```

---

## Task 13: Commit-on-folder-change

**Files:**
- Modify: `internal/ui/app.go`
- Test: `internal/ui/app_test.go`

- [ ] **Step 1: Write the failing test**

Add `TestApp_FolderChangeCommitsToast`:

1. Set `app.toast` to a non-zero pendingAction.
2. Send a folder-change Msg (whatever Msg the existing folder-jump path emits — e.g. `folderQueryDoneMsg{reset:true}`).
3. Assert `app.toast.IsZero()` afterwards. (The inverse must NOT have run — folder change *commits*, doesn't undo.)

Also add: pressing a folder-jump key (`I`/`D`/etc.) when a toast is up commits the toast.

- [ ] **Step 2: Run test to confirm failure**

```bash
go test ./internal/ui/ -run TestApp_FolderChangeCommitsToast -v
```

Expected: FAIL.

- [ ] **Step 3: Commit-on-folder-change**

In `App.Update`, find the case that handles folder-load completion (likely `folderQueryDoneMsg` with `reset: true`) and add at the top:

```go
case folderQueryDoneMsg:
	if msg.reset && !a.toast.IsZero() {
		a.toast = pendingAction{}
		// onUndo is NOT invoked — folder change commits the action.
	}
	// existing handling...
```

(If `folderQueryDoneMsg` is the wrong Msg, find whatever fires when `currentFolder` changes — `currentFolder` is set in `AccountTab` and may emit a different Msg up to App.)

- [ ] **Step 4: Run test to confirm pass**

```bash
go test ./internal/ui/ -run TestApp_FolderChangeCommitsToast -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "Commit pending toast on folder change"
```

---

## Task 14: Footer + help popover wired flags

**Files:**
- Modify: `internal/ui/footer.go` (or wherever `wired` flags live for the footer)
- Modify: `internal/ui/help_popover.go`

- [ ] **Step 1: Flip wired flags**

In the help popover's binding tables (search `help_popover.go` for `wired:`), set `wired: true` on the rows for `d`, `a`, `s`, `.`, `v`, `Space (select)`, `u`. Leave `r`/`R`/`f`/`c`/`m` (compose/reply/move) `wired: false`.

In the footer rendering (search for the `. read` aspirational hint), no code change is required if the footer already uses the same `wired` flag indirection. If the footer hard-codes dim-vs-bright, flip the styling for the four triage hints + `v select` to bright.

- [ ] **Step 2: Verify visually**

```bash
make install
poplar
```

Confirm that the help popover (`?`) renders `d`, `a`, `s`, `.`, `v`, `u`, `Space` as bright/wired rows. Confirm the footer's `d del a archive s star . read v select` block renders bright instead of dim.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/footer.go internal/ui/help_popover.go
git commit -m "Mark triage + visual + undo bindings as wired in help/footer"
```

---

## Task 15: Live tmux verification

**Files:**
- (no code change; verification step per `bubbletea-conventions` §10)

- [ ] **Step 1: Capture at 120×40**

Per `.claude/docs/tmux-testing.md`:

1. Launch poplar in a 120×40 tmux pane.
2. Press `j` a few times, then `d`. Expect: row vanishes optimistically, toast row reads `✓ Deleted 1 message   [u undo]`.
3. Press `u`. Expect: row reappears at the same index, toast clears.
4. Press `d` again, then `j`/`k`. Expect: cursor moves but toast persists.
5. Wait 6 seconds. Expect: toast clears silently; row stays gone.
6. Enter visual mode (`v`), Space-mark two rows, press `d`. Expect: both rows vanish, toast reads `✓ Deleted 2 messages`, mode exits.
7. Press `s` on a row, then `s` again to verify star toggle. Toast op alternates between "Starred" / "Unstarred".
8. Force a backend error (drop the network or use a flag on the mock if available). Press `d`. Expect: row vanishes, toast appears, then ErrorMsg fires → row restored, toast cleared, banner shows `⚠ delete: ...`.

- [ ] **Step 2: Capture at minimum viable width**

Repeat at the narrowest width the chrome supports (look up the existing minimum in `app.go` — likely 80 cols). Confirm the toast truncates with ellipsis and the `[u undo]` tail remains visible.

- [ ] **Step 3: Save the captures**

If the capture path follows ADR-0084's matrix-style convention, drop them in `docs/poplar/testing/triage/` (or the existing screenshot home). Otherwise, drop them in the conversation as evidence.

- [ ] **Step 4: Final check**

```bash
make check && make install
```

Expected: PASS, binary installed.

---

## Self-review notes (built into the plan)

- **Spec coverage.** D1 (timer + commit triggers) → Tasks 7, 13. D2 (mode-agnostic scope, auto-exit) → Tasks 3, 9–11. D3 (cursor placement) → Task 4. D4 (WYSIWYG) → Tasks 3, 12. D5 (delete = move to Trash) → Task 9 (delete uses `Backend.Delete`, the existing soft-delete; inverse is `Move` from Trash). D6 (`MarkUnread`) → Task 1. D7 (`undo_seconds`) → Task 2. D8 (banner-wins) → Task 8.
- **Task ordering.** 1–6 are pure-additive scaffolding (no behavior change). 7–8 give App a working toast. 9–10 wire triage Cmds. 11 connects keys. 12 validates WYSIWYG. 13 closes the commit-trigger surface. 14–15 finalize chrome + verification.
- **Type consistency.** `pendingAction` is shared by `app.go` and `toast.go`; defined in `toast.go` (Task 5). `triageStartedMsg`, `toastExpireMsg`, `undoRequestedMsg` defined in `cmds.go` (Task 6). Method names: `EnterVisual`, `ExitVisual`, `ToggleMark`, `ActionTargets`, `ApplyDelete`, `ApplyInsert`, `ApplyFlag`, `ApplySeen`, `SnapshotSource`, `dispatchTriage` — all referenced consistently across tasks.
- **Pass-end ritual is not a task.** The eight ADRs listed in the spec, the invariants update, plan archival, and `make install` all live in the `poplar-pass` skill's consolidation ritual — run that on completion.
