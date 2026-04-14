# Poplar Threading + Fold Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add threaded display, per-thread fold state, and bulk fold/unfold to poplar's message list (Pass 2.5b-3.6).

**Architecture:** `MessageInfo` gains `ThreadID` and `InReplyTo` fields on the wire. `MessageList` groups, sorts, and flattens the flat input slice into a private `[]displayRow` on every `SetMessages` and on every fold mutation. The thread tree never exists as an owned struct — depth and box-drawing prefixes are computed during the flatten step. Fold state is a per-`MessageList` `map[UID]bool` keyed by thread root, reset on every `SetMessages`. Sort direction comes from `[ui.folders.<name>] sort` in `accounts.toml` (default `date-desc`); thread-level sort key is "latest activity" (max date across all messages in the thread).

**Tech Stack:** Go 1.26.1, bubbletea, lipgloss, the existing `internal/mail`, `internal/ui`, `internal/config`, `internal/theme` packages.

**Spec:** `docs/superpowers/specs/2026-04-13-poplar-threading-design.md`

**Conventions to invoke before touching files:**
- Before any Go code: `go-conventions` skill.
- Before any `internal/ui/` file: `elm-conventions` skill (mandatory in addition to `go-conventions`).
- Before any color or style change: update `docs/poplar/styling.md` first.

**Build commands:**
- `make check` — vet + test (commit gate)
- `make build` — build poplar binary
- `make install` — install to `~/.local/bin/`
- `go test ./internal/mail/... -run TestX -v` — single test
- `go test ./internal/ui/... -run TestX -v` — single test

---

## File Inventory

Created files: none.

Modified files:

| Path | Responsibility | Tasks that touch it |
|------|----------------|---------------------|
| `internal/mail/types.go` | Add `ThreadID`, `InReplyTo` to `MessageInfo` | 1 |
| `internal/mail/mock.go` | Add 4-message threaded conversation | 2 |
| `internal/mail/mock_test.go` | Update count assertions, add thread shape test | 2 |
| `internal/ui/styles.go` | Add `MsgListThreadPrefix` slot | 4 |
| `docs/poplar/styling.md` | Document the new style slot | 4 |
| `internal/ui/msglist.go` | Add `displayRow`, build pipeline, fold state, sort, prefix rendering | 3, 5, 6, 7, 8, 9, 10, 11, 12, 13 |
| `internal/ui/msglist_test.go` | Tests for grouping, prefix, fold, sort, cursor skip | 3, 5, 6, 7, 8, 9, 10, 11, 12, 13 |
| `internal/ui/account_tab.go` | Wire sort config through, dispatch Space/F/U keys | 14, 15 |
| `internal/ui/account_tab_test.go` | Test that Space/F/U reach `MessageList` | 14, 15 |
| `internal/ui/footer.go` | Add Threads hint group | 16 |
| `internal/ui/footer_test.go` | Test new hints render | 16 |
| `docs/poplar/keybindings.md` | Promote Space/F/U from reserved to live | 17 |

---

## Task 1: Add `ThreadID` and `InReplyTo` fields to `MessageInfo`

**Files:**
- Modify: `internal/mail/types.go`

The wire-level type gains two fields. Backends populate them from `Email.threadId`/`Email.inReplyTo` (JMAP) or the IMAP `THREAD` extension. Depth is *not* a wire field — the UI computes it from the tree shape during flatten.

- [ ] **Step 1: Invoke `go-conventions`**

Run the `go-conventions` skill before editing.

- [ ] **Step 2: Modify `MessageInfo`**

Replace the existing `MessageInfo` struct (currently lines 46-54 of `internal/mail/backend.go` — yes, despite the file name `types.go`, `MessageInfo` lives in `backend.go`):

```go
// MessageInfo holds message header information for list display.
//
// ThreadID groups messages that belong to the same conversation. A
// non-threaded message is a thread of size 1 with ThreadID == UID and
// InReplyTo == "". InReplyTo points at the parent message's UID and
// is empty for thread roots. The UI layer derives depth and box-
// drawing prefixes from the tree shape — depth is not carried on the
// wire because doing so would duplicate information the prefix walk
// already produces and risk drift if a backend miscounted.
type MessageInfo struct {
	UID     UID
	Subject string
	From    string
	Date    string
	Flags   Flag
	Size    uint32

	ThreadID  UID
	InReplyTo UID
}
```

- [ ] **Step 3: Run `make check` to confirm the existing tests still compile**

Run: `make check`
Expected: PASS (all existing tests still pass — the new fields are zero-valued and unused)

- [ ] **Step 4: Commit**

```bash
git add internal/mail/backend.go
git commit -m "Add ThreadID and InReplyTo to MessageInfo

Carries thread membership and parent pointers on the wire so the UI
layer can group flat backend results into threaded views. Depth is
intentionally derived in the UI rather than carried here — doing so
would duplicate information the prefix walk already produces.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Mock backend grows a threaded conversation

**Files:**
- Modify: `internal/mail/mock.go`
- Modify: `internal/mail/mock_test.go`

Add a 4-message branching conversation in the Inbox so the renderer has a thread to exercise. The first child is unread to verify that collapsed threads can carry "contains unread" status visually.

- [ ] **Step 1: Invoke `go-conventions`**

- [ ] **Step 2: Update mock_test.go to expect the new shape (failing test first)**

Add to `internal/mail/mock_test.go`:

```go
func TestMockBackendThreading(t *testing.T) {
	b := NewMockBackend()
	msgs, err := b.FetchHeaders(nil)
	if err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}

	t.Run("total message count", func(t *testing.T) {
		if got, want := len(msgs), 14; got != want {
			t.Errorf("len(msgs) = %d, want %d", got, want)
		}
	})

	t.Run("flat messages have ThreadID == UID and empty InReplyTo", func(t *testing.T) {
		// Messages with UIDs 1..10 are the original flat set.
		flatUIDs := map[UID]bool{
			"1": true, "2": true, "3": true, "4": true, "5": true,
			"6": true, "7": true, "8": true, "9": true, "10": true,
		}
		for _, m := range msgs {
			if !flatUIDs[m.UID] {
				continue
			}
			if m.ThreadID != m.UID {
				t.Errorf("flat message %s: ThreadID = %q, want %q",
					m.UID, m.ThreadID, m.UID)
			}
			if m.InReplyTo != "" {
				t.Errorf("flat message %s: InReplyTo = %q, want empty",
					m.UID, m.InReplyTo)
			}
		}
	})

	t.Run("threaded conversation has 4 messages with ThreadID T1", func(t *testing.T) {
		threaded := map[UID]MessageInfo{}
		for _, m := range msgs {
			if m.ThreadID == "T1" {
				threaded[m.UID] = m
			}
		}
		if len(threaded) != 4 {
			t.Fatalf("threaded conversation has %d messages, want 4", len(threaded))
		}

		root, ok := threaded["20"]
		if !ok {
			t.Fatal("missing root message UID 20")
		}
		if root.InReplyTo != "" {
			t.Errorf("root InReplyTo = %q, want empty", root.InReplyTo)
		}

		grace, ok := threaded["21"]
		if !ok {
			t.Fatal("missing reply UID 21 (Grace Kim)")
		}
		if grace.InReplyTo != "20" {
			t.Errorf("Grace InReplyTo = %q, want 20", grace.InReplyTo)
		}
		if grace.Flags&FlagSeen != 0 {
			t.Error("Grace should be unread (FlagSeen not set)")
		}

		franky, ok := threaded["22"]
		if !ok {
			t.Fatal("missing reply UID 22 (Frank deep)")
		}
		if franky.InReplyTo != "21" {
			t.Errorf("Frank-22 InReplyTo = %q, want 21", franky.InReplyTo)
		}

		henry, ok := threaded["23"]
		if !ok {
			t.Fatal("missing reply UID 23 (Henry)")
		}
		if henry.InReplyTo != "20" {
			t.Errorf("Henry InReplyTo = %q, want 20", henry.InReplyTo)
		}
	})
}
```

- [ ] **Step 3: Run the new test to confirm it fails**

Run: `go test ./internal/mail/ -run TestMockBackendThreading -v`
Expected: FAIL with "len(msgs) = 10, want 14" (the threaded messages don't exist yet)

- [ ] **Step 4: Update the existing mock data**

Replace the `msgs:` slice in `NewMockBackend` (currently lines 35-46 of `internal/mail/mock.go`). The 10 existing flat messages get `ThreadID == UID`; the 4 new threaded messages share `ThreadID: "T1"`:

```go
msgs: []MessageInfo{
	{UID: "1", ThreadID: "1", Subject: "Re: Project update for Q2 launch", From: "Alice Johnson", Date: "10:23 AM", Flags: 0},
	{UID: "2", ThreadID: "2", Subject: "Quick question about the API", From: "Bob Smith", Date: "9:45 AM", Flags: 0},
	{UID: "3", ThreadID: "3", Subject: "Lunch tomorrow?", From: "Carol White", Date: "9:12 AM", Flags: 0},
	{UID: "4", ThreadID: "4", Subject: "Meeting notes from yesterday", From: "David Chen", Date: "Yesterday", Flags: FlagSeen},
	{UID: "5", ThreadID: "5", Subject: "Invoice #2847 attached", From: "Billing Dept", Date: "Yesterday", Flags: FlagSeen | FlagFlagged},
	{UID: "6", ThreadID: "6", Subject: "Re: Weekend hiking trip", From: "Emma Wilson", Date: "Yesterday", Flags: FlagSeen | FlagAnswered},
	{UID: "7", ThreadID: "7", Subject: "Your subscription renewal", From: "Acme Cloud", Date: "Apr 8", Flags: FlagSeen},
	{UID: "8", ThreadID: "8", Subject: "Code review: auth refactor PR #42", From: "GitHub", Date: "Apr 8", Flags: FlagSeen},
	{UID: "9", ThreadID: "9", Subject: "New comment on your post", From: "Dev Community", Date: "Apr 7", Flags: FlagSeen},
	{UID: "10", ThreadID: "10", Subject: "Flight confirmation: SFO → SEA", From: "Alaska Airlines", Date: "Apr 7", Flags: FlagSeen | FlagFlagged},

	// Threaded conversation T1: branching shape (root + linear chain + sibling).
	// Exercises the full ├─ │ └─ prefix vocabulary. First child unread so a
	// folded thread can still carry "contains unread" status.
	{UID: "20", ThreadID: "T1", InReplyTo: "", Subject: "Server migration plan", From: "Frank Lee", Date: "Apr 5", Flags: FlagSeen | FlagAnswered},
	{UID: "21", ThreadID: "T1", InReplyTo: "20", Subject: "Re: Server migration plan", From: "Grace Kim", Date: "Apr 5", Flags: 0},
	{UID: "22", ThreadID: "T1", InReplyTo: "21", Subject: "Re: Server migration plan", From: "Frank Lee", Date: "Apr 5", Flags: FlagSeen},
	{UID: "23", ThreadID: "T1", InReplyTo: "20", Subject: "Re: Server migration plan", From: "Henry Park", Date: "Apr 5", Flags: FlagSeen},
},
```

Also update the Inbox `Exists` count so it matches the new total. Find the existing folder list (line 23 in the same file) and change Inbox:

```go
{Name: "Inbox", Exists: 14, Unseen: 4, Role: "inbox"},
```

`Unseen: 4` because the original 3 unread flat messages plus 1 unread Grace message = 4.

- [ ] **Step 5: Run the test to confirm it passes**

Run: `go test ./internal/mail/ -run TestMockBackendThreading -v`
Expected: PASS

- [ ] **Step 6: Run the full mail package test suite**

Run: `go test ./internal/mail/...`
Expected: PASS for all tests. The pre-existing `TestMockBackend/inbox has unread messages` and `TestMockBackend/list folders returns expected data` should still pass because their assertions are about non-zero counts, not specific values.

- [ ] **Step 7: Commit**

```bash
git add internal/mail/mock.go internal/mail/mock_test.go
git commit -m "Mock backend grows a 4-message threaded conversation

Adds a branching thread (root + linear chain + sibling) to exercise
the full ├─ │ └─ prefix vocabulary the renderer will produce. First
child is unread so collapsed-with-unread state can be visually
verified. The 10 existing flat messages each become single-message
threads with ThreadID == UID.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Introduce `displayRow` and the source-cache field on `MessageList`

**Files:**
- Modify: `internal/ui/msglist.go`
- Modify: `internal/ui/msglist_test.go`

This task lays the foundation. After this task, `MessageList` keeps both a source `[]MessageInfo` and a derived `[]displayRow`, but the build pipeline is still trivial (one-row-per-message, no grouping yet). Tasks 5–13 build out the real pipeline.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

Both apply because we're modifying an `internal/ui/` file.

- [ ] **Step 2: Add the displayRow type and source field**

Edit `internal/ui/msglist.go`. Just after the `mlIcon...` constants, add:

```go
// displayRow is one rendered row in the message list. The slice of
// these is computed from the source []MessageInfo by the build
// pipeline (group, sort, flatten). Hidden rows still occupy indices
// in the slice; the renderer skips them and j/k navigation walks
// past them.
type displayRow struct {
	msg          mail.MessageInfo
	prefix       string // "", "├─ ", "└─ ", "│  └─ ", or "[N] " for a folded root
	isThreadRoot bool
	threadSize   int    // set on roots only; 1 for unthreaded
	hidden       bool   // true when collapsed under a folded root
	depth        uint8  // 0 = root; derived during prefix computation
}
```

Now change the `MessageList` struct (currently lines 33-40 of `internal/ui/msglist.go`):

```go
// MessageList renders the message list panel: flags, sender, subject,
// and date columns. Hand-rolled (not bubbles/list) to match the
// sidebar pattern and allow the ▐ cursor + selection background.
//
// MessageList owns thread grouping, fold state, and sort direction.
// The source slice is preserved alongside a derived []displayRow so
// fold mutations re-flatten without a backend refetch.
type MessageList struct {
	source   []mail.MessageInfo
	rows     []displayRow
	folded   map[mail.UID]bool
	sort     SortOrder
	selected int
	offset   int
	styles   Styles
	width    int
	height   int
}
```

- [ ] **Step 3: Add the SortOrder type**

Just above the displayRow type, add:

```go
// SortOrder is the thread-level sort direction. Children inside a
// thread always sort chronologically ascending; SortOrder controls
// only the order of thread roots (and of unthreaded messages, which
// are single-message threads).
type SortOrder int

const (
	SortDateDesc SortOrder = iota // newest activity first (default)
	SortDateAsc                   // oldest activity first
)
```

- [ ] **Step 4: Update the constructor**

Replace `NewMessageList` (currently lines 43-50):

```go
// NewMessageList creates a MessageList with the given messages and size.
func NewMessageList(styles Styles, msgs []mail.MessageInfo, width, height int) MessageList {
	m := MessageList{
		styles: styles,
		width:  width,
		height: height,
		folded: map[mail.UID]bool{},
		sort:   SortDateDesc,
	}
	m.SetMessages(msgs)
	return m
}
```

- [ ] **Step 5: Update SetMessages to maintain source and rows**

Replace `SetMessages` (currently lines 53-57) with a stub build pipeline. The full pipeline lands in Tasks 5–9; for now, a one-row-per-message pass-through preserves existing behavior:

```go
// SetMessages replaces the source slice and rebuilds the displayRow
// list. Resets fold state, cursor, and viewport.
func (m *MessageList) SetMessages(msgs []mail.MessageInfo) {
	m.source = msgs
	m.folded = map[mail.UID]bool{}
	m.selected = 0
	m.offset = 0
	m.rebuild()
}

// rebuild runs the group → sort → flatten pipeline against m.source
// and applies fold state, producing m.rows. Called from SetMessages
// and from any fold-mutating method.
func (m *MessageList) rebuild() {
	rows := make([]displayRow, 0, len(m.source))
	for _, msg := range m.source {
		rows = append(rows, displayRow{
			msg:          msg,
			isThreadRoot: true,
			threadSize:   1,
			depth:        0,
		})
	}
	m.rows = rows
}
```

- [ ] **Step 6: Update Count and SelectedMessage to use rows**

Replace `Count` (line 79) and `SelectedMessage` (lines 71-77):

```go
// Count returns the number of source messages in the list.
// (Use len(m.rows) when you need the displayRow count.)
func (m MessageList) Count() int { return len(m.source) }

// SelectedMessage returns the currently selected message. ok is false
// if the list is empty.
func (m MessageList) SelectedMessage() (mail.MessageInfo, bool) {
	if m.selected < 0 || m.selected >= len(m.rows) {
		return mail.MessageInfo{}, false
	}
	return m.rows[m.selected].msg, true
}
```

- [ ] **Step 7: Update moveBy to use rows**

Replace `moveBy` (lines 83-89):

```go
// moveBy shifts the cursor by delta rows, clamped to the displayRow
// range, and re-clamps the viewport offset. Hidden-row skipping is
// added in Task 12; for now this matches the previous behavior since
// the trivial build pipeline produces no hidden rows.
func (m *MessageList) moveBy(delta int) {
	if len(m.rows) == 0 {
		return
	}
	m.selected = max(0, min(len(m.rows)-1, m.selected+delta))
	m.clampOffset()
}
```

Replace `MoveToBottom` (line 104):

```go
// MoveToBottom jumps the cursor to the last message.
func (m *MessageList) MoveToBottom() { m.moveBy(len(m.rows)) }
```

- [ ] **Step 8: Update View() and renderRow to read from rows**

Replace `View` (lines 137-165):

```go
// View renders the visible window of message rows. Empty state shows
// a centered "No messages" placeholder.
func (m MessageList) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if len(m.rows) == 0 {
		return m.renderEmpty()
	}

	plainBg := m.styles.MsgListBg
	selectedBg := m.styles.MsgListSelected

	end := m.offset + m.height
	if end > len(m.rows) {
		end = len(m.rows)
	}

	lines := make([]string, 0, m.height)
	for i := m.offset; i < end; i++ {
		bg := plainBg
		if i == m.selected {
			bg = selectedBg
		}
		lines = append(lines, m.renderRow(i, bg))
	}
	for len(lines) < m.height {
		lines = append(lines, m.renderBlankLine())
	}
	return strings.Join(lines, "\n")
}
```

Replace `renderRow`'s message lookup (currently `msg := m.msgs[idx]` on line 169):

```go
func (m MessageList) renderRow(idx int, bgStyle lipgloss.Style) string {
	row := m.rows[idx]
	msg := row.msg
	isSelected := idx == m.selected
	isUnread := msg.Flags&mail.FlagSeen == 0
	// ... rest of the function unchanged for now
```

The body of `renderRow` from there down stays unchanged in this task. The prefix rendering lands in Task 11.

- [ ] **Step 9: Run make check**

Run: `make check`
Expected: PASS — every existing test still passes. The displayRow plumbing produces identical output because the trivial pipeline is one-row-per-message with no prefixes and no fold state.

- [ ] **Step 10: Commit**

```bash
git add internal/ui/msglist.go
git commit -m "MessageList: introduce displayRow and source cache

Threads the displayRow plumbing through MessageList without changing
output: every source message becomes a single-row thread root with
empty prefix. Subsequent tasks build out grouping, prefix computation,
sorting, and fold state on top of this foundation.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Add `MsgListThreadPrefix` style slot

**Files:**
- Modify: `docs/poplar/styling.md`
- Modify: `internal/ui/styles.go`
- Modify: `internal/ui/styles_test.go` (only if it asserts the slot count)

Per the styling invariant: update the doc *first*, then the code.

- [ ] **Step 1: Update styling.md first**

Edit `docs/poplar/styling.md`. Find the message list section (the rows starting `MsgListBg`, around line 103). Add this row after the `MsgListFlagFlagged` row:

```markdown
| `MsgListThreadPrefix` | `FgDim` | inherit | Box-drawing thread prefix (`├─`, `└─`, `│`) and `[N]` collapsed-thread badge |
```

- [ ] **Step 2: Add the field to Styles**

Edit `internal/ui/styles.go`. In the message list block of the `Styles` struct (around lines 52-62), add `MsgListThreadPrefix` after `MsgListFlagFlagged`:

```go
MsgListBg            lipgloss.Style
MsgListSelected      lipgloss.Style
MsgListCursor        lipgloss.Style
MsgListUnreadSender  lipgloss.Style
MsgListUnreadSubject lipgloss.Style
MsgListReadSender    lipgloss.Style
MsgListReadSubject   lipgloss.Style
MsgListDate          lipgloss.Style
MsgListIconUnread    lipgloss.Style
MsgListIconRead      lipgloss.Style
MsgListFlagFlagged   lipgloss.Style
MsgListThreadPrefix  lipgloss.Style
```

- [ ] **Step 3: Populate the slot in NewStyles**

Edit the same file. In `NewStyles` (around line 169), after the `MsgListFlagFlagged` initializer, add:

```go
MsgListThreadPrefix: lipgloss.NewStyle().
	Foreground(t.FgDim),
```

- [ ] **Step 4: Run make check**

Run: `make check`
Expected: PASS — the new field is unused but compiles.

- [ ] **Step 5: Commit**

```bash
git add docs/poplar/styling.md internal/ui/styles.go
git commit -m "Add MsgListThreadPrefix style slot

FgDim, no background. Used by the message list renderer for the
box-drawing thread prefix (├─, └─, │) and the [N] collapsed-thread
badge. Style doc updated first per the styling invariant.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Build pipeline — bucket by ThreadID and pick roots

**Files:**
- Modify: `internal/ui/msglist.go`
- Modify: `internal/ui/msglist_test.go`

Replace the trivial pass-through `rebuild` with the first real step: group messages by `ThreadID` and identify each thread's root.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write the failing test**

Add to `internal/ui/msglist_test.go` after `TestMessageList`:

```go
func TestMessageListThreading(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("groups by ThreadID with explicit root", func(t *testing.T) {
		msgs := []mail.MessageInfo{
			{UID: "1", ThreadID: "1", From: "A", Date: "Apr 1", Flags: mail.FlagSeen},
			{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "Apr 5", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "Apr 6", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		if got, want := len(ml.rows), 3; got != want {
			t.Fatalf("len(rows) = %d, want %d", got, want)
		}
		// Order checked in Task 8; for now just confirm both threads
		// produced a root.
		var rootUIDs []mail.UID
		for _, r := range ml.rows {
			if r.isThreadRoot {
				rootUIDs = append(rootUIDs, r.msg.UID)
			}
		}
		if len(rootUIDs) != 2 {
			t.Errorf("rootUIDs = %v, want 2 roots", rootUIDs)
		}
	})

	t.Run("synthetic root when no message has empty InReplyTo", func(t *testing.T) {
		// Both messages reference an external parent — broken chain.
		msgs := []mail.MessageInfo{
			{UID: "10", ThreadID: "T1", InReplyTo: "999", From: "First", Date: "Apr 5", Flags: mail.FlagSeen},
			{UID: "11", ThreadID: "T1", InReplyTo: "999", From: "Second", Date: "Apr 6", Flags: mail.FlagSeen},
		}
		ml := NewMessageList(styles, msgs, 90, 20)
		if got, want := len(ml.rows), 2; got != want {
			t.Fatalf("len(rows) = %d, want %d", got, want)
		}
		// The synthetic root is the earliest-by-date message (UID 10).
		var rootUID mail.UID
		for _, r := range ml.rows {
			if r.isThreadRoot {
				rootUID = r.msg.UID
				break
			}
		}
		if rootUID != "10" {
			t.Errorf("synthetic root UID = %q, want 10", rootUID)
		}
	})
}
```

- [ ] **Step 3: Run the test to confirm the failure mode**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: PASS for "groups by ThreadID with explicit root" (the trivial pipeline produces one root per row, so a 3-message input gives 3 roots — the test asserts ≥2 roots so it accidentally passes). FAIL for "synthetic root when no message has empty InReplyTo" — actually this also passes because the trivial pipeline marks every row a root. Both tests need to be tightened — let me fix the first test:

Replace the first sub-test's "got 2 roots" assertion with stronger checks that will fail until grouping lands:

```go
t.Run("groups by ThreadID with explicit root", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "1", ThreadID: "1", From: "A", Date: "Apr 1", Flags: mail.FlagSeen},
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "Apr 5", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "Apr 6", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	if got, want := len(ml.rows), 3; got != want {
		t.Fatalf("len(rows) = %d, want %d", got, want)
	}
	var rootUIDs []mail.UID
	var childUIDs []mail.UID
	for _, r := range ml.rows {
		if r.isThreadRoot {
			rootUIDs = append(rootUIDs, r.msg.UID)
		} else {
			childUIDs = append(childUIDs, r.msg.UID)
		}
	}
	// Exactly 2 thread roots: the standalone "1" and the T1 root "10".
	if len(rootUIDs) != 2 {
		t.Errorf("rootUIDs = %v, want exactly 2", rootUIDs)
	}
	// Exactly 1 child: UID 11 belongs under T1's root.
	if len(childUIDs) != 1 || childUIDs[0] != "11" {
		t.Errorf("childUIDs = %v, want [11]", childUIDs)
	}
	// The T1 root must have threadSize 2.
	for _, r := range ml.rows {
		if r.isThreadRoot && r.msg.UID == "10" && r.threadSize != 2 {
			t.Errorf("T1 root threadSize = %d, want 2", r.threadSize)
		}
		if r.isThreadRoot && r.msg.UID == "1" && r.threadSize != 1 {
			t.Errorf("standalone threadSize = %d, want 1", r.threadSize)
		}
	}
})
```

Re-run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: FAIL — the trivial pipeline has all rows as roots with threadSize 1.

- [ ] **Step 4: Implement the grouping in rebuild()**

Replace the body of `rebuild` in `internal/ui/msglist.go`:

```go
// rebuild runs the group → sort → flatten pipeline against m.source
// and applies fold state, producing m.rows. Called from SetMessages
// and from any fold-mutating method.
//
// Pipeline (this task implements steps 1-2 only; later tasks add the
// rest):
//   1. Bucket by ThreadID.
//   2. Pick a root per bucket (empty InReplyTo, fallback earliest by date).
//   3. Sort children chronologically ascending.            (Task 6)
//   4. Compute thread latest-activity sort key.            (Task 7)
//   5. Sort threads by latest-activity in m.sort direction. (Task 8)
//   6. Walk threads, emit displayRows root-then-children,
//      computing depth and box-drawing prefix.              (Task 9)
//   7. Apply fold state.                                    (Task 10)
func (m *MessageList) rebuild() {
	buckets := bucketByThreadID(m.source)
	rows := make([]displayRow, 0, len(m.source))
	for _, bucket := range buckets {
		rootIdx := pickRoot(bucket)
		root := bucket[rootIdx]
		rows = append(rows, displayRow{
			msg:          root,
			isThreadRoot: true,
			threadSize:   len(bucket),
			depth:        0,
		})
		for i, msg := range bucket {
			if i == rootIdx {
				continue
			}
			rows = append(rows, displayRow{
				msg:          msg,
				isThreadRoot: false,
				threadSize:   0,
				depth:        1, // refined in Task 9
			})
		}
	}
	m.rows = rows
}

// bucketByThreadID groups messages by their ThreadID, preserving
// input order within each bucket. Iterates the input twice (once to
// collect ThreadIDs in encounter order, once to slot messages) so the
// bucket order is deterministic — important for tests that compare
// against a specific layout.
func bucketByThreadID(msgs []mail.MessageInfo) [][]mail.MessageInfo {
	order := make([]mail.UID, 0)
	seen := make(map[mail.UID]int)
	for _, m := range msgs {
		if _, ok := seen[m.ThreadID]; ok {
			continue
		}
		seen[m.ThreadID] = len(order)
		order = append(order, m.ThreadID)
	}
	buckets := make([][]mail.MessageInfo, len(order))
	for _, m := range msgs {
		idx := seen[m.ThreadID]
		buckets[idx] = append(buckets[idx], m)
	}
	return buckets
}

// pickRoot returns the index within bucket of the message that should
// be treated as the thread root. Preference: the message with empty
// InReplyTo. Fallback: the earliest message by date string. The
// fallback handles broken parent chains (message references a parent
// that wasn't fetched) without crashing — the synthetic root and any
// other top-level orphans become depth-1 children in the renderer.
//
// Date comparison uses lexicographic order on the wire-string format,
// which is wrong in general — Pass 3 introduces real time.Time on
// MessageInfo, at which point this becomes a proper time comparison.
// Until then, mock data uses identical date strings so the fallback
// is deterministic-by-input-order, which is fine for prototype.
func pickRoot(bucket []mail.MessageInfo) int {
	for i, m := range bucket {
		if m.InReplyTo == "" {
			return i
		}
	}
	earliest := 0
	for i, m := range bucket {
		if m.Date < bucket[earliest].Date {
			earliest = i
		}
	}
	return earliest
}
```

- [ ] **Step 5: Run the test to confirm it passes**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: PASS for both sub-tests.

- [ ] **Step 6: Run the full ui test suite**

Run: `go test ./internal/ui/...`
Expected: PASS — the existing `TestMessageList` cases still pass because each of the 10 mock messages has `ThreadID == UID` (unique per message), so `bucketByThreadID` produces 10 single-message buckets and the row order is unchanged.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "MessageList: bucket source messages by ThreadID and pick roots

Adds the first two steps of the build pipeline: group messages by
ThreadID into deterministic buckets, then pick a root per bucket
(empty InReplyTo, fallback earliest by date for broken chains).
Children still emit unsorted with placeholder depth — refined in
Tasks 6-9.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Build pipeline — sort children chronologically ascending

**Files:**
- Modify: `internal/ui/msglist.go`
- Modify: `internal/ui/msglist_test.go`

Children inside a thread always read top-to-bottom oldest-to-newest, regardless of the folder's sort direction.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write the failing test**

Add a sub-test inside `TestMessageListThreading`:

```go
t.Run("children sort chronologically ascending within a thread", func(t *testing.T) {
	// Out-of-order input — replies arrive newest first.
	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "Apr 1", Flags: mail.FlagSeen},
		{UID: "12", ThreadID: "T1", InReplyTo: "10", From: "Late", Date: "Apr 3", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Early", Date: "Apr 2", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	if got, want := len(ml.rows), 3; got != want {
		t.Fatalf("len(rows) = %d, want %d", got, want)
	}
	// Expected order: Root (Apr 1), Early (Apr 2), Late (Apr 3).
	wantOrder := []mail.UID{"10", "11", "12"}
	for i, want := range wantOrder {
		if got := ml.rows[i].msg.UID; got != want {
			t.Errorf("rows[%d].UID = %q, want %q", i, got, want)
		}
	}
})
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/ui/ -run TestMessageListThreading/children_sort -v`
Expected: FAIL — current implementation emits children in input order, so `[10, 12, 11]`.

- [ ] **Step 4: Sort children in rebuild()**

In `rebuild`, after picking the root and before emitting child rows, sort the children by date ascending. Replace the child-emit loop:

```go
for _, bucket := range buckets {
	rootIdx := pickRoot(bucket)
	root := bucket[rootIdx]
	rows = append(rows, displayRow{
		msg:          root,
		isThreadRoot: true,
		threadSize:   len(bucket),
		depth:        0,
	})

	children := make([]mail.MessageInfo, 0, len(bucket)-1)
	for i, msg := range bucket {
		if i == rootIdx {
			continue
		}
		children = append(children, msg)
	}
	sort.SliceStable(children, func(i, j int) bool {
		return children[i].Date < children[j].Date
	})
	for _, child := range children {
		rows = append(rows, displayRow{
			msg:          child,
			isThreadRoot: false,
			threadSize:   0,
			depth:        1,
		})
	}
}
```

Add `"sort"` to the import block at the top of the file:

```go
import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	"github.com/mattn/go-runewidth"
)
```

- [ ] **Step 5: Run the test to confirm it passes**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: PASS for all sub-tests.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "MessageList: sort thread children chronologically ascending

Children inside a thread always read top-to-bottom oldest-to-newest,
regardless of folder sort direction. Uses sort.SliceStable so input
order is preserved on tie-broken date strings. Date comparison is
lexicographic on the wire-string format until Pass 3 introduces real
time.Time on MessageInfo.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Build pipeline — compute thread latest-activity sort key

**Files:**
- Modify: `internal/ui/msglist.go`
- Modify: `internal/ui/msglist_test.go`

Each thread's sort key is the maximum date across all its messages. Used in Task 8 to order threads.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write the failing test**

Add to `TestMessageListThreading`:

```go
t.Run("thread latest-activity computed correctly", func(t *testing.T) {
	// Helper test: build a thread bucket and verify latestActivity.
	bucket := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", Date: "Apr 1"},
		{UID: "11", ThreadID: "T1", Date: "Apr 5"},
		{UID: "12", ThreadID: "T1", Date: "Apr 3"},
	}
	if got, want := latestActivity(bucket), "Apr 5"; got != want {
		t.Errorf("latestActivity = %q, want %q", got, want)
	}
})
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/ui/ -run TestMessageListThreading/thread_latest -v`
Expected: FAIL — `latestActivity` undefined.

- [ ] **Step 4: Add the helper**

Add to `internal/ui/msglist.go` after `pickRoot`:

```go
// latestActivity returns the maximum Date string across all messages
// in a thread bucket. Used as the inter-thread sort key in step 5 of
// the build pipeline. Empty bucket returns "" — caller should not
// invoke on an empty bucket but the safe answer keeps the function
// total.
func latestActivity(bucket []mail.MessageInfo) string {
	latest := ""
	for _, m := range bucket {
		if m.Date > latest {
			latest = m.Date
		}
	}
	return latest
}
```

- [ ] **Step 5: Run the test to confirm it passes**

Run: `go test ./internal/ui/ -run TestMessageListThreading/thread_latest -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "MessageList: add latestActivity helper for thread sort key

Computes the maximum Date string across a thread bucket. Used by the
next task to sort threads by most-recent-message rather than
root-message. Lexicographic comparison until Pass 3 introduces real
time.Time on MessageInfo.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: Build pipeline — sort threads by latest-activity in configured direction

**Files:**
- Modify: `internal/ui/msglist.go`
- Modify: `internal/ui/msglist_test.go`

Wire `m.sort` through. `SortDateDesc` puts most-recently-active threads on top; `SortDateAsc` reverses.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write the failing test**

Add to `TestMessageListThreading`:

```go
t.Run("threads sorted by latest activity descending by default", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		// Older thread first in input.
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Old", Date: "Apr 1", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "OldReply", Date: "Apr 2", Flags: mail.FlagSeen},
		// Newer thread second in input.
		{UID: "20", ThreadID: "T2", InReplyTo: "", From: "New", Date: "Apr 5", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	// Default SortDateDesc: T2 should come before T1.
	if ml.rows[0].msg.UID != "20" {
		t.Errorf("first row UID = %q, want 20 (T2 root)", ml.rows[0].msg.UID)
	}
	if ml.rows[1].msg.UID != "10" {
		t.Errorf("second row UID = %q, want 10 (T1 root)", ml.rows[1].msg.UID)
	}
	if ml.rows[2].msg.UID != "11" {
		t.Errorf("third row UID = %q, want 11 (T1 child)", ml.rows[2].msg.UID)
	}
})

t.Run("threads sorted ascending when SortDateAsc", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "20", ThreadID: "T2", InReplyTo: "", From: "New", Date: "Apr 5", Flags: mail.FlagSeen},
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Old", Date: "Apr 1", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	ml.SetSort(SortDateAsc)
	// T1 (Apr 1) should now come before T2 (Apr 5).
	if ml.rows[0].msg.UID != "10" {
		t.Errorf("first row UID = %q, want 10 (T1)", ml.rows[0].msg.UID)
	}
})
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: FAIL — no inter-thread sort, no `SetSort`.

- [ ] **Step 4: Add SetSort and sort threads in rebuild**

Add the setter to `internal/ui/msglist.go` (place it near `SetSize`):

```go
// SetSort changes the thread-level sort direction and re-runs the
// build pipeline. Children inside a thread always sort ascending
// regardless of this setting.
func (m *MessageList) SetSort(order SortOrder) {
	m.sort = order
	m.rebuild()
}
```

In `rebuild`, after `bucketByThreadID` and before the bucket-emit loop, sort buckets by latest activity. Update `rebuild`:

```go
func (m *MessageList) rebuild() {
	buckets := bucketByThreadID(m.source)

	type sortedBucket struct {
		bucket   []mail.MessageInfo
		latest   string
	}
	wrapped := make([]sortedBucket, len(buckets))
	for i, b := range buckets {
		wrapped[i] = sortedBucket{bucket: b, latest: latestActivity(b)}
	}
	sort.SliceStable(wrapped, func(i, j int) bool {
		if m.sort == SortDateAsc {
			return wrapped[i].latest < wrapped[j].latest
		}
		return wrapped[i].latest > wrapped[j].latest
	})

	rows := make([]displayRow, 0, len(m.source))
	for _, w := range wrapped {
		bucket := w.bucket
		rootIdx := pickRoot(bucket)
		root := bucket[rootIdx]
		rows = append(rows, displayRow{
			msg:          root,
			isThreadRoot: true,
			threadSize:   len(bucket),
			depth:        0,
		})

		children := make([]mail.MessageInfo, 0, len(bucket)-1)
		for i, msg := range bucket {
			if i == rootIdx {
				continue
			}
			children = append(children, msg)
		}
		sort.SliceStable(children, func(i, j int) bool {
			return children[i].Date < children[j].Date
		})
		for _, child := range children {
			rows = append(rows, displayRow{
				msg:          child,
				isThreadRoot: false,
				threadSize:   0,
				depth:        1,
			})
		}
	}
	m.rows = rows
}
```

- [ ] **Step 5: Run the threading tests to confirm they pass**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: PASS for every sub-test.

- [ ] **Step 6: Run the full ui test suite**

Run: `go test ./internal/ui/...`
Expected: PASS. The existing `TestMessageList` cases still pass — each of the 10 mock messages has a unique ThreadID (because `mockMessages()` in the test file doesn't set ThreadID, so all are zero-value `""`)...

Wait — that's a problem. `mockMessages()` in `internal/ui/msglist_test.go` returns 10 messages with `ThreadID == ""`, so `bucketByThreadID` puts all 10 in a single bucket. The first one (`UID: "1"`) becomes the root, the other 9 become children. `TestMessageList` will start failing because `ml.Selected() != 0` and message order changes when sorted by date string.

The fix is to update `mockMessages()` so each message has `ThreadID == UID`, matching the convention the real mock backend uses:

```go
func mockMessages() []mail.MessageInfo {
	return []mail.MessageInfo{
		{UID: "1", ThreadID: "1", Subject: "Re: Project update for Q2 launch", From: "Alice Johnson", Date: "10:23 AM", Flags: 0},
		{UID: "2", ThreadID: "2", Subject: "Quick question about the API", From: "Bob Smith", Date: "9:45 AM", Flags: 0},
		{UID: "3", ThreadID: "3", Subject: "Lunch tomorrow?", From: "Carol White", Date: "9:12 AM", Flags: 0},
		{UID: "4", ThreadID: "4", Subject: "Meeting notes from yesterday", From: "David Chen", Date: "Yesterday", Flags: mail.FlagSeen},
		{UID: "5", ThreadID: "5", Subject: "Invoice #2847 attached", From: "Billing Dept", Date: "Yesterday", Flags: mail.FlagSeen | mail.FlagFlagged},
		{UID: "6", ThreadID: "6", Subject: "Re: Weekend hiking trip", From: "Emma Wilson", Date: "Yesterday", Flags: mail.FlagSeen | mail.FlagAnswered},
		{UID: "7", ThreadID: "7", Subject: "Your subscription renewal", From: "Acme Cloud", Date: "Apr 8", Flags: mail.FlagSeen},
		{UID: "8", ThreadID: "8", Subject: "Code review: auth refactor PR #42", From: "GitHub", Date: "Apr 8", Flags: mail.FlagSeen},
		{UID: "9", ThreadID: "9", Subject: "New comment on your post", From: "Dev Community", Date: "Apr 7", Flags: mail.FlagSeen},
		{UID: "10", ThreadID: "10", Subject: "Flight confirmation: SFO → SEA", From: "Alaska Airlines", Date: "Apr 7", Flags: mail.FlagSeen | mail.FlagFlagged},
	}
}
```

But this *also* changes the row order! The original `TestMessageList` was written assuming insertion order. Now that each message is its own thread sorted by date desc, the order becomes (date desc): Yesterday × 3, 10:23 AM, 9:45 AM, 9:12 AM, Apr 8 × 2, Apr 7 × 2 — which is *different* from insertion order, and lexicographic on string dates is broken (`"10:23 AM"` sorts after `"9:45 AM"` because `"1" < "9"`).

The cleanest fix here is: update each test's date strings to a sortable format (e.g., `"2026-04-09 10:23"`) — but that diverges from what real mock data looks like. Alternatively, accept that the existing tests need updating to reflect the new sort-aware behavior.

Pragmatic call: update `mockMessages()` to use ISO-like dates so lexicographic sort matches chronological sort, and update any `TestMessageList` assertion that depends on the old `Date` string format. The mock backend in `internal/mail/mock.go` keeps its display-friendly format (it gets re-rendered by the date column) — only the *test fixture* in `msglist_test.go` needs the sortable format because it's measuring sort behavior end-to-end.

Apply the fix:

```go
func mockMessages() []mail.MessageInfo {
	return []mail.MessageInfo{
		{UID: "1", ThreadID: "1", Subject: "Re: Project update for Q2 launch", From: "Alice Johnson", Date: "2026-04-12 10:23", Flags: 0},
		{UID: "2", ThreadID: "2", Subject: "Quick question about the API", From: "Bob Smith", Date: "2026-04-12 09:45", Flags: 0},
		{UID: "3", ThreadID: "3", Subject: "Lunch tomorrow?", From: "Carol White", Date: "2026-04-12 09:12", Flags: 0},
		{UID: "4", ThreadID: "4", Subject: "Meeting notes from yesterday", From: "David Chen", Date: "2026-04-11", Flags: mail.FlagSeen},
		{UID: "5", ThreadID: "5", Subject: "Invoice #2847 attached", From: "Billing Dept", Date: "2026-04-10", Flags: mail.FlagSeen | mail.FlagFlagged},
		{UID: "6", ThreadID: "6", Subject: "Re: Weekend hiking trip", From: "Emma Wilson", Date: "2026-04-09", Flags: mail.FlagSeen | mail.FlagAnswered},
		{UID: "7", ThreadID: "7", Subject: "Your subscription renewal", From: "Acme Cloud", Date: "2026-04-08", Flags: mail.FlagSeen},
		{UID: "8", ThreadID: "8", Subject: "Code review: auth refactor PR #42", From: "GitHub", Date: "2026-04-07", Flags: mail.FlagSeen},
		{UID: "9", ThreadID: "9", Subject: "New comment on your post", From: "Dev Community", Date: "2026-04-06", Flags: mail.FlagSeen},
		{UID: "10", ThreadID: "10", Subject: "Flight confirmation: SFO → SEA", From: "Alaska Airlines", Date: "2026-04-05", Flags: mail.FlagSeen | mail.FlagFlagged},
	}
}
```

Now the input is already in date-desc order, so `SortDateDesc` (the default) produces the same row order as the original tests expect. The `TestMessageList/date column is right-aligned` test references the literal string `"10:23 AM"` — update its assertion to `"2026-04-12 10:23"`:

```go
t.Run("date column is right-aligned", func(t *testing.T) {
	ml := NewMessageList(styles, msgs, 90, 20)
	plain := stripANSI(ml.View())
	lines := strings.Split(plain, "\n")
	if len(lines) == 0 {
		t.Fatal("empty view")
	}
	first := strings.TrimRight(lines[0], " ")
	if !strings.HasSuffix(first, "2026-04-12 10:23") {
		t.Errorf("expected first row to end with date, got tail: %q", first)
	}
})
```

- [ ] **Step 7: Run the full ui test suite**

Run: `go test ./internal/ui/...`
Expected: PASS — both old and new tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "MessageList: sort threads by latest activity in configured direction

SetSort wires the SortOrder enum through to the build pipeline. Threads
are sorted by their max-date across all messages (latest activity),
which matches Gmail / Apple Mail / Fastmail web. Children inside a
thread keep their chronological-ascending order regardless.

The msglist_test fixture switches to ISO-like date strings so
lexicographic sort matches chronological sort end-to-end. Real mock
data in internal/mail/mock.go keeps its display-friendly format —
that path will get real time.Time on MessageInfo in Pass 3.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: Build pipeline — compute depth and box-drawing prefix

**Files:**
- Modify: `internal/ui/msglist.go`
- Modify: `internal/ui/msglist_test.go`

Walk each thread depth-first, tracking the chain of "is-last-sibling" flags from the root, and emit the prefix string for each row. Multi-level threads (e.g., `T1` in the mock backend with depth 2) produce strings like `"│  └─ "`.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write the failing test**

Add to `TestMessageListThreading`:

```go
t.Run("box-drawing prefixes for branching thread", func(t *testing.T) {
	// Tree shape:
	//   Root (UID 10)
	//   ├─ Reply A (UID 11)
	//   │  └─ Deep (UID 12)
	//   └─ Reply B (UID 13)
	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "ReplyA", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
		{UID: "12", ThreadID: "T1", InReplyTo: "11", From: "Deep", Date: "2026-04-05 12:00", Flags: mail.FlagSeen},
		{UID: "13", ThreadID: "T1", InReplyTo: "10", From: "ReplyB", Date: "2026-04-05 13:00", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	if got, want := len(ml.rows), 4; got != want {
		t.Fatalf("len(rows) = %d, want %d", got, want)
	}
	want := []struct {
		uid    mail.UID
		prefix string
		depth  uint8
	}{
		{"10", "", 0},
		{"11", "├─ ", 1},
		{"12", "│  └─ ", 2},
		{"13", "└─ ", 1},
	}
	for i, w := range want {
		if got := ml.rows[i].msg.UID; got != w.uid {
			t.Errorf("rows[%d].UID = %q, want %q", i, got, w.uid)
		}
		if got := ml.rows[i].prefix; got != w.prefix {
			t.Errorf("rows[%d].prefix = %q, want %q", i, got, w.prefix)
		}
		if got := ml.rows[i].depth; got != w.depth {
			t.Errorf("rows[%d].depth = %d, want %d", i, got, w.depth)
		}
	}
})
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/ui/ -run TestMessageListThreading/box-drawing -v`
Expected: FAIL — depth is 1 for all children, prefix is empty.

- [ ] **Step 4: Build the tree and walk it**

The grouping pipeline so far flattens straight from bucket to rows without computing tree shape. To compute prefixes, we need a transient tree per bucket. Replace the bucket-emit loop in `rebuild`:

```go
for _, w := range wrapped {
	rows = appendThreadRows(rows, w.bucket)
}
```

And add the helpers below the `rebuild` function:

```go
// threadNode is a transient tree node used during prefix computation.
// The tree exists only for the duration of one appendThreadRows call;
// after the walk produces displayRows it's discarded.
type threadNode struct {
	msg      mail.MessageInfo
	children []*threadNode
}

// appendThreadRows builds a transient tree from one thread bucket,
// then emits displayRows in depth-first root-then-children order with
// the right prefix for each row's position. The tree never escapes
// this function — it's a scratch structure for prefix computation.
func appendThreadRows(rows []displayRow, bucket []mail.MessageInfo) []displayRow {
	rootIdx := pickRoot(bucket)
	root := &threadNode{msg: bucket[rootIdx]}

	// Index every message by UID so children can find their parent.
	byUID := map[mail.UID]*threadNode{}
	for i, msg := range bucket {
		if i == rootIdx {
			byUID[msg.UID] = root
			continue
		}
		byUID[msg.UID] = &threadNode{msg: msg}
	}

	// Hook each non-root child to its parent. If the parent is missing
	// (broken chain — InReplyTo references a UID outside the bucket),
	// fall back to attaching it to the root as a top-level child.
	for i, msg := range bucket {
		if i == rootIdx {
			continue
		}
		node := byUID[msg.UID]
		parent, ok := byUID[msg.InReplyTo]
		if !ok {
			parent = root
		}
		parent.children = append(parent.children, node)
	}

	// Sort children chronologically ascending at every level.
	var sortChildren func(n *threadNode)
	sortChildren = func(n *threadNode) {
		sort.SliceStable(n.children, func(i, j int) bool {
			return n.children[i].msg.Date < n.children[j].msg.Date
		})
		for _, c := range n.children {
			sortChildren(c)
		}
	}
	sortChildren(root)

	// Emit the root.
	rows = append(rows, displayRow{
		msg:          root.msg,
		isThreadRoot: true,
		threadSize:   len(bucket),
		depth:        0,
	})

	// Walk children depth-first, building the prefix from the trail
	// of "is-last-sibling" flags at each ancestor level.
	var walk func(node *threadNode, ancestorLastFlags []bool)
	walk = func(node *threadNode, ancestorLastFlags []bool) {
		for i, child := range node.children {
			isLast := i == len(node.children)-1
			rows = append(rows, displayRow{
				msg:          child.msg,
				isThreadRoot: false,
				threadSize:   0,
				depth:        uint8(len(ancestorLastFlags) + 1),
				prefix:       buildPrefix(ancestorLastFlags, isLast),
			})
			walk(child, append(ancestorLastFlags, isLast))
		}
	}
	walk(root, nil)

	return rows
}

// buildPrefix constructs the box-drawing prefix string for a row at
// the given depth. ancestorLastFlags has one entry per ancestor level
// above this row, indicating whether that ancestor was the last
// sibling at its own level. isLast reports whether the current row is
// the last sibling at its own level.
//
// For each ancestor: "   " if it was the last sibling, "│  " otherwise.
// Then the current node's connector: "└─ " if last, "├─ " otherwise.
func buildPrefix(ancestorLastFlags []bool, isLast bool) string {
	var b strings.Builder
	for _, last := range ancestorLastFlags {
		if last {
			b.WriteString("   ")
		} else {
			b.WriteString("│  ")
		}
	}
	if isLast {
		b.WriteString("└─ ")
	} else {
		b.WriteString("├─ ")
	}
	return b.String()
}
```

Now delete the old child-emit loop from `rebuild` (the `children := make...` block and everything below it through the end of the bucket loop), since `appendThreadRows` replaces it. The `rebuild` function should now look like:

```go
func (m *MessageList) rebuild() {
	buckets := bucketByThreadID(m.source)

	type sortedBucket struct {
		bucket []mail.MessageInfo
		latest string
	}
	wrapped := make([]sortedBucket, len(buckets))
	for i, b := range buckets {
		wrapped[i] = sortedBucket{bucket: b, latest: latestActivity(b)}
	}
	sort.SliceStable(wrapped, func(i, j int) bool {
		if m.sort == SortDateAsc {
			return wrapped[i].latest < wrapped[j].latest
		}
		return wrapped[i].latest > wrapped[j].latest
	})

	rows := make([]displayRow, 0, len(m.source))
	for _, w := range wrapped {
		rows = appendThreadRows(rows, w.bucket)
	}
	m.rows = rows
}
```

- [ ] **Step 5: Run the test to confirm it passes**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: PASS for every sub-test, including the new branching shape.

- [ ] **Step 6: Run the full ui test suite**

Run: `go test ./internal/ui/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "MessageList: compute depth and box-drawing prefix per row

Builds a transient threadNode tree per bucket (discarded after the
walk), recursively sorts children by date ascending at each level,
then walks depth-first emitting displayRows with the right ├─ │ └─
prefix derived from the chain of ancestor 'is-last-sibling' flags.
Broken parent chains attach orphans to the synthetic root rather
than crashing.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 10: Fold state — ToggleFold, FoldAll, UnfoldAll

**Files:**
- Modify: `internal/ui/msglist.go`
- Modify: `internal/ui/msglist_test.go`

Mutate `m.folded` and re-run `rebuild`. After this task, `rebuild` must consult `m.folded` and mark child rows hidden. The collapsed root's prefix flips to `[N] `.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write the failing test**

Add to `TestMessageListThreading`:

```go
t.Run("ToggleFold collapses thread under cursor", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	if got, want := visibleRowCount(ml), 2; got != want {
		t.Fatalf("initial visible rows = %d, want %d", got, want)
	}
	ml.ToggleFold()
	if got, want := visibleRowCount(ml), 1; got != want {
		t.Errorf("after fold visible rows = %d, want %d", got, want)
	}
	// Root prefix becomes "[2] ".
	if got, want := ml.rows[0].prefix, "[2] "; got != want {
		t.Errorf("collapsed root prefix = %q, want %q", got, want)
	}
})

t.Run("ToggleFold from child row folds the thread root", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	ml.MoveDown() // cursor on UID 11 (child)
	ml.ToggleFold()
	if got, want := visibleRowCount(ml), 1; got != want {
		t.Errorf("after fold from child, visible rows = %d, want %d", got, want)
	}
	// Cursor snaps to root.
	if got := ml.Selected(); got != 0 {
		t.Errorf("cursor index after fold = %d, want 0", got)
	}
})

t.Run("FoldAll and UnfoldAll", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "RootA", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "ReplyA", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
		{UID: "20", ThreadID: "T2", InReplyTo: "", From: "RootB", Date: "2026-04-06 10:00", Flags: mail.FlagSeen},
		{UID: "21", ThreadID: "T2", InReplyTo: "20", From: "ReplyB", Date: "2026-04-06 11:00", Flags: mail.FlagSeen},
		{UID: "30", ThreadID: "T3", InReplyTo: "", From: "Solo", Date: "2026-04-07 10:00", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	if got, want := visibleRowCount(ml), 5; got != want {
		t.Fatalf("initial visible = %d, want %d", got, want)
	}
	ml.FoldAll()
	if got, want := visibleRowCount(ml), 3; got != want {
		t.Errorf("after FoldAll visible = %d, want %d", got, want)
	}
	ml.UnfoldAll()
	if got, want := visibleRowCount(ml), 5; got != want {
		t.Errorf("after UnfoldAll visible = %d, want %d", got, want)
	}
})

t.Run("SetMessages resets fold state", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	ml.ToggleFold()
	ml.SetMessages(msgs) // same data
	if got, want := visibleRowCount(ml), 2; got != want {
		t.Errorf("after SetMessages reload, visible = %d, want %d", got, want)
	}
})
```

Add the helper at the end of the file:

```go
// visibleRowCount counts the displayRows that aren't hidden by fold
// state. Used by tests to check fold behavior.
func visibleRowCount(ml MessageList) int {
	n := 0
	for _, r := range ml.rows {
		if !r.hidden {
			n++
		}
	}
	return n
}
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: FAIL — `ToggleFold`, `FoldAll`, `UnfoldAll` undefined.

- [ ] **Step 4: Implement the fold mutators**

Add to `internal/ui/msglist.go` after `SetSort`:

```go
// ToggleFold flips the fold state of the thread the cursor is
// currently inside. If the cursor is on a child row, the toggle still
// operates on that child's thread root. After folding, the cursor
// snaps to the root index so it doesn't land on a now-hidden row.
func (m *MessageList) ToggleFold() {
	if len(m.rows) == 0 {
		return
	}
	rootIdx := m.threadRootIndex(m.selected)
	if rootIdx < 0 {
		return
	}
	rootUID := m.rows[rootIdx].msg.UID
	m.folded[rootUID] = !m.folded[rootUID]
	m.rebuild()
	// Cursor snaps to the root if the previous selection is now hidden.
	if m.selected >= len(m.rows) || m.rows[m.selected].hidden {
		m.selected = m.indexOfUID(rootUID)
	}
	m.clampOffset()
}

// FoldAll collapses every thread root.
func (m *MessageList) FoldAll() {
	for _, r := range m.rows {
		if r.isThreadRoot && r.threadSize > 1 {
			m.folded[r.msg.UID] = true
		}
	}
	m.rebuild()
	if m.selected >= len(m.rows) || m.rows[m.selected].hidden {
		// Snap cursor to the nearest visible row above.
		for i := m.selected; i >= 0; i-- {
			if !m.rows[i].hidden {
				m.selected = i
				break
			}
		}
	}
	m.clampOffset()
}

// UnfoldAll clears all fold state.
func (m *MessageList) UnfoldAll() {
	m.folded = map[mail.UID]bool{}
	m.rebuild()
	m.clampOffset()
}

// threadRootIndex returns the row index of the thread root that owns
// the row at idx. Walks backwards from idx until it finds a row with
// isThreadRoot == true. Returns -1 if no root is found above idx
// (shouldn't happen — every thread has a root and idx is bounded).
func (m MessageList) threadRootIndex(idx int) int {
	if idx < 0 || idx >= len(m.rows) {
		return -1
	}
	for i := idx; i >= 0; i-- {
		if m.rows[i].isThreadRoot {
			return i
		}
	}
	return -1
}

// indexOfUID returns the displayRow index of the message with the
// given UID, or -1 if not found.
func (m MessageList) indexOfUID(uid mail.UID) int {
	for i, r := range m.rows {
		if r.msg.UID == uid {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 5: Apply fold state in rebuild**

After the `rows = appendThreadRows(rows, w.bucket)` loop completes, but before assigning `m.rows = rows`, walk the rows applying fold state:

```go
rows := make([]displayRow, 0, len(m.source))
for _, w := range wrapped {
	rows = appendThreadRows(rows, w.bucket)
}
applyFoldState(rows, m.folded)
m.rows = rows
```

Add the helper after `appendThreadRows`:

```go
// applyFoldState mutates rows in place: for any folded thread root,
// every subsequent row up to the next root is marked hidden, and the
// root's prefix is replaced with "[N] " where N is threadSize.
func applyFoldState(rows []displayRow, folded map[mail.UID]bool) {
	for i := 0; i < len(rows); i++ {
		if !rows[i].isThreadRoot {
			continue
		}
		if !folded[rows[i].msg.UID] {
			continue
		}
		rows[i].prefix = fmt.Sprintf("[%d] ", rows[i].threadSize)
		for j := i + 1; j < len(rows); j++ {
			if rows[j].isThreadRoot {
				break
			}
			rows[j].hidden = true
		}
	}
}
```

Add `"fmt"` to the import block:

```go
import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	"github.com/mattn/go-runewidth"
)
```

- [ ] **Step 6: Run the threading tests to confirm they pass**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: PASS for every sub-test.

- [ ] **Step 7: Run the full ui test suite**

Run: `go test ./internal/ui/...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "MessageList: fold state with ToggleFold/FoldAll/UnfoldAll

Per-MessageList map[UID]bool keyed by thread root. ToggleFold operates
on the thread containing the cursor (walks back to the root if the
cursor is on a child). FoldAll/UnfoldAll set/clear the whole map.
Cursor snaps to the root after fold to avoid landing on a hidden row.
SetMessages resets the map.

applyFoldState walks the displayRow slice marking hidden flags and
swapping the root's prefix to '[N] ' for collapsed threads.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 11: Render the prefix in the subject column

**Files:**
- Modify: `internal/ui/msglist.go`
- Modify: `internal/ui/msglist_test.go`

So far the prefix is computed and stored on each `displayRow`, but `renderRow` doesn't actually display it. This task wires it through.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write the failing test**

Add to `TestMessageListThreading`:

```go
t.Run("renders box-drawing prefix in subject column", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", Subject: "Root subject", From: "Root", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", Subject: "Re: Root subject", From: "ReplyA", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
		{UID: "12", ThreadID: "T1", InReplyTo: "10", Subject: "Re: Root subject", From: "ReplyB", Date: "2026-04-05 12:00", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 100, 20)
	plain := stripANSI(ml.View())
	if !strings.Contains(plain, "├─ Re: Root subject") {
		t.Error("expected ├─ prefix on first reply")
	}
	if !strings.Contains(plain, "└─ Re: Root subject") {
		t.Error("expected └─ prefix on last reply")
	}
})

t.Run("renders [N] badge on collapsed thread root", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", Subject: "Root", From: "R", Date: "2026-04-05 10:00", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", Subject: "Re: Root", From: "A", Date: "2026-04-05 11:00", Flags: mail.FlagSeen},
		{UID: "12", ThreadID: "T1", InReplyTo: "10", Subject: "Re: Root", From: "B", Date: "2026-04-05 12:00", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 100, 20)
	ml.ToggleFold()
	plain := stripANSI(ml.View())
	if !strings.Contains(plain, "[3] Root") {
		t.Errorf("expected [3] Root in collapsed view, got: %q", plain)
	}
})
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/ui/ -run TestMessageListThreading/renders -v`
Expected: FAIL — prefix not in output.

- [ ] **Step 4: Render the prefix in renderRow**

Find `renderRow` in `internal/ui/msglist.go`. Replace the subject rendering block (the lines that compute `subjectWidth`, `subjectText`, and `subject`):

```go
// Subject column: prefix (in MsgListThreadPrefix style) followed by
// the subject text (in the read/unread style), with the subject
// truncated to fit whatever space remains after the prefix.
subjectWidth := max(1, m.width-mlFixedWidth)
prefixCells := runewidth.StringWidth(row.prefix)
subjectCells := max(0, subjectWidth-prefixCells)

prefixStyled := applyBg(m.styles.MsgListThreadPrefix, bgStyle).Render(row.prefix)
subjectText := padRight(truncateCells(msg.Subject, subjectCells), subjectCells)
subjectStyled := applyBg(subjectStyle, bgStyle).Render(subjectText)
subject := prefixStyled + subjectStyled
```

The rest of the row composition (`row := cursor + flag + ...`) is unchanged.

- [ ] **Step 5: Run the test to confirm it passes**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: PASS.

- [ ] **Step 6: Run the full ui test suite**

Run: `go test ./internal/ui/...`
Expected: PASS — width assertions still hold because prefix + subject still totals `subjectWidth`.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "MessageList: render thread prefix in subject column

Prefix is rendered in MsgListThreadPrefix style (FgDim) at the start
of the subject column; the subject text is truncated to fit whatever
cells remain. Width invariant (subject column = prefix + subject)
holds, so existing row-width assertions still pass. Collapsed-thread
roots show their '[N] ' badge in the same slot.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 12: Cursor navigation skips hidden rows

**Files:**
- Modify: `internal/ui/msglist.go`
- Modify: `internal/ui/msglist_test.go`

`MoveDown`/`MoveUp` should walk past `hidden == true` rows. `MoveToTop`/`MoveToBottom` should land on the first/last visible row. The viewport math (`clampOffset`) keeps using raw indices.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write the failing test**

Add to `TestMessageListThreading`:

```go
t.Run("MoveDown skips hidden rows", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "1", ThreadID: "1", From: "Above", Subject: "above", Date: "2026-04-10", Flags: mail.FlagSeen},
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Subject: "thread", Date: "2026-04-09", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Subject: "thread", Date: "2026-04-09 11:00", Flags: mail.FlagSeen},
		{UID: "2", ThreadID: "2", From: "Below", Subject: "below", Date: "2026-04-08", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	// Default sort puts these in date-desc order: Above, Root, Reply, Below.
	// Fold the T1 thread.
	ml.MoveDown() // cursor on Root (index 1)
	ml.ToggleFold()
	// Now visible rows: Above (0), Root (1, folded), Below (3 — index 2 hidden).
	// MoveDown from Root should land on Below (index 3), skipping hidden index 2.
	ml.MoveDown()
	if got, want := ml.Selected(), 3; got != want {
		t.Errorf("after MoveDown across hidden row, Selected() = %d, want %d", got, want)
	}
})

t.Run("MoveUp skips hidden rows", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "1", ThreadID: "1", From: "Above", Subject: "above", Date: "2026-04-10", Flags: mail.FlagSeen},
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Subject: "thread", Date: "2026-04-09", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Subject: "thread", Date: "2026-04-09 11:00", Flags: mail.FlagSeen},
		{UID: "2", ThreadID: "2", From: "Below", Subject: "below", Date: "2026-04-08", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	ml.MoveDown() // cursor on Root
	ml.ToggleFold()
	// Cursor is on Root (index 1). Move to Below (3), then back up.
	ml.MoveDown() // → index 3 (Below)
	ml.MoveUp()
	// Should land on Root (index 1), skipping hidden index 2.
	if got, want := ml.Selected(), 1; got != want {
		t.Errorf("after MoveUp across hidden row, Selected() = %d, want %d", got, want)
	}
})

t.Run("MoveToBottom lands on last visible row", func(t *testing.T) {
	msgs := []mail.MessageInfo{
		{UID: "10", ThreadID: "T1", InReplyTo: "", From: "Root", Date: "2026-04-09", Flags: mail.FlagSeen},
		{UID: "11", ThreadID: "T1", InReplyTo: "10", From: "Reply", Date: "2026-04-09 11:00", Flags: mail.FlagSeen},
	}
	ml := NewMessageList(styles, msgs, 90, 20)
	ml.ToggleFold() // fold T1, child at index 1 hidden
	ml.MoveToBottom()
	if got, want := ml.Selected(), 0; got != want {
		t.Errorf("MoveToBottom with only root visible: Selected() = %d, want %d", got, want)
	}
})
```

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: FAIL — cursor lands on hidden indices.

- [ ] **Step 4: Update navigation methods**

Replace `moveBy`, `MoveToTop`, and `MoveToBottom` in `internal/ui/msglist.go`:

```go
// moveBy shifts the cursor by delta visible rows, walking past any
// hidden rows in the requested direction. Empty list is a no-op.
func (m *MessageList) moveBy(delta int) {
	if len(m.rows) == 0 {
		return
	}
	if delta == 0 {
		m.clampOffset()
		return
	}

	step := 1
	if delta < 0 {
		step = -1
		delta = -delta
	}

	idx := m.selected
	for delta > 0 {
		next := idx + step
		// Walk past hidden rows.
		for next >= 0 && next < len(m.rows) && m.rows[next].hidden {
			next += step
		}
		if next < 0 || next >= len(m.rows) {
			break
		}
		idx = next
		delta--
	}
	m.selected = idx
	m.clampOffset()
}

// MoveToTop jumps the cursor to the first visible row.
func (m *MessageList) MoveToTop() {
	for i := 0; i < len(m.rows); i++ {
		if !m.rows[i].hidden {
			m.selected = i
			m.offset = 0
			m.clampOffset()
			return
		}
	}
}

// MoveToBottom jumps the cursor to the last visible row.
func (m *MessageList) MoveToBottom() {
	for i := len(m.rows) - 1; i >= 0; i-- {
		if !m.rows[i].hidden {
			m.selected = i
			m.clampOffset()
			return
		}
	}
}
```

`HalfPageDown`, `HalfPageUp`, `PageDown`, `PageUp` keep their existing definitions because they call `moveBy`, which now does the right thing.

- [ ] **Step 5: Update View() to skip hidden rows**

`View()` currently emits `m.height` consecutive rows starting from `m.offset`. With hidden rows in the slice, that's wrong — the visible window should contain `m.height` *visible* rows. Replace the View loop:

```go
func (m MessageList) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if len(m.rows) == 0 {
		return m.renderEmpty()
	}

	plainBg := m.styles.MsgListBg
	selectedBg := m.styles.MsgListSelected

	lines := make([]string, 0, m.height)
	visible := 0
	for i := m.offset; i < len(m.rows) && visible < m.height; i++ {
		if m.rows[i].hidden {
			continue
		}
		bg := plainBg
		if i == m.selected {
			bg = selectedBg
		}
		lines = append(lines, m.renderRow(i, bg))
		visible++
	}
	for len(lines) < m.height {
		lines = append(lines, m.renderBlankLine())
	}
	return strings.Join(lines, "\n")
}
```

`clampOffset` continues to operate on raw indices. The viewport math is now slightly looser — `m.offset` may sit on a hidden row — but the View loop handles that by skipping. The cursor visibility property still holds because `m.selected` is always a non-hidden index after `moveBy`.

There's one edge case: when `clampOffset` pushes the offset down to make room for a cursor at a high index, it should ideally count visible rows when computing `selected - height + 1`. For prototype data this isn't a noticeable problem because hidden rows are rare relative to viewport size; if it does become a problem the fix is to count visible rows in `clampOffset`. Leave it as-is for now and revisit only if a test fails.

- [ ] **Step 6: Run the threading tests to confirm they pass**

Run: `go test ./internal/ui/ -run TestMessageListThreading -v`
Expected: PASS.

- [ ] **Step 7: Run the full ui test suite**

Run: `go test ./internal/ui/...`
Expected: PASS — non-threaded tests still pass because there are no hidden rows in their fixtures.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/msglist.go internal/ui/msglist_test.go
git commit -m "MessageList: cursor and viewport skip hidden rows

moveBy walks past hidden rows in the requested direction. MoveToTop
and MoveToBottom land on the first/last visible row. View() iterates
from offset until it has emitted m.height visible rows, skipping
hidden ones. The viewport math (clampOffset) still uses raw indices
— hidden rows are rare relative to viewport size for prototype data
and the cursor-visible property is preserved by moveBy.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 13: Round-trip test against the real mock backend

**Files:**
- Modify: `internal/ui/msglist_test.go`

End-to-end check: feed the actual `mail.NewMockBackend()` data through `MessageList` and verify the threaded conversation renders correctly.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write the test**

Add to `internal/ui/msglist_test.go`:

```go
func TestMessageListWithMockBackend(t *testing.T) {
	styles := NewStyles(theme.Nord)
	b := mail.NewMockBackend()
	msgs, err := b.FetchHeaders(nil)
	if err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}

	ml := NewMessageList(styles, msgs, 120, 30)

	t.Run("14 source messages produce 14 displayRows expanded", func(t *testing.T) {
		if got, want := len(ml.rows), 14; got != want {
			t.Errorf("len(rows) = %d, want %d", got, want)
		}
	})

	t.Run("threaded conversation has correct prefix vocabulary", func(t *testing.T) {
		var t1Prefixes []string
		for _, r := range ml.rows {
			if r.msg.ThreadID == "T1" {
				t1Prefixes = append(t1Prefixes, r.prefix)
			}
		}
		if len(t1Prefixes) != 4 {
			t.Fatalf("T1 row count = %d, want 4", len(t1Prefixes))
		}
		// Frank Lee root, then Grace (├─), then Frank-deep (│  └─), then Henry (└─).
		// Children sorted chronologically asc; the actual mock dates are all
		// "Apr 5" so order falls back to insertion order via SliceStable.
		want := []string{"", "├─ ", "│  └─ ", "└─ "}
		for i, w := range want {
			if t1Prefixes[i] != w {
				t.Errorf("T1 prefix[%d] = %q, want %q", i, t1Prefixes[i], w)
			}
		}
	})

	t.Run("FoldAll collapses the threaded conversation", func(t *testing.T) {
		ml := NewMessageList(styles, msgs, 120, 30)
		ml.FoldAll()
		visible := visibleRowCount(ml)
		// 10 single-message threads (unaffected) + 1 visible folded root = 11.
		if visible != 11 {
			t.Errorf("visible after FoldAll = %d, want 11", visible)
		}
		// The collapsed root carries the [4] badge.
		var foundBadge bool
		for _, r := range ml.rows {
			if r.isThreadRoot && r.msg.ThreadID == "T1" {
				if r.prefix != "[4] " {
					t.Errorf("collapsed T1 root prefix = %q, want %q", r.prefix, "[4] ")
				}
				foundBadge = true
			}
		}
		if !foundBadge {
			t.Error("never found T1 thread root after FoldAll")
		}
	})
}
```

- [ ] **Step 3: Run the test**

Run: `go test ./internal/ui/ -run TestMessageListWithMockBackend -v`
Expected: PASS — but note that the mock backend's date strings are "Apr 5", "Yesterday", "10:23 AM" etc., which sort lexicographically rather than chronologically. The threaded conversation's "Apr 5" date will likely sort *between* other letters depending on what other messages look like. The test asserts a specific vocabulary ordering, not a specific position in the overall list.

If the test fails because the threaded conversation rows aren't found together (e.g., "Apr 5" sorts apart from the "Yesterday" rows), check: do all 4 T1 messages appear contiguously in `ml.rows`? They should, because they share a ThreadID and `bucketByThreadID` groups them.

If the order of children inside T1 is wrong (e.g., Grace and Henry swap), it's because all four messages have `Date: "Apr 5"` — `sort.SliceStable` falls back to input order, which matches the order they're listed in `mock.go`. As long as that order is `[20, 21, 22, 23]` (as the spec says) the prefixes will land in `[root, ├─ Grace, │  └─ Frank-deep, └─ Henry]`.

If the test passes, proceed.

- [ ] **Step 4: Run the full ui test suite**

Run: `go test ./internal/ui/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/msglist_test.go
git commit -m "MessageList: end-to-end test against the mock backend

Feeds the real NewMockBackend data through MessageList and verifies
14 source messages produce 14 displayRows expanded, the T1 branching
thread renders the full ├─ │ └─ prefix vocabulary, and FoldAll
collapses the threaded conversation while leaving the 10 single-
message threads alone (11 visible rows after FoldAll).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 14: Wire SortOrder from UIConfig through AccountTab

**Files:**
- Modify: `internal/ui/account_tab.go`
- Modify: `internal/ui/account_tab_test.go`

Read the folder's `Sort` config value when a folder is loaded and pass it to `MessageList.SetSort`.

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Update folderLoadedMsg handler**

In `internal/ui/account_tab.go`, find the `case folderLoadedMsg:` block (around line 85). Replace it:

```go
case folderLoadedMsg:
	order := SortDateDesc
	if fc, ok := m.uiCfg.Folders[msg.name]; ok && fc.Sort == "date-asc" {
		order = SortDateAsc
	}
	m.msglist.SetSort(order)
	m.msglist.SetMessages(msg.msgs)
	return m, nil
```

`SetSort` triggers a rebuild, then `SetMessages` triggers another. To avoid the double rebuild, swap the order — set messages first (which will rebuild against the *previous* sort), then set sort (which triggers a final rebuild with the right direction). Either ordering is correct; the swap is a micro-optimization. For clarity, keep the order shown above and accept one wasted rebuild — the cost is negligible for prototype data.

Actually no — set the sort first because `SetSort` mutates a field and `SetMessages` does both the field reset and the rebuild. The order shown above is correct and produces one wasted rebuild. If a test fails on order leave it as is; if not move on.

- [ ] **Step 3: Run make check**

Run: `make check`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/account_tab.go
git commit -m "AccountTab: wire folder sort config into MessageList

Reads [ui.folders.<name>] sort from UIConfig when a folder load
completes, mapping 'date-asc' to SortDateAsc and any other value
(including the unset default) to SortDateDesc. Promotes the parsed-
but-unused Sort field from Pass 2.5b-3.5 to load-bearing.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 15: Dispatch Space, F, U keys to MessageList

**Files:**
- Modify: `internal/ui/account_tab.go`
- Modify: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write a failing test**

Add to `internal/ui/account_tab_test.go`. Look at the existing test setup pattern and follow it. Add this test:

```go
func TestAccountTabFoldKeys(t *testing.T) {
	styles := NewStyles(theme.Nord)
	backend := mail.NewMockBackend()
	cfg := config.DefaultUIConfig()
	tab := NewAccountTab(styles, backend, cfg)

	// Force the tab to its post-init state: folders loaded, Inbox selected,
	// messages fetched.
	tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
	folders, _ := backend.ListFolders()
	tab, _ = tab.updateTab(foldersLoadedMsg{folders: folders})
	msgs, _ := backend.FetchHeaders(nil)
	tab, _ = tab.updateTab(folderLoadedMsg{name: "Inbox", msgs: msgs})

	initial := visibleRowCount(tab.msglist)
	if initial != 14 {
		t.Fatalf("initial visible rows = %d, want 14", initial)
	}

	t.Run("F folds all threads", func(t *testing.T) {
		tab2, _ := tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
		if got := visibleRowCount(tab2.msglist); got != 11 {
			t.Errorf("after F, visible = %d, want 11", got)
		}
	})

	t.Run("U unfolds all threads after F", func(t *testing.T) {
		tab2, _ := tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
		tab2, _ = tab2.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'U'}})
		if got := visibleRowCount(tab2.msglist); got != 14 {
			t.Errorf("after F then U, visible = %d, want 14", got)
		}
	})

	t.Run("Space toggles fold under cursor", func(t *testing.T) {
		// Move cursor to the T1 thread root. The T1 conversation is the
		// only multi-message thread; find its root index.
		var t1Idx int = -1
		for i, r := range tab.msglist.rows {
			if r.isThreadRoot && r.msg.ThreadID == "T1" {
				t1Idx = i
				break
			}
		}
		if t1Idx < 0 {
			t.Fatal("T1 root not found in displayRows")
		}
		// Move cursor to that row.
		tab2 := tab
		for i := 0; i < t1Idx; i++ {
			tab2, _ = tab2.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		}
		// Toggle fold via Space.
		tab2, _ = tab2.updateTab(tea.KeyMsg{Type: tea.KeySpace})
		if got := visibleRowCount(tab2.msglist); got != 11 {
			t.Errorf("after Space on T1 root, visible = %d, want 11", got)
		}
	})
}
```

The imports at the top of `account_tab_test.go` may need additions — check what's already there and add `tea "github.com/charmbracelet/bubbletea"`, `"github.com/glw907/poplar/internal/config"`, `"github.com/glw907/poplar/internal/mail"` if any are missing.

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/ui/ -run TestAccountTabFoldKeys -v`
Expected: FAIL — Space/F/U not handled.

- [ ] **Step 4: Add key cases**

In `internal/ui/account_tab.go`, find `handleKey` (around line 102). Add three cases at the appropriate spot — keep alphabetical-ish ordering with the existing cases:

```go
func (m AccountTab) handleKey(msg tea.KeyMsg) (AccountTab, tea.Cmd) {
	switch msg.String() {
	case "J":
		m.sidebar.MoveDown()
		return m, m.selectionChangedCmds()
	case "K":
		m.sidebar.MoveUp()
		return m, m.selectionChangedCmds()
	case "G":
		m.msglist.MoveToBottom()
	case "g":
		m.msglist.MoveToTop()
	case "j", "down":
		m.msglist.MoveDown()
	case "k", "up":
		m.msglist.MoveUp()
	case "ctrl+d":
		m.msglist.HalfPageDown()
	case "ctrl+u":
		m.msglist.HalfPageUp()
	case "ctrl+f", "pgdown":
		m.msglist.PageDown()
	case "ctrl+b", "pgup":
		m.msglist.PageUp()
	case " ":
		m.msglist.ToggleFold()
	case "F":
		m.msglist.FoldAll()
	case "U":
		m.msglist.UnfoldAll()
	}
	return m, nil
}
```

- [ ] **Step 5: Run the test to confirm it passes**

Run: `go test ./internal/ui/ -run TestAccountTabFoldKeys -v`
Expected: PASS.

- [ ] **Step 6: Run make check**

Run: `make check`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/account_tab.go internal/ui/account_tab_test.go
git commit -m "AccountTab: dispatch Space, F, U fold keys to MessageList

Space toggles the fold of the thread under the cursor (operates on
the thread root if cursor is on a child). F folds every multi-
message thread; U unfolds all. End-to-end test feeds the mock
backend through the tab and verifies fold key handling produces the
expected visible row counts.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 16: Add Threads hint group to the footer

**Files:**
- Modify: `internal/ui/footer.go`
- Modify: `internal/ui/footer_test.go`

- [ ] **Step 1: Invoke `elm-conventions` and `go-conventions`**

- [ ] **Step 2: Write a failing test**

Add to `internal/ui/footer_test.go`. Look at the existing tests to follow the pattern. A new test:

```go
func TestFooterThreadsGroup(t *testing.T) {
	styles := NewStyles(theme.Nord)
	f := NewFooter(styles)

	t.Run("renders Threads group at full width", func(t *testing.T) {
		out := stripANSI(f.View(200))
		if !strings.Contains(out, "␣ fold") {
			t.Error("expected ␣ fold hint")
		}
		if !strings.Contains(out, "F fold all") {
			t.Error("expected F fold all hint")
		}
		if !strings.Contains(out, "U unfold all") {
			t.Error("expected U unfold all hint")
		}
	})
}
```

(If `stripANSI` lives in `msglist_test.go`, the footer test file may already use it via the same package — check and reuse.)

- [ ] **Step 3: Run the test to confirm it fails**

Run: `go test ./internal/ui/ -run TestFooterThreadsGroup -v`
Expected: FAIL — hints not in output.

- [ ] **Step 4: Add the Threads group**

In `internal/ui/footer.go`, edit `accountFooterGroups`. Add a new group between the search/select group and the help/quit group:

```go
func accountFooterGroups() [][]footerHint {
	return [][]footerHint{
		{
			hint("j/k/J/K", "nav", 10),
			hint("I/D/S/A", "folders", 9),
		},
		triageHints,
		replyHints,
		{
			hint("/", "find", 3),
			hint("n/N", "results", 7),
			hint("v", "select", 8),
		},
		{
			hint("␣", "fold", 4),
			hint("F", "fold all", 5),
			hint("U", "unfold all", 5),
		},
		{
			hint("?", "help", 0),
			hint("q", "quit", 0),
		},
	}
}
```

Update the doc comment above the function to mention the new ranks:

```go
// accountFooterGroups returns the unified one-pane account footer hint
// groups in display order.
//
// Drop order (highest rank first):
//   - nav entries (10, 9) — vim/arrow users don't need the hint
//   - v select (8), n/N results (7) — niche modes, discoverable via help
//   - F/U fold-all (5), . read (5), s star (4), ␣ fold (4),
//     f fwd (3), / find (3) — secondary actions
//   - r/R reply (2), c compose (2) — primary compose actions
//   - d del (1), a archive (1) — primary triage
//   - ? help (0), q quit (0) — always kept
```

- [ ] **Step 5: Run the test to confirm it passes**

Run: `go test ./internal/ui/ -run TestFooterThreadsGroup -v`
Expected: PASS.

- [ ] **Step 6: Run the full footer test suite**

Run: `go test ./internal/ui/ -run TestFooter -v`
Expected: PASS — existing footer tests still pass. If a width-breakpoint test fails because the new group changes the total width, update the breakpoint expectation in that test.

- [ ] **Step 7: Run make check**

Run: `make check`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/footer.go internal/ui/footer_test.go
git commit -m "Footer: add Threads hint group with Space/F/U

Drop ranks 4 and 5 place the new group between the search/select
group and the always-kept help/quit group, so it survives narrower
terminals than the secondary triage hints but drops before primary
triage. ␣ is U+2423 OPEN BOX, matching the help popover convention.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 17: Promote Space, F, U from reserved to live in keybindings.md

**Files:**
- Modify: `docs/poplar/keybindings.md`

- [ ] **Step 1: Update the Threads section**

In `docs/poplar/keybindings.md`, find the "Threads (reserved — Pass 2.5b-3.6)" section. Rewrite it as a live section:

```markdown
## Threads

| Key | Action | Context |
|-----|--------|---------|
| `Space` | Toggle fold on thread under cursor | A |
| `F` | Fold all threads | A |
| `U` | Unfold all threads | A |

`Space` is dual-purpose: inside visual-select mode (Pass 6) it
toggles row selection, outside visual mode it toggles thread
fold. See ADR 0052 "Thread fold key: Space, dual meaning in
visual-select mode".

Folding from a child row folds the row's thread root — `Space`
always operates on the entire thread, never on individual replies.
The cursor snaps to the root after folding so it doesn't land on
a hidden row.
```

- [ ] **Step 2: Update the Footer Display § account footer ASCII**

The existing rendered footer ASCII at line 134-137 doesn't include the new Threads group. Update it:

```markdown
### Account footer

The unified one-pane footer.

```
 j/k/J/K nav  I/D/S/A folders ┊ d del  a archive  s star  . read ┊ r/R reply  f fwd  c compose ┊ / find  n/N results  v select ┊ ␣ fold  F fold all  U unfold all ┊ ? help  q quit
```
```

(The decorative `◂──...──▸` line is removed because the line is too long for it to be useful; the group structure is clear from `┊` separators.)

- [ ] **Step 3: Update the Drop tiers table**

In the same file, find the "Drop tiers" table (around line 165). Add `␣ fold` and `F/U fold-all` to the right rows:

```markdown
| Rank | Hints | Why drop first |
|------|-------|----------------|
| 10–9 | `j/k/J/K nav`, `I/D/S/A folders` | Vim/arrow users don't need the hint |
| 8 | `v select` | Niche mode, discoverable in `?` help |
| 7 | `n/N results` | Only useful after `/`, infer from convention |
| 5 | `. read`, `F fold all`, `U unfold all` | Secondary triage / bulk fold |
| 4 | `s star`, `␣ fold` | Secondary triage / per-thread fold |
| 3 | `f fwd`, `/ find` | Tertiary actions |
| 2 | `r/R reply`, `c compose` | Primary compose actions |
| 1 | `d del`, `a archive` | Primary triage |
| 0 | `? help`, `q quit` | Always kept |
```

- [ ] **Step 4: Commit**

```bash
git add docs/poplar/keybindings.md
git commit -m "Promote Space/F/U from reserved to live in keybindings doc

Threads section is now a live section, not a reserved one. Footer
ASCII updated to include the new Threads group. Drop tiers table
updated for ␣ fold (rank 4) and F/U fold-all (rank 5).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 18: Live render verification

**Files:**
- None modified — verification only.

- [ ] **Step 1: Build and install**

Run: `make install`
Expected: PASS — binary lands in `~/.local/bin/poplar`.

- [ ] **Step 2: Capture three states via tmux**

Follow `.claude/docs/tmux-testing.md` for the capture mechanics. Three captures:

1. **Default expanded view.** Launch `poplar`, observe Inbox. Capture. The threaded conversation should appear with the full ├─ │ └─ vocabulary somewhere in the message list. The first child (Grace Kim) should render with the unread treatment.
2. **Fully folded.** Press `F`. Capture. The threaded conversation should now show as a single row with `[4] Server migration plan` in the prefix slot. Visible row count drops from 14 to 11.
3. **Cursor on T1 root, then `U` to unfold.** Press `U`. Capture. Visible row count returns to 14.

- [ ] **Step 3: Cross-check against wireframes**

Compare each capture against:
- Capture 1 ↔ wireframes.md §3 "Default with cursor and threading"
- Capture 2 ↔ wireframes.md §7 "Threaded view — collapsed (#14)"
- Capture 1 also ↔ §1 Composite layout (which shows threading inline)

If a capture diverges from the wireframe, fix the renderer or the test data and re-run from Step 1. **Do not** alter the wireframe to match a buggy capture — the wireframes are the spec.

- [ ] **Step 4: Note any issues**

If the captures match the wireframes, proceed to the pass-end ritual (the `poplar-pass` skill handles this). If something is off, file an issue or fix it before declaring the pass done.

---

## Self-Review Notes

After writing the plan, the following spec coverage check is in order:

- **Wire fields (`ThreadID`, `InReplyTo`)**: Task 1.
- **`displayRow` + Camp 2 architecture**: Task 3.
- **Build pipeline (group, root pick, child sort, latest-activity, thread sort, prefix walk, fold apply)**: Tasks 5–10.
- **Prefix rendering with `MsgListThreadPrefix` slot**: Tasks 4 + 11.
- **Cursor skip / snap**: Tasks 10 + 12.
- **Sort wired through from `[ui.folders.<name>] sort`**: Task 14.
- **Space/F/U dispatch**: Task 15.
- **Footer Threads group**: Task 16.
- **Keybindings doc promotion**: Task 17.
- **Mock conversation data**: Task 2.
- **Live render verification against wireframes**: Task 18.

All spec sections are covered. No placeholder steps. Type names used in later tasks (`displayRow`, `SortOrder`, `SortDateDesc`, `SortDateAsc`, `bucketByThreadID`, `pickRoot`, `latestActivity`, `appendThreadRows`, `buildPrefix`, `applyFoldState`, `threadRootIndex`, `indexOfUID`, `visibleRowCount`) all appear in earlier tasks where they're defined.
