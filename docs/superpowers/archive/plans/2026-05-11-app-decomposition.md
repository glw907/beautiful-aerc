# Plan — Pass 29: app.go decomposition

**Goal.** Split `internal/ui/app.go` (1639 lines; `App.Update` is
877 lines) into per-domain files so no single file exceeds ~600
lines and `App.Update`'s outer body is a thin dispatcher.

**No behavior change.** This is a mechanical move. All tests stay
green; tmux render is byte-identical.

## Settled

- **Receiver stays `App`.** No new "controller" types own state.
  Per-domain handlers are methods on `App`, just moved to sibling
  files. This keeps state ownership at the root per Elm conventions.
- **Boundary by msg-package, not active screen.** A handler lives
  in the file named after the package that emits the msg
  (`uicompose.*` → `app_compose.go`, `outbox.*` /
  `outbox<Whatever>Msg` → `app_outbox.go`, `contacts.*` →
  `app_contacts.go`, `ConfirmModal*` / overlay-open/close →
  `app_modals.go`). Chrome msgs (toast, banner, notice, triage,
  backend update, undo) → `app_chrome.go`. Anything left over
  delegates to `m.acct` and lives in the default branch.
- **One genuine cross-domain msg: `uicompose.ScheduleAcceptedMsg`
  / `ScheduleCancelledMsg`.** Outbox-side reschedule reuses the
  same picker. Encoded as an explicit precedence rule inside
  `app_compose.go`: if `m.reschedule.picker != nil`, route to
  outbox via `rescheduleOpCmd`; else compose claims. The handler
  lives in `app_compose.go` because the msg type comes from
  `uicompose`; the outbox branch is one early-return inside it.
- **Key cascade unchanged.** `app_keys.go` owns the
  `tea.KeyPressMsg` branch verbatim: helpOpen → confirm → conflict
  → outbox → reschedule → outboxView → linkPicker → attachPicker
  → movePicker → form → popover → compose → contactsMode → drafts
  Enter override → App-level shortcuts → `m.acct.Update`. Same
  ordering, same guards.
- **`WindowSizeMsg` fan-out stays single-pass.** Moved to
  `updateSize` in `app_chrome.go` (it touches every overlay +
  account + compose + outboxView + reschedule + form + contacts).
  This is sizing, not a "domain" per se, but chrome is the
  closest fit — alternative was a dedicated `app_size.go` and
  that's a one-function file.
- **Wizard not in scope.** The wizard runs as a separate
  `tea.Program` in `cmd/poplar/config_cmd.go`; no msg type
  reaches `App.Update`. Drop `updateWizard` from the starter
  prompt's list.
- **Dispatcher shape.** Per-domain dispatchers return
  `(App, tea.Cmd, claimed bool)`. Outer `Update`:

  ```go
  func (m App) Update(msg tea.Msg) (App, tea.Cmd) {
      switch msg := msg.(type) {
      case tea.WindowSizeMsg:
          return m.updateSize(msg)
      case tea.KeyPressMsg:
          return m.updateKey(msg)
      }
      var cmd tea.Cmd
      var ok bool
      if m, cmd, ok = m.updateModalsMsg(msg); ok { return m, cmd }
      if m, cmd, ok = m.updateComposeMsg(msg); ok { return m, cmd }
      if m, cmd, ok = m.updateOutboxMsg(msg); ok { return m, cmd }
      if m, cmd, ok = m.updateContactsMsg(msg); ok { return m, cmd }
      if m, cmd, ok = m.updateChromeMsg(msg); ok { return m, cmd }
      // Default: account tab.
      m.acct, cmd = m.acct.Update(msg)
      m = m.deriveChromeFromAcct()
      return m, cmd
  }
  ```

  Each `updateXMsg` is a type-switch returning `(App, tea.Cmd,
  true)` on a matched case and the zero value `(m, nil, false)`
  on default.
- **Idiomatic-bubbletea unchanged.** Receiver still `App` (value);
  children still own size contract; `WindowSizeMsg` still
  forwarded; key dispatch still `key.Matches`; no new closures,
  no I/O in View. The §10 review checklist is a no-op-by-design
  for this pass.

## File layout after pass

- `app.go` — Model, `NewApp`, `Init`, accessors (`IsLinkPickerOpen`,
  `IsConfirmOpen`, `selectedMessage`, `hasBannerRow`,
  `contentHeight`, `rightPaneSize`, `deriveChromeFromAcct`,
  `suggestAddresses`), and the skinny `Update` dispatcher above.
  Target: <400 lines.
- `app_view.go` — `View`, `renderFrame`, `view`, `viewWithCursor`,
  `viewOverlay`, `windowTitle`, `frameCursor`, `chromeBannerRow`,
  `renderContactsFrame`, `contactsColumnWidths`,
  `sizedContactsChildren`, `contactsBodyHeight`, `formSize`,
  `parseSender`.
- `app_keys.go` — `updateKey(tea.KeyPressMsg)` and
  `updateContactsKey`.
- `app_modals.go` — `updateModalsMsg`. Confirm yes/no/closed,
  link/attach/move picker open/close, form open/save/cancel,
  popover open/close, `OpenConflictsFromOutboxMsg`, unsubscribe
  confirm open.
- `app_compose.go` — `updateComposeMsg`. `uicompose.*` (Send/Sent/
  Seeded/Cancel/Attach*/Schedule*), `RestoreFromDraftMsg`,
  `openDraftMsg`, `EnqueuePushDraftMsg`.
- `app_outbox.go` — `updateOutboxMsg`. `outbox*Msg` (depth,
  summary, conflicts, scheduled, cancelled, reschedule), conflict
  retry/discard, `outbox.Close/Cancel/Reschedule/EditAsDraft`,
  `refreshBackfillSegment`, `OpenConfirmEmptyMsg`,
  `EmptyFolderConfirmedMsg`.
- `app_contacts.go` — `updateContactsMsg`. `contacts.*` msgs and
  `contactsTickMsg`/`contactsSyncedMsg`.
- `app_chrome.go` — `updateChromeMsg`, `updateSize`. Toast/notice/
  error/triage/backend update/undo/unsubscribe-done helpers
  (`maybeResizeChild`, `unsubscribeHost`, `dispatchUnsubscribe`,
  `refreshBackfillSegment` — wait, that one's outbox-shaped;
  keep it in `app_outbox.go`).

## Tasks

1. `app_view.go` — extract render helpers. Run `go build`; smoke a
   tmux capture to confirm no visible drift.
2. `app_keys.go` — extract the `tea.KeyPressMsg` case body verbatim
   into `updateKey`. `App.Update` keeps the case dispatching to it.
3. `app_modals.go` — extract modal msg handlers. Return claimed-
   bool. Wire `updateModalsMsg` into Update.
4. `app_compose.go` — same for compose msgs.
5. `app_outbox.go` — same for outbox msgs.
6. `app_contacts.go` — same for contacts msgs.
7. `app_chrome.go` — same for chrome msgs; move `updateSize` here.
8. Thin `App.Update` to the dispatcher form above. Confirm
   `app.go` is under the 600-line budget.
9. `make check`; tmux 120×40 polish + 80×24 smoke captures.
10. `/simplify` on the diff. Apply genuine wins.
11. ADR-0214 (app.go decomposition); update `invariants.md` only
    if the architecture section needs a sentence about per-domain
    update files (probably yes — Architecture/Elm subsection).
    Add INDEX row.
12. Update STATUS: mark Pass 29 done; next starter prompt for
    Pass 30 (Audit B.1 — Elm + bubbletea v2 conformance).
13. Archive plan; commit + push + install.

## Risks

- **Method-name shadowing.** Splitting into files in the same
  package is mechanical; no name changes. `go vet`/`go build`
  catches typos.
- **Hidden ordering dependencies in the outer switch.** The
  current `App.Update` has fall-through behavior in a few cases
  (`case account.FolderLoadedMsg:` doesn't `return` if the outbox
  branch doesn't match — it falls into `m.acct.Update` at the
  bottom). Preserve this: `updateOutboxMsg` returns
  `claimed = true` only for the outbox-folder branch; the toast-
  commit branch and default account delegation both return
  `claimed = false` so the dispatcher falls through to the
  account default. Verify by reading each case body before
  moving it.
- **Test fragility.** `app_test.go` is 1749 lines and exercises
  App.Update heavily. Run it after each file extraction, not
  just at the end.

## Out of scope

- Renaming, simplifying, or merging msg types.
- Touching `app_test.go` beyond import fixes.
- Touching subpackages.
- ADR-0202 / wired-flag refactors.
