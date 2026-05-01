# Pass 6.8 — Docs Refactor

**Goal.** Bring poplar's docs structure in line with current Claude
Code best practice (`.claude/rules/` with path-scoped frontmatter),
reconcile drifted reference docs, and shrink the always-loaded
context budget.

**Why now.** `CLAUDE.md` (80) + `@docs/poplar/invariants.md` (364)
auto-load ~440 lines into every session, against the 200-line
guideline. `system-map.md` references the abandoned aerc fork
(superseded by ADR-0075) and a non-existent `fix-corpus` skill.
Wireframes is 805 lines and contains sections marked "out of date."
Pass 6.7 (verification + research) will reference these docs; clean
them first.

## Settled (do not re-brainstorm)

- `.claude/rules/<name>.md` with `paths:` frontmatter is the
  current Claude Code mechanism for conditional context loading.
  Verified against the official memory docs.
- Path globs accept multiple unrelated patterns and non-code paths
  (e.g. `docs/superpowers/plans/**/*.md`).
- We will move component/UX invariants into a path-scoped rule,
  scoped to both code edits and planning-doc reads so brainstorming
  picks them up.
- Wireframes get a strong trim (one canonical wireframe per screen
  state; drop superseded variants).
- `keybindings.md` is the single source of truth for the key map;
  invariants prose describes philosophy and behavior, not key
  tables.

## Plan

### 1. Carve invariants → path-scoped UI rule

**Keep in `docs/poplar/invariants.md`** (universal facts only,
target ≤180 lines):

- Architecture (repo + libraries, Elm-arch one-liner, idiomatic
  bubbletea pointer).
- Mail model (Backend interface, Classify, MessageInfo,
  Destroy primitive — these are read by every component).
- Build & verification.
- Decision index.

**Move to `.claude/rules/ui-invariants.md`** with frontmatter:

```yaml
---
description: UI/UX invariants for poplar's bubbletea layer
paths:
  - "internal/ui/**/*.go"
  - "docs/superpowers/plans/**/*.md"
  - "docs/superpowers/specs/**/*.md"
  - "docs/poplar/wireframes.md"
  - "docs/poplar/keybindings.md"
---
```

Sections moved:

- Components (Sidebar, Message list, Viewer, Triage/undo/error,
  Compose).
- UX (Keybinding philosophy, Overlays, Reading & navigation,
  Visual language).
- Icon mode (covers `internal/ui/icons.go` + `internal/term`).

The path globs ensure the rule loads when:
- Editing UI source.
- Reading or writing plan docs (brainstorming).
- Reading or writing specs.
- Reading wireframes or keybindings reference docs.

**Decision to ADR.** New ADR documenting the move from a single
auto-loaded invariants file to a split: universal facts in
`invariants.md`, component/UX facts in path-scoped rule.

### 2. Reconcile system-map.md

Current drift (from line read):

- Lists `internal/mailworker/` as the JMAP+IMAP carrier (forked
  aerc). ADR-0075 retired this.
- Describes JMAP as "async→sync adapter wrapping the forked JMAP
  worker." Current invariants: synchronous direct calls.
- Missing `internal/mailauth/` (vendored XOAUTH2 + keepalive).
- Missing `internal/term/` (icon-mode capability detection).
- UI tree shows only Sidebar + MessageList; missing Viewer,
  ConfirmModal, LinkPicker, MovePicker, HelpPopover.
- Skill list mentions `fix-corpus` — does not exist on disk.

Fix all six.

### 3. Strong-trim wireframes.md

Current: 805 lines, 9 numbered sections, many sub-states.

Trim to one canonical wireframe per screen state. Concrete cuts:

- §5 "Sidebar context *(out of date — merged into account context)*"
  → delete entirely.
- §7 "Threaded view — expanded / collapsed / partially collapsed"
  → keep one combined wireframe with annotation showing the three
  states.
- §6 "Transient UI" — keep one example per element (toast, undo
  bar, error banner, spinner, connection status); drop duplicates.
- Long annotation prose where the rule now lives in invariants or
  the UI rule → trim to a one-line cross-reference.
- §"Coverage" trailing section → drop if it's a checklist (the
  decision index now serves this purpose).

Target ≤500 lines.

### 4. Single-source keybindings

Audit invariants for key tables. Replace with prose ("vim-first
single-key bindings; see `docs/poplar/keybindings.md`"). The
philosophy lines stay in the UX rule (no command mode, no
modifiers, no multi-key) since they're behavioral invariants, not
key listings.

### 5. CLAUDE.md audit

- Verify every `@`-import and pointer resolves.
- Add the new rule files to the on-demand reading list with a
  one-line description so I know they exist when I'm not editing
  matching paths.
- Confirm size ≤200 lines.

### 6. Verify hooks

- `.claude/hooks/claude-md-size.sh` caps invariants at 400 — should
  pass easily after the trim.
- `.claude/hooks/elm-architecture-lint.sh` is unaffected (no Go
  changes this pass).

### 7. Pass-end consolidation

Standard ritual via `poplar-pass`. New ADR for the rule split.
Update STATUS.md to mark 6.8 done and write the 6.7 starter prompt
(since 6.7 was deferred to come after this).

## Out of scope

- Pass 6.7 work (tmux verification + retention research) — that
  pass runs after this one.
- Code changes — this is a docs-only pass.
- Adding new wireframes for unbuilt screens (compose, search) —
  trim existing only.
