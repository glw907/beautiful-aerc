# Pass 9h.1 — Core organizational reorg

**Date:** 2026-05-06
**Status:** approved
**Pass:** 9h.1
**Successor of:** Pass 9h (ComposeTab + r/R/f wiring)

## Goal

Lock in a clean organizational shape for `internal/ui/` before
v1.0. Drop misleading names, split the kitchen-sink files into
bubbles-shaped subpackages, and codify a Msg-namespace policy that
accommodates post-1.0 contacts and calendar surfaces.

This is a **pure structural pass.** No feature work, no behavior
changes. The pre-beta posture (ADR-0105) endorses the breaking
renames; on-disk data (cache schema, outbox, OAuth tokens) is
untouched.

## Scope

**In:**

- `internal/ui/` — package split, file moves, type renames.
- Msg-namespace policy across the App ↔ child-package boundary.
- `cmds.go` decomposition — App-level cmds stay; per-screen cmds
  move into their subpackages.
- ADRs documenting layout, naming, and msg-namespace policy.

**Out:**

- `internal/mail/`, `internal/cache/`, `internal/compose/` (the
  domain package — distinct from the new `internal/ui/compose/` UI
  sub-model), `internal/catkin/`, `internal/content/`,
  `internal/filter/`, `internal/tidy/`, `internal/humanize/`,
  `internal/term/`, `internal/theme/`, `internal/config/`,
  `internal/mailauth/`, `internal/mailimap/`, `internal/mailjmap/`.
  Their names and shapes survive contact with future consumers as
  judged in this brainstorm; revisit case-by-case if a future pass
  proves otherwise.
- Any feature work. Drafts persistence (#33), CardDAV
  autocomplete (#34), signatures (#32), .ics viewer (#37) all
  ride on top of this pass — they are not part of it.
- Cache schema migrations. Schema is frozen at v6 for this pass.

## Settled (inputs from STATUS + brainstorm)

- The `Tab` suffix on `AccountTab` / `ComposeTab` is misleading
  — these aren't tab-bar entries; they're sub-models. Drop it.
- `package.Model` is the strongest external convention
  (`bubbles/list.Model`, `bubbles/textinput.Model`,
  `catkin.Model`, `glamour.TermRenderer`). Apply it tree-wide.
- Pre-beta posture endorses breaking renames inside the binary
  (ADR-0105). On-disk data is untouched.
- `internal/compose/` (the domain package owning `Editor`,
  `Draft`, `AssembleMIME`, `SeedReply*`) is unchanged. The new
  UI sub-model lives at `internal/ui/compose/` and imports
  `internal/compose`.

## Brainstorm decisions

### Q1 — Subpackage split timing: **split now (option A)**

All four candidate screens (`compose`, `reader`, `messagelist`,
`sidebar`) get extracted in this pass plus `account` (the parent
that composes them). Three near-term passes (drafts, CardDAV
autocomplete, signatures) all land in the new compose package
from day one; reader's .ics viewer (9.7) lands in the new reader
package; the cost of one large structural pass is paid against
the savings on every following feature pass.

### Q2 — Future contacts/calendar surfaces: **overlays (option B)**

Post-1.0 contact card and .ics RSVP surfaces appear as modal
overlays, mirroring the existing `attachpicker`, `movepicker`,
`linkpicker` pattern. The reader emits an `OpenContactCardMsg`
or `OpenInviteMsg`; the App opens the overlay. This keeps
`reader/` focused on rendering email and lets the contacts/
calendar UI live as sibling subpackages (`internal/ui/contactcard/`,
`internal/ui/inviteviewer/` when those passes land).

The contacts sidebar micro-highlight memory note is orthogonal —
a sidebar-shelf augmentation that fits any of the A/B/C
compositions.

### Q3 — Shared UI types location: **parent `internal/ui` package**

`Styles`, `IconSet`, the theme glue, overlay primitives
(`modal_shell`, `overlay`, `dim`), and `key.Binding` vocabulary
stay in the root `internal/ui` package. Subpackages import them.
This mirrors bubbles (each component package depends on
lipgloss/key from outside the package).

### Q4 — Msg taxonomy: **per-package exports for cross-boundary, unexported within**

Each subpackage exports its own `Msg` types for events crossing
the package boundary:

- `compose.SentMsg` — fired up to App when a draft is queued.
- `compose.CancelledMsg` — fired up to App on Esc cancel.
- `reader.OpenLinkPickerMsg` — fired up to App.
- `reader.LinkPickerClosedMsg` — fired down from App.
- `account.OutboxDepthMsg`, `account.ToastExpireMsg`, etc.

Msgs routed entirely within a package stay unexported
(`bodyLoadedMsg`, `folderAppendedMsg`).

App-level cross-cutting Msgs stay in `internal/ui/cmds.go`:
`ErrorMsg`, `LaunchURLMsg`. These are not owned by any single
subpackage.

**Naming rule:** keep the `Msg` suffix even with package
qualifier — `compose.SentMsg`, not `compose.Sent`. Bubbletea
convention: `tea.QuitMsg`, `tea.KeyMsg`, `spinner.TickMsg`,
`textinput.SetCursorMsg`. The suffix marks "Update tag" at use
sites where Msg types mix with `Model`, `Cmd`, and value types
from the same package.

### Q5 — `cmds.go` fate: **fragments per subpackage**

Each subpackage owns its cmds in its own `cmds.go`. The
remaining `internal/ui/cmds.go` shrinks to App-scoped pumps
(`pumpUpdatesCmd`, `pumpCacheCmd`) and cross-cutting helpers
(`launchURLCmd`, `xdgOpenURL`).

### Q6 — `messagelist` vs `msglist`: **`messagelist`**

Long form. Package names are read more often than typed; bubbles
uses `bubbles/list`, not `bubbles/lst`.

## Proposed package layout

```
internal/ui/
  app.go                              # root model, top-level routing, overlay open/close
  cmds.go                              # backend pump, cache pump, ErrorMsg, LaunchURLMsg
  keys.go                              # global key.Binding vocabulary
  layout.go, top_line.go               # chrome geometry
  styles.go                            # Styles type, parent for theme glue
  icons.go, iconwidth.go               # IconSet, displayCells, spuaCellWidth
  modal_shell.go, overlay.go, dim.go   # overlay primitives
  status_bar.go, footer.go
  error_banner.go                      # App-level error display
  toast.go                             # App-level toast display
  confirm_modal.go                     # App-level confirm overlay
  conflict_overlay.go                  # outbox conflict overlay
  outbox_overlay.go                    # outbox overlay
  movepicker/      # was movepicker.go
  helppopover/     # was help_popover.go
  date_format.go                       # shared formatter

  account/                             # was account_tab.go (844 LOC)
    model.go        # account.Model
    cmds.go         # folder load, queue ops, outbox, triage, sweep
    msgs.go         # exported Msg types

  sidebar/                             # was sidebar*.go
    model.go        # sidebar.Model
    column.go       # was sidebar_column.go
    search.go       # was sidebar_search.go

  messagelist/                         # was msglist.go (1083 LOC)
    model.go        # messagelist.Model
    msgs.go

  reader/                              # was viewer.go + linkpicker + attachpicker
    model.go        # reader.Model
    cmds.go         # body load, mark read, attachment load
    msgs.go
    linkpicker.go   # reader.LinkPicker (sub-overlay)
    attachpicker.go # reader.AttachPicker (sub-overlay)

  compose/                             # was compose_tab.go (305 LOC) — UI sub-model
    model.go        # compose.Model
    cmds.go         # composeSeedCmd, composeSendCmd
    msgs.go
```

## Naming calls (all sites)

| Old | New |
|---|---|
| `ui.AccountTab` | `account.Model` |
| `ui.ComposeTab` | `compose.Model` (UI; distinct from `internal/compose`) |
| `ui.MessageList` | `messagelist.Model` |
| `ui.Viewer` | `reader.Model` |
| `ui.Sidebar` | `sidebar.Model` |
| `ui.MovePicker` | `movepicker.Model` |
| `ui.HelpPopover` | `helppopover.Model` |
| `ui.LinkPicker` | `reader.LinkPicker` |
| `ui.AttachPicker` | `reader.AttachPicker` |

App imports the new compose subpackage with an alias to avoid
collision with the domain package. The convention:

- The domain package keeps the unaliased name `compose` (it's
  the older, more fundamental name and is imported in more
  places — `cache`, future drafts code, the editor seam).
- The UI sub-model is imported as `uicompose` at App scope.

```go
import (
    "github.com/glw907/poplar/internal/compose"
    uicompose "github.com/glw907/poplar/internal/ui/compose"
)
```

Inside `internal/ui/compose/` itself the package declaration is
`package compose`; that file imports the domain package as
`mailcompose "github.com/glw907/poplar/internal/compose"`.

## Out-of-scope renames considered and rejected

- `internal/mail/` → `internal/mailbackend/`. Rejected — `mail`
  is the domain noun; `MessageInfo`, `Folder`, `Backend` all
  read fine under it. Future consumers (autocomplete, search)
  fit under the same name.
- `internal/cache/` → `internal/store/` or `internal/db/`.
  Rejected — `cache.Account` and `cache.Op` are the load-bearing
  names and they accurately describe a cache (with backstops
  and lazy population), not a store of record.
- `internal/filter/` → `internal/markdown/`. Rejected —
  `filter.MarkdownToHTML` is the central function but the
  package will absorb other filtering passes (HTML sanitize,
  link rewrite) per the v1.x plan. `markdown` would lock the
  scope down.
- `internal/tidy/` → `internal/aitidy/` or similar. Rejected —
  the package is empty; it'll grow under its consumer in pass
  9i and the consumer will dictate the name.

## Migration mechanics

Each subpackage extraction follows the same shape:

1. `git mv internal/ui/<file>.go internal/ui/<pkg>/model.go`
   (and tests). Adjust `package` declarations.
2. Move associated types and unexported helpers; rename the
   exported model type to `Model`.
3. Lift cmds and msgs from `internal/ui/cmds.go` into
   `internal/ui/<pkg>/cmds.go` and `msgs.go`. Cross-boundary
   Msgs become exported; within-package Msgs stay unexported.
4. Update `app.go` and `account/model.go` to consume the new
   imports; promote field types (`a.Compose compose.Model`,
   `a.Reader reader.Model`).
5. Run `make check`. Fix call sites in tests.

The Pass 9h.1 commit history will run roughly one commit per
subpackage extraction (compose, reader, messagelist, sidebar,
account, movepicker/helppopover) plus a final naming/cmds
cleanup commit.

## Pass-end ADRs (anticipated)

Two tightly-coupled ADRs, matching the pass budget:

1. **ADR-0161 — Package layout for `internal/ui/`.** Defines the
   subpackage tree, what stays at root, what becomes a sibling
   package. Cites bubbles convention. Includes the future
   contacts/calendar overlay-style composition decision so the
   layout's extensibility is recorded with the layout itself.
2. **ADR-0162 — Naming + Msg-namespace policy.** Drops `Tab`
   suffix; mandates `package.Model`; documents the
   `internal/compose` / `internal/ui/compose` namespace overlap
   and import alias convention; codifies the per-package Msg
   export rule (cross-boundary exported, within-package
   unexported, App-level cross-cutting in `internal/ui/cmds.go`)
   with the `Msg` suffix preserved. These collapse because the
   Msg taxonomy is a direct consequence of the package-naming
   shape — they share the same justification.

## Pass budget check

Tasks anticipated:

1. Extract `compose` subpackage.
2. Extract `reader` subpackage (incl. linkpicker, attachpicker).
3. Extract `messagelist` subpackage.
4. Extract `sidebar` subpackage.
5. Extract `account` subpackage (the largest — 844 LOC parent).
6. Extract `movepicker` and `helppopover`.
7. Decompose `cmds.go` — leave only App-scoped cmds.
8. Final pass: rename audit, msg taxonomy verification, golden
   test refresh.

Eight tasks. Within the 8–12 budget. If task 5 (account)
balloons, split it into 5a (extraction) and 5b (cmds cleanup).

## Verification

- `make check` green (gofmt, vet, voice, tests).
- Live tmux capture at 80×24 and 120×40 confirming no visual
  regression — pure refactor must look identical.
- Idiomatic-bubbletea checklist (poplar-pass §1b) — each
  extracted subpackage's `View()` honors size, no defensive
  parent-side clipping, key bindings continue to dispatch.

## Risks

- **Import cycle risk** between `account` and child subpackages
  if `account.Model` exposes types that children consume. Plan
  is for children to be fully independent leaves; `account`
  imports them, not the reverse. Verify with `go list -deps`
  before final commit.
- **Test golden churn.** Golden tests in `internal/ui/testdata/`
  may need package-path updates if any encode type names. Check
  during task 8.
- **Rename fatigue review.** The diff will be large. Each
  commit must be a clean atomic extraction so review can
  proceed file-by-file.

## Non-risks

- No on-disk format change. Cache schema v6 unchanged.
- No keybinding change. The user-visible map in
  `docs/poplar/keybindings.md` is untouched.
- No theme/styling change. Palette and the styling.md map are
  untouched.
