# Pass 6.6 — Trash retention + manual empty

**Date:** 2026-05-01
**Status:** Design approved, pre-implementation.

## Goal

Give poplar a permanent-delete primitive and two surfaces that use
it: a per-session retention sweep that purges expired messages from
Disposal folders, and a manual "empty folder" key for Trash and Spam.
Default behavior defers to the provider — the knobs exist for users
who want tighter local enforcement and for future local backends
(POP3, maildir) where there is no server-side retention.

## Non-goals

- Querying the provider for its actual retention policy.
- Surfacing "days until server-purge" in the UI.
- Bulk destructive operations across multiple folders at once.
- IMAP wiring of `Destroy` (deferred to Pass 8 with the Gmail
  backend rewrite).

## Provider context

- **Gmail (IMAP):** `[Gmail]/Trash` and `[Gmail]/Spam` both
  auto-purge at 30 days, hard-coded, no user override. Manual
  empty in IMAP is `UID STORE +FLAGS \Deleted` + `UID EXPUNGE`
  while the folder is selected.
- **Fastmail (JMAP):** Trash retention is user-configurable in
  Fastmail account settings (default ~14 days). Spam auto-purges
  at 30 days, also configurable. Manual empty is
  `Email/set { destroy: [...uids] }` — true delete, bypasses
  Trash.

Both providers already enforce server-side retention. Poplar's
knobs only matter when the user wants tighter local enforcement
than the provider's policy, or for backends with no server-side
policy at all.

## Config

In `internal/config/ui.go`:

```toml
[ui]
trash_retention_days = 0   # 0 = disabled (default; provider handles it)
spam_retention_days  = 0   # 0 = disabled (default)
```

- Both clamped to `[0, 365]`.
- Both default to `0` (disabled).
- Field comments must explicitly note that retention is normally
  enforced by the provider (Fastmail/Gmail/etc.) and these knobs
  only matter for tighter local enforcement or for future local
  backends.

`config.LoadUI` decodes both fields; existing `[ui]` decode path
extends without restructure.

## Backend

Add one method to `mail.Backend` (`internal/mail/backend.go`):

```go
// Destroy permanently deletes the given UIDs in the currently
// selected folder. Bypasses Trash. Irreversible.
Destroy(uids []UID) error
```

- **JMAP (`internal/mailjmap/jmap.go`):** `Email/set { destroy: [...uids] }`.
  On partial failure, return an error wrapping the `notDestroyed`
  map. UIDs that the server reports as already-gone are treated
  as success (idempotent).
- **Mock (`internal/mail/mock.go`):** removes the matching entries
  from the in-memory message list.
- **IMAP (`internal/mailimap/`):** stub returning
  `errors.New("destroy: not yet implemented")` until Pass 8.

`Backend.Delete` keeps its current "move to Trash" semantics
(`internal/mailjmap/jmap.go:666`). The two methods are now clearly
distinct: `Delete` is soft (move to Trash), `Destroy` is hard
(permanent).

## Auto-purge sweep

Per-session, on first visit to a Disposal folder.

State on `AccountTab`:

```go
swept map[string]bool   // canonical folder name -> already swept this session
```

On `selectFolder(name)`:

1. Resolve the folder's classified role. Continue only if role is
   `Trash` or `Spam`.
2. Look up the relevant knob (`trashRetentionDays` for Trash,
   `spamRetentionDays` for Spam).
3. If knob is `0` (disabled) or `swept[name]` is true, skip.
4. Otherwise, after the folder's messages are loaded, fire a
   `tea.Cmd` that:
   - Computes `cutoff := now.Add(-time.Duration(knob) * 24 * time.Hour)`.
   - Walks the loaded message list and collects UIDs where the
     authoritative timestamp (`SentAt` if non-zero, else parsed
     `Date`) is before the cutoff.
   - If the UID list is non-empty, calls
     `backend.Destroy(uids)`.
   - Returns `sweepCompletedMsg{folder, n}` on success, or
     `ErrorMsg{Op: "purge expired", Err}` on failure.
5. Mark `swept[name] = true` immediately (before the Cmd runs)
   so an error doesn't trigger a retry loop within the session.

On `sweepCompletedMsg{folder, n}`: call
`MessageList.ApplyDelete(uids)` so the destroyed rows disappear
from the local view. No toast — the sweep is silent. The user
still sees the result via the message list updating.

**Sweep scope (v1):** the sweep operates on already-loaded
messages only — it does not page through the full folder via
`QueryFolder`. This is a deliberate scope choice. Disposal
folders are paginated like any other folder, and walking every
UID + fetching headers just to compute a cutoff is heavy for
folders with thousands of expired messages. v1 accepts a partial
sweep (older expired messages may sit unpurged until they're
loaded into view); a future pass can introduce a backend
`QueryOlderThan(name, cutoff)` primitive if this becomes
inadequate. Manual `E` remains the escape hatch for full
purging.

## Manual empty

New keybinding: **`E`** (capital E). Lives in the Triage group of
the help popover.

**Activation:** Only when the current folder's role is `Trash` or
`Spam`. Inert (with no help-popover advertisement of "wired") on
all other folders.

**Flow:**

1. `E` opens a `ConfirmModal` overlay. Body:

   ```
   Empty <Trash|Spam>?

   <N> messages will be permanently deleted.

   [y] yes   [n] no   [Esc] cancel
   ```

   `<N>` is the count of currently loaded messages in the folder.

2. While the modal is open, `App.Update` short-circuits keys into
   it (same overlay discipline as `MovePicker` and `LinkPicker`).
3. On `y`: close modal, fire a Cmd that calls
   `backend.QueryFolder(name, 0, total)` to get the full UID list
   (paginated load may have only fetched a window), then
   `backend.Destroy(allUIDs)`. On success, toast:
   `Emptied <folder> (N)`. On failure: `ErrorMsg{Op: "empty
   <folder>", Err}` flows through the standard error banner.
   The displayed count `<N>` in the modal body uses the folder's
   `total` (from the most recent `QueryFolder`), not the size of
   the loaded page — so the user sees the true folder size before
   confirming.
4. On `n`, `Esc`, or `q`: close modal, no action.
5. **No undo.** The standard triage toast bar shows the
   `Emptied …` text for the configured `undo_seconds` but with no
   `[u undo]` hint — backend commit is irreversible.

## ConfirmModal component

New file: `internal/ui/confirm_modal.go`.

**Bubbles analogue:** none directly. Mirrors the existing
App-owned overlay pattern from `LinkPicker` (ADR-0087) and
`MovePicker` (ADR-0091): dim underlay via `DimANSI`, composite via
`PlaceOverlay` at the centered top-left from `centerOverlay`.
Owns no I/O.

**Shape:**

```go
type ConfirmModal struct {
    title   string
    body    string
    onYes   tea.Cmd     // built by caller, fires on 'y'
    width   int
    height  int
    keys    confirmKeys // Yes (y), No (n), Cancel (Esc/q)
    open    bool
    theme   *theme.CompiledTheme
}
```

- `key.Binding`s declared in `confirmKeys`; dispatch via
  `key.Matches`.
- `View()` self-clips via `clipPane`, honors width via
  `wordwrap` + `hardwrap` for the body.
- Yes-key, no-key, and cancel-key are configurable so the same
  component is usable for any future destructive confirmation.

**App-owned state** in `internal/ui/app.go`:

```go
confirmOpen bool
confirm     ConfirmModal
```

`E` handling lives in `AccountTab` for the role check, then sends
a `tea.Msg` (`openConfirmEmptyMsg{folder, count, destroyCmd}`) up
to `App` to open the modal. This keeps modal ownership at `App`
(consistent with ADR-0087/0091).

## Help popover

Add `E empty` to the Triage group, marked `wired = true`. Existing
unwired Triage entries stay unwired. ADR-0072 future-binding
policy unchanged.

## Toast / error banner

Shared chrome row above the status bar (existing). Cases:

- **Empty success:** `Emptied Trash (247)` — same as a triage
  toast but with no `[u undo]` hint glyph.
- **Empty failure:** error banner wins (standard
  `ErrorMsg{Op: "empty trash", Err}` rendering).
- **Sweep failure:** error banner shows
  `⚠ purge expired: <err>`. Sweep success is silent.

## ADRs to write at pass end

1. **Retention policy:** default-disabled, provider-authoritative.
   Knobs are for tighter local enforcement and future local
   backends. Per-session sweep with `swept` map. Snapshot from
   loaded messages, no new list query.
2. **`Destroy` vs `Delete`:** two distinct backend primitives.
   `Delete` = soft (move to Trash), `Destroy` = hard (permanent).
   JMAP shape mirrored.
3. **ConfirmModal + manual empty:** generic confirm overlay,
   destructive-no-undo discipline, `E` key activation gated by
   folder role.

(Number of ADRs may consolidate during pass — this is the
inventory, not the final shape.)

## Testing

- Unit: `mail.Mock.Destroy` removes UIDs.
- Unit: retention-sweep cutoff filter (in `AccountTab` or a
  pure helper) — cutoff math, empty result, all-expired result,
  `SentAt`-zero falls back to `Date`.
- Unit: `ConfirmModal` key dispatch — `y`/`n`/`Esc`/`q`/unknown.
- Unit: `AccountTab.swept` flag is set on first visit and not
  re-fired on second visit.
- Unit: knob clamping in `LoadUI`.
- Live (tmux): `E` on Trash → modal renders → `y` → toast → list
  empty. `E` on Inbox → inert. Sweep on Trash with synthetic old
  messages → rows disappear silently.

## Out of scope (explicit)

- IMAP wiring of `Destroy` (Pass 8).
- Surfacing provider retention policy.
- Background/timer-driven sweep (only on-folder-load is in scope).
- Bulk multi-folder operations.
- Per-account retention overrides (`[ui]` is global).
