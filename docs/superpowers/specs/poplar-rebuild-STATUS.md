# Poplar Rebuild: STATUS

**Track:** Greenfield spec-first rebuild. Charter: `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`. This is separate from the live poplar tracker at `docs/poplar/STATUS.md`.

## Current state (2026-05-29)

Pass 0 (Charter) complete:
- Charter: `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`.
- Gap analysis: `docs/poplar/research/2026-05-29-mail-client-gap-analysis.md`.
- Infra-refresh research: `docs/poplar/research/2026-05-29-infra-refresh-research.md`.
- Memories: `project_poplar_rebuild_initiative`, `project_catkin_standalone_spinoff`.

Verified: `CLAUDE_CODE_SUBAGENT_MODEL=sonnet` (at `~/.dotfiles/bash/.bashrc:157`) overrides all subagent `model:` frontmatter and per-dispatch params (confirmed against the model-config doc). Per-role model pinning does not work until this is set to `inherit`. Details in the infra research doc.

## Next: Pass I, Claude infrastructure refresh

Starter prompt (paste after /clear, or just say "start Pass I"):

```
Start Pass I of the poplar rebuild (Claude infrastructure refresh). Read first:
docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md (esp. section 9 Pass I scope)
and docs/poplar/research/2026-05-29-infra-refresh-research.md.

Execute Pass I:
1. Model resolution fix (confirm with me first, it is workstation-wide): set
   CLAUDE_CODE_SUBAGENT_MODEL=inherit in ~/.dotfiles/bash/.bashrc (restow/source),
   or move the default into settings.json. Then audit user-level agents so the cheap
   ones pin claude-sonnet-4-6 / claude-haiku-4-5.
2. Write the rebuild implementer agents (mechanical = claude-sonnet-4-6; judgment-heavy
   = claude-opus-4-8) and reviewer agents (spec/quality = claude-opus-4-8; convention
   = claude-sonnet-4-6). Full model IDs, effort: high. Each clears make check before
   reporting done, with pasted evidence.
3. Decide and document the orchestration default (subagent-driven-development for
   8-12 task passes; Workflow for wide independent fan-out).
4. Carry forward and improve the Go-idiom tuning: the go-conventions skill, the
   go-comment-voice catalogue, and the make check voice/modern-go/skip checks. Add
   golangci-lint linters iface, containedctx, contextcheck, nonamedreturns, godot;
   harden modern-go-check.sh to exit 1; consider gofumpt over gofmt.
5. Author the rebuild's make check gate and .golangci.yml as drop-in artifacts (the
   greenfield code tree is created later; Pass I produces the definitions and decisions).
6. Decide the greenfield code-tree location if ready.

Pass I is infra and tooling, not poplar feature code. After Pass I, the domain spec
passes (1 through 7) begin.
```

## Pass roadmap

(Charter section 9.) 0 Charter [done] -> I Infra refresh [next] -> 1 Accounts/sync -> 2 Organization -> 3 Reading/triage -> 4 Rendering -> 5 Compose -> 6 Search -> 7 Contacts/calendar/security -> 8 Consolidation -> build plans + clean build.
