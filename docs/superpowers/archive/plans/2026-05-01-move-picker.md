# Move-to-Folder Picker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Pass 6.5 — modal picker triggered by `m`, type-to-filter folder list, optimistic move with toast + undo.

**Architecture:** New `MovePicker` modal owned by `App`, mirroring the LinkPicker overlay shape (centerOverlay + DimANSI + PlaceOverlay). Trigger lives in `AccountTab.Update`; dispatch reuses `buildTriageCmd` for forward/inverse/onUndo Cmds. Toast layer extended to render the destination folder.

**Tech Stack:** Go, charmbracelet/bubbletea, charmbracelet/bubbles/key, charmbracelet/lipgloss.

**Spec:** `docs/superpowers/specs/2026-05-01-move-picker-design.md`.

**Conventions:**
- Invoke `go-conventions` skill before writing any Go.
- Invoke `elm-conventions` skill before any `internal/ui/` file. Bubbles analogue: **none** (custom; documented in spec).
- All test code lives next to source (`_test.go`).
- Do not use assertion libraries; standard library only.
- After every code task, run `make check` (`go vet` + `go test ./...`).
- After all tasks complete, run the `poplar-pass` skill end-of-pass ritual (ADRs, invariants update, STATUS update, plan archival, commit + push + install).

---

## File Structure

**Create:**
- `internal/ui/movepicker.go` — `MovePicker` model, Update, View, Box, Position; `movePickerKeys`.
- `internal/ui/movepicker_test.go` — picker behavior tests.

**Modify:**
- `internal/ui/sidebar.go` — add `FolderGroup` enum, `FolderEntry` type, `OrderedFolders()` method.
- `internal/ui/sidebar_test.go` — test `OrderedFolders()`.
- `internal/ui/cmds.go` — add `OpenMovePickerMsg`, `MovePickerPickedMsg`, `MovePickerClosedMsg`; extend `triageStartedMsg` with `dest string`.
- `internal/ui/toast.go` — extend `pendingAction` with `dest string`; add "move" branch in `renderToast` + `toastVerb`.
- `internal/ui/toast_test.go` — test "moved N to <dest>" rendering.
- `internal/ui/keys.go` — add `Move` key.Binding to `AccountKeys`.
- `internal/ui/account_tab.go` — handle `m` key; handle `MovePickerPickedMsg`.
- `internal/ui/account_tab_test.go` — test `m` emits OpenMovePickerMsg, picked msg dispatches.
- `internal/ui/app.go` — hold `movePicker MovePicker`; route keys while open; handle Open/Picked/Closed msgs; composite overlay.
- `internal/ui/app_test.go` — test overlay routing + folder-jump inert.
- `internal/ui/help_popover.go` — add `{"m", "move", true}` to account Triage group.

**Touch (no code change, only verify):**
- `docs/poplar/decisions/` — write ADR at pass end (handled by `poplar-pass` skill).
- `docs/poplar/invariants.md` — update at pass end.
- `docs/poplar/STATUS.md` — update at pass end.

---

## Task 1: Sidebar accessor — `OrderedFolders()`

**Files:**
- Modify: `internal/ui/sidebar.go` (add types + method near the other accessors around line 100)
- Test: `internal/ui/sidebar_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/sidebar_test.go`:

```go
func TestSidebar_OrderedFolders(t *testing.T) {
	classified := []mail.ClassifiedFolder{
		{Folder: mail.Folder{Name: "INBOX"}, Canonical: "Inbox", Group: mail.GroupPrimary},
		{Folder: mail.Folder{Name: "Drafts"}, Canonical: "Drafts", Group: mail.GroupPrimary},
		{Folder: mail.Folder{Name: "Trash"}, Canonical: "Trash", Group: mail.GroupDisposal},
		{Folder: mail.Folder{Name: "Receipts/2026"}, Canonical: "Receipts/2026", Group: mail.GroupCustom},
	}
	s := NewSidebar(NewStyles(theme.Default()), classified, config.UIConfig{}, 30, 20, SimpleIcons)
	got := s.OrderedFolders()
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	want := []FolderEntry{
		{Display: "Inbox", Provider: "INBOX", Group: GroupPrimary},
		{Display: "Drafts", Provider: "Drafts", Group: GroupPrimary},
		{Display: "Trash", Provider: "Trash", Group: GroupDisposal},
		{Display: "Receipts/2026", Provider: "Receipts/2026", Group: GroupCustom},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], w)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```
go test ./internal/ui/ -run TestSidebar_OrderedFolders
```

Expected: FAIL — `FolderEntry` and `OrderedFolders` undefined.

- [ ] **Step 3: Implement the types and method**

In `internal/ui/sidebar.go`, add near the top (above `folderEntry`):

```go
// FolderGroup mirrors mail.FolderGroup at the UI layer so consumers
// outside sidebar.go (the move picker) don't need to import mail.
// Values intentionally match mail.GroupPrimary / GroupDisposal /
// GroupCustom by ordinal — translateGroup performs the conversion.
type FolderGroup int

const (
	GroupPrimary FolderGroup = iota
	GroupDisposal
	GroupCustom
)

// FolderEntry is a flat record describing one sidebar folder for
// consumers that need ordered access (the move picker). Display is
// the canonical name shown to users; Provider is the backend name
// passed to mail.Backend methods; Group is the sidebar group it
// belongs to.
type FolderEntry struct {
	Display  string
	Provider string
	Group    FolderGroup
}
```

After `FolderNameByCanonical` (line ~110), add:

```go
// OrderedFolders returns one FolderEntry per visible sidebar folder,
// in sidebar render order (Primary → Disposal → Custom; ranked
// within each group by UIConfig). Used by the move picker to
// populate its list at open time.
func (s Sidebar) OrderedFolders() []FolderEntry {
	out := make([]FolderEntry, 0, len(s.entries))
	for _, e := range s.entries {
		display := e.cf.Canonical
		if display == "" {
			display = e.cf.Folder.Name
		}
		out = append(out, FolderEntry{
			Display:  display,
			Provider: e.cf.Folder.Name,
			Group:    translateGroup(e.cf.Group),
		})
	}
	return out
}

// translateGroup converts mail.FolderGroup to the UI-layer mirror.
func translateGroup(g mail.FolderGroup) FolderGroup {
	switch g {
	case mail.GroupPrimary:
		return GroupPrimary
	case mail.GroupDisposal:
		return GroupDisposal
	default:
		return GroupCustom
	}
}
```

- [ ] **Step 4: Run the test to verify it passes**

```
go test ./internal/ui/ -run TestSidebar_OrderedFolders
```

Expected: PASS.

- [ ] **Step 5: Run vet + full UI test suite**

```
go vet ./internal/ui/ && go test ./internal/ui/
```

Expected: green.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/sidebar.go internal/ui/sidebar_test.go
git commit -m "Pass 6.5: add Sidebar.OrderedFolders for move picker

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: New tea.Msg types for move picker

**Files:**
- Modify: `internal/ui/cmds.go` (append after `undoRequestedMsg`)

- [ ] **Step 1: Add the message types**

In `internal/ui/cmds.go`, after the existing `undoRequestedMsg` block (around line 339), append:

```go
// OpenMovePickerMsg requests the App open the move picker overlay.
// Emitted by AccountTab when the user presses `m` on a non-empty
// ActionTargets snapshot. Fields are snapshotted at trigger time so
// state changes between open and pick don't break the dispatch.
type OpenMovePickerMsg struct {
	UIDs    []mail.UID
	Src     string        // source folder provider name
	Folders []FolderEntry // sidebar-order, src already excluded
}

// MovePickerPickedMsg signals the user picked a destination. App
// closes the picker and forwards this msg into AccountTab so the
// triage dispatch fires from the same component that owns the
// MessageList state.
type MovePickerPickedMsg struct {
	UIDs []mail.UID
	Src  string
	Dest string // destination folder provider name
}

// MovePickerClosedMsg signals the picker has closed (Esc, or after
// a Picked emission). App flips movePicker.open.
type MovePickerClosedMsg struct{}
```

- [ ] **Step 2: Verify the package still builds**

```
go build ./internal/ui/
```

Expected: green (types are unused but well-formed).

- [ ] **Step 3: Commit**

```bash
git add internal/ui/cmds.go
git commit -m "Pass 6.5: add move picker tea.Msg types

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Toast extension for the "move" op

The toast renders `Moved N messages to <dest>`. Requires a `dest` field on `pendingAction` and `triageStartedMsg`, plus a "move" branch in `toastVerb` and `renderToast`.

**Files:**
- Modify: `internal/ui/toast.go`
- Modify: `internal/ui/cmds.go`
- Modify: `internal/ui/app.go` (carry dest into the toast)
- Test: `internal/ui/toast_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/toast_test.go`:

```go
func TestRenderToast_Move(t *testing.T) {
	styles := NewStyles(theme.Default())
	p := pendingAction{op: "move", n: 3, dest: "Receipts/2026"}
	got := renderToast(p, 80, styles)
	if !strings.Contains(got, "Moved 3 messages to Receipts/2026") {
		t.Errorf("render = %q, want it to contain %q", got, "Moved 3 messages to Receipts/2026")
	}
	if !strings.Contains(got, "[u undo]") {
		t.Errorf("render = %q, want undo hint", got)
	}
}

func TestRenderToast_MoveSingle(t *testing.T) {
	styles := NewStyles(theme.Default())
	p := pendingAction{op: "move", n: 1, dest: "Inbox"}
	got := renderToast(p, 80, styles)
	if !strings.Contains(got, "Moved 1 message to Inbox") {
		t.Errorf("render = %q, want singular form", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```
go test ./internal/ui/ -run TestRenderToast_Move
```

Expected: FAIL — `dest` field unknown.

- [ ] **Step 3: Add `dest` to `pendingAction`**

In `internal/ui/toast.go`, change the struct:

```go
type pendingAction struct {
	op       string
	n        int
	dest     string    // destination folder display name; only set when op == "move"
	inverse  tea.Cmd
	deadline time.Time
	onUndo   func()
}
```

Update `toastVerb`:

```go
func toastVerb(op string) string {
	switch op {
	case "delete":
		return "Deleted"
	case "archive":
		return "Archived"
	case "move":
		return "Moved"
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
```

Update `renderToast` body construction so the "move" branch appends ` to <dest>`:

```go
func renderToast(p pendingAction, width int, styles Styles) string {
	if p.IsZero() {
		return ""
	}
	verb := toastVerb(p.op)
	var body string
	switch p.op {
	case "star", "unstar", "read", "unread":
		if p.n > 1 {
			body = fmt.Sprintf("%s %d", verb, p.n)
		} else {
			body = verb
		}
	case "move":
		body = fmt.Sprintf("%s %d %s to %s", verb, p.n, pluralize("message", p.n), p.dest)
	default:
		body = fmt.Sprintf("%s %d %s", verb, p.n, pluralize("message", p.n))
	}
	hint := "[u undo]"
	full := "✓ " + body + "   " + hint
	if lipgloss.Width(full) <= width {
		return styles.Toast.Render(full)
	}
	hintW := lipgloss.Width(hint)
	bodyBudget := width - hintW - 4
	if bodyBudget < 1 {
		return styles.Toast.Render(truncateToWidth(full, width))
	}
	bodyTrunc := truncateToWidth("✓ "+body, bodyBudget+2)
	return styles.Toast.Render(bodyTrunc + "   " + hint)
}
```

- [ ] **Step 4: Add `dest` to `triageStartedMsg` and propagate through App**

In `internal/ui/cmds.go`, extend the struct:

```go
type triageStartedMsg struct {
	op      string
	n       int
	uids    []mail.UID
	dest    string  // populated for op == "move"; ignored otherwise
	inverse tea.Cmd
	onUndo  func()
}
```

In `internal/ui/app.go`, in the `case triageStartedMsg:` branch (around line 117), copy `dest` into the toast:

```go
m.toast = pendingAction{
	op:       msg.op,
	n:        msg.n,
	dest:     msg.dest,
	inverse:  msg.inverse,
	onUndo:   msg.onUndo,
	deadline: deadline,
}
```

- [ ] **Step 5: Run the toast tests + full UI suite**

```
go test ./internal/ui/
```

Expected: green.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/toast.go internal/ui/toast_test.go internal/ui/cmds.go internal/ui/app.go
git commit -m "Pass 6.5: extend toast to render 'Moved N to <dest>'

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: MovePicker model + filter (no rendering yet)

**Files:**
- Create: `internal/ui/movepicker.go`
- Test: `internal/ui/movepicker_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/movepicker_test.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

func sampleFolders() []FolderEntry {
	return []FolderEntry{
		{Display: "Inbox", Provider: "INBOX", Group: GroupPrimary},
		{Display: "Drafts", Provider: "Drafts", Group: GroupPrimary},
		{Display: "Sent", Provider: "Sent", Group: GroupPrimary},
		{Display: "Archive", Provider: "Archive", Group: GroupDisposal},
		{Display: "Trash", Provider: "Trash", Group: GroupDisposal},
		{Display: "Receipts/2026", Provider: "Receipts/2026", Group: GroupCustom},
		{Display: "Receipts/2025", Provider: "Receipts/2025", Group: GroupCustom},
	}
}

func newTestPicker() MovePicker {
	return NewMovePicker(NewStyles(theme.Default()), theme.Default())
}

func TestMovePicker_OpenSetsState(t *testing.T) {
	p := newTestPicker()
	p = p.Open([]mail.UID{1, 2}, "INBOX", sampleFolders())
	if !p.IsOpen() {
		t.Fatal("picker should be open after Open")
	}
	if got, want := len(p.all), len(sampleFolders()); got != want {
		t.Errorf("all len = %d, want %d", got, want)
	}
	if len(p.matches) != len(p.all) {
		t.Errorf("matches len = %d, want %d (no filter)", len(p.matches), len(p.all))
	}
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0", p.cursor)
	}
}

func TestMovePicker_FilterNarrows(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{1}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if p.filter != "rec" {
		t.Errorf("filter = %q, want %q", p.filter, "rec")
	}
	if len(p.matches) != 2 {
		t.Errorf("matches = %d, want 2 (Receipts/2026, Receipts/2025)", len(p.matches))
	}
	for _, idx := range p.matches {
		if !strings.Contains(strings.ToLower(p.all[idx].Display), "rec") {
			t.Errorf("match %q does not contain 'rec'", p.all[idx].Display)
		}
	}
}

func TestMovePicker_FilterCaseInsensitive(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{1}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}})
	if len(p.matches) == 0 {
		t.Fatal("expected matches for 'I' (Inbox, Receipts), got 0")
	}
}

func TestMovePicker_BackspaceWidens(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{1}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.filter != "r" {
		t.Errorf("filter = %q, want %q", p.filter, "r")
	}
}

func TestMovePicker_BackspaceEmptyNoOp(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{1}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.filter != "" {
		t.Errorf("filter = %q, want empty", p.filter)
	}
}

func TestMovePicker_CursorClampsOnFilter(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{1}, "INBOX", sampleFolders())
	for i := 0; i < 5; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if p.cursor != 5 {
		t.Fatalf("cursor = %d, want 5 (precondition)", p.cursor)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after filter change", p.cursor)
	}
}

func TestMovePicker_NavigationBounds(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{1}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.cursor != 0 {
		t.Errorf("up at top: cursor = %d, want 0", p.cursor)
	}
	for i := 0; i < 100; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if p.cursor != len(p.matches)-1 {
		t.Errorf("down past bottom: cursor = %d, want %d", p.cursor, len(p.matches)-1)
	}
}

func TestMovePicker_EnterEmitsPickedMsg(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{42}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown}) // cursor=1 (Drafts)
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter returned nil cmd")
	}
	msgs := drainBatch(cmd)
	var picked *MovePickerPickedMsg
	var sawClosed bool
	for _, m := range msgs {
		switch v := m.(type) {
		case MovePickerPickedMsg:
			picked = &v
		case MovePickerClosedMsg:
			sawClosed = true
		}
	}
	if picked == nil {
		t.Fatal("did not see MovePickerPickedMsg")
	}
	if !sawClosed {
		t.Error("did not see MovePickerClosedMsg")
	}
	if picked.Dest != "Drafts" {
		t.Errorf("Dest = %q, want %q", picked.Dest, "Drafts")
	}
	if picked.Src != "INBOX" {
		t.Errorf("Src = %q, want %q", picked.Src, "INBOX")
	}
	if len(picked.UIDs) != 1 || picked.UIDs[0] != 42 {
		t.Errorf("UIDs = %v, want [42]", picked.UIDs)
	}
}

func TestMovePicker_EnterInertOnEmpty(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{1}, "INBOX", sampleFolders())
	for _, r := range "zzzzz" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(p.matches) != 0 {
		t.Fatalf("matches = %d, want 0 (precondition)", len(p.matches))
	}
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("Enter on empty matches returned non-nil cmd")
	}
}

func TestMovePicker_EscClosesNoOp(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{1}, "INBOX", sampleFolders())
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc returned nil cmd")
	}
	msgs := drainBatch(cmd)
	var sawClosed bool
	for _, m := range msgs {
		if _, ok := m.(MovePickerClosedMsg); ok {
			sawClosed = true
		}
		if _, ok := m.(MovePickerPickedMsg); ok {
			t.Error("Esc emitted PickedMsg")
		}
	}
	if !sawClosed {
		t.Error("Esc did not emit ClosedMsg")
	}
}

func TestMovePicker_QSwallowed(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{1}, "INBOX", sampleFolders())
	beforeFilter := p.filter
	p2, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Errorf("q produced cmd, want nil (swallowed)")
	}
	if p2.filter != beforeFilter {
		t.Errorf("q modified filter to %q, want unchanged %q", p2.filter, beforeFilter)
	}
}

// drainBatch invokes a tea.Batch (or single Cmd) and returns the
// resulting messages. Mirrors the helper used by app_test.go;
// duplicated here to keep the file self-contained for readers
// scanning movepicker_test.go.
func drainBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drainBatch(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```
go test ./internal/ui/ -run TestMovePicker
```

Expected: FAIL — types undefined.

- [ ] **Step 3: Implement the picker model**

Create `internal/ui/movepicker.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// MovePicker is the modal overlay launched by `m` from the account
// view. Single-column folder list with type-to-filter input;
// arrow-key navigation; Enter picks; Esc cancels. App owns the open
// state and the overlay composition (mirrors LinkPicker, ADR-0087).
type MovePicker struct {
	open    bool
	uids    []mail.UID
	src     string
	all     []FolderEntry
	filter  string
	matches []int
	cursor  int
	offset  int
	width   int
	height  int
	styles  Styles
	theme   *theme.CompiledTheme
	keys    movePickerKeys
}

type movePickerKeys struct {
	Up        key.Binding
	Down      key.Binding
	Pick      key.Binding
	Close     key.Binding
	Backspace key.Binding
}

// NewMovePicker returns a closed picker.
func NewMovePicker(styles Styles, t *theme.CompiledTheme) MovePicker {
	return MovePicker{
		styles: styles,
		theme:  t,
		keys: movePickerKeys{
			Up:        key.NewBinding(key.WithKeys("up")),
			Down:      key.NewBinding(key.WithKeys("down")),
			Pick:      key.NewBinding(key.WithKeys("enter")),
			Close:     key.NewBinding(key.WithKeys("esc")),
			Backspace: key.NewBinding(key.WithKeys("backspace")),
		},
	}
}

// IsOpen reports whether the picker is visible.
func (p MovePicker) IsOpen() bool { return p.open }

// Open transitions the picker into the open state with the given
// snapshot. Filter resets to empty; cursor + offset reset to 0.
// Source folder is excluded from the list.
func (p MovePicker) Open(uids []mail.UID, src string, folders []FolderEntry) MovePicker {
	p.open = true
	p.uids = uids
	p.src = src
	p.all = make([]FolderEntry, 0, len(folders))
	for _, f := range folders {
		if f.Provider == src {
			continue
		}
		p.all = append(p.all, f)
	}
	p.filter = ""
	p.cursor = 0
	p.offset = 0
	p.recompute()
	return p
}

// Close transitions the picker out of view. Caller is responsible
// for any chrome-revert side effects.
func (p MovePicker) Close() MovePicker {
	p.open = false
	return p
}

// SetSize updates the picker's box dimensions.
func (p MovePicker) SetSize(width, height int) MovePicker {
	p.width = width
	p.height = height
	return p
}

// recompute rebuilds matches from filter, resets cursor + offset.
func (p *MovePicker) recompute() {
	p.matches = p.matches[:0]
	if cap(p.matches) < len(p.all) {
		p.matches = make([]int, 0, len(p.all))
	}
	needle := strings.ToLower(p.filter)
	for i, f := range p.all {
		if needle == "" || strings.Contains(strings.ToLower(f.Display), needle) {
			p.matches = append(p.matches, i)
		}
	}
	p.cursor = 0
	p.offset = 0
}

// Update dispatches a tea.Msg while the picker is open.
func (p MovePicker) Update(msg tea.Msg) (MovePicker, tea.Cmd) {
	if !p.open {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Down):
		if p.cursor < len(p.matches)-1 {
			p.cursor++
		}
		return p, nil
	case key.Matches(keyMsg, p.keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case key.Matches(keyMsg, p.keys.Pick):
		if p.cursor < 0 || p.cursor >= len(p.matches) {
			return p, nil
		}
		dest := p.all[p.matches[p.cursor]].Provider
		picked := MovePickerPickedMsg{UIDs: p.uids, Src: p.src, Dest: dest}
		return p, tea.Batch(
			func() tea.Msg { return picked },
			func() tea.Msg { return MovePickerClosedMsg{} },
		)
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return MovePickerClosedMsg{} }
	case key.Matches(keyMsg, p.keys.Backspace):
		if p.filter == "" {
			return p, nil
		}
		// Strip last rune.
		_, size := utf8.DecodeLastRuneInString(p.filter)
		p.filter = p.filter[:len(p.filter)-size]
		p.recompute()
		return p, nil
	}
	// q is swallowed (consistent with help/link picker overlays).
	if keyMsg.String() == "q" {
		return p, nil
	}
	// Treat any single printable rune as filter input.
	if r, ok := singlePrintableRune(keyMsg); ok {
		p.filter += string(r)
		p.recompute()
		return p, nil
	}
	return p, nil
}

// singlePrintableRune returns (r, true) when keyMsg represents a
// single printable rune. Filters out control keys, multi-rune
// sequences, and chord keys.
func singlePrintableRune(k tea.KeyMsg) (rune, bool) {
	if len(k.Runes) != 1 {
		return 0, false
	}
	r := k.Runes[0]
	if !unicode.IsPrint(r) {
		return 0, false
	}
	return r, true
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```
go test ./internal/ui/ -run TestMovePicker
```

Expected: PASS for all model/filter tests. (View-related tests come in Task 5.)

- [ ] **Step 5: Commit**

```bash
git add internal/ui/movepicker.go internal/ui/movepicker_test.go
git commit -m "Pass 6.5: add MovePicker model + filter

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: MovePicker rendering — Box, View, Position

**Files:**
- Modify: `internal/ui/movepicker.go`
- Modify: `internal/ui/movepicker_test.go`

- [ ] **Step 1: Write the failing rendering tests**

Append to `internal/ui/movepicker_test.go`:

```go
import "github.com/charmbracelet/lipgloss"

func TestMovePicker_BoxFitsWidth(t *testing.T) {
	p := newTestPicker().Open(nil, "", sampleFolders()).SetSize(80, 24)
	box := p.Box(80, 24)
	for i, line := range strings.Split(box, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line %d width = %d, want <= 80: %q", i, w, line)
		}
	}
}

func TestMovePicker_BoxHeightBounded(t *testing.T) {
	p := newTestPicker().Open(nil, "", sampleFolders()).SetSize(80, 24)
	box := p.Box(80, 24)
	if h := strings.Count(box, "\n") + 1; h > 24 {
		t.Errorf("box height = %d, want <= 24", h)
	}
}

func TestMovePicker_RendersGroupSeparators(t *testing.T) {
	p := newTestPicker().Open(nil, "", sampleFolders()).SetSize(80, 30)
	box := p.Box(80, 30)
	// Expect at least one fully-blank list row between Disposal and Custom.
	// (We can't easily assert on exact rows; check the box mentions all groups.)
	for _, want := range []string{"Inbox", "Trash", "Receipts/2026"} {
		if !strings.Contains(box, want) {
			t.Errorf("box missing %q", want)
		}
	}
}

func TestMovePicker_FilterEmptyMatchHint(t *testing.T) {
	p := newTestPicker().Open(nil, "", sampleFolders()).SetSize(80, 24)
	for _, r := range "zzzzz" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	box := p.Box(80, 24)
	if !strings.Contains(box, "no folders match") {
		t.Errorf("box missing empty-match hint, got:\n%s", box)
	}
}

func TestMovePicker_FilterHintRowShown(t *testing.T) {
	p := newTestPicker().Open(nil, "", sampleFolders()).SetSize(80, 24)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	box := p.Box(80, 24)
	if !strings.Contains(box, "filter: r") {
		t.Errorf("box missing filter hint, got:\n%s", box)
	}
}

func TestMovePicker_HelpRowAlwaysShown(t *testing.T) {
	p := newTestPicker().Open(nil, "", sampleFolders()).SetSize(80, 24)
	box := p.Box(80, 24)
	if !strings.Contains(box, "select") || !strings.Contains(box, "pick") || !strings.Contains(box, "cancel") {
		t.Errorf("box missing help row, got:\n%s", box)
	}
}

func TestMovePicker_ViewClosedEmpty(t *testing.T) {
	p := newTestPicker()
	if p.View() != "" {
		t.Errorf("closed picker View = %q, want empty", p.View())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```
go test ./internal/ui/ -run TestMovePicker
```

Expected: rendering tests FAIL (Box not implemented, View returns empty for closed only).

- [ ] **Step 3: Implement Box, View, Position, helpers**

Append to `internal/ui/movepicker.go`:

```go
const (
	movePickerMaxWidth = 50
	movePickerMinWidth = 24
)

// View renders the picker as a standalone string. Returns "" when
// closed. App composes via Box + Position + PlaceOverlay; this
// method is the fallback used by tests and when the box doesn't fit.
func (p MovePicker) View() string {
	if !p.open {
		return ""
	}
	return p.Box(p.width, p.height)
}

// Box returns the rendered modal at the size derived from (w, h).
func (p MovePicker) Box(w, h int) string {
	boxW := movePickerMaxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < movePickerMinWidth {
		boxW = movePickerMinWidth
	}
	contentW := boxW - 2 // left/right border

	// list rows = h - 7 (top border + rule + 2 footer + bottom border + 2 slack)
	maxListRows := h - 7
	if maxListRows < 1 {
		maxListRows = 1
	}

	rows := p.buildListRows(contentW)
	if len(rows) > maxListRows {
		// Apply offset window.
		if p.cursor < p.offset {
			p.offset = p.cursor
		}
		if p.cursor >= p.offset+maxListRows {
			p.offset = p.cursor - maxListRows + 1
		}
		end := p.offset + maxListRows
		if end > len(rows) {
			end = len(rows)
		}
		rows = rows[p.offset:end]
	}

	var b strings.Builder
	title := " Move to (" + itoa(len(p.matches)) + ") "
	rest := boxW - 2 - len(title)
	if rest < 0 {
		rest = 0
	}
	b.WriteString("┌─" + title + strings.Repeat("─", rest) + "┐\n")

	for _, row := range rows {
		padded := padOrTruncate(row, contentW)
		b.WriteString("│" + padded + "│\n")
	}
	// Pad remaining list rows to fixed height so the box doesn't shrink.
	for i := len(rows); i < maxListRows; i++ {
		b.WriteString("│" + strings.Repeat(" ", contentW) + "│\n")
	}

	b.WriteString("├" + strings.Repeat("─", contentW) + "┤\n")

	// Filter hint row (always present; empty padding when filter == "").
	hint := ""
	if p.filter != "" {
		hint = "filter: " + p.filter
	}
	b.WriteString("│" + p.styles.FgDim.Render(padOrTruncate(hint, contentW)) + "│\n")

	// Help row.
	help := "↑↓ select · enter pick · esc cancel"
	b.WriteString("│" + p.styles.FgDim.Render(padOrTruncate(help, contentW)) + "│\n")

	b.WriteString("└" + strings.Repeat("─", contentW) + "┘")

	return b.String()
}

// buildListRows constructs the list area rows: folder names, group
// separators (only when filter is empty), or a single empty-match
// hint when no matches.
func (p MovePicker) buildListRows(contentW int) []string {
	if len(p.matches) == 0 && p.filter != "" {
		return []string{"  no folders match \"" + truncateToWidth(p.filter, contentW-22) + "\""}
	}
	rows := make([]string, 0, len(p.matches)+2)
	prevGroup := FolderGroup(-1)
	for i, idx := range p.matches {
		entry := p.all[idx]
		if p.filter == "" && i > 0 && entry.Group != prevGroup {
			rows = append(rows, "")
		}
		prevGroup = entry.Group
		marker := "  "
		if i == p.cursor {
			marker = "> "
		}
		row := marker + entry.Display
		if i == p.cursor {
			row = p.styles.MsgListCursor.Render(padOrTruncate(row, contentW))
		}
		rows = append(rows, row)
	}
	return rows
}

// padOrTruncate makes s exactly width display cells using
// lipgloss.Width (folder names + UI strings have no SPUA glyphs).
func padOrTruncate(s string, width int) string {
	w := lipgloss.Width(s)
	if w == width {
		return s
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return truncateToWidth(s, width)
}

// itoa is a 1-line strconv.Itoa to avoid importing strconv just for
// the title count.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Position returns the centered top-left for the rendered box.
func (p MovePicker) Position(box string, totalW, totalH int) (int, int) {
	return centerOverlay(box, totalW, totalH)
}
```

Add the lipgloss import at the top of `movepicker.go`:

```go
import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)
```

(Ensure the test file's `lipgloss` import lands at the top with the other imports — Go's import grouping rules will reject a mid-file `import` statement. Move `import "github.com/charmbracelet/lipgloss"` from the test code block above into the existing `import (...)` group at the top of `movepicker_test.go`.)

- [ ] **Step 4: Run all picker tests**

```
go test ./internal/ui/ -run TestMovePicker
```

Expected: PASS.

- [ ] **Step 5: Run full UI suite + vet**

```
go vet ./internal/ui/ && go test ./internal/ui/
```

Expected: green.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/movepicker.go internal/ui/movepicker_test.go
git commit -m "Pass 6.5: MovePicker rendering — Box, View, Position

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Add `m` key + AccountTab dispatch

**Files:**
- Modify: `internal/ui/keys.go`
- Modify: `internal/ui/account_tab.go`
- Test: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/account_tab_test.go`:

```go
func TestAccountTab_MKeyEmitsOpenMovePickerMsg(t *testing.T) {
	tab := newTestAccountTab(t) // existing helper; constructs AccountTab with mock backend
	tab = loadTestFolderAndMessages(t, tab) // existing helper; populates msglist
	_, cmd := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if cmd == nil {
		t.Fatal("m returned nil cmd, want OpenMovePickerMsg")
	}
	msgs := drainBatch(cmd)
	var open *OpenMovePickerMsg
	for _, m := range msgs {
		if v, ok := m.(OpenMovePickerMsg); ok {
			open = &v
		}
	}
	if open == nil {
		t.Fatalf("did not see OpenMovePickerMsg in %v", msgs)
	}
	if len(open.UIDs) == 0 {
		t.Error("UIDs empty")
	}
	if open.Src == "" {
		t.Error("Src empty")
	}
	if len(open.Folders) == 0 {
		t.Error("Folders empty")
	}
	for _, f := range open.Folders {
		if f.Provider == open.Src {
			t.Errorf("Folders contains source %q; should be excluded", open.Src)
		}
	}
}

func TestAccountTab_MKeyNoOpOnEmpty(t *testing.T) {
	tab := newTestAccountTab(t) // no folder loaded, so ActionTargets empty
	_, cmd := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if cmd != nil {
		t.Errorf("m on empty msglist returned cmd %v, want nil", cmd)
	}
}

func TestAccountTab_MovePickerPickedDispatchesMove(t *testing.T) {
	tab := newTestAccountTab(t)
	tab = loadTestFolderAndMessages(t, tab)
	uids := tab.msglist.ActionTargets()
	src := tab.currentFolderName()
	beforeLen := tab.msglist.SourceLen() // existing accessor used by other tests

	_, cmd := tab.Update(MovePickerPickedMsg{UIDs: uids, Src: src, Dest: "Archive"})
	if cmd == nil {
		t.Fatal("MovePickerPickedMsg produced nil cmd")
	}
	if got := tab.msglist.SourceLen(); got >= beforeLen {
		t.Errorf("after Picked: SourceLen = %d, want < %d (optimistic delete)", got, beforeLen)
	}
	msgs := drainBatch(cmd)
	var sawStart bool
	for _, m := range msgs {
		if ts, ok := m.(triageStartedMsg); ok {
			sawStart = true
			if ts.op != "move" {
				t.Errorf("triageStartedMsg.op = %q, want %q", ts.op, "move")
			}
			if ts.dest != "Archive" {
				t.Errorf("triageStartedMsg.dest = %q, want %q", ts.dest, "Archive")
			}
		}
	}
	if !sawStart {
		t.Errorf("no triageStartedMsg in %v", msgs)
	}
}
```

(If `tab.msglist.SourceLen()` does not exist as a public/test accessor, substitute with whatever existing helper the surrounding tests use to inspect msglist length — check `account_tab_test.go` for the pattern. The intent is "SourceLen decreases after the optimistic delete".)

- [ ] **Step 2: Run the tests to verify they fail**

```
go test ./internal/ui/ -run TestAccountTab_M
```

Expected: FAIL — `m` not bound, `MovePickerPickedMsg` not handled.

- [ ] **Step 3: Add the `Move` keybinding**

In `internal/ui/keys.go`, in the `AccountKeys` struct (around line 33), add a field:

```go
Move          key.Binding
```

In `NewAccountKeys()` (around line 61), add the binding before the closing brace:

```go
Move: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move")),
```

- [ ] **Step 4: Implement `m` handler in AccountTab**

In `internal/ui/account_tab.go`, in the key switch in `Update` (the block around line 317 with `Delete`/`Archive`/`Star`/`ReadToggle` cases), add:

```go
case key.Matches(msg, m.keys.Move):
	return m, m.dispatchMove()
```

Then add the helper method (place it next to `dispatchTriage` at the bottom of the file, around line 470):

```go
// dispatchMove emits an OpenMovePickerMsg with a snapshot of the
// current ActionTargets, source folder, and sidebar folder list.
// Returns nil when there are no targets (silent no-op, mirrors
// dispatchTriage on empty).
func (m *AccountTab) dispatchMove() tea.Cmd {
	uids := m.msglist.ActionTargets()
	if len(uids) == 0 {
		return nil
	}
	src := m.currentFolderName()
	folders := m.sidebar.OrderedFolders()
	return func() tea.Msg {
		return OpenMovePickerMsg{UIDs: uids, Src: src, Folders: folders}
	}
}
```

- [ ] **Step 5: Handle `MovePickerPickedMsg` in AccountTab**

In `internal/ui/account_tab.go`, in `Update`, add a case to the outer `switch msg := msg.(type)` (find the existing handling of msgs above the keymsg block — search for `case backendUpdateMsg:` or similar in the AccountTab Update). Add:

```go
case MovePickerPickedMsg:
	return m, m.dispatchMoveFromPicker(msg)
```

Then add the helper method near `dispatchMove`:

```go
// dispatchMoveFromPicker performs the optimistic move: snapshots
// state for inverse, ApplyDeletes the messages from the local list,
// exits visual mode, and returns the buildTriageCmd Cmd. The dest
// is carried in the triageStartedMsg so the toast can render
// "Moved N to <dest>".
func (m *AccountTab) dispatchMoveFromPicker(msg MovePickerPickedMsg) tea.Cmd {
	uids := msg.UIDs
	if len(uids) == 0 {
		return nil
	}
	snapshot, positions := m.msglist.SnapshotSource(uids)
	m.msglist.ApplyDelete(uids)
	m.msglist.ExitVisual()

	onUndo := func() { m.msglist.ApplyInsert(snapshot, positions) }
	fwd := func() error { return m.backend.Move(uids, msg.Dest) }
	rev := func() error { return m.backend.Move(uids, msg.Src) }
	return buildTriageCmdWithDest("move", uids, msg.Dest, onUndo, fwd, rev)
}
```

- [ ] **Step 6: Add `buildTriageCmdWithDest` (DRY: factor `buildTriageCmd`)**

In `internal/ui/account_tab.go`, replace the existing `buildTriageCmd` with a thin wrapper over a new `buildTriageCmdWithDest`:

```go
// buildTriageCmd is the canonical Cmd assembler for triage actions
// without a destination folder (delete, archive, flag, seen toggles).
func buildTriageCmd(op string, uids []mail.UID, onUndo func(), fwd, rev func() error) tea.Cmd {
	return buildTriageCmdWithDest(op, uids, "", onUndo, fwd, rev)
}

// buildTriageCmdWithDest is the variant for ops that need to carry a
// destination folder name into the toast (currently "move"). dest is
// stored on the triageStartedMsg and copied into pendingAction.dest
// by App.
func buildTriageCmdWithDest(op string, uids []mail.UID, dest string, onUndo func(), fwd, rev func() error) tea.Cmd {
	forward := func() tea.Msg {
		if err := fwd(); err != nil {
			return ErrorMsg{Op: op, Err: err}
		}
		return nil
	}
	inverse := func() tea.Msg {
		if err := rev(); err != nil {
			return ErrorMsg{Op: op + " undo", Err: err}
		}
		return nil
	}
	start := func() tea.Msg {
		return triageStartedMsg{op: op, n: len(uids), uids: uids, dest: dest, inverse: inverse, onUndo: onUndo}
	}
	return tea.Batch(start, forward)
}
```

- [ ] **Step 7: Run the tests + full suite + vet**

```
go vet ./internal/ui/ && go test ./internal/ui/
```

Expected: green.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/keys.go internal/ui/account_tab.go internal/ui/account_tab_test.go
git commit -m "Pass 6.5: bind m, dispatch move via picker

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: App — overlay wiring (open, route keys, render)

**Files:**
- Modify: `internal/ui/app.go`
- Test: `internal/ui/app_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/app_test.go`:

```go
func TestApp_OpenMovePickerOpensOverlay(t *testing.T) {
	app := newTestApp(t) // existing helper
	folders := []FolderEntry{
		{Display: "Inbox", Provider: "INBOX", Group: GroupPrimary},
		{Display: "Archive", Provider: "Archive", Group: GroupDisposal},
	}
	app, _ = app.Update(OpenMovePickerMsg{UIDs: []mail.UID{1}, Src: "INBOX", Folders: folders})
	if !app.movePicker.IsOpen() {
		t.Error("movePicker should be open after OpenMovePickerMsg")
	}
}

func TestApp_MovePickerClosedFlipsState(t *testing.T) {
	app := newTestApp(t)
	folders := []FolderEntry{{Display: "Archive", Provider: "Archive", Group: GroupDisposal}}
	app, _ = app.Update(OpenMovePickerMsg{UIDs: []mail.UID{1}, Src: "INBOX", Folders: folders})
	app, _ = app.Update(MovePickerClosedMsg{})
	if app.movePicker.IsOpen() {
		t.Error("movePicker should be closed after MovePickerClosedMsg")
	}
}

func TestApp_FolderJumpInertWhilePickerOpen(t *testing.T) {
	app := newTestApp(t)
	folders := []FolderEntry{{Display: "Archive", Provider: "Archive", Group: GroupDisposal}}
	app, _ = app.Update(OpenMovePickerMsg{UIDs: []mail.UID{1}, Src: "INBOX", Folders: folders})
	beforeFolder := app.acct.currentFolderName()
	app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}})
	if got := app.acct.currentFolderName(); got != beforeFolder {
		t.Errorf("folder changed to %q while picker open; want %q (key should be swallowed)", got, beforeFolder)
	}
}

func TestApp_MovePickerPickedRoutesToAccountTab(t *testing.T) {
	app := newTestApp(t)
	folders := []FolderEntry{{Display: "Archive", Provider: "Archive", Group: GroupDisposal}}
	app, _ = app.Update(OpenMovePickerMsg{UIDs: []mail.UID{42}, Src: "INBOX", Folders: folders})
	_, cmd := app.Update(MovePickerPickedMsg{UIDs: []mail.UID{42}, Src: "INBOX", Dest: "Archive"})
	if cmd == nil {
		t.Fatal("Picked produced nil cmd; expected triageStartedMsg + forward")
	}
	msgs := drainBatch(cmd)
	var sawStart bool
	for _, m := range msgs {
		if ts, ok := m.(triageStartedMsg); ok {
			sawStart = true
			if ts.dest != "Archive" {
				t.Errorf("triageStartedMsg.dest = %q, want %q", ts.dest, "Archive")
			}
		}
	}
	if !sawStart {
		t.Errorf("no triageStartedMsg")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

```
go test ./internal/ui/ -run TestApp_MovePicker
go test ./internal/ui/ -run TestApp_FolderJumpInertWhilePickerOpen
```

Expected: FAIL.

- [ ] **Step 3: Add `movePicker` field to App and wire constructor**

In `internal/ui/app.go`, in the `App` struct (around line 18):

```go
type App struct {
	acct        AccountTab
	backend     mail.Backend
	icons       IconSet
	styles      Styles
	topLine     TopLine
	statusBar   StatusBar
	footer      Footer
	keys        GlobalKeys
	viewerOpen  bool
	helpOpen    bool
	help        HelpPopover
	linkPicker  LinkPicker
	movePicker  MovePicker
	lastErr     ErrorMsg
	toast       pendingAction
	undoSeconds int
	now    func() time.Time
	width  int
	height int
}
```

In `NewApp` (around line 42), initialize:

```go
movePicker:  NewMovePicker(styles, t),
```

- [ ] **Step 4: Wire `WindowSizeMsg` to forward to picker**

In `App.Update`'s `case tea.WindowSizeMsg:` (around line 98), after the existing `m.linkPicker = m.linkPicker.SetSize(...)` line:

```go
m.movePicker = m.movePicker.SetSize(m.width, m.height)
```

- [ ] **Step 5: Handle the new msg types**

In `App.Update`, after the `case LaunchURLMsg:` block (around line 115), add:

```go
case OpenMovePickerMsg:
	m.movePicker = m.movePicker.Open(msg.UIDs, msg.Src, msg.Folders)
	return m, nil

case MovePickerClosedMsg:
	m.movePicker = m.movePicker.Close()
	return m, nil

case MovePickerPickedMsg:
	// Forward to AccountTab so dispatch fires from the same component
	// that owns MessageList state.
	var cmd tea.Cmd
	m.acct, cmd = m.acct.Update(msg)
	m = m.deriveChromeFromAcct()
	return m, cmd
```

- [ ] **Step 6: Route keys into picker while open**

In the `case tea.KeyMsg:` branch (around line 220), after the existing `if m.linkPicker.IsOpen() { ... }` block, insert:

```go
if m.movePicker.IsOpen() {
	var cmd tea.Cmd
	m.movePicker, cmd = m.movePicker.Update(msg)
	return m, cmd
}
```

- [ ] **Step 7: Composite the picker overlay in View**

In `App.View()` (around line 318), after the existing `if m.linkPicker.IsOpen() { ... }` block (around line 341), insert:

```go
if m.movePicker.IsOpen() {
	box := m.movePicker.Box(m.width, m.height)
	x, y := m.movePicker.Position(box, m.width, m.height)
	dimmed := DimANSI(frame, m.styles.FgDim)
	return PlaceOverlay(x, y, box, dimmed)
}
```

(Pattern-match the LinkPicker block precisely; `frame` and `DimANSI`/`PlaceOverlay` are already in scope from that block.)

- [ ] **Step 8: Run the tests + full suite + vet**

```
go vet ./internal/ui/ && go test ./internal/ui/
```

Expected: green.

- [ ] **Step 9: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "Pass 6.5: wire MovePicker overlay into App

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: Help popover — add `m  move` row

**Files:**
- Modify: `internal/ui/help_popover.go`
- Test: `internal/ui/help_popover_test.go` (only if existing tests assert on Triage rows)

- [ ] **Step 1: Add the row to the account Triage group**

In `internal/ui/help_popover.go`, in `accountGroups` Triage section (around line 57), insert after the `{".", "read/unrd", true}` row:

```go
{"m", "move", true},
```

- [ ] **Step 2: Update any existing test that pinned the Triage row count**

```
go test ./internal/ui/ -run Help
```

If a test fails because it counts Triage rows or asserts on the rendered popover string, update the assertion to reflect the new row.

- [ ] **Step 3: Run the full UI suite + vet**

```
go vet ./internal/ui/ && go test ./internal/ui/
```

Expected: green.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/help_popover.go internal/ui/help_popover_test.go
git commit -m "Pass 6.5: add 'm move' to help popover Triage group

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: End-to-end live render check (tmux)

Per `internal/ui/` size contract (ADR-0079, ADR-0084) and the bubbletea conventions §10 review checklist, every UI pass needs a live tmux capture at 120×40 (and at minimum viable width if layout changed).

**Files:** none modified; capture artifact only.

- [ ] **Step 1: Build + install**

```
make install
```

- [ ] **Step 2: Live verify in tmux**

Follow `.claude/docs/tmux-testing.md` to:

1. Start poplar in a 120×40 tmux pane.
2. Open Inbox, navigate to a message.
3. Press `m`. Capture the picker rendering.
4. Type `arc` to filter; capture the narrowed list.
5. Press Backspace until empty; capture full list with group separators.
6. Type `zzzz`; capture the empty-match hint.
7. Press Esc to close; verify cursor stays put, no toast.
8. Press `m` again, navigate with `↓` to Archive, press `Enter`. Verify:
   - Picker closes
   - Toast appears: `✓ Moved 1 message to Archive   [u undo]`
   - Message disappears from list
9. Press `u` within 6s. Verify:
   - Toast clears
   - Message reappears at original position
10. Repeat the move and let the toast expire (wait > 6s). Verify the move is committed (no longer reversible).
11. Resize the pane to 60×24 and re-press `m`; verify the picker still fits (capped to `w-4 = 56` cells).

- [ ] **Step 3: Verify ADR-0083 size contract on picker**

Check that no rendered row exceeds the picker's `boxW` and that the box does not exceed the terminal height. The test `TestMovePicker_BoxFitsWidth` covers this programmatically; the live capture is the cross-check.

- [ ] **Step 4: Note any visual issues**

If anything renders wrong in tmux that the unit tests didn't catch, add a regression test before fixing — that's the rule under "evidence before assertions" (verification-before-completion skill).

---

## Task 10: Pass-end ritual

After every task above is committed and `make check` is green, invoke the `poplar-pass` skill to run the consolidation ritual:

- [ ] **Step 1: Run `/simplify`**

Apply genuine wins. Move-picker code is small and self-contained; expect minimal hits.

- [ ] **Step 2: Run §10 idiomatic-bubbletea review checklist**

From `docs/poplar/bubbletea-conventions.md`. Items to confirm against the diff:

- `MovePicker.View()` returns no row wider than its assigned width and no rows beyond `height` (covered by `TestMovePicker_BoxFitsWidth` + `TestMovePicker_BoxHeightBounded`; cross-checked with tmux capture at 120×40 and 60×24).
- No state mutation in `View()` — all mutation in `Update`.
- No I/O in picker — only Msg-emitter closures.
- Width math via `lipgloss.Width` only.
- Renderers honor width via `padOrTruncate` + `truncateToWidth` (no wordwrap; folder names are unbreakable).
- No defensive parent-side clipping — App calls `Box(m.width, m.height)` and trusts the result.
- Children signal parent via `tea.Msg` (Open/Picked/Closed) — no callbacks.
- `WindowSizeMsg` forwarded into `movePicker.SetSize` after App stores dims.
- Keys declared as `key.Binding`; dispatched with `key.Matches`.
- New `m` key in help popover vocabulary (Task 8).
- No deprecated API usage.

Note in the ADR that the picker uses `↑`/`↓` for navigation (not `j`/`k`) because letters are filter input; deviation explicitly named.

- [ ] **Step 3: Write ADR**

Create `docs/poplar/decisions/0091-move-picker.md` (or next available number — verify with `ls docs/poplar/decisions/ | tail -5`):

```markdown
---
title: Move-to-folder picker — type-to-filter modal, arrow-key nav
status: accepted
date: 2026-05-01
---

## Context

Pass 6 shipped triage with toast + undo for delete / archive / star /
read. The remaining triage primitive — moving messages to an
arbitrary folder — needs a picker UX. The starter prompt left three
open questions: filter UX, folder ordering, no-match handling.

## Decision

The move picker (`internal/ui/movepicker.go`) is a modal overlay
owned by App, mirroring the LinkPicker shape (centerOverlay +
DimANSI + PlaceOverlay; ADR-0087). Triggered by `m` from the
account view on a non-empty `ActionTargets` snapshot.

- **Filter:** type-to-filter, implicit input. Letter keys narrow
  the list (substring, case-insensitive) and the matched portion
  shrinks. Backspace widens. No separate textinput.
- **Navigation inside the modal:** `↑`/`↓` only. `j`/`k` are
  filter input while the picker is open. The modifier-free
  keybinding rule still holds (arrow keys are modifier-free).
- **Folder ordering:** sidebar order at open time (Primary →
  Disposal → Custom). Source folder excluded.
- **No-match feedback:** when filter matches nothing, the list
  area renders a single dim hint `no folders match "<filter>"`;
  Enter is inert. No `ErrorMsg`.
- **Dispatch:** picker emits `MovePickerPickedMsg`; App routes it
  back into AccountTab; AccountTab uses the same
  `buildTriageCmd` shape as delete/archive (factored as
  `buildTriageCmdWithDest`). Toast extended with a `dest` field
  to render `Moved N messages to <Display>`.

## Consequences

- Substring filter scales to Gmail-sized label sets without a new
  textinput dependency.
- `↑`/`↓` is a documented carve-out from "vim-first" navigation,
  scoped to modal text-input contexts only.
- Reusing `buildTriageCmd` keeps the optimistic / undo / error
  rollback flow identical to delete and archive.
- Recent-first ordering and folder creation from the picker are
  deferred to post-1.0.
```

- [ ] **Step 4: Update `docs/poplar/invariants.md`**

Add a binding fact for the move picker. Find the existing
"optimistic triage" fact (the multi-paragraph block under
"Triage actions (delete/archive/star/read) are optimistic..."),
and either:

- Extend it to include "move" in the op list, OR
- Add a new fact directly after it specific to move picker
  (filter UX, arrow-nav, sidebar-order, ADR-0091 reference).

Update the decision index table at the bottom: add `0091` to the
"Optimistic triage with toast/undo" row.

Verify total file size:

```
wc -l docs/poplar/invariants.md
```

Expected: ≤ 300 lines (enforced by `.claude/hooks/claude-md-size.sh`).

- [ ] **Step 5: Update `docs/poplar/STATUS.md`**

- Mark Pass 6.5 `done` in the pass table.
- Replace the starter prompt with the next one (Pass 6.6 — Trash
  retention + manual empty, per the table).
- Keep STATUS ≤ 60 lines.

- [ ] **Step 6: Archive plan + spec**

```
git mv docs/superpowers/plans/2026-05-01-move-picker.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-01-move-picker-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 7: `make check`**

```
make check
```

Expected: green.

- [ ] **Step 8: Commit, push, install**

```bash
git add -A
git commit -m "Pass 6.5: move-to-folder picker with toast + undo

Co-Authored-By: Claude <noreply@anthropic.com>"
git push
make install
```

---

## Self-Review Checklist (writer's notes)

Spec coverage check:
- ✅ Sidebar accessor `OrderedFolders()` — Task 1
- ✅ Msg types — Task 2
- ✅ Toast extension for "move" — Task 3
- ✅ MovePicker model + filter — Task 4
- ✅ MovePicker rendering — Task 5
- ✅ `m` key + dispatch — Task 6
- ✅ App overlay wiring + folder-jump inert — Task 7
- ✅ Help popover row — Task 8
- ✅ Live tmux verify — Task 9
- ✅ Pass-end ritual — Task 10

Type consistency:
- `FolderEntry`, `FolderGroup` defined in sidebar.go (Task 1), used in cmds.go / movepicker.go.
- `MovePickerPickedMsg` carries `UIDs`, `Src`, `Dest` consistently across cmds.go / movepicker.go / account_tab.go / app.go.
- `triageStartedMsg.dest` and `pendingAction.dest` field name match.
- `buildTriageCmd` retained for non-move callers; `buildTriageCmdWithDest` only used by the move path (DRY).

Placeholder scan: none beyond the explicit "verify with `ls docs/poplar/decisions/`" for the next ADR number, which is intentional.

Open ambiguity from spec: spec deferred the choice of "extend `triageStartedMsg` vs separate move-specific msg." Plan picks "extend `triageStartedMsg` with `dest string`" — keeps the dispatch path uniform.
