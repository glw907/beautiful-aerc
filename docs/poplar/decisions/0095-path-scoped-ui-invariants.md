---
title: Split component/UX invariants into a path-scoped rule
status: accepted
date: 2026-05-01
---

## Context

`docs/poplar/invariants.md` is auto-loaded into every conversation
via `@`-import in `CLAUDE.md`. By Pass 6.6 it had grown to 364
lines (cap 400) and `CLAUDE.md` to 80, putting always-loaded
context at ~440 lines — well above the 200-line guideline in the
official Claude Code memory docs. The pressure was structural: the
file mixed universal architecture facts (read by every package)
with deep component/UX prose (Sidebar, MessageList, Viewer,
triage/undo, overlays, keybinding philosophy) that only matters
when working in `internal/ui/` or planning UI passes.

Claude Code now supports `.claude/rules/*.md` with `paths:`
frontmatter — rules that auto-load only when Claude reads files
matching the globs. The mechanism explicitly accepts non-code paths
(`docs/**/*.md`) and multiple unrelated globs in one rule file.

## Decision

Split poplar's binding facts into two layers:

1. `docs/poplar/invariants.md` — **universal** facts only:
   architecture (repo, libraries, Elm/idiomatic-bubbletea pointer,
   config/theming, icon mode), mail model, build & verification,
   decision index. Always auto-loaded via the `@`-import. Target
   ≤180 lines.

2. `.claude/rules/ui-invariants.md` — **component + UX** facts:
   Sidebar, Message list, Viewer, Triage/undo/error banner,
   Compose, Keybinding philosophy, Overlays, Reading & navigation,
   Visual language. Path-scoped to:
   - `internal/ui/**/*.go` (UI source edits)
   - `docs/superpowers/plans/**/*.md`,
     `docs/superpowers/specs/**/*.md` (planning, brainstorming)
   - `docs/poplar/wireframes.md`, `docs/poplar/keybindings.md`
     (UI reference reads)

The `paths:` list is deliberately broad on the planning-doc side:
brainstorming a UI pass naturally reads or writes a plan doc, so
the rule fires when needed without an explicit invocation.

## Consequences

- Always-loaded context drops from ~440 lines to ~250
  (CLAUDE.md + slim invariants), well within the official
  guideline.
- Brainstorming and planning passes still see the UI/UX
  invariants because the rule's `paths` includes the plan and
  spec directories.
- A pass that crosses package boundaries without touching UI
  code or planning docs (e.g. pure mail-backend work) won't load
  the UI rule — correctly, since those facts don't apply.
- Cross-package planning that hasn't yet opened a plan doc may
  miss the UI rule. Mitigation: `CLAUDE.md` advertises the rule
  and its scope so Claude can `Read` it explicitly when needed.
- Future invariant growth on the mail-backend or compose side can
  follow the same pattern (e.g. `mail-invariants.md` scoped to
  `internal/mail*/**/*.go`) without inflating the always-loaded
  budget.
- The hook `.claude/hooks/claude-md-size.sh` cap on `invariants.md`
  (400 lines) becomes effectively redundant — the new file is far
  below it — but the cap stays as a tripwire.
