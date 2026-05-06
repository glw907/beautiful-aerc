---
title: Path-scoped subsystem invariants
status: accepted
date: 2026-05-05
---

## Context

`docs/poplar/invariants.md` is the always-loaded single source of
binding facts for poplar. It hit the 400-line size-hook ceiling
in Pass 9d after Catkin's annotation pipeline and spellcheck
consumer landed. Pass 9d.2 compacted the Catkin block but the
Cache section (~98 lines) and the attachment lines scattered
across Mail model + Config & theming continued to grow with
each pass.

The path-scoped pattern proven by `.claude/rules/ui-invariants.md`
(ADR-0095) shows that subsystem facts can live in auto-loaded
rule files without losing presence in the relevant work surface:
edit cache code, the cache rule auto-loads.

## Decision

Subsystem invariants live in `.claude/rules/<name>-invariants.md`,
not under `docs/poplar/`. Single source of truth — content lives
directly in the rule file with `paths:` frontmatter that auto-
loads when matching files are edited or referenced (the same
pattern as `ui-invariants.md`).

Pass 9d.2a extracts three subsystems:

- `cache-invariants.md` — per-account SQLite, schema versions,
  drainer, outbox state machine, body + attachment storage. Loads
  on `internal/cache/**/*.go`, `cmd/poplar/cache*.go`, plan/spec
  docs.
- `catkin-invariants.md` — markdown editor core, live styling,
  commands, QoL, annotation pipeline + spellcheck, render-cursor
  splice. Loads on `internal/catkin/**/*.go`, plan/spec docs.
- `attachments-invariants.md` — `mail.Attachment` wire shape +
  `[ui] download_dir` save target. Loads on
  `internal/ui/attachpicker*.go`, `internal/ui/viewer*.go`,
  plan/spec docs. Cross-links to `cache-invariants.md` for
  storage details.

`invariants.md` retains a one-paragraph pointer in the section
where each extracted subsystem used to live, so a human reader
sees the redirect at the natural lookup point. The decision
index stays in `invariants.md` — one source of truth for ADR
mapping.

Mail backends (JMAP, IMAP) are deferred until after Pass 9.6
(OAuth refresh), which will rewrite the IMAP backend invariants
block.

## Extraction-readiness criteria

A subsystem becomes a candidate for extraction when:

- **Settled.** No queued pass is scheduled to rewrite its binding
  facts. (Mail backends fail this — OAuth refresh is queued.)
- **≥ ~25 lines.** Smaller blocks aren't worth the indirection.
- **Natural path scope.** A directory or file pattern where edits
  are unambiguously "this subsystem."

## Consequences

- `invariants.md` drops from 400 → 262 lines, well under the
  300-line target.
- Future passes that touch a subsystem auto-load its invariants
  via the path-scoped rule, without bloating the always-loaded
  doc.
- Splitting brings a small drift risk: when a fact spans
  subsystems (e.g. attachment storage in the cache), the rule
  pair must cross-link. The attachments rule explicitly points
  at the cache rule for storage facts.
- Future extractions follow the criteria above. Mail backends
  extract after Pass 9.6 lands.
- Extends the ADR-0095 path-scoped policy from UI-only to a
  general split mechanism.
