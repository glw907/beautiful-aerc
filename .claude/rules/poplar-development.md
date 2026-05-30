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
pass flow described in that STATUS. On any pass close ("ship pass",
"finish pass", or a completed pass), run the **Pass-end ritual** section at
the top of that STATUS by default. Updating the STATUS is its first and
non-optional step. The legacy `poplar-pass` skill does not apply.

The `poplar-pass` skill and `docs/poplar/STATUS.md` belong to the
archived dogfood track. Ignore them unless the user explicitly asks to
revisit the old client.
