---
description: Development workflow for poplar — routes to the active rebuild track
---

Poplar has one active track: the greenfield spec-first rebuild. The old
dogfood client is archived at tag `poplar-legacy` and branch `legacy`,
kept only as reference. Do not resume work on it.

When the user says "continue", "continue development", "next pass",
"start the next pass", "finish pass", or "ship pass", the work is the
rebuild. Read `docs/superpowers/specs/poplar-rebuild-STATUS.md` for the
current pass and its starter prompt, then the charter at
`docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`. Follow the
pass flow described in that STATUS. The rebuild's pass-end ritual lives
there, not in the legacy `poplar-pass` skill.

The `poplar-pass` skill and `docs/poplar/STATUS.md` belong to the
archived dogfood track. Ignore them unless the user explicitly asks to
revisit the old client.
