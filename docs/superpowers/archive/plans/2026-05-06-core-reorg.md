# Pass 9h.1 — Core organizational reorg implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split `internal/ui/` from a flat 17k-LOC package into bubbles-shaped subpackages (`account`, `sidebar`, `messagelist`, `reader`, `compose`, `movepicker`, `helppopover`); drop the misleading `Tab` suffix; codify the per-package Msg-namespace policy.

**Architecture:** Pure structural refactor. Each task extracts one subpackage by `git mv`-ing source files, rewriting `package` declarations, exporting the cross-boundary identifiers (`Model`, exported `*Msg` types), updating call sites in the parent `internal/ui` package, and running `make check`. Order: leaves first (compose, movepicker, helppopover), then renderer leaves (messagelist, sidebar, reader), then the `account` parent that composes them, then a final `cmds.go` decomposition + voice/golden cleanup. No behavior changes; cache schema v6 unchanged; user-visible keymap unchanged.

**Tech Stack:** Go 1.26.1, bubbletea/lipgloss/bubbles, gofmt + `go vet` + table-driven tests gated by `make check`.

**Spec:** `docs/superpowers/specs/2026-05-06-core-reorg-design.md`.

**Note on TDD:** This is a pure-refactor pass. No new behavior, no new tests. The "test first" loop is replaced by `make check` after each move — the existing test suite is the regression net. If `make check` regresses, the move is wrong; revert and retry.

---

## File map (target end state)

```
internal/ui/
  app.go, app_test.go
  cmds.go, cmds_test.go              (App-scoped cmds only)
  keys.go, layout.go, top_line.go, top_line_test.go, layout_test.go
  styles.go, styles_test.go
  icons.go, icons_test.go, iconwidth.go, iconwidth_test.go
  modal_shell.go, modal_shell_test.go
  overlay.go, overlay_test.go
  dim.go, dim_test.go
  status_bar.go, status_bar_test.go
  footer.go, footer_test.go
  error_banner.go, error_banner_test.go
  toast.go, toast_test.go
  confirm_modal.go, confirm_modal_test.go
  conflict_overlay.go, conflict_overlay_test.go
  outbox_overlay.go, outbox_overlay_test.go
  date_format.go, date_format_test.go
  golden_test.go
  cache_helpers_test.go

  account/
    model.go, model_test.go        (was account_tab.go, account_tab_test.go)
    cmds.go                         (folder + outbox + triage + sweep cmds)
    msgs.go                         (exported account.*Msg types)

  sidebar/
    model.go, model_test.go        (was sidebar.go, sidebar_test.go)
    column.go, column_test.go      (was sidebar_column.go, sidebar_column_test.go)
    search.go, search_test.go      (was sidebar_search.go, sidebar_search_test.go)

  messagelist/
    model.go, model_test.go        (was msglist.go, msglist_test.go)

  reader/
    model.go, model_test.go        (was viewer.go, viewer_test.go)
    cmds.go                         (body + mark-read + attachment cmds)
    msgs.go                         (exported reader.*Msg types)
    linkpicker.go, linkpicker_test.go
    attachpicker.go, attachpicker_test.go

  compose/
    model.go, model_test.go        (was compose_tab.go, compose_tab_test.go)
    cmds.go                         (composeSeed/composeSend cmds)
    msgs.go                         (exported compose.*Msg types)

  movepicker/
    model.go, model_test.go        (was movepicker.go, movepicker_test.go)

  helppopover/
    model.go, model_test.go        (was help_popover.go, help_popover_test.go)
```

---

## Task 1: Extract `internal/ui/compose/` (UI sub-model)

The compose UI sub-model is App-owned (per ADR-0159) and has no other UI consumer. Smallest leaf with a non-trivial cmd surface — good warm-up.

**Files:**
- `git mv internal/ui/compose_tab.go internal/ui/compose/model.go`
- `git mv internal/ui/compose_tab_test.go internal/ui/compose/model_test.go`
- Create: `internal/ui/compose/cmds.go`, `internal/ui/compose/msgs.go`
- Modify: `internal/ui/cmds.go` (lift compose cmds + msgs), `internal/ui/app.go` (import alias + field type), `internal/compose/draft.go` (comment), `internal/compose/editor.go` (comments), `internal/catkin/indent.go` (comment)

- [ ] **Step 1: Create the subdir and move files**

```bash
cd /home/glw907/Projects/poplar
mkdir -p internal/ui/compose
git mv internal/ui/compose_tab.go internal/ui/compose/model.go
git mv internal/ui/compose_tab_test.go internal/ui/compose/model_test.go
```

- [ ] **Step 2: Rewrite `package` declaration and rename type**

In `internal/ui/compose/model.go`:
- Change `package ui` to `package compose`.
- Rename the type `ComposeTab` to `Model` everywhere in the file (definition + methods).
- Rename `NewComposeTab` to `New`.
- Add the import alias for the domain package at the top:

```go
import (
    mailcompose "github.com/glw907/poplar/internal/compose"
    // ... existing imports
)
```

- Replace all references to `compose.Draft`, `compose.Editor`, `compose.AssembleMIME`, `compose.SeedReply`, `compose.SeedReplyAll`, `compose.SeedForward`, `compose.CatkinEditor` with the `mailcompose.` prefix. Use this exact substitution (verify by grep first):

```bash
grep -n "compose\." internal/ui/compose/model.go
# Manually rewrite each match to mailcompose.<name>
```

- Same rewrite in `internal/ui/compose/model_test.go` for its `package` line and any `compose.<X>` references. Test file is `package compose_test` — keep it that way; rename internal `package ui` if present.

- [ ] **Step 3: Lift compose cmds and msgs out of `internal/ui/cmds.go`**

Move these symbols from `internal/ui/cmds.go` to `internal/ui/compose/cmds.go` (functions) and `internal/ui/compose/msgs.go` (types):

| Symbol | Destination file | Renamed to |
|---|---|---|
| `composeSeedCmd` | `cmds.go` | `SeedCmd` (exported — App calls it) |
| `composeSendCmd` | `cmds.go` | `SendCmd` |
| `envelopeFromDraft` | `cmds.go` | unexported, stays as `envelopeFromDraft` |
| `resolveSentFolder` | `cmds.go` | unexported, stays as `resolveSentFolder` |
| `composeSeedKind` (the enum) | `cmds.go` | `SeedKind` |
| `composeSeededMsg` | `msgs.go` | `SeededMsg` |
| `composeSentMsg` | `msgs.go` | `SentMsg` |

Both files: `package compose`. The domain package import is `mailcompose "github.com/glw907/poplar/internal/compose"`.

The `TidyFn` type — this is App-level (ADR-0160 has it as a function pointer on App). It stays at `internal/ui/`. The compose cmd accepts it as a parameter typed `func(...) (...)`; the App threads its own `TidyFn` value through.

- [ ] **Step 4: Update `internal/ui/app.go`**

Add the alias import:

```go
import (
    "github.com/glw907/poplar/internal/compose"
    uicompose "github.com/glw907/poplar/internal/ui/compose"
)
```

Change the App field:

```go
// before
Compose ComposeTab
// after
Compose uicompose.Model
```

Update construction site (was `NewComposeTab(...)` → `uicompose.New(...)`).

Update Msg type-switch arms in `App.Update`:

```go
// before
case composeSentMsg:
case composeSeededMsg:
// after
case uicompose.SentMsg:
case uicompose.SeededMsg:
```

Update cmd call sites (`composeSeedCmd(...)` → `uicompose.SeedCmd(...)`; same for `SendCmd`).

- [ ] **Step 5: Fix neighbouring comments**

Update the prose-only references in `internal/compose/draft.go`, `internal/compose/editor.go`, `internal/catkin/indent.go` from `ComposeTab` to `compose.Model` (or `the compose UI sub-model`, depending on which reads better — they're comments). Verify with `grep -n ComposeTab internal/`. After the edit, `grep -rn ComposeTab internal/ cmd/` must return nothing.

- [ ] **Step 6: Run `make check`**

```bash
make check
```

Expected: all green. If a compile fails on a missed call site, fix and re-run. If a test fails for a reason that isn't a missed import or rename, stop — that's a real regression and must be diagnosed before commit.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 9h.1 task 1: extract internal/ui/compose/

Moves the ComposeTab UI sub-model into its own bubbles-shaped
subpackage. ComposeTab → compose.Model; compose cmds (Seed,
Send) and msgs (Seeded, Sent) lift out of internal/ui/cmds.go.
App imports the new package as uicompose to disambiguate from
the existing internal/compose domain package.

Pure refactor — no behavior change.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Extract `internal/ui/movepicker/`

Self-contained leaf. No cmds, no exported msgs to App beyond Msg types it already defines.

**Files:**
- `git mv internal/ui/movepicker.go internal/ui/movepicker/model.go`
- `git mv internal/ui/movepicker_test.go internal/ui/movepicker/model_test.go`
- Modify: `internal/ui/account_tab.go` (call site), `internal/ui/app.go` (call site)

- [ ] **Step 1: Move files**

```bash
mkdir -p internal/ui/movepicker
git mv internal/ui/movepicker.go internal/ui/movepicker/model.go
git mv internal/ui/movepicker_test.go internal/ui/movepicker/model_test.go
```

- [ ] **Step 2: Rewrite `package` and rename type**

- `package ui` → `package movepicker` in `model.go`.
- `MovePicker` → `Model` (definition, methods, callers within the file).
- `NewMovePicker` → `New`.
- Test file: `package movepicker_test` (use the `_test` suffix variant) or `package movepicker` if the existing tests touch unexported state. Grep the test for unexported identifiers; if any are referenced, keep `package movepicker` and rewrite type names to match the renames.
- Identify any exported Msg types defined in this file. Leave them at their current names — they'll be referenced as `movepicker.OpenMsg`, `movepicker.ClosedMsg`, etc.

- [ ] **Step 3: Update call sites in `internal/ui/`**

```bash
grep -rn "MovePicker\|NewMovePicker" internal/ui/ cmd/poplar/
```

Each match outside `internal/ui/movepicker/` becomes `movepicker.Model` / `movepicker.New`. Add the import to each file that needs it:

```go
"github.com/glw907/poplar/internal/ui/movepicker"
```

- [ ] **Step 4: Run `make check`**

```bash
make check
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 9h.1 task 2: extract internal/ui/movepicker/

MovePicker → movepicker.Model; subpackage holds the overlay
model and tests. Pure refactor.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Extract `internal/ui/helppopover/`

Same shape as Task 2.

**Files:**
- `git mv internal/ui/help_popover.go internal/ui/helppopover/model.go`
- `git mv internal/ui/help_popover_test.go internal/ui/helppopover/model_test.go`

- [ ] **Step 1: Move files**

```bash
mkdir -p internal/ui/helppopover
git mv internal/ui/help_popover.go internal/ui/helppopover/model.go
git mv internal/ui/help_popover_test.go internal/ui/helppopover/model_test.go
```

- [ ] **Step 2: Rewrite `package` and rename**

- `package ui` → `package helppopover`.
- `HelpPopover` → `Model`.
- `NewHelpPopover` → `New` (if it exists; otherwise the existing constructor name lower-cased per the rename rule).
- Any cache-overlay pointer/dirty-flag escape hatch field stays as-is — its scope was already private to the file.

- [ ] **Step 3: Update call sites**

```bash
grep -rn "HelpPopover\|NewHelpPopover" internal/ui/ cmd/poplar/
```

Each match outside the new package becomes `helppopover.Model` / `helppopover.New`. Import added at use sites:

```go
"github.com/glw907/poplar/internal/ui/helppopover"
```

- [ ] **Step 4: Run `make check`**

```bash
make check
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 9h.1 task 3: extract internal/ui/helppopover/

HelpPopover → helppopover.Model. Pure refactor.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Extract `internal/ui/messagelist/`

Renderer leaf. 1083 LOC; the largest single-file move.

**Files:**
- `git mv internal/ui/msglist.go internal/ui/messagelist/model.go`
- `git mv internal/ui/msglist_test.go internal/ui/messagelist/model_test.go`
- Modify: `internal/ui/account_tab.go`, `internal/ui/app.go` (if it touches the list directly), `internal/ui/golden_test.go`, `internal/ui/cache_helpers_test.go`

- [ ] **Step 1: Move files**

```bash
mkdir -p internal/ui/messagelist
git mv internal/ui/msglist.go internal/ui/messagelist/model.go
git mv internal/ui/msglist_test.go internal/ui/messagelist/model_test.go
```

- [ ] **Step 2: Rewrite `package` and rename type**

- `package ui` → `package messagelist`.
- `MessageList` → `Model`.
- `NewMessageList` → `New`.
- Any exported helpers (e.g. `MessageList.RefreshSource`, `MessageList.ApplyTriage` if they survive — verify with `grep '^func (.*MessageList)' model.go`) get their receiver type rewritten to `Model`.
- Unexported helpers stay unexported.

- [ ] **Step 3: Update call sites**

```bash
grep -rn "MessageList\|NewMessageList" internal/ui/ cmd/poplar/
```

Each match outside `internal/ui/messagelist/` becomes `messagelist.Model` / `messagelist.New`. Import:

```go
"github.com/glw907/poplar/internal/ui/messagelist"
```

`internal/ui/account_tab.go` likely has a field like `Msgs MessageList` — rewrite to `Msgs messagelist.Model`.

- [ ] **Step 4: Run `make check`**

```bash
make check
```

If `golden_test.go` fails because golden fixtures encode a type name in their key, regenerate with `go test ./internal/ui/... -update` (or whichever flag the project uses — check `golden_test.go` for the convention before doing this).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 9h.1 task 4: extract internal/ui/messagelist/

msglist.go (1083 LOC) → messagelist subpackage; MessageList →
messagelist.Model. Long-form name follows bubbles convention
(bubbles/list, not bubbles/lst).

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Extract `internal/ui/sidebar/`

Three files into one subpackage.

**Files:**
- `git mv internal/ui/sidebar.go internal/ui/sidebar/model.go`
- `git mv internal/ui/sidebar_test.go internal/ui/sidebar/model_test.go`
- `git mv internal/ui/sidebar_column.go internal/ui/sidebar/column.go`
- `git mv internal/ui/sidebar_column_test.go internal/ui/sidebar/column_test.go`
- `git mv internal/ui/sidebar_search.go internal/ui/sidebar/search.go`
- `git mv internal/ui/sidebar_search_test.go internal/ui/sidebar/search_test.go`

- [ ] **Step 1: Move files**

```bash
mkdir -p internal/ui/sidebar
git mv internal/ui/sidebar.go internal/ui/sidebar/model.go
git mv internal/ui/sidebar_test.go internal/ui/sidebar/model_test.go
git mv internal/ui/sidebar_column.go internal/ui/sidebar/column.go
git mv internal/ui/sidebar_column_test.go internal/ui/sidebar/column_test.go
git mv internal/ui/sidebar_search.go internal/ui/sidebar/search.go
git mv internal/ui/sidebar_search_test.go internal/ui/sidebar/search_test.go
```

- [ ] **Step 2: Rewrite `package` declarations and rename types**

All six files: `package ui` → `package sidebar`.

In `model.go`:
- `Sidebar` → `Model` (struct + receivers).
- `NewSidebar` → `New`.

In `column.go`:
- `SidebarColumn` → `Column` (the composite type from ADR-0130).
- Constructor renames to `NewColumn` if one exists.

In `search.go`:
- The search shelf type — currently likely `SidebarSearch` or similar — becomes `Search`.
- Any Msg type for clearing search (`ClearSidebarSearchMsg`) — this Msg flows from outside *into* the subpackage. It currently lives in `internal/ui/cmds.go`. Decide:
  - If `ClearSidebarSearchMsg` is fired by the App after a global key (e.g., Esc clears all overlays including search), keep it in `internal/ui/cmds.go` as App-level cross-cutting. Sidebar's Update arm references `ClearSearchMsg` via the ui package import — no, that creates a cycle. Better: move it to `internal/ui/sidebar/msgs.go` as `sidebar.ClearMsg`. App imports sidebar to fire it.
  - Decision: move `ClearSidebarSearchMsg` → `sidebar.ClearSearchMsg` (or `sidebar.ClearMsg` if context makes it unambiguous; pick `ClearSearchMsg` for clarity).

Confirm by reading both files first; document in the commit message which was chosen.

- [ ] **Step 3: Update call sites**

```bash
grep -rn "Sidebar\|SidebarColumn\|SidebarSearch\|ClearSidebarSearchMsg\|NewSidebar" internal/ui/ cmd/poplar/
```

Outside `internal/ui/sidebar/`: each becomes `sidebar.Model`, `sidebar.Column`, `sidebar.Search`, `sidebar.ClearSearchMsg`, `sidebar.New`. Add the import:

```go
"github.com/glw907/poplar/internal/ui/sidebar"
```

- [ ] **Step 4: Run `make check`**

```bash
make check
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 9h.1 task 5: extract internal/ui/sidebar/

Sidebar/SidebarColumn/SidebarSearch consolidate as
sidebar.Model + sidebar.Column + sidebar.Search.
ClearSidebarSearchMsg → sidebar.ClearSearchMsg per the
per-package Msg-namespace policy.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Extract `internal/ui/reader/`

Viewer + the two sub-overlays it owns (linkpicker, attachpicker) plus reader-scoped cmds. Largest task by file count.

**Files:**
- `git mv internal/ui/viewer.go internal/ui/reader/model.go`
- `git mv internal/ui/viewer_test.go internal/ui/reader/model_test.go`
- `git mv internal/ui/linkpicker.go internal/ui/reader/linkpicker.go`
- `git mv internal/ui/linkpicker_test.go internal/ui/reader/linkpicker_test.go`
- `git mv internal/ui/attachpicker.go internal/ui/reader/attachpicker.go`
- `git mv internal/ui/attachpicker_test.go internal/ui/reader/attachpicker_test.go`
- Create: `internal/ui/reader/cmds.go`, `internal/ui/reader/msgs.go`
- Modify: `internal/ui/cmds.go` (lift reader cmds), `internal/ui/account_tab.go`, `internal/ui/app.go`, `internal/ui/cmds_test.go`, `internal/ui/cache_helpers_test.go`

- [ ] **Step 1: Move files**

```bash
mkdir -p internal/ui/reader
git mv internal/ui/viewer.go internal/ui/reader/model.go
git mv internal/ui/viewer_test.go internal/ui/reader/model_test.go
git mv internal/ui/linkpicker.go internal/ui/reader/linkpicker.go
git mv internal/ui/linkpicker_test.go internal/ui/reader/linkpicker_test.go
git mv internal/ui/attachpicker.go internal/ui/reader/attachpicker.go
git mv internal/ui/attachpicker_test.go internal/ui/reader/attachpicker_test.go
```

- [ ] **Step 2: Rewrite `package` declarations and rename types**

All six files: `package ui` → `package reader`.

In `model.go`:
- `Viewer` → `Model`.
- `NewViewer` → `New`.

In `linkpicker.go`:
- `LinkPicker` → `LinkPicker` (keep — the qualified name `reader.LinkPicker` is descriptive). It's an exported sub-overlay type that lives under reader.
- Constructor renames if needed.

In `attachpicker.go`:
- `AttachPicker` → `AttachPicker`. Same reasoning.

- [ ] **Step 3: Lift reader cmds and msgs from `internal/ui/cmds.go`**

Move these symbols from `internal/ui/cmds.go` to `internal/ui/reader/cmds.go` (functions) and `internal/ui/reader/msgs.go` (types):

| Symbol | Destination | Renamed to |
|---|---|---|
| `loadBodyCmd` | `cmds.go` | `LoadBodyCmd` |
| `markReadCmd` | `cmds.go` | `MarkReadCmd` |
| `loadAttachmentsCmd` | `cmds.go` | `LoadAttachmentsCmd` |
| `openAttachmentCmd` | `cmds.go` | `OpenAttachmentCmd` |
| `saveAttachmentCmd` | `cmds.go` | `SaveAttachmentCmd` |
| `sanitizeAttachFilename` | `cmds.go` | unexported — stays as is |
| `resolveSaveTarget` | `cmds.go` | unexported — stays as is |
| `bodyLoadedMsg` | `msgs.go` | `BodyLoadedMsg` (App's account.Model handles this routing) |
| `attachmentsLoadedMsg` | `msgs.go` | `AttachmentsLoadedMsg` |
| `attachmentSavedMsg` | `msgs.go` | `AttachmentSavedMsg` |
| `OpenLinkPickerMsg` | `msgs.go` | already exported, keep as `OpenLinkPickerMsg` |
| `LinkPickerClosedMsg` | `msgs.go` | already exported, keep |
| `OpenAttachPickerMsg` | `msgs.go` | already exported, keep |
| `AttachPickerClosedMsg` | `msgs.go` | already exported, keep |
| `OpenAttachmentMsg` | `msgs.go` | keep |
| `SaveAttachmentMsg` | `msgs.go` | keep |

Both new files: `package reader`.

The `URLOpener` type is App-level (ADR-0128) — stays in `internal/ui/`. The reader cmds accept it as a function type (or interface) parameter.

- [ ] **Step 4: Update call sites**

```bash
grep -rn "Viewer\|NewViewer\|LinkPicker\|AttachPicker\|loadBodyCmd\|markReadCmd\|bodyLoadedMsg\|attachmentsLoadedMsg\|OpenLinkPickerMsg\|LinkPickerClosedMsg\|OpenAttachPickerMsg\|AttachPickerClosedMsg\|OpenAttachmentMsg\|SaveAttachmentMsg" internal/ui/ cmd/poplar/
```

Each match outside `internal/ui/reader/`:
- `Viewer` → `reader.Model`
- `NewViewer` → `reader.New`
- `LinkPicker` → `reader.LinkPicker`
- `AttachPicker` → `reader.AttachPicker`
- `loadBodyCmd` → `reader.LoadBodyCmd` (and similar for the other cmds)
- Lower-case msg types → exported `reader.<Name>Msg` (e.g., `bodyLoadedMsg` → `reader.BodyLoadedMsg`)
- Already-exported msg types add the `reader.` prefix (e.g., `OpenLinkPickerMsg` → `reader.OpenLinkPickerMsg`)

Add the import to each file that needs it:

```go
"github.com/glw907/poplar/internal/ui/reader"
```

- [ ] **Step 5: Run `make check`**

```bash
make check
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 9h.1 task 6: extract internal/ui/reader/

Viewer → reader.Model; LinkPicker and AttachPicker move under
reader as reader.LinkPicker and reader.AttachPicker. Reader-
scoped cmds (LoadBodyCmd, MarkReadCmd, LoadAttachmentsCmd,
OpenAttachmentCmd, SaveAttachmentCmd) and msgs (BodyLoadedMsg,
AttachmentsLoadedMsg, AttachmentSavedMsg, OpenLinkPickerMsg,
LinkPickerClosedMsg, OpenAttachPickerMsg, AttachPickerClosedMsg,
OpenAttachmentMsg, SaveAttachmentMsg) lift out of cmds.go.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Extract `internal/ui/account/`

The largest extraction by responsibility. `AccountTab` is the parent that composes sidebar, messagelist, reader, and threads outbox/triage/sweep state. After this task, `internal/ui/app.go` mostly composes `account.Model` plus overlays.

**Files:**
- `git mv internal/ui/account_tab.go internal/ui/account/model.go`
- `git mv internal/ui/account_tab_test.go internal/ui/account/model_test.go`
- Create: `internal/ui/account/cmds.go`, `internal/ui/account/msgs.go`
- Modify: `internal/ui/cmds.go` (lift account-scoped cmds), `internal/ui/app.go` (field type + Update arms)

- [ ] **Step 1: Move files**

```bash
mkdir -p internal/ui/account
git mv internal/ui/account_tab.go internal/ui/account/model.go
git mv internal/ui/account_tab_test.go internal/ui/account/model_test.go
```

- [ ] **Step 2: Rewrite `package` and rename type**

In `model.go` and `model_test.go`:
- `package ui` → `package account`.
- `AccountTab` → `Model`.
- `NewAccountTab` → `New`.
- The `Backend()` accessor (used by `pumpUpdatesCmd`) is exported and stays exported. The `now` seam (ADR-0128) stays unexported.
- Imports: add the freshly-extracted subpackages — `sidebar`, `messagelist`, `reader`. The existing field types (`Sidebar`, `MessageList`, `Viewer`) become `sidebar.Model`, `messagelist.Model`, `reader.Model`.

- [ ] **Step 3: Lift account-scoped cmds and msgs**

Move these symbols from `internal/ui/cmds.go` to `internal/ui/account/cmds.go` (functions) and `internal/ui/account/msgs.go` (types):

| Symbol | Destination | Renamed to |
|---|---|---|
| `loadFoldersCmd` | `cmds.go` | `LoadFoldersCmd` |
| `queryFolderCmd` | `cmds.go` | `QueryFolderCmd` |
| `openFolderCmd` | `cmds.go` | `OpenFolderCmd` |
| `refreshFolderCmd` | `cmds.go` | `RefreshFolderCmd` |
| `loadMoreCmd` | `cmds.go` | `LoadMoreCmd` |
| `queueOpsCmd` | `cmds.go` | `QueueOpsCmd` |
| `enqueueDestroys` | `cmds.go` | unexported helper, stays as is |
| `emptyFolderCmd` | `cmds.go` | `EmptyFolderCmd` |
| `destroyCmd` | `cmds.go` | `DestroyCmd` |
| `refreshOutboxDepthCmd` | `cmds.go` | `RefreshOutboxDepthCmd` |
| `loadOutboxSummaryCmd` | `cmds.go` | `LoadOutboxSummaryCmd` |
| `loadOutboxConflictsCmd` | `cmds.go` | `LoadOutboxConflictsCmd` |
| `retryConflictCmd` | `cmds.go` | `RetryConflictCmd` |
| `discardConflictCmd` | `cmds.go` | `DiscardConflictCmd` |
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

**Stays in `internal/ui/cmds.go` (App-level, cross-cutting):**

| Symbol | Reason |
|---|---|
| `pumpUpdatesCmd`, `backendUpdateMsg` | App owns the backend-update routing |
| `pumpCacheCmd`, `cacheEventMsg` | App owns the cache-event pump |
| `ErrorMsg` | global error banner |
| `LaunchURLMsg`, `launchURLCmd`, `xdgOpenURL` | App-level URL launcher |
| `OpenConfirmEmptyMsg`, `EmptyFolderConfirmedMsg`, `ConfirmModalClosedMsg` | App-level confirm modal (ADR-0128) |

These are not owned by `account` because the App opens the confirm modal, fires the URL launcher, and routes the pumps.

- [ ] **Step 4: Update `internal/ui/app.go`**

```go
import (
    "github.com/glw907/poplar/internal/ui/account"
    // ...
)

// before
Account AccountTab
// after
Account account.Model
```

Update construction site (`NewAccountTab(...)` → `account.New(...)`).

In the App's `Update`, type-switch arms that referenced unexported account msgs become qualified — e.g., `case foldersLoadedMsg:` → `case account.FoldersLoadedMsg:`. App likely doesn't switch on most of these (they go to `account.Model.Update`); only the ones App reads (probably the outbox-depth msg for the status bar, the toast-expire msg, the confirm-flow msgs).

Verify before changing: `grep -nE "foldersLoadedMsg|folderLoadedMsg|folderAppendedMsg|triageStartedMsg|toastExpireMsg|undoRequestedMsg|emptyFolderDoneMsg|sweepCompletedMsg|outboxDepthMsg|outboxSummaryMsg|outboxConflictsMsg|conflictResolvedMsg" internal/ui/app.go`. Only those App genuinely switches on need a rename in `app.go`; the rest were always handled in `account_tab.go` and stay there (now in `internal/ui/account/model.go`).

- [ ] **Step 5: Update call sites in `internal/ui/account/model.go`**

The newly-moved file references all of the cmds and msgs above with their old, unexported names. Since the cmds/msgs are now in the same package (`account`), the names need rewriting to the *new* names (exported). Within the file: `loadFoldersCmd` → `LoadFoldersCmd`, `foldersLoadedMsg` → `FoldersLoadedMsg`, etc.

Same for `internal/ui/account/model_test.go`.

- [ ] **Step 6: Run `make check`**

```bash
make check
```

This is the highest-risk task. If a reverse-import cycle appears (account importing ui, ui importing account), the cause is almost always: a type App owns (e.g., `URLOpener`, `Styles`, `IconSet`, `TidyFn`) is being passed to `account.New` but currently typed as a value defined inside `account.Model`'s file. Fix by ensuring those types continue to live in `internal/ui/` and `account` imports them via … no, that creates a cycle. The fix is: define those types in a leaf-package or in `internal/ui/` *and* have App pass them as function-typed parameters (Go's structural typing — `func(string) error` rather than `ui.URLOpener`). Use that escape hatch consistently for any App-owned seam consumed by `account`.

If a cycle is unavoidable, abandon the task, revert with `git reset --hard HEAD`, and surface the cycle in a sub-task before retrying.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 9h.1 task 7: extract internal/ui/account/

AccountTab → account.Model; folder/outbox/triage/sweep cmds and
msgs lift out of cmds.go. App now composes account.Model
(plus overlays). Pump cmds, ErrorMsg, LaunchURLMsg, and the
confirm-modal msgs stay App-level in internal/ui/cmds.go.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Final cleanup — `cmds.go` decomposition verification, audit, voice

The structural moves are done. This task verifies the end state and tightens leftovers.

**Files:** `internal/ui/cmds.go`, `internal/ui/cmds_test.go`, `docs/poplar/system-map.md`, any golden test data with package paths in keys.

- [ ] **Step 1: Audit residual `internal/ui/cmds.go`**

Read the file. Confirm it contains only:
- `pumpUpdatesCmd`, `pumpCacheCmd` and their msgs
- `ErrorMsg`
- `LaunchURLMsg`, `launchURLCmd`, `xdgOpenURL`
- `OpenConfirmEmptyMsg`, `EmptyFolderConfirmedMsg`, `ConfirmModalClosedMsg` (the confirm-modal trio)

If anything else remains, it's either (a) genuinely cross-cutting and stays, or (b) was missed and belongs in a subpackage. Decide per the per-package Msg policy: if exactly one subpackage produces and consumes it, move it. If more than one, leave it App-level.

Run `wc -l internal/ui/cmds.go` — should be ≪ 711 (the Pass 9h state). A reasonable target is under 200 lines.

- [ ] **Step 2: Verify no stale imports**

```bash
goimports -w internal/ui/...
gofmt -l internal/
```

Should produce no output on either.

- [ ] **Step 3: Run the bubbletea idiomatic checklist**

Open `docs/poplar/bubbletea-conventions.md` §10. Walk the checklist against the diff — most items are vacuously preserved since this is a refactor, but verify:
- Each new subpackage's `View()` still returns no lines wider than its assigned width (unchanged from pre-refactor).
- No new defensive parent-side clipping introduced (none should be — no behavior changes).
- `WindowSizeMsg` still forwards from App into the freshly-imported subpackage children (sizes propagate via `account.Model.SetSize` which forwards into sidebar/messagelist/reader — same as before).
- Keys still dispatch via `key.Matches` — unchanged.

If any item fails the checklist, it was a regression introduced during the moves. Fix in this task.

- [ ] **Step 4: Update `docs/poplar/system-map.md`**

The package layout changed; the system-map's "package layout" section needs to reflect the new tree. Quick edit — replace the `internal/ui/` listing with the table from the spec's "Proposed package layout" section (or a tightened version of it).

- [ ] **Step 5: Live tmux capture at 80×24 and 120×40**

Per `.claude/docs/tmux-testing.md`. Confirm visual parity with pre-pass screenshots (Inbox load, message viewer, compose surface, sidebar search, help popover, move picker). Pure refactor must look identical.

If any capture differs from the pre-pass baseline, the cause is a regression — diagnose before commit.

- [ ] **Step 6: Run `make check`**

```bash
make check
```

Final green gate.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
Pass 9h.1 task 8: cmds.go decomposition verification + system-map refresh

internal/ui/cmds.go now holds only App-scoped cross-cutting
cmds (pumps, ErrorMsg, LaunchURLMsg, confirm-modal trio).
system-map.md package-layout section updated to reflect the
new subpackage tree.

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

---

## Pass-end consolidation (handled by poplar-pass skill, not this plan)

After Task 8 commits, invoke the `poplar-pass` skill's pass-end ritual:

1. `/simplify` — review Pass 9h.1 changes, fix anything flagged.
2. Idiomatic-bubbletea checklist (already done in Task 8 step 3 — re-confirm).
3. Write the two ADRs:
   - **ADR-0161** — Package layout for `internal/ui/`. References the spec.
   - **ADR-0162** — Naming + Msg-namespace policy. Drops `Tab` suffix; mandates `package.Model`; documents the `internal/compose` / `internal/ui/compose` namespace overlap and `uicompose` import alias; codifies per-package Msg exports with the `Msg` suffix preserved.
4. Update `docs/poplar/invariants.md` — narrow or rewrite the "Elm architecture & idiomatic bubbletea" and "Repo & libraries" sections to mention the new subpackage tree and the per-package Msg policy. Update the decision index with ADRs 0161, 0162.
5. Update `docs/poplar/STATUS.md` — mark 9h.1 done; replace starter prompt with Pass 9h.5 (drafts persistence) per the queue.
6. Archive `docs/superpowers/plans/2026-05-06-core-reorg.md` and `docs/superpowers/specs/2026-05-06-core-reorg-design.md` via `git mv` to `docs/superpowers/archive/plans/` and `docs/superpowers/archive/specs/`.
7. `make check`, commit, push, `make install`.

---

## Self-review notes

Spec coverage check:

- Q1 (split now) — Tasks 1–7 each extract one subpackage. ✓
- Q2 (overlays for contacts/calendar) — recorded in spec; ADR-0161 will cite it; no code change needed this pass. ✓
- Q3 (shared UI types in parent) — Tasks 1–7 leave `Styles`, `IconSet`, overlay primitives in `internal/ui/`. Task 8 verifies. ✓
- Q4 (msg taxonomy) — every task's "Lift cmds and msgs" step exports cross-boundary msgs and keeps within-package msgs unexported. The ADR draft in pass-end records the rule. ✓
- Q5 (cmds.go fragments) — Tasks 1, 6, 7 lift per-screen cmds; Task 8 verifies the residual is App-scoped only. ✓
- Q6 (`messagelist` long form) — Task 4. ✓
- Naming calls table from spec — every entry has a matching task. ✓
- Out-of-scope items (`internal/mail/`, `internal/cache/`, etc.) — no task touches them. ✓
- Live tmux verification — Task 8 step 5. ✓
- Idiomatic-bubbletea checklist — Task 8 step 3. ✓

Type consistency check:

- `Model` is used consistently as the renamed type in every subpackage. ✓
- `New` is the constructor name in every subpackage. ✓
- `Msg` suffix preserved on every renamed Msg type. ✓
- `uicompose` is the App-side import alias for `internal/ui/compose/`; the domain package is unaliased. Tasks 1, 7 reference this consistently. ✓
- `mailcompose` is the alias used *inside* `internal/ui/compose/` for the domain package. Task 1 step 2 sets it; no later task contradicts. ✓
- `URLOpener`, `TidyFn`, `Styles`, `IconSet` stay in `internal/ui/` and are passed through structural function types where needed (Task 7 step 6 makes this explicit). ✓
