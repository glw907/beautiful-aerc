# Pass 9h.2 — core reorg finish

**Goal.** Extract `AccountTab` into `internal/ui/account/`, lift its
folder/outbox/triage/sweep cmds and msgs out of `internal/ui/cmds.go`,
audit the residual `cmds.go` down to App-only cross-cutting cmds,
refresh `docs/poplar/system-map.md`, and capture live tmux at 80×24
and 120×40 (the verification 9h.1 deferred).

Pure structural refactor. No behavior change. Runs `make check` after
each task.

**Spec / parent plan:** `docs/superpowers/archive/plans/2026-05-06-core-reorg.md`
(Tasks 7 + 8 of the original plan; 9h.1 landed Tasks 1–6).

---

## Open questions — resolved before coding

**Q1. Cycle risk in the account extract.**

The account subpackage cannot import `internal/ui/`. Anything
account references that currently lives in package `ui` has to
move. Audit:

| Symbol | Current location | Move to |
|---|---|---|
| `ErrorMsg` | cmds.go | `uicore` (multiple producers: account, reader cmds, App banner) |
| `triageOp` + `op*` constants | toast.go | `uicore` (shared with App's toast renderer) |
| `ComputeLayout` | layout.go | `uicore` (already returns `uicore.LayoutMode`) |
| `NewSpinner` | styles.go | `uicore` |
| `AccountKeys` + `NewAccountKeys` | keys.go | `account/keys.go` |
| `pendingAction` + `toastVerb` + toast rendering | toast.go | stays App-level (App owns the toast row) |
| `Styles` (account-relevant fields) | styles.go | narrow projection `account.Styles` (ADR-0161 pattern) |

Confirm-modal trio: `OpenConfirmEmptyMsg` and
`EmptyFolderConfirmedMsg` hoist to `account/msgs.go` (account is
producer/consumer, App is the generic mechanism in between).
`ConfirmModalClosedMsg` moves into `confirm_modal.go` next to
`ConfirmModalYesMsg`.

`URLOpener` and `TidyFn` stay App-level — account never calls
them; reader cmds and compose cmds (App-side) thread them
already. The function-typed parameter pattern is reserved for
those existing seams.

**Q2. Final residual `internal/ui/cmds.go`.**

After this pass, `cmds.go` holds only App-scoped cross-cutting
cmds:

| Symbol | Reason |
|---|---|
| `pumpUpdatesCmd` + `backendUpdateMsg` | App owns backend-update routing |
| `URLOpener` + `launchURLCmd` + `xdgOpenURL` | App-level URL launcher (reader fires `LaunchURLMsg`) |
| reader/compose cmds (`loadBodyCmd`, `markReadCmd`, `loadAttachmentsCmd`, `openAttachmentCmd`, `saveAttachmentCmd`, `composeSeedCmd`, `composeSendCmd`, `envelopeFromDraft`, `resolveSentFolder`, `sanitizeAttachFilename`, `resolveSaveTarget`) | stay (App orchestrates these; reader/compose subpackages have no `cmds.go` yet — that's a future pass) |

`pumpCacheCmd` + `cacheEventMsg` lift to account/ (account is the
primary consumer; App also consumes via `account.CacheEventMsg`,
which is fine since App imports account).

Outbox cmds (`refreshOutboxDepthCmd`, `loadOutboxSummaryCmd`,
`loadOutboxConflictsCmd`, `retryConflictCmd`, `discardConflictCmd`)
and their msgs lift to account/ — App calls them via the
exported names (`account.RefreshOutboxDepthCmd`, etc.).

Target: ≤ 250 LOC. ADR-0161's Pass 9h.1 prediction (that
reader/compose cmds + ErrorMsg + the confirm trio stay
App-level) narrows on three points: ErrorMsg/triageOp/spinner
hoist to `uicore`; cache pump + outbox hoist to account; and the
pre/post-modal msgs belong to the actor who opened the modal.

---

## Task 1: Extract `internal/ui/account/`

**Files:**
- `git mv internal/ui/account_tab.go internal/ui/account/model.go`
- `git mv internal/ui/account_tab_test.go internal/ui/account/model_test.go`
- Create: `internal/ui/account/cmds.go`, `internal/ui/account/msgs.go`
- Modify: `internal/ui/cmds.go` (lift cmds + msgs), `internal/ui/app.go`
  (field type, Update arms, type-switch qualifications), `internal/ui/confirm_modal.go`
  (absorb `ConfirmModalClosedMsg`)

### Step 1: Move files

```bash
mkdir -p internal/ui/account
git mv internal/ui/account_tab.go internal/ui/account/model.go
git mv internal/ui/account_tab_test.go internal/ui/account/model_test.go
```

### Step 2: Rewrite package + rename type in `model.go`/`model_test.go`

- `package ui` → `package account`.
- `AccountTab` → `Model` (every receiver, every field type).
- `NewAccountTab` → `New`.
- `Backend()` accessor stays exported. `now` seam stays unexported.
- Existing field references unchanged: `sidebar.Model`,
  `messagelist.Model`, `reader.Model`, `movepicker.Model`,
  `sidebar.Column`, `sidebar.Search` already qualified.
- `Styles` reference (line 37) — App-level type from
  `internal/ui/`. **Cycle risk.** Fix: change the field type from
  `Styles` to a narrow projection `account.Styles` constructed by
  `NewStyles(*theme.CompiledTheme)` per the per-subpackage Styles
  pattern (ADR-0161). Pull only the Styles fields account
  actually uses (toast, banner, footer-relevant). Audit
  by grep — most of account's styling is delegated to children.
- `uicore.IconSet` reference stays — `uicore` is the shared sibling.
- Test file: `package account` (existing test touches unexported state).

### Step 3: Lift cmds and msgs from `internal/ui/cmds.go`

Move to `internal/ui/account/cmds.go` (functions) and
`internal/ui/account/msgs.go` (types). All exported (cross-package).

| Symbol | Destination | Renamed to |
|---|---|---|
| `loadFoldersCmd` | `cmds.go` | `LoadFoldersCmd` |
| `queryFolderCmd` | `cmds.go` | `QueryFolderCmd` |
| `openFolderCmd` | `cmds.go` | `OpenFolderCmd` |
| `refreshFolderCmd` | `cmds.go` | `RefreshFolderCmd` |
| `loadMoreCmd` | `cmds.go` | `LoadMoreCmd` |
| `queueOpsCmd` | `cmds.go` | `QueueOpsCmd` |
| `enqueueDestroys` | `cmds.go` | unexported helper |
| `emptyFolderCmd` | `cmds.go` | `EmptyFolderCmd` |
| `destroyCmd` | `cmds.go` | `DestroyCmd` |
| `refreshOutboxDepthCmd` | `cmds.go` | `RefreshOutboxDepthCmd` |
| `loadOutboxSummaryCmd` | `cmds.go` | `LoadOutboxSummaryCmd` |
| `loadOutboxConflictsCmd` | `cmds.go` | `LoadOutboxConflictsCmd` |
| `retryConflictCmd` | `cmds.go` | `RetryConflictCmd` |
| `discardConflictCmd` | `cmds.go` | `DiscardConflictCmd` |
| `initialWindow` | `cmds.go` | unexported const |
| `triageOp` (the enum) | `cmds.go` | exported `TriageOp` if used by App; otherwise unexported |
| `foldersLoadedMsg` | `msgs.go` | `FoldersLoadedMsg` |
| `folderLoadedMsg` | `msgs.go` | `FolderLoadedMsg` |
| `folderAppendedMsg` | `msgs.go` | `FolderAppendedMsg` |
| `triageStartedMsg` | `msgs.go` | `TriageStartedMsg` |
| `toastExpireMsg` | `msgs.go` | `ToastExpireMsg` |
| `undoRequestedMsg` | `msgs.go` | `UndoRequestedMsg` |
| `emptyFolderDoneMsg` | `msgs.go` | `EmptyFolderDoneMsg` |
| `sweepCompletedMsg` | `msgs.go` | `SweepCompletedMsg` |
| `outboxDepthMsg` | `msgs.go` | `OutboxDepthMsg` |
| `outboxSummaryMsg` | `msgs.go` | `OutboxSummaryMsg` |
| `outboxConflictsMsg` | `msgs.go` | `OutboxConflictsMsg` |
| `conflictResolvedMsg` | `msgs.go` | `ConflictResolvedMsg` |
| `OpenConfirmEmptyMsg` | `msgs.go` | already exported, keep as `OpenConfirmEmptyMsg` |
| `EmptyFolderConfirmedMsg` | `msgs.go` | already exported, keep |

`ConfirmModalClosedMsg` moves to `confirm_modal.go` (not account).

### Step 4: Rewrite name references inside `account/`

Within `account/model.go` + `account/model_test.go`, every now-renamed
symbol gets the new name (since they're same-package now, unqualified).
For example: `loadFoldersCmd(...)` → `LoadFoldersCmd(...)`,
`foldersLoadedMsg{...}` → `FoldersLoadedMsg{...}`.

### Step 5: Update `internal/ui/app.go`

Add import:

```go
import (
    "github.com/glw907/poplar/internal/ui/account"
    // ...
)
```

Rewrites:

- Field: `Account AccountTab` → `Account account.Model`.
- Constructor: `NewAccountTab(...)` → `account.New(...)`.
- Type-switch arms — qualify the msgs App genuinely consumes:
  - `case OpenConfirmEmptyMsg:` → `case account.OpenConfirmEmptyMsg:`
  - `case EmptyFolderConfirmedMsg:` → `case account.EmptyFolderConfirmedMsg:`
  - any outbox/toast/triage msg App reads (audit via `grep -nE
    "foldersLoadedMsg|folderLoadedMsg|folderAppendedMsg|triageStartedMsg|toastExpireMsg|undoRequestedMsg|emptyFolderDoneMsg|sweepCompletedMsg|outboxDepthMsg|outboxSummaryMsg|outboxConflictsMsg|conflictResolvedMsg" internal/ui/app.go`).
- `pendingEmptyConfirm` field + struct stays in `app.go`.
- App's confirm-Yes branch fires `account.EmptyFolderConfirmedMsg{}`.

### Step 6: Update `internal/ui/confirm_modal.go`

Move `ConfirmModalClosedMsg` from `cmds.go` to `confirm_modal.go`,
right next to `ConfirmModalYesMsg`. Same package, so no import or
rename churn — only the declaration site changes.

### Step 7: Run `make check`

```bash
make check
```

Cycle-detection note: if `account` ends up importing `internal/ui/`,
the cause is almost certainly an App-level type leaking into account
that should be either (a) hoisted to `uicore`, (b) projected into a
narrow `account.Styles`, or (c) passed through as a structural
function type. Diagnose before retrying.

### Step 8: Commit

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 9h.2 task 1: extract internal/ui/account/

AccountTab → account.Model; folder/outbox/triage/sweep cmds and
msgs lift out of internal/ui/cmds.go. OpenConfirmEmptyMsg and
EmptyFolderConfirmedMsg hoist to account/msgs.go (account is
their actionable owner); ConfirmModalClosedMsg moves into
confirm_modal.go next to ConfirmModalYesMsg.

App now composes account.Model (plus overlays). Pump cmds,
ErrorMsg, and the URL launcher trio stay App-level in cmds.go.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Final residual `cmds.go` audit + system-map refresh

### Step 1: Audit `internal/ui/cmds.go`

Read the file end-to-end. It must contain only:

- `pumpUpdatesCmd` + `backendUpdateMsg`
- `pumpCacheCmd` + `cacheEventMsg`
- `ErrorMsg`
- `URLOpener` + `launchURLCmd` + `xdgOpenURL`

Any stray symbol = missed lift; move it to the right subpackage.

`wc -l internal/ui/cmds.go` should be ≤ 150. Pre-pass it was 635;
post-9h.1 it's 635 minus the compose+reader lifts already done,
post-9h.2 it should be ~120.

### Step 2: gofmt + goimports

```bash
goimports -w ./internal/ui/...
gofmt -l internal/
```

Both produce no output.

### Step 3: Idiomatic-bubbletea checklist (`docs/poplar/bubbletea-conventions.md` §10)

Walk the diff against §10. Most items vacuously preserved (pure
refactor). Verify:

- Each subpackage's `View()` width/height contract unchanged.
- No new defensive parent-side clipping.
- `WindowSizeMsg` still forwards from App into account into
  sidebar/messagelist/reader.
- Keys still dispatch via `key.Matches`.

### Step 4: Update `docs/poplar/system-map.md`

The `internal/ui/` listing now includes `account/` as a sibling of
the other subpackages. Update the package-layout section.

### Step 5: Live tmux capture at 80×24 and 120×40

Per `.claude/docs/tmux-testing.md`. Inbox load, viewer, compose,
sidebar search, help popover, move picker. Pure refactor — visuals
must match pre-pass baseline.

### Step 6: `make check`

Final green gate.

### Step 7: Commit

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 9h.2 task 2: cmds.go residual audit + system-map refresh

internal/ui/cmds.go now holds only App-scoped cross-cutting cmds
(pumps, ErrorMsg, URL launcher trio). system-map.md package
layout updated to list internal/ui/account/.

Live tmux capture at 80×24 and 120×40 confirmed visual parity
with pre-pass baseline.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Pass-end consolidation (handled by poplar-pass skill)

- `/simplify` review.
- ADR-0163: account/ extract details + the confirm-trio
  ownership clarification (narrows ADR-0161's prediction).
- Update `docs/poplar/invariants.md` — add `account` to the
  subpackage list, update the `cmds.go` description, refresh
  decision-index row for ADRs 0161–0163.
- Update `docs/poplar/STATUS.md` — mark 9h.2 done; replace starter
  prompt with the next pass per the queue (9h.5 drafts persistence).
- `git mv` plan + spec into `docs/superpowers/archive/`.
- `make check`, commit, push, `make install`.
