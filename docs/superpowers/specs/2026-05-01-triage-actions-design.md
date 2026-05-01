# Pass 6 — Triage actions design

Date: 2026-05-01
Status: brainstormed, awaiting plan

## Goal

Wire the four triage actions on the message list — delete (`d`),
archive (`a`), star/unstar (`s`), read/unread (`.`) — with
optimistic local mutation, a one-row toast/undo bar above the
status row, and visual-mode multi-select. Add `u` to undo the
most recent action while its toast is visible.

## Settled inputs

- Optimistic mutation (ADR-0086): UI flips state immediately; the
  backend Cmd runs in parallel; failures roll back via ErrorMsg.
- Foreground-only banner shape (ADR-0073): one row above the
  status bar, no overlay.
- Accessor-after-delegation (ADR-0088): no callbacks, no parent
  pointers; child→parent communication via typed `tea.Msg`.
- Modifier-free keybindings (ADR-0068).
- Triage key vocabulary already pinned in `keybindings.md`:
  `d`/`a`/`s`/`.` for delete/archive/star/read-toggle.
- Folder-jump uppercase letters (`I/D/S/A/X/T`) and `r` (reply)
  are reserved — no collisions with the triage keys above.

## Decisions (this pass)

### D1. Undo window: 6s hybrid timer with selective commit

A `pendingAction` is tracked while the toast is visible. Triggers
that **commit** (drop the toast, discard the inverse Cmd):

- Timer expiry (`undo_seconds`, default 6).
- Next mutating triage keypress (`d`/`a`/`s`/`.`).
- Folder change (`I/D/S/A/X/T` or `J/K` that actually moves to a
  different folder).

Browse keys do **not** commit: `j`/`k`/`g`/`G`, search interactions,
fold toggles, viewer open/close. The user can read while moving
around; only an action that changes mailbox state commits.

`u` while toast active → applies the saved inverse Cmd locally
and fires it to the backend, clears the toast.
`u` with no active toast → no-op.

Rationale: Material/Carbon/NN/g all converge on 6–8s for actionable
toasts; "next mutation commits" preserves the safety affordance
without leaking state into the user's next deliberate action.

### D2. Visual-mode scope: mode-agnostic marked-set or cursor

Single rule, identical inside and outside visual mode:

> A triage action operates on the marked set if non-empty,
> otherwise on the cursor row.

`v` enters visual-select mode and `Space` toggles selection on the
current row (unbinds Space-as-fold while the mode is active).
After a triage action runs, visual mode auto-exits and `marked`
clears.

Visual-mode-with-zero-marks falls through to cursor row by the
same rule — not a special case.

### D3. Cursor placement: stay + index-hold

After a delete or archive:

- Sidebar cursor stays on the source folder.
- Message-list cursor stays at the same row index. The row that
  took the deleted row's place is now under the cursor.
- If the action removed the last row(s), clamp to `len(rows)-1`.
- Bulk action: cursor lands at the index of the **first** removed
  display-row, clamped.

For star and read-toggle the row count is unchanged; cursor index
is trivially stable.

### D4. WYSIWYG triage on collapsed threads

A folded thread renders one display row representing N messages.
All four triage actions on a folded-thread row act on the **whole
thread** (root + all child UIDs).

This applies uniformly to delete, archive, star, and read-toggle.
The toast for a thread-scope action reports the message count
(`Deleted thread (5 messages) [u undo]`).

Rationale: the row visually represents N messages; triage that
silently affects only the root would be surprising.

### D5. Delete = move to Trash

Both backends already implement `Delete([]UID)`. For Pass 6 the
contract is: delete is a soft delete, semantically equivalent to
"move to Trash". No expunge in v1. Trash retention/auto-empty
ships in Pass 6.6.

The undo Cmd for delete is `Move(uids, sourceFolder)` — restoring
to the folder the messages came from.

### D6. Backend symmetry: add `MarkUnread`

`mail.Backend` gains `MarkUnread([]UID) error`. Both providers
support it natively (JMAP `keywords/$seen=false`; IMAP `STORE
-FLAGS \Seen`). Symmetric inverse for `MarkRead` makes the read-
toggle Cmd construction trivial.

### D7. Config knob: `[ui] undo_seconds`

```toml
[ui]
undo_seconds = 6  # default; clamp [2, 30]
```

Decoded by `config.LoadUI`; threaded into `App` at startup. Out-
of-bounds values clamp silently. Disabling the toast entirely is
not supported in v1.

### D8. Toast row contention

The single row above the status bar is shared between the error
banner and the toast. Precedence rule: **error banner wins** when
`lastErr.Err != nil`. Otherwise, if a toast is active, render it.
Otherwise, the row collapses and the account region grows by one.

When a backend Cmd fails after an optimistic mutation, the
inverse is applied locally to roll back state; the toast clears;
the banner takes the row.

## Architecture

### Backend additions (`internal/mail/`)

- Add `MarkUnread([]UID) error` to `mail.Backend`.
- Implement in `mock`, `mailjmap`, `mailimap`.

No other backend changes; existing `Delete`, `Move`, `Flag`,
`MarkRead` cover the rest.

### `internal/ui/msglist.go` — owned state

```go
visualMode bool
marked     map[mail.UID]struct{}
```

Helpers (unexported):

- `actionTargets() []mail.UID` — marked set if non-empty, else
  cursor UID. For thread-scope actions on a folded root, expand
  to all child UIDs (WYSIWYG, D4).
- `enterVisual()` / `exitVisual()` / `toggleMark(uid)`.
- `applyDelete(uids)` / `applyMove(uids)` / `applyFlag(uids, flag, set)` /
  `applySeen(uids, seen)` — local-only state mutations used both
  for the optimistic flip and for inverse roll-back. None of these
  fires a Cmd.

Cursor index tracking: existing row-index field. After mutation
and `rebuild()`, clamp to `len(rows)-1`; for bulk delete/archive,
seek to the saved "first-removed index" before clamping.

### `internal/ui/account_tab.go` — Cmd construction

For each triage keypress `AccountTab.Update`:

1. Resolve `targets := list.actionTargets()`.
2. Mutate `list` locally (D1 optimistic).
3. Exit visual mode if on.
4. Build forward Cmd and inverse Cmd (both closures over
   `mail.Backend`).
5. Return `triageStartedMsg{op, n, inverse}` plus the forward
   Cmd via `tea.Batch`.

`triageStartedMsg` is a typed Msg (ADR-0088). The inverse is a
prebuilt `tea.Cmd` value; App stores it without inspecting.

Archive destination: classify folders, find the canonical Archive
folder. If absent, return `ErrorMsg{Op: "archive", Err: ...}` and
skip the optimistic mutation.

### `internal/ui/app.go` — toast ownership

```go
type pendingAction struct {
    op       string         // "delete" | "archive" | ...
    n        int            // message count
    inverse  tea.Cmd
    deadline time.Time
}

type App struct {
    // ... existing
    toast pendingAction
    undoSeconds int  // from config.LoadUI
}
```

App handles three new Msg types:

- `triageStartedMsg{op, n, inverse}` — sets `toast`, fires
  `tea.Tick(undoSeconds * time.Second)` returning
  `toastExpireMsg{deadline}`.
- `toastExpireMsg{deadline}` — if `deadline == toast.deadline`,
  clear toast. Otherwise ignore (stale tick).
- `undoRequestedMsg` — if `toast.inverse != nil`, return the
  inverse Cmd, clear toast.

Existing `ErrorMsg` handler gains a side-effect: if `toast` is
non-zero, apply its inverse locally and clear it.

### `internal/ui/toast.go` — pure renderer

```go
func renderToast(p pendingAction, width int, styles Styles) string
```

Output shape: `✓ Deleted 3 messages   [u undo]`. Empty
`pendingAction` returns `""`. Truncation via the existing
`truncateToWidth` helper. Styles:

- `✓` and message text: `FgDim`.
- `[u undo]`: `AccentPrimary`, key glyph bold.

### `internal/ui/keys.go` — key bindings

New `key.Binding` entries on `accountKeyMap`:

- `Delete` — `d`.
- `Archive` — `a`.
- `Star` — `s`.
- `ReadToggle` — `.`.
- `EnterVisual` — `v` (new).
- `Undo` — `u` (new).

`Space` becomes context-sensitive: dispatched to `toggleMark` when
`visualMode == true`, to fold-toggle otherwise. This is a single-
key dual binding inside `MessageList.Update`, not two separate
bindings.

### `internal/config/ui.go` — config knob

Add `UndoSeconds int` to `UIConfig`, default 6, clamp `[2, 30]`
on parse.

## Data flow walk-through (delete one message)

1. User presses `d` on row 12.
2. `MessageList.Update` returns to `AccountTab.Update`. (Visual
   mode off, `marked` empty.) `targets = [uid12]`.
3. `AccountTab` calls `list.applyDelete([uid12])` — row 12 gone,
   row 13 shifts up under cursor.
4. `AccountTab` builds:
   - forward: `func() tea.Msg { if err := backend.Delete([uid12]); err != nil { return ErrorMsg{Op: "delete", Err: err} }; return nil }`.
   - inverse: a closure that calls `Move([uid12], sourceFolder)` *and* re-inserts the message locally.
5. Returns `triageStartedMsg{op: "delete", n: 1, inverse}` + forward Cmd.
6. App receives the Msg, sets `toast`, schedules `tea.Tick`.
7. Forward Cmd resolves: success → nil Msg, no further action;
   failure → ErrorMsg → App applies inverse locally, clears toast,
   sets `lastErr`.
8. While toast visible: user presses `u` → App applies inverse
   locally, fires inverse Cmd, clears toast.
9. While toast visible: user presses `j` → no-op for the toast,
   cursor moves.
10. While toast visible: user presses `a` → App commits the
    delete (drops toast), then this turn re-enters from step 1
    for the archive action.
11. Timer fires `toastExpireMsg{deadline}` → App clears toast if
    `deadline` matches. Action is now permanently committed
    locally; backend Cmd already resolved.

## Testing plan

Unit tests, table-driven, alongside source:

- `msglist_test.go` — `actionTargets` (cursor, marked-set, folded-
  thread expansion); visual-mode entry/exit/toggle; cursor placement
  after single + bulk delete (mid, last-row, only-row); applyFlag
  and applySeen no-movement; visual-mode auto-exit on triage.
- `account_tab_test.go` — forward + inverse Cmd construction per
  op (assertions via mock backend); failure path rolls back local
  state and surfaces ErrorMsg; archive when no Archive folder
  classified → ErrorMsg, no mutation; folded-thread WYSIWYG
  expansion.
- `app_test.go` — `triageStartedMsg` sets toast and schedules
  Tick; `toastExpireMsg` deadline match clears; stale Tick
  ignored; commit on next mutation; commit on folder change; no
  commit on j/k/g/G/search/fold; `u` fires inverse + clears; `u`
  with no toast no-op.
- `toast_test.go` — render output per op with count and `[u undo]`;
  width truncation; empty pendingAction → empty string.
- `error_banner_test.go` (extension) — banner-wins-row precedence
  with active toast.
- Live tmux capture per `bubbletea-conventions` §10: 120×40 with
  optimistic delete + toast row visible; minimum-viable-width
  capture confirming toast truncation.

## Out of scope

- Move-to-arbitrary-folder picker (`m` modal) — Pass 6.5.
- Trash retention + manual empty — Pass 6.6.
- Multi-step undo stack (only the most recent action is undoable
  in v1).
- Per-action timer overrides (single `undo_seconds` knob).
- Disabling the toast entirely (`undo_seconds = 0` clamps to 2).
- Spam/not-spam triage actions (`!`/`^`) — not in current
  keybindings vocabulary.

## ADRs to write at pass-end

1. Triage action vocabulary + key bindings (`d`/`a`/`s`/`.`/`u`).
2. Toast/undo bar shape: 6s timer, hybrid commit, error-banner
   row contention rule.
3. Visual mode is mode-agnostic for triage scope: marked-set-or-
   cursor, auto-exit on action.
4. Cursor placement: stay + index-hold for delete and archive.
5. Folded-thread triage is WYSIWYG: actions apply to the whole
   thread.
6. Delete = move to Trash (not expunge) for both backends.
7. Backend addition: `MarkUnread([]UID) error`.
8. Config knob: `[ui] undo_seconds` default 6, clamp `[2, 30]`.
