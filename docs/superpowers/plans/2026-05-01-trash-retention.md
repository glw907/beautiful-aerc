# Trash Retention + Manual Empty — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a permanent-delete primitive (`Backend.Destroy`) plus
two consumers — a per-session retention sweep on Disposal folders
and a manual "empty folder" key (`E`) — guarded by a confirmation
modal.

**Architecture:** Disabled by default; opt-in via two new `[ui]`
config knobs. Retention sweep runs once per session per Disposal
folder over already-loaded messages. Manual empty fetches the full
UID list via `QueryFolder(name, 0, total)` then `Destroy`s. New
`ConfirmModal` overlay component mirrors the App-owned overlay
pattern of `LinkPicker` / `MovePicker`.

**Tech Stack:** Go 1.26, bubbletea, bubbles/key, lipgloss,
emersion/go-jmap (JMAP `Email/set { destroy }`), BurntSushi/toml.

**Reference:** spec at
`docs/superpowers/specs/2026-05-01-trash-retention-design.md`.
Idiomatic-bubbletea conventions at
`docs/poplar/bubbletea-conventions.md` (size contract, `key.Matches`
dispatch, `WindowSizeMsg` forwarding, no parent-side clipping).

---

## File map

**Create:**
- `internal/ui/confirm_modal.go` — generic confirm overlay.
- `internal/ui/confirm_modal_test.go` — unit tests.

**Modify:**
- `internal/config/ui.go` — add `TrashRetentionDays`,
  `SpamRetentionDays` fields, parse, clamp.
- `internal/config/ui_test.go` — clamp + default tests.
- `internal/mail/backend.go` — add `Destroy` method to interface.
- `internal/mail/mock.go` — implement `Destroy`, record calls.
- `internal/mail/mock_test.go` — `Destroy` test (if file exists; else add).
- `internal/mailjmap/jmap.go` — implement `Destroy` via
  `Email/set { destroy }`.
- `internal/mailjmap/jmap_test.go` — `Destroy` round-trip test.
- `internal/mailimap/` — stub `Destroy` returning a sentinel error
  (file location: confirm during Task 4).
- `internal/ui/cmds.go` — add `destroyCmd`, `emptyFolderCmd`,
  `sweepCompletedMsg`, `EmptyFolderConfirmedMsg`,
  `OpenConfirmEmptyMsg`, `ConfirmModalClosedMsg`.
- `internal/ui/keys.go` — add `Empty` to `AccountKeys`.
- `internal/ui/account_tab.go` — `swept map[string]bool`,
  retention-sweep dispatch in `selectionChangedCmds`, `E` handling
  in `handleKey`, `dispatchEmpty`.
- `internal/ui/account_tab_test.go` — sweep + `E` gating tests.
- `internal/ui/app.go` — own `confirm ConfirmModal`, route keys
  while open, render overlay, handle `OpenConfirmEmptyMsg` /
  `EmptyFolderConfirmedMsg` / `ConfirmModalClosedMsg`.
- `internal/ui/app_test.go` — overlay + dispatch tests.
- `internal/ui/toast.go` — support no-undo toast (new field on
  `pendingAction` or new render path).
- `internal/ui/toast_test.go` — no-undo render assertion.
- `internal/ui/help_popover.go` — append `{"E", "empty", true}`
  to the Triage `accountGroups` entry.

---

## Task 1: Config — add retention knobs

**Files:**
- Modify: `internal/config/ui.go`
- Modify: `internal/config/ui_test.go`

- [ ] **Step 1: Write failing tests for the new fields**

Append to `internal/config/ui_test.go`:

```go
func TestLoadUI_TrashRetentionDefaults(t *testing.T) {
	path := writeTempUI(t, `[ui]
threading = true
`)
	cfg, err := LoadUI(path)
	if err != nil {
		t.Fatalf("LoadUI: %v", err)
	}
	if cfg.TrashRetentionDays != 0 {
		t.Errorf("TrashRetentionDays = %d, want 0", cfg.TrashRetentionDays)
	}
	if cfg.SpamRetentionDays != 0 {
		t.Errorf("SpamRetentionDays = %d, want 0", cfg.SpamRetentionDays)
	}
}

func TestLoadUI_TrashRetentionParsed(t *testing.T) {
	path := writeTempUI(t, `[ui]
trash_retention_days = 14
spam_retention_days = 7
`)
	cfg, err := LoadUI(path)
	if err != nil {
		t.Fatalf("LoadUI: %v", err)
	}
	if cfg.TrashRetentionDays != 14 {
		t.Errorf("TrashRetentionDays = %d, want 14", cfg.TrashRetentionDays)
	}
	if cfg.SpamRetentionDays != 7 {
		t.Errorf("SpamRetentionDays = %d, want 7", cfg.SpamRetentionDays)
	}
}

func TestLoadUI_TrashRetentionClamp(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{-5, 0},
		{0, 0},
		{30, 30},
		{365, 365},
		{1000, 365},
	}
	for _, tc := range cases {
		path := writeTempUI(t, fmt.Sprintf(`[ui]
trash_retention_days = %d
`, tc.in))
		cfg, err := LoadUI(path)
		if err != nil {
			t.Fatalf("LoadUI(%d): %v", tc.in, err)
		}
		if cfg.TrashRetentionDays != tc.want {
			t.Errorf("TrashRetentionDays for %d = %d, want %d", tc.in, cfg.TrashRetentionDays, tc.want)
		}
	}
}
```

If `writeTempUI` does not yet exist in the test file, add it:

```go
func writeTempUI(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
```

Add the `fmt`, `os`, `path/filepath`, `testing` imports if missing.

- [ ] **Step 2: Run the tests to confirm they fail**

```
go test ./internal/config/ -run TestLoadUI_TrashRetention -v
```
Expected: build error or `undefined: TrashRetentionDays`.

- [ ] **Step 3: Add fields + parse + clamp**

Edit `internal/config/ui.go`:

In `UIConfig`, add (after `UndoSeconds`):

```go
	// TrashRetentionDays is the per-session sweep cutoff for the
	// Trash folder. 0 disables (default; the provider's own
	// retention policy applies). Clamped to [0, 365] on parse.
	// Set this only when you want tighter local enforcement than
	// your provider's policy or use a backend without server-side
	// retention.
	TrashRetentionDays int

	// SpamRetentionDays is the per-session sweep cutoff for the
	// Spam folder. 0 disables (default). See TrashRetentionDays
	// for context — providers normally enforce this server-side
	// (Gmail Spam: 30d; Fastmail: configurable per-account).
	SpamRetentionDays int
```

In `rawUI`, add:

```go
	TrashRetentionDays *int `toml:"trash_retention_days"`
	SpamRetentionDays  *int `toml:"spam_retention_days"`
```

In `LoadUI` (after the `UndoSeconds` block), append:

```go
	if raw.UI.TrashRetentionDays != nil {
		out.TrashRetentionDays = clampRetention(*raw.UI.TrashRetentionDays)
	}
	if raw.UI.SpamRetentionDays != nil {
		out.SpamRetentionDays = clampRetention(*raw.UI.SpamRetentionDays)
	}
```

Add at file end:

```go
// clampRetention bounds retention day values to [0, 365]. Negative
// inputs collapse to 0 (disabled).
func clampRetention(v int) int {
	if v < 0 {
		return 0
	}
	if v > 365 {
		return 365
	}
	return v
}
```

`DefaultUIConfig` does **not** need changes (zero values are 0).

- [ ] **Step 4: Run the tests to confirm they pass**

```
go test ./internal/config/ -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/config/ui.go internal/config/ui_test.go
git commit -m "Pass 6.6: add trash/spam retention config knobs"
```

---

## Task 2: Backend — add `Destroy` interface method + mock impl

**Files:**
- Modify: `internal/mail/backend.go`
- Modify: `internal/mail/mock.go`
- Modify: `internal/mail/mock_test.go` (or create `mock_destroy_test.go`)

- [ ] **Step 1: Write failing test for mock `Destroy`**

Append to `internal/mail/mock_test.go` (or a new file
`internal/mail/mock_destroy_test.go`):

```go
func TestMockBackend_Destroy_RecordsAndRemoves(t *testing.T) {
	m := NewMockBackend()
	headers, err := m.FetchHeaders(nil)
	if err != nil {
		t.Fatalf("FetchHeaders: %v", err)
	}
	startCount := len(headers)
	if startCount < 2 {
		t.Fatalf("mock seed too small: %d", startCount)
	}
	target := []UID{headers[0].UID, headers[1].UID}

	if err := m.Destroy(target); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	if len(m.DestroyCalls) != 1 {
		t.Fatalf("DestroyCalls len = %d, want 1", len(m.DestroyCalls))
	}
	if !equalUIDs(m.DestroyCalls[0], target) {
		t.Errorf("DestroyCalls[0] = %v, want %v", m.DestroyCalls[0], target)
	}

	after, err := m.FetchHeaders(nil)
	if err != nil {
		t.Fatalf("FetchHeaders after: %v", err)
	}
	if len(after) != startCount-2 {
		t.Errorf("after-Destroy count = %d, want %d", len(after), startCount-2)
	}
	for _, msg := range after {
		if msg.UID == target[0] || msg.UID == target[1] {
			t.Errorf("destroyed UID %q still present", msg.UID)
		}
	}
}

func TestMockBackend_Destroy_EmptyIsNoop(t *testing.T) {
	m := NewMockBackend()
	if err := m.Destroy(nil); err != nil {
		t.Fatalf("Destroy(nil): %v", err)
	}
	if len(m.DestroyCalls) != 0 {
		t.Errorf("DestroyCalls len = %d, want 0 (empty input is a no-op)", len(m.DestroyCalls))
	}
}

func equalUIDs(a, b []UID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

If `equalUIDs` already exists in the test file, drop the duplicate.

- [ ] **Step 2: Run the test to confirm it fails**

```
go test ./internal/mail/ -run TestMockBackend_Destroy -v
```
Expected: build error (`m.Destroy undefined`, `m.DestroyCalls
undefined`).

- [ ] **Step 3: Add `Destroy` to the `Backend` interface**

Edit `internal/mail/backend.go`. Add a new method just below
`Delete`:

```go
	// Destroy permanently deletes uids from the currently-selected
	// folder, bypassing Trash. Irreversible. Used by the retention
	// sweep and by manual "empty folder" actions on Trash/Spam.
	// Empty input is a no-op.
	Destroy(uids []UID) error
```

- [ ] **Step 4: Implement `Destroy` on `MockBackend`**

Edit `internal/mail/mock.go`. Add field to the `MockBackend` struct
(after `DeleteCalls`):

```go
	DestroyCalls [][]UID
```

Add the method (place it next to `Delete`):

```go
func (m *MockBackend) Destroy(uids []UID) error {
	if len(uids) == 0 {
		return nil
	}
	m.DestroyCalls = append(m.DestroyCalls, append([]UID(nil), uids...))
	gone := make(map[UID]struct{}, len(uids))
	for _, u := range uids {
		gone[u] = struct{}{}
	}
	kept := m.msgs[:0]
	for _, msg := range m.msgs {
		if _, drop := gone[msg.UID]; drop {
			continue
		}
		kept = append(kept, msg)
	}
	m.msgs = kept
	return nil
}
```

- [ ] **Step 5: Run the test to confirm it passes**

```
go test ./internal/mail/ -v
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/mail/backend.go internal/mail/mock.go internal/mail/mock_test.go
git commit -m "Pass 6.6: add mail.Backend.Destroy + mock impl"
```

---

## Task 3: JMAP — implement `Destroy`

**Files:**
- Modify: `internal/mailjmap/jmap.go`
- Modify: `internal/mailjmap/jmap_test.go`

- [ ] **Step 1: Write failing test for JMAP `Destroy`**

Look at `internal/mailjmap/jmap_test.go` for existing JMAP test
patterns (in particular, how `TestBackend_Move` or similar mocks
the JMAP server). Mirror that pattern for `Destroy`. The test
should verify that a `Backend.Destroy(uids)` call issues an
`Email/set` request whose `Destroy` field contains the UIDs as
`jmap.ID`s.

Append a new test (adapt to the existing helper conventions in the
file — look for `newTestBackend` or `withFakeServer` style helpers
and reuse them):

```go
func TestBackend_Destroy_IssuesEmailSetDestroy(t *testing.T) {
	b, server := newTestBackend(t)
	defer server.Close()

	server.Respond(func(req *jmap.Request) *jmap.Response {
		// Find the Email/set invocation and assert it carries the
		// destroy IDs we expect. Then return an empty response.
		// (Exact assertion shape mirrors existing tests.)
		// ... see existing test helpers for the canned-response pattern.
		return canned.EmailSetEmpty()
	})

	if err := b.Destroy([]mail.UID{"id-1", "id-2"}); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	got := server.LastEmailSetDestroy()
	want := []jmap.ID{"id-1", "id-2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("destroy = %v, want %v", got, want)
	}
}

func TestBackend_Destroy_EmptyIsNoop(t *testing.T) {
	b, server := newTestBackend(t)
	defer server.Close()

	if err := b.Destroy(nil); err != nil {
		t.Fatalf("Destroy(nil): %v", err)
	}
	if server.RequestCount() != 0 {
		t.Errorf("expected no JMAP requests for empty input, got %d", server.RequestCount())
	}
}
```

If the existing JMAP test harness does **not** expose
`LastEmailSetDestroy`/`RequestCount` helpers, write the test
against the actual harness shape used in the file. The intent is:
(a) one Email/set request; (b) destroy field equals input;
(c) empty input issues no request. **Re-shape the test code to
match the harness in the file before committing — do not invent
helpers that aren't there.**

- [ ] **Step 2: Run the test to confirm it fails**

```
go test ./internal/mailjmap/ -run TestBackend_Destroy -v
```
Expected: build error (`Destroy undefined on *Backend`).

- [ ] **Step 3: Implement `Destroy`**

Edit `internal/mailjmap/jmap.go`. Add a new method below
`Delete` (the existing `Delete` is at line ~666 and soft-deletes
to Trash; `Destroy` is its hard counterpart). Mirror the `Move`
shape for the request/response checking:

```go
// Destroy satisfies mail.Backend. It permanently deletes uids via
// Email/set destroy. Empty input is a no-op.
func (b *Backend) Destroy(uids []mail.UID) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	accountID := b.accountIDLocked()
	b.mu.Unlock()

	ids := make([]jmap.ID, 0, len(uids))
	for _, u := range uids {
		ids = append(ids, jmap.ID(u))
	}
	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	callID := req.Invoke(&email.Set{
		Account: accountID,
		Destroy: ids,
	})
	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("destroy: %w", err)
	}
	if err := checkEmailSetDestroyed(resp, callID); err != nil {
		return fmt.Errorf("destroy: %w", err)
	}
	return nil
}

// checkEmailSetDestroyed finds the Email/setResponse matching
// callID and returns an error if any ids appear in NotDestroyed.
// IDs already absent server-side are treated as success
// (idempotent).
func checkEmailSetDestroyed(resp *jmap.Response, callID string) error {
	for _, inv := range resp.Responses {
		if inv.CallID != callID {
			continue
		}
		sr, ok := inv.Args.(*email.SetResponse)
		if !ok {
			continue
		}
		for id, se := range sr.NotDestroyed {
			// "notFound" means the message is already gone
			// server-side — treat as success.
			if se.Type == "notFound" {
				continue
			}
			return fmt.Errorf("not destroyed %s: %s", id, se.Type)
		}
		return nil
	}
	return fmt.Errorf("no Email/set response")
}
```

If `email.Set.Destroy` or `email.SetResponse.NotDestroyed` is named
differently in this version of `git.sr.ht/~rockorager/go-jmap`,
adjust to the actual field names — confirm by reading the package
docs or grepping the vendored module.

- [ ] **Step 4: Run the test to confirm it passes**

```
go test ./internal/mailjmap/ -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/mailjmap/jmap.go internal/mailjmap/jmap_test.go
git commit -m "Pass 6.6: implement JMAP Destroy via Email/set"
```

---

## Task 4: IMAP stub for `Destroy`

**Files:**
- Modify: file in `internal/mailimap/` that implements
  `mail.Backend` methods (find via grep — see step 1).

- [ ] **Step 1: Locate the IMAP backend struct + existing method group**

```
grep -nR "func (b \*Backend) Delete" internal/mailimap/
```

Open the file containing `Delete` on the IMAP backend.

- [ ] **Step 2: Add a stub `Destroy` next to `Delete`**

Append to that file:

```go
// Destroy satisfies mail.Backend. Permanent delete is wired in
// Pass 8 alongside the Gmail IMAP backend rewrite.
func (b *Backend) Destroy(uids []mail.UID) error {
	return errors.New("destroy: not yet implemented (Pass 8)")
}
```

Add `"errors"` to imports if not already present.

- [ ] **Step 3: Verify the IMAP package still builds**

```
go build ./internal/mailimap/
```
Expected: success.

- [ ] **Step 4: Run the full test suite**

```
go test ./...
```
Expected: all pass. The IMAP stub is not invoked by tests yet,
but the module must compile for downstream packages.

- [ ] **Step 5: Commit**

```bash
git add internal/mailimap/
git commit -m "Pass 6.6: stub IMAP Destroy until Pass 8"
```

---

## Task 5: Cmd plumbing — `destroyCmd`, `emptyFolderCmd`, msg types

**Files:**
- Modify: `internal/ui/cmds.go`

- [ ] **Step 1: Add msg types and helpers**

Append to `internal/ui/cmds.go` (after the existing Move-picker
msg types at the bottom):

```go
// OpenConfirmEmptyMsg asks App to open the empty-folder confirm
// modal. Carries the folder display name and the UID-fetch +
// destroy chain so App's overlay handler does not need backend
// access. The total count is read from the most recent
// QueryFolder result and shown in the modal body.
type OpenConfirmEmptyMsg struct {
	Folder string // display name shown in modal title and toast
	Total  int    // count shown in the modal body
	Source string // provider folder name passed to Destroy
}

// EmptyFolderConfirmedMsg signals the user pressed `y` in the
// confirm modal. Triggers the actual destroy Cmd.
type EmptyFolderConfirmedMsg struct {
	Folder string
	Source string
}

// ConfirmModalClosedMsg signals the modal was dismissed without
// confirmation (n/Esc/q).
type ConfirmModalClosedMsg struct{}

// emptyFolderCmd queries every UID in src then issues Destroy.
// Returns an emptyFolderDoneMsg on success or ErrorMsg on failure.
// The display name in op is used by the error banner; the toast
// message uses the same name.
func emptyFolderCmd(b mail.Backend, displayName, src string) tea.Cmd {
	return func() tea.Msg {
		op := "empty " + strings.ToLower(displayName)
		if err := b.OpenFolder(src); err != nil {
			return ErrorMsg{Op: op, Err: err}
		}
		// QueryFolder returns up to limit; loop in pages so we
		// catch every UID even on very large folders.
		var all []mail.UID
		const page = 1000
		for offset := 0; ; {
			uids, total, err := b.QueryFolder(src, offset, page)
			if err != nil {
				return ErrorMsg{Op: op, Err: err}
			}
			all = append(all, uids...)
			offset += len(uids)
			if len(uids) == 0 || offset >= total {
				break
			}
		}
		if len(all) == 0 {
			return emptyFolderDoneMsg{folder: displayName, n: 0}
		}
		if err := b.Destroy(all); err != nil {
			return ErrorMsg{Op: op, Err: err}
		}
		return emptyFolderDoneMsg{folder: displayName, n: len(all)}
	}
}

// emptyFolderDoneMsg reports a successful manual empty.
type emptyFolderDoneMsg struct {
	folder string
	n      int
}

// destroyCmd issues a permanent delete for uids in src. Used by
// the retention sweep. Returns sweepCompletedMsg on success or
// ErrorMsg on failure. Empty input issues no backend call and
// returns sweepCompletedMsg{n:0}.
func destroyCmd(b mail.Backend, folder string, uids []mail.UID) tea.Cmd {
	return func() tea.Msg {
		if len(uids) == 0 {
			return sweepCompletedMsg{folder: folder, uids: nil}
		}
		if err := b.Destroy(uids); err != nil {
			return ErrorMsg{Op: "purge expired", Err: err}
		}
		return sweepCompletedMsg{folder: folder, uids: uids}
	}
}

// sweepCompletedMsg reports the result of a retention sweep.
// AccountTab applies ApplyDelete(uids) so destroyed rows leave
// the visible message list. n=0 sweep results carry nil uids.
type sweepCompletedMsg struct {
	folder string
	uids   []mail.UID
}
```

Add `"strings"` to the import block if not present.

- [ ] **Step 2: Build to confirm there are no compile errors**

```
go build ./internal/ui/
```
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/cmds.go
git commit -m "Pass 6.6: add destroy/empty cmds and msg types"
```

---

## Task 6: ConfirmModal component

**Files:**
- Create: `internal/ui/confirm_modal.go`
- Create: `internal/ui/confirm_modal_test.go`

- [ ] **Step 1: Write failing tests for ConfirmModal**

Create `internal/ui/confirm_modal_test.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestConfirmModal_OpenClose(t *testing.T) {
	m := NewConfirmModal(NewStyles(testTheme(t)))
	if m.IsOpen() {
		t.Fatal("new modal should be closed")
	}
	m = m.Open(ConfirmRequest{
		Title:   "Empty Trash",
		Body:    "247 messages will be permanently deleted.",
		OnYes:   func() tea.Msg { return EmptyFolderConfirmedMsg{Folder: "Trash"} },
	})
	if !m.IsOpen() {
		t.Fatal("opened modal should report IsOpen()")
	}
	m = m.Close()
	if m.IsOpen() {
		t.Fatal("closed modal should not report IsOpen()")
	}
}

func TestConfirmModal_YesEmitsOnYesAndCloses(t *testing.T) {
	m := NewConfirmModal(NewStyles(testTheme(t)))
	m = m.SetSize(80, 24)
	m = m.Open(ConfirmRequest{
		Title: "Empty Trash",
		Body:  "247 messages will be permanently deleted.",
		OnYes: func() tea.Msg { return EmptyFolderConfirmedMsg{Folder: "Trash"} },
	})

	got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("y should emit a Cmd batch (OnYes + Close)")
	}
	// The batch should contain both: a confirmed msg and a closed msg.
	msgs := drainBatch(cmd)
	if !containsMsg[EmptyFolderConfirmedMsg](msgs) {
		t.Errorf("missing EmptyFolderConfirmedMsg in batch: %#v", msgs)
	}
	if !containsMsg[ConfirmModalClosedMsg](msgs) {
		t.Errorf("missing ConfirmModalClosedMsg in batch: %#v", msgs)
	}
	_ = got
}

func TestConfirmModal_NoAndEscClose(t *testing.T) {
	for name, k := range map[string]tea.KeyMsg{
		"n":   {Type: tea.KeyRunes, Runes: []rune{'n'}},
		"esc": {Type: tea.KeyEsc},
	} {
		t.Run(name, func(t *testing.T) {
			m := NewConfirmModal(NewStyles(testTheme(t)))
			m = m.SetSize(80, 24)
			m = m.Open(ConfirmRequest{
				Title: "Empty Trash",
				Body:  "x",
				OnYes: func() tea.Msg { return EmptyFolderConfirmedMsg{} },
			})
			_, cmd := m.Update(k)
			if cmd == nil {
				t.Fatal("dismiss should emit Cmd")
			}
			msgs := drainBatch(cmd)
			if containsMsg[EmptyFolderConfirmedMsg](msgs) {
				t.Errorf("dismiss must not emit confirm: %#v", msgs)
			}
			if !containsMsg[ConfirmModalClosedMsg](msgs) {
				t.Errorf("dismiss must emit closed: %#v", msgs)
			}
		})
	}
}

func TestConfirmModal_QSwallowed(t *testing.T) {
	m := NewConfirmModal(NewStyles(testTheme(t))).SetSize(80, 24).Open(
		ConfirmRequest{Title: "x", Body: "x", OnYes: func() tea.Msg { return nil }},
	)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Errorf("q should be swallowed (nil cmd), got %#v", cmd)
	}
}

func TestConfirmModal_ViewWidthContract(t *testing.T) {
	m := NewConfirmModal(NewStyles(testTheme(t))).SetSize(80, 24).Open(
		ConfirmRequest{
			Title: "Empty Trash",
			Body:  "247 messages will be permanently deleted.",
			OnYes: func() tea.Msg { return nil },
		},
	)
	box := m.Box(80, 24)
	for i, line := range strings.Split(box, "\n") {
		w := lipgloss.Width(line)
		if w == 0 {
			continue
		}
		if w > 80 {
			t.Errorf("line %d width = %d, exceeds assigned 80", i, w)
		}
	}
}

// drainBatch and containsMsg are shared with movepicker_test.go /
// linkpicker_test.go. If those helpers exist, do not redeclare.
// Otherwise, add them here.
```

If `drainBatch` / `containsMsg` / `testTheme` already exist in
the package's test helpers, do not redeclare. Search:

```
grep -n "func drainBatch\|func containsMsg\|func testTheme" internal/ui/*_test.go
```

If they don't exist, add lightweight equivalents at the top of
`confirm_modal_test.go`:

```go
func drainBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		out := make([]tea.Msg, 0, len(batch))
		for _, c := range batch {
			if c == nil {
				continue
			}
			out = append(out, c())
		}
		return out
	}
	return []tea.Msg{msg}
}

func containsMsg[T any](msgs []tea.Msg) bool {
	for _, m := range msgs {
		if _, ok := m.(T); ok {
			return true
		}
	}
	return false
}
```

For `testTheme`, look at how other UI tests build a theme. If
there's no helper, use:

```go
func testTheme(t *testing.T) *theme.CompiledTheme {
	t.Helper()
	return theme.OneDark()  // or whatever the default constructor is
}
```

Confirm the theme constructor name with:

```
grep -n "func .*CompiledTheme\b" internal/theme/*.go
```

- [ ] **Step 2: Run the tests to confirm they fail**

```
go test ./internal/ui/ -run TestConfirmModal -v
```
Expected: build errors (`undefined: NewConfirmModal`,
`ConfirmRequest`, etc.).

- [ ] **Step 3: Implement ConfirmModal**

Create `internal/ui/confirm_modal.go`:

```go
// SPDX-License-Identifier: MIT

package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmRequest is the data the caller passes when opening the
// modal. Title goes in the box header, Body in the box content.
// OnYes is invoked as a tea.Cmd when the user presses `y`; it
// returns the application-specific confirmation message (e.g.
// EmptyFolderConfirmedMsg).
type ConfirmRequest struct {
	Title string
	Body  string
	OnYes func() tea.Msg
}

// ConfirmModal is a generic destructive-action confirmation
// overlay. App owns open state and overlay composition (mirrors
// MovePicker / LinkPicker; ADR-0087 / ADR-0091).
type ConfirmModal struct {
	open    bool
	req     ConfirmRequest
	width   int
	height  int
	styles  Styles
	keys    confirmKeys
}

type confirmKeys struct {
	Yes    key.Binding
	No     key.Binding
	Cancel key.Binding
}

// NewConfirmModal returns a closed, ready-to-Open modal.
func NewConfirmModal(styles Styles) ConfirmModal {
	return ConfirmModal{
		styles: styles,
		keys: confirmKeys{
			Yes:    key.NewBinding(key.WithKeys("y")),
			No:     key.NewBinding(key.WithKeys("n")),
			Cancel: key.NewBinding(key.WithKeys("esc")),
		},
	}
}

// IsOpen reports whether the modal is currently visible.
func (m ConfirmModal) IsOpen() bool { return m.open }

// Open snapshots the request and marks the modal open.
func (m ConfirmModal) Open(req ConfirmRequest) ConfirmModal {
	m.open = true
	m.req = req
	return m
}

// Close marks the modal closed.
func (m ConfirmModal) Close() ConfirmModal {
	m.open = false
	m.req = ConfirmRequest{}
	return m
}

// SetSize stores the viewport dimensions used by Box / View.
func (m ConfirmModal) SetSize(width, height int) ConfirmModal {
	m.width = width
	m.height = height
	return m
}

// Update handles key events while open. Returns the (possibly
// updated) modal and any Cmd batches.
func (m ConfirmModal) Update(msg tea.Msg) (ConfirmModal, tea.Cmd) {
	if !m.open {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, m.keys.Yes):
		onYes := m.req.OnYes
		return m, tea.Batch(
			func() tea.Msg {
				if onYes == nil {
					return nil
				}
				return onYes()
			},
			func() tea.Msg { return ConfirmModalClosedMsg{} },
		)
	case key.Matches(keyMsg, m.keys.No), key.Matches(keyMsg, m.keys.Cancel):
		return m, func() tea.Msg { return ConfirmModalClosedMsg{} }
	}
	// q is swallowed — consistent with help/link/move pickers.
	if keyMsg.String() == "q" {
		return m, nil
	}
	return m, nil
}

const (
	confirmModalMaxWidth = 50
	confirmModalMinWidth = 24
)

// View renders the modal box. Returns "" when closed.
func (m ConfirmModal) View() string {
	if !m.open {
		return ""
	}
	return m.Box(m.width, m.height)
}

// Box renders the modal at the supplied terminal dimensions.
// Honors the assigned width via wordwrap of the body.
func (m ConfirmModal) Box(termW, termH int) string {
	boxW := confirmModalMaxWidth
	if termW-4 < boxW {
		boxW = termW - 4
	}
	if boxW < confirmModalMinWidth {
		boxW = confirmModalMinWidth
	}
	contentW := boxW - 2

	title := " " + m.req.Title + " "
	rest := boxW - 2 - lipgloss.Width(title)
	if rest < 0 {
		rest = 0
	}

	bodyLines := wrapToWidth(m.req.Body, contentW)

	var b strings.Builder
	b.WriteString("┌─" + title + strings.Repeat("─", rest) + "┐\n")
	b.WriteString("│" + strings.Repeat(" ", contentW) + "│\n")
	for _, line := range bodyLines {
		b.WriteString("│" + padOrTruncate(line, contentW) + "│\n")
	}
	b.WriteString("│" + strings.Repeat(" ", contentW) + "│\n")
	b.WriteString("├" + strings.Repeat("─", contentW) + "┤\n")
	help := "[y] yes   [n] no   [esc] cancel"
	b.WriteString("│" + m.styles.Dim.Render(padOrTruncate(help, contentW)) + "│\n")
	b.WriteString("└" + strings.Repeat("─", contentW) + "┘")

	_ = termH // height not yet used; modal is naturally short
	return b.String()
}

// Position returns the top-left coordinate to place box on a
// total terminal of size totalW × totalH (centered).
func (m ConfirmModal) Position(box string, totalW, totalH int) (int, int) {
	return centerOverlay(box, totalW, totalH)
}

// wrapToWidth splits s into lines no wider than width display
// cells. Uses simple word-boundary wrapping; falls back to hard
// truncation for tokens longer than width.
func wrapToWidth(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := ""
	for _, w := range words {
		switch {
		case cur == "":
			cur = w
		case lipgloss.Width(cur+" "+w) <= width:
			cur = cur + " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	// Hard-truncate any line that's still too wide (single
	// over-long token).
	for i, ln := range lines {
		if lipgloss.Width(ln) > width {
			lines[i] = truncateToWidth(ln, width)
		}
	}
	return lines
}
```

`padOrTruncate`, `truncateToWidth`, and `centerOverlay` already
exist in the package (used by `MovePicker`).

- [ ] **Step 4: Run the tests to confirm they pass**

```
go test ./internal/ui/ -run TestConfirmModal -v
```
Expected: all pass.

- [ ] **Step 5: Run the full UI test suite to confirm nothing regressed**

```
go test ./internal/ui/
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/confirm_modal.go internal/ui/confirm_modal_test.go
git commit -m "Pass 6.6: ConfirmModal overlay component"
```

---

## Task 7: AccountKeys — add `Empty` binding

**Files:**
- Modify: `internal/ui/keys.go`

- [ ] **Step 1: Add the `Empty` field**

Edit `internal/ui/keys.go`. In the `AccountKeys` struct (after
`Move`), add:

```go
	Empty key.Binding
```

In `NewAccountKeys`, append:

```go
		Empty: key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "empty")),
```

- [ ] **Step 2: Build to confirm**

```
go build ./internal/ui/
```
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/keys.go
git commit -m "Pass 6.6: bind E to manual empty"
```

---

## Task 8: AccountTab — retention sweep + `E` dispatch

**Files:**
- Modify: `internal/ui/account_tab.go`
- Modify: `internal/ui/account_tab_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/ui/account_tab_test.go`:

```go
func TestAccountTab_RetentionSweep_FiresOnFirstTrashVisit(t *testing.T) {
	mock := mail.NewMockBackend()
	uiCfg := config.DefaultUIConfig()
	uiCfg.TrashRetentionDays = 30
	tab := NewAccountTab(NewStyles(testTheme(t)), testTheme(t), mock, uiCfg, SimpleIcons)
	tab = applyInitAndFolders(t, tab, mock)

	// Visit Trash. The sweep should fire once.
	tab = jumpAndLoad(t, tab, mock, "Trash")
	if len(mock.DestroyCalls) == 0 {
		t.Errorf("expected Destroy to be called on first Trash visit")
	}
	prevCalls := len(mock.DestroyCalls)

	// Bounce to Inbox and back. No second sweep.
	tab = jumpAndLoad(t, tab, mock, "Inbox")
	tab = jumpAndLoad(t, tab, mock, "Trash")
	if len(mock.DestroyCalls) != prevCalls {
		t.Errorf("sweep fired again on revisit; calls=%d want=%d", len(mock.DestroyCalls), prevCalls)
	}
}

func TestAccountTab_RetentionSweep_DisabledByDefault(t *testing.T) {
	mock := mail.NewMockBackend()
	uiCfg := config.DefaultUIConfig() // both retention knobs = 0
	tab := NewAccountTab(NewStyles(testTheme(t)), testTheme(t), mock, uiCfg, SimpleIcons)
	tab = applyInitAndFolders(t, tab, mock)
	tab = jumpAndLoad(t, tab, mock, "Trash")
	if len(mock.DestroyCalls) != 0 {
		t.Errorf("sweep fired with retention=0; calls=%d", len(mock.DestroyCalls))
	}
}

func TestAccountTab_EmptyKey_OnlyActiveOnDisposalFolders(t *testing.T) {
	mock := mail.NewMockBackend()
	uiCfg := config.DefaultUIConfig()
	tab := NewAccountTab(NewStyles(testTheme(t)), testTheme(t), mock, uiCfg, SimpleIcons)
	tab = applyInitAndFolders(t, tab, mock)
	tab = jumpAndLoad(t, tab, mock, "Inbox")

	tab2, cmd := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	_ = tab2
	if cmd != nil {
		// If a Cmd is emitted on Inbox, fail — E should be inert.
		// Drain it to confirm it's not an OpenConfirmEmptyMsg.
		msgs := drainBatch(cmd)
		if containsMsg[OpenConfirmEmptyMsg](msgs) {
			t.Error("E on Inbox should not open confirm modal")
		}
	}
}

func TestAccountTab_EmptyKey_OpensConfirmOnTrash(t *testing.T) {
	mock := mail.NewMockBackend()
	uiCfg := config.DefaultUIConfig()
	tab := NewAccountTab(NewStyles(testTheme(t)), testTheme(t), mock, uiCfg, SimpleIcons)
	tab = applyInitAndFolders(t, tab, mock)
	tab = jumpAndLoad(t, tab, mock, "Trash")

	_, cmd := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	if cmd == nil {
		t.Fatal("E on Trash should emit a Cmd")
	}
	msgs := drainBatch(cmd)
	if !containsMsg[OpenConfirmEmptyMsg](msgs) {
		t.Errorf("E on Trash should emit OpenConfirmEmptyMsg, got %#v", msgs)
	}
}

// applyInitAndFolders runs Init's loadFoldersCmd synchronously
// and feeds the result back into the tab so the sidebar is
// populated before the test continues.
func applyInitAndFolders(t *testing.T, tab AccountTab, mock *mail.MockBackend) AccountTab {
	t.Helper()
	cmd := tab.Init()
	if cmd == nil {
		t.Fatal("Init returned nil cmd")
	}
	for _, m := range drainBatch(cmd) {
		var c tea.Cmd
		tab, c = tab.Update(m)
		for _, follow := range drainBatch(c) {
			tab, _ = tab.Update(follow)
		}
	}
	return tab
}

// jumpAndLoad simulates a folder-jump key, then drains the resulting
// load chain (open/query/fetch headers) so the tab settles into the
// new folder.
func jumpAndLoad(t *testing.T, tab AccountTab, mock *mail.MockBackend, canonical string) AccountTab {
	t.Helper()
	keyByName := map[string]rune{
		"Inbox": 'I', "Drafts": 'D', "Sent": 'S',
		"Archive": 'A', "Spam": 'X', "Trash": 'T',
	}
	r, ok := keyByName[canonical]
	if !ok {
		t.Fatalf("no jump key for %q", canonical)
	}
	tab, cmd := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	for _, m := range drainBatch(cmd) {
		var c tea.Cmd
		tab, c = tab.Update(m)
		for _, follow := range drainBatch(c) {
			tab, _ = tab.Update(follow)
		}
	}
	return tab
}
```

Reuse `drainBatch` / `containsMsg` / `testTheme` from Task 6.

- [ ] **Step 2: Run the tests to confirm they fail**

```
go test ./internal/ui/ -run TestAccountTab_RetentionSweep -v
go test ./internal/ui/ -run TestAccountTab_EmptyKey -v
```
Expected: failures (sweep tests find 0 destroy calls; E tests
find no `OpenConfirmEmptyMsg`).

- [ ] **Step 3: Add `swept` field + retention sweep dispatch**

Edit `internal/ui/account_tab.go`. In the `AccountTab` struct
(after `pages`), add:

```go
	swept             map[string]bool
```

In `NewAccountTab`, initialize:

```go
		swept:         make(map[string]bool),
```

Modify `selectionChangedCmds` to also dispatch a sweep on entry to
a Disposal folder. Replace the existing function with:

```go
func (m *AccountTab) selectionChangedCmds() tea.Cmd {
	folder, ok := m.sidebar.SelectedFolderInfo()
	if !ok {
		return nil
	}
	m.loading = true
	cmds := []tea.Cmd{
		openFolderCmd(m.backend, folder.Name),
		m.spinner.Tick,
	}
	if sweep := m.maybeRetentionSweep(folder); sweep != nil {
		cmds = append(cmds, sweep)
	}
	return tea.Batch(cmds...)
}

// maybeRetentionSweep returns a destroyCmd over already-loaded
// messages older than the configured cutoff. Returns nil when the
// folder is not a Disposal folder, the relevant retention knob is
// 0, or the folder has already been swept this session. Marks the
// folder swept regardless of outcome (failures land in the error
// banner; we don't retry-loop within a session).
func (m *AccountTab) maybeRetentionSweep(folder mail.Folder) tea.Cmd {
	role := folder.Role
	var days int
	switch role {
	case "trash":
		days = m.uiCfg.TrashRetentionDays
	case "junk", "spam":
		days = m.uiCfg.SpamRetentionDays
	default:
		return nil
	}
	if days <= 0 {
		return nil
	}
	if m.swept[folder.Name] {
		return nil
	}
	m.swept[folder.Name] = true

	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	var expired []mail.UID
	for _, msg := range m.msglist.Source() {
		ts := msg.SentAt
		if ts.IsZero() {
			// Date string fallback parsing is best-effort; the
			// msglist.Source path is empty before headers load,
			// so most cases never hit this branch. When SentAt is
			// zero, skip the message — partial sweep is by design
			// (see spec §Sweep scope).
			continue
		}
		if ts.Before(cutoff) {
			expired = append(expired, msg.UID)
		}
	}
	return destroyCmd(m.backend, folder.Name, expired)
}
```

**Note:** the sweep above iterates `m.msglist.Source()` which is
**empty until the load Cmd resolves**. That's OK for the v1
"loaded-only" semantics — but it means the sweep on the **first
visit** finds zero expired UIDs because the load hasn't happened
yet. Fix: dispatch the sweep on `headersAppliedMsg` instead, which
is when the loaded message list is first populated.

Replace the above with this revised approach. Remove the call to
`maybeRetentionSweep` from `selectionChangedCmds` (revert that
function to its original shape) and instead dispatch the sweep
when headers arrive. In `updateTab`, modify the
`headersAppliedMsg` case:

```go
	case headersAppliedMsg:
		m.loading = false
		page := m.pageFor(msg.name)
		page.loaded = len(msg.msgs)
		fc := m.uiCfg.Folders[m.sidebar.ConfigKey(msg.name)]
		order := SortDateDesc
		if fc.Sort == "date-asc" {
			order = SortDateAsc
		}
		threaded := m.uiCfg.Threading
		if fc.ThreadingSet {
			threaded = fc.Threading
		}
		m.msglist.SetSort(order)
		m.msglist.SetThreaded(threaded)
		m.msglist.SetMessages(msg.msgs)
		// Retention sweep fires once per session per Disposal
		// folder, after the message list has been populated.
		if sweep := m.maybeRetentionSweep(msg.name, msg.msgs); sweep != nil {
			return m, sweep
		}
		return m, nil
```

And rewrite `maybeRetentionSweep` to take the loaded msgs
directly (no dependency on `msglist.Source`):

```go
// maybeRetentionSweep returns a destroyCmd over loaded messages
// older than the retention cutoff for the folder, when the folder
// is a Disposal folder, retention is enabled, and the folder
// hasn't been swept this session. Returns nil otherwise. Marks
// the folder swept on first call regardless of outcome.
func (m *AccountTab) maybeRetentionSweep(folderName string, loaded []mail.MessageInfo) tea.Cmd {
	folder, ok := m.folderByName(folderName)
	if !ok {
		return nil
	}
	var days int
	switch folder.Role {
	case "trash":
		days = m.uiCfg.TrashRetentionDays
	case "junk", "spam":
		days = m.uiCfg.SpamRetentionDays
	default:
		return nil
	}
	if days <= 0 {
		return nil
	}
	if m.swept[folder.Name] {
		return nil
	}
	m.swept[folder.Name] = true

	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	var expired []mail.UID
	for _, msg := range loaded {
		if msg.SentAt.IsZero() {
			continue
		}
		if msg.SentAt.Before(cutoff) {
			expired = append(expired, msg.UID)
		}
	}
	return destroyCmd(m.backend, folder.Name, expired)
}

// folderByName returns the backend Folder with the given provider
// name from the sidebar's folder list.
func (m AccountTab) folderByName(name string) (mail.Folder, bool) {
	for _, e := range m.sidebar.OrderedFolders() {
		if e.Provider == name {
			// OrderedFolders does not carry Role; do a second lookup
			// via SelectedFolderInfo only when name matches selected.
			// Fall through to the matched-by-canonical path instead.
			break
		}
	}
	// Easier path: snapshot folder list from sidebar entries.
	folder, ok := m.sidebar.FolderByProviderName(name)
	return folder, ok
}
```

The above relies on a `Sidebar.FolderByProviderName` accessor that
may not exist yet. If grep shows it does:

```
grep -n "FolderByProviderName" internal/ui/sidebar.go
```

returns nothing, add it to `internal/ui/sidebar.go`:

```go
// FolderByProviderName returns the raw backend Folder whose
// provider name (e.g. "Trash", "[Gmail]/Trash") matches name.
// Returns (zero, false) when not found.
func (s Sidebar) FolderByProviderName(name string) (mail.Folder, bool) {
	for _, e := range s.entries {
		if e.cf.Folder.Name == name {
			return e.cf.Folder, true
		}
	}
	return mail.Folder{}, false
}
```

Add `import "time"` at the top of `account_tab.go` if not already
present.

Also handle `sweepCompletedMsg` in `updateTab`:

```go
	case sweepCompletedMsg:
		if len(msg.uids) > 0 {
			m.msglist.ApplyDelete(msg.uids)
		}
		return m, nil
```

- [ ] **Step 4: Add `E` dispatch in `handleKey`**

Add a new case in the switch in `handleKey` (alongside `Move`):

```go
	case key.Matches(msg, m.keys.Empty):
		return m, m.dispatchEmpty()
```

Add the helper:

```go
// dispatchEmpty checks the current folder is a Disposal folder
// and emits OpenConfirmEmptyMsg. Returns nil otherwise (E is
// inert outside Trash/Spam).
func (m *AccountTab) dispatchEmpty() tea.Cmd {
	folder, ok := m.sidebar.SelectedFolderInfo()
	if !ok {
		return nil
	}
	var display string
	switch folder.Role {
	case "trash":
		display = "Trash"
	case "junk", "spam":
		display = "Spam"
	default:
		return nil
	}
	page := m.pages[folder.Name]
	total := 0
	if page != nil {
		total = page.total
	}
	return func() tea.Msg {
		return OpenConfirmEmptyMsg{
			Folder: display,
			Total:  total,
			Source: folder.Name,
		}
	}
}
```

Also handle `EmptyFolderConfirmedMsg` in `updateTab` so the
destroy-all Cmd fires when App forwards the confirmation:

```go
	case EmptyFolderConfirmedMsg:
		return m, emptyFolderCmd(m.backend, msg.Folder, msg.Source)

	case emptyFolderDoneMsg:
		// Clear loaded rows so the folder shows empty immediately.
		// MessageList exposes ClearAll for this; if not, fall back
		// to ApplyDelete on every loaded UID.
		all := make([]mail.UID, 0, m.msglist.Count())
		for _, msg := range m.msglist.Source() {
			all = append(all, msg.UID)
		}
		if len(all) > 0 {
			m.msglist.ApplyDelete(all)
		}
		// Emit a no-undo toast via App. App reads this through the
		// triageStartedMsg pathway with a sentinel inverse=nil and
		// a flag suppressing [u undo].
		return m, func() tea.Msg {
			return triageStartedMsg{
				op:      "empty",
				n:       msg.n,
				dest:    msg.folder,
				inverse: nil,
				onUndo:  nil,
			}
		}
```

If `MessageList.Source()` does not exist on `*MessageList`, look
for the equivalent (likely `m.msglist.Messages()` or by iterating
`m.msglist.Count()` rows). Confirm:

```
grep -n "func (.*MessageList).*Source\|func (.*MessageList).*Messages\b" internal/ui/msglist.go
```

If neither exists, add a `Source()` accessor returning the
underlying `[]mail.MessageInfo` slice. Existing code already uses
the source slice (for example via `SnapshotSource`) so an exposed
accessor is a natural addition.

- [ ] **Step 5: Run the tests to confirm they pass**

```
go test ./internal/ui/ -run TestAccountTab_RetentionSweep -v
go test ./internal/ui/ -run TestAccountTab_EmptyKey -v
go test ./internal/ui/
```
Expected: all pass. If `MockBackend.DestroyCalls` is recorded
twice for the seed messages (which are not in time.April-old
range), adjust the test to seed older messages or to assert
"sweep was attempted" rather than "messages were destroyed."
Specifically, the mock seed dates are in April 2026 — for a
30-day cutoff at "today", expect zero expired UIDs and therefore
zero `DestroyCalls`. Update the test assertion to verify the
sweep was attempted (e.g., the swept-flag side effect via a
public `IsSwept(folder)` accessor) rather than counting
`DestroyCalls`. Add this accessor to AccountTab:

```go
// IsSwept reports whether the retention sweep has already run
// for the given provider folder name in the current session.
// Used by tests; not currently consumed by the UI.
func (m AccountTab) IsSwept(folder string) bool {
	return m.swept[folder]
}
```

And rewrite the sweep test to assert `tab.IsSwept("Trash")` is
true after the first visit and stays true (with no extra calls)
on the second.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/account_tab.go internal/ui/account_tab_test.go internal/ui/sidebar.go internal/ui/msglist.go
git commit -m "Pass 6.6: AccountTab retention sweep + E dispatch"
```

---

## Task 9: App — own ConfirmModal + route keys + render overlay

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/ui/app_test.go`:

```go
func TestApp_OpensConfirmModalOnEmptyMsg(t *testing.T) {
	app := buildTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	app, _ = app.Update(OpenConfirmEmptyMsg{
		Folder: "Trash",
		Total:  247,
		Source: "Trash",
	})
	if !app.IsConfirmOpen() {
		t.Fatal("expected confirm modal open after OpenConfirmEmptyMsg")
	}
	view := app.View()
	if !strings.Contains(view, "Empty Trash") {
		t.Errorf("view missing modal title; got:\n%s", view)
	}
	if !strings.Contains(view, "247") {
		t.Errorf("view missing message count; got:\n%s", view)
	}
}

func TestApp_ConfirmYesEmitsConfirmedMsgAndCloses(t *testing.T) {
	app := buildTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app, _ = app.Update(OpenConfirmEmptyMsg{Folder: "Trash", Total: 5, Source: "Trash"})

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	msgs := drainBatch(cmd)
	if !containsMsg[EmptyFolderConfirmedMsg](msgs) {
		t.Errorf("expected EmptyFolderConfirmedMsg, got %#v", msgs)
	}
}

func TestApp_ConfirmEscClosesWithoutEmit(t *testing.T) {
	app := buildTestApp(t)
	app, _ = app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app, _ = app.Update(OpenConfirmEmptyMsg{Folder: "Trash", Total: 5, Source: "Trash"})

	app2, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	msgs := drainBatch(cmd)
	if containsMsg[EmptyFolderConfirmedMsg](msgs) {
		t.Error("Esc should not emit confirmation")
	}
	// After draining the close msg, the modal should be closed.
	for _, m := range msgs {
		app2, _ = app2.Update(m)
	}
	if app2.IsConfirmOpen() {
		t.Error("Esc should close the modal")
	}
}
```

If `buildTestApp` does not exist, locate the equivalent in the
file (search for `NewApp(` calls in tests) and reuse / wrap.

- [ ] **Step 2: Run the tests to confirm they fail**

```
go test ./internal/ui/ -run TestApp_(OpensConfirm|ConfirmYes|ConfirmEsc) -v
```
Expected: failures (`IsConfirmOpen` undefined; `OpenConfirmEmptyMsg`
not handled).

- [ ] **Step 3: Wire ConfirmModal into App**

Edit `internal/ui/app.go`. Add to the `App` struct (after
`movePicker`):

```go
	confirm     ConfirmModal
```

In `NewApp`, after the `movePicker` initialization:

```go
		confirm:     NewConfirmModal(styles),
```

Add a public accessor (used by tests):

```go
// IsConfirmOpen reports whether the confirmation modal is visible.
func (m App) IsConfirmOpen() bool { return m.confirm.IsOpen() }
```

In `Update`, add cases for the new messages. Place them with the
other overlay msg cases:

```go
	case OpenConfirmEmptyMsg:
		body := strconv.Itoa(msg.Total) + " messages will be permanently deleted."
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Empty " + msg.Folder,
			Body:  body,
			OnYes: func() tea.Msg {
				return EmptyFolderConfirmedMsg{
					Folder: msg.Folder,
					Source: msg.Source,
				}
			},
		})
		return m, nil

	case ConfirmModalClosedMsg:
		m.confirm = m.confirm.Close()
		return m, nil

	case EmptyFolderConfirmedMsg:
		var cmd tea.Cmd
		m.acct, cmd = m.acct.Update(msg)
		m = m.deriveChromeFromAcct()
		return m, cmd
```

Add `"strconv"` to imports.

In `WindowSizeMsg` handling, also forward size to the confirm
modal:

```go
		m.confirm = m.confirm.SetSize(m.width, m.height)
```

In the key-routing branch (the `case tea.KeyMsg:` block), add a
short-circuit for the confirm modal — placed **before** the
linkPicker/movePicker checks so confirm wins precedence (the
modal is the topmost overlay; the user must dismiss it first):

```go
		if m.confirm.IsOpen() {
			var cmd tea.Cmd
			m.confirm, cmd = m.confirm.Update(msg)
			return m, cmd
		}
```

In `View`, add overlay rendering for the confirm modal — placed
**before** the link/move picker checks for the same precedence
reason:

```go
	if m.confirm.IsOpen() {
		box := m.confirm.Box(m.width, m.height)
		x, y := m.confirm.Position(box, m.width, m.height)
		dimmed := DimANSI(frame)
		return PlaceOverlay(x, y, box, dimmed)
	}
```

- [ ] **Step 4: Run the tests to confirm they pass**

```
go test ./internal/ui/ -run TestApp_ -v
```
Expected: all pass.

- [ ] **Step 5: Run the full UI suite**

```
go test ./internal/ui/
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/app.go internal/ui/app_test.go
git commit -m "Pass 6.6: App owns ConfirmModal + handles empty-folder flow"
```

---

## Task 10: Toast — support no-undo render path

**Files:**
- Modify: `internal/ui/toast.go`
- Modify: `internal/ui/toast_test.go`

The empty-folder success toast uses the existing `pendingAction`
shape but must render without the `[u undo]` hint, since the
backend commit is irreversible. The simplest expression is a new
`op == "empty"` case in `toastVerb` plus an `inverse == nil` check
in `renderToast` to suppress the hint.

- [ ] **Step 1: Write failing test**

Append to `internal/ui/toast_test.go`:

```go
func TestRenderToast_EmptyOpHasNoUndoHint(t *testing.T) {
	p := pendingAction{
		op:      "empty",
		n:       247,
		dest:    "Trash",
		inverse: nil,
	}
	out := renderToast(p, 80, NewStyles(testTheme(t)))
	if out == "" {
		t.Fatal("renderToast returned empty for empty op")
	}
	if strings.Contains(out, "[u undo]") {
		t.Errorf("empty toast should not advertise undo; got %q", out)
	}
	if !strings.Contains(out, "Emptied Trash") {
		t.Errorf("expected 'Emptied Trash' in toast; got %q", out)
	}
	if !strings.Contains(out, "247") {
		t.Errorf("expected count in toast; got %q", out)
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```
go test ./internal/ui/ -run TestRenderToast_EmptyOpHasNoUndoHint -v
```
Expected: failure (toast contains `[u undo]` or doesn't render
"Emptied Trash" yet).

- [ ] **Step 3: Add the empty case to `toastVerb` + suppress hint when inverse is nil**

Edit `internal/ui/toast.go`. Add to the switch in `toastVerb`:

```go
	case "empty":
		return "Emptied"
```

In `renderToast`, change the body block for `move` and add an
`empty` case alongside (since both render `verb + dest`):

Replace:

```go
	case "move":
		body = fmt.Sprintf("%s %d %s to %s", verb, p.n, pluralize("message", p.n), p.dest)
```

With:

```go
	case "move":
		body = fmt.Sprintf("%s %d %s to %s", verb, p.n, pluralize("message", p.n), p.dest)
	case "empty":
		body = fmt.Sprintf("%s %s (%d)", verb, p.dest, p.n)
```

Then conditionally suppress the hint. Replace:

```go
	hint := "[u undo]"
	full := "✓ " + body + "   " + hint
```

With:

```go
	hint := "[u undo]"
	if p.inverse == nil {
		hint = ""
	}
	full := "✓ " + body
	if hint != "" {
		full = full + "   " + hint
	}
```

Also fix the truncation branch — when `hint == ""`, skip the
hint-budget arithmetic:

```go
	if lipgloss.Width(full) <= width {
		return styles.Toast.Render(full)
	}
	if hint == "" {
		return styles.Toast.Render(truncateToWidth(full, width))
	}
	hintW := lipgloss.Width(hint)
	bodyBudget := width - hintW - 4 // "✓ " + "   "
	if bodyBudget < 1 {
		return styles.Toast.Render(truncateToWidth(full, width))
	}
	bodyTrunc := truncateToWidth("✓ "+body, bodyBudget+2)
	return styles.Toast.Render(bodyTrunc + "   " + hint)
```

- [ ] **Step 4: Run the test**

```
go test ./internal/ui/ -run TestRenderToast -v
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/toast.go internal/ui/toast_test.go
git commit -m "Pass 6.6: toast supports no-undo render for empty op"
```

---

## Task 11: Help popover — advertise `E empty`

**Files:**
- Modify: `internal/ui/help_popover.go`

- [ ] **Step 1: Add the binding row**

Edit `internal/ui/help_popover.go`. In the `accountGroups` slice,
locate the Triage group (rows include `d delete`, `a archive`,
`s star`, `. read/unrd`, `m move`, `u undo`). Append an `E` row:

```go
				{"E", "empty", true},
```

- [ ] **Step 2: Verify the help popover still renders without overflow**

```
go test ./internal/ui/ -run TestHelp -v
```
Expected: all pass. If a test enforces a fixed row count for the
Triage group, update it.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/help_popover.go internal/ui/help_popover_test.go
git commit -m "Pass 6.6: advertise E empty in help popover"
```

---

## Task 12: Live verification (tmux)

**Files:** none.

This step has no code change — it verifies the feature in a real
terminal. See `.claude/docs/tmux-testing.md` for the tmux session
workflow.

- [ ] **Step 1: Build + install**

```
make install
```

- [ ] **Step 2: Run poplar in a tmux pane against the mock backend**

(Use the standard tmux harness from `.claude/docs/tmux-testing.md`.)

- [ ] **Step 3: Verify cases**

- `T` jumps to Trash, no error banner.
- `E` while on Trash opens the confirm modal centered, dimmed
  underlay visible.
- `n` closes the modal without destroying anything.
- `E` again, then `y` — toast `Emptied Trash (5)` appears
  with no `[u undo]` hint, message list empties.
- `E` while on Inbox is inert.
- With `trash_retention_days = 0` (default), no sweep on Trash
  visit (no destroy traffic).
- With `trash_retention_days = 30` set in `accounts.toml`, Trash
  visit triggers a sweep (mock seed has no expired messages so
  nothing visibly changes; check via debug logs or by adding a
  synthetic old message to the mock seed for verification).

- [ ] **Step 4: If anything fails, fix it before continuing**

Capture the failing case via tmux capture-pane, fix in the
relevant file, re-run `make install`, re-verify.

- [ ] **Step 5: No commit (this task is verification only)**

---

## Task 13: `make check`

**Files:** none.

- [ ] **Step 1: Run the commit gate**

```
make check
```
Expected: vet + tests pass.

- [ ] **Step 2: If anything fails, fix it before continuing**

---

## Pass-end consolidation (handled by poplar-pass skill)

The remaining steps — running `/simplify`, writing ADRs, updating
`docs/poplar/invariants.md`, updating `docs/poplar/STATUS.md`,
archiving this plan + the spec, final `make check`, and the
commit + push + install — are the consolidation ritual. They are
performed via the `poplar-pass` skill, not as plan tasks.

Trigger phrase: "ship pass" or "finish pass."
