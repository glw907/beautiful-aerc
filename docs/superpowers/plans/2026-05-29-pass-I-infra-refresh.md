# Pass I: Claude Infrastructure Refresh

**Track:** Poplar rebuild (greenfield, spec-first). Charter:
`docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md` §9.
Research: `docs/poplar/research/2026-05-29-infra-refresh-research.md`.
STATUS: `docs/superpowers/specs/poplar-rebuild-STATUS.md`.

Pass I is tooling, not poplar feature code. It re-derives the
agents, model pinning, the Go-idiom gate, and the orchestration
default from current Opus 4.8 practice, so the domain passes (1
through 7) lean on a machine tuned for this model rather than 4.7.

## Decisions settled before coding

The charter and research already brainstormed this pass, so the
action list is prescriptive. Two decisions were the user's; both
were delegated back with "use the standard way" and "the best
process":

- **Model resolution.** Set `CLAUDE_CODE_SUBAGENT_MODEL=inherit` in
  `~/.dotfiles/bash/.bashrc` and restore each agent's declared
  model. `inherit` is the documented switch for normal resolution
  (per-dispatch `model` beats frontmatter `model:` beats the main
  model). It is explicit where unsetting the var relies on
  undocumented default behavior. The main-model default stays
  `claude-opus-4-8` in `settings.json`.
- **Artifact home.** Rebuild agents land in this repo's
  `.claude/agents/`. The gate is authored as drop-in reference
  artifacts under `docs/superpowers/specs/rebuild-gate/`. The
  greenfield tree location stays deferred to the spec-to-build
  boundary per charter §11. The agents and gate are usable now
  regardless of where the tree lands.

Settled inline from the research:

- **gofumpt** replaces gofmt in the rebuild gate. It is a
  backward-compatible superset that catches grouping and blank-line
  tells.
- **Orchestration default** is `subagent-driven-development` for
  the 8 to 12 task domain passes; Workflow for wide independent
  fan-out; `ultracode` as the middle path on judgment-heavy passes.
- **modern-go-check** runs exit-1 in the rebuild gate. The
  greenfield tree is clean from day one, so soft-warn buys nothing.

## The env-var change restores intent, it does not downgrade

Today every subagent runs Sonnet regardless of frontmatter. The
five user agents already declare their intended models:
`cairn-implementer` is sonnet, the four reviewers
(`cloudflare-workers`, `daisyui-a11y`, `svelte`,
`web-auth-security`) are opus. The env var was overriding all of
them down to Sonnet. Setting `inherit` restores each agent to what
its author declared. The reviewers run on Opus again, which is
their written intent and what the research role table prescribes
for spec/quality and security review. This raises reviewer cost on
the cairn, ecnordic, and 907-life projects. That is the fix
working, not a regression.

## Tasks

1. **Model resolution fix.** Edit `~/.dotfiles/bash/.bashrc`:
   `CLAUDE_CODE_SUBAGENT_MODEL=sonnet` becomes `inherit`. Restow
   bash, confirm the live shell and a fresh login both reflect it.
2. **Audit the five user agents.** Convert alias `model:` values to
   full IDs (`claude-sonnet-4-6`, `claude-opus-4-8`) so they hold
   when 4.9 ships. Preserve declared intent. Add `effort: high` to
   the reviewers, since judgment-heavy review is where 4.8 gains
   most.
3. **Write the rebuild implementer agents** in `.claude/agents/`:
   `poplar-implementer` (mechanical, `claude-sonnet-4-6`) and a
   judgment-heavy variant dispatched with `model:opus`. Test-first,
   clears `make check` before reporting done, pastes evidence.
4. **Write the rebuild reviewer agents** in `.claude/agents/`:
   `poplar-spec-reviewer` and `poplar-quality-reviewer`
   (`claude-opus-4-8`, `effort: high`) and `poplar-go-reviewer`
   (convention, `claude-sonnet-4-6`).
5. **Document the orchestration default** in the rebuild-gate
   reference dir: when to use subagent-driven-development, Workflow,
   ultracode.
6. **Carry forward and improve the Go-idiom catalogue.** Add the
   semantic tells the research names (pointer-receiver-on-value-
   type, goroutine-per-task without errgroup or WaitGroup, `cmp.Or`
   nil-coalescing) to `~/.claude/docs/go-comment-voice.md` and the
   go-conventions skill, scoped to the /simplify voice lens. They
   are too semantic for grep.
7. **Author the rebuild `.golangci.yml`** (golangci-lint v2 schema)
   under `docs/superpowers/specs/rebuild-gate/`: carry errcheck,
   govet, ineffassign, staticcheck, unused, unparam; add `iface`,
   `containedctx`, `contextcheck`, `nonamedreturns`, `godot`.
   Install golangci-lint v2 and gofumpt and validate the config
   parses.
8. **Author the rebuild `make check` gate** as a reference Makefile
   snippet under the same dir: gofumpt-check, vet, voice-check,
   modern-go-check (exit-1), skipcheck, golangci-lint run, test.
   Carry forward the voice, modern-go, and skip scripts as the
   gate's companions.
9. **Record the greenfield-tree decision** (deferred) and update
   the rebuild STATUS plus this plan's execution record.

## Out of scope

- The live poplar tree's `internal/ui/`. The uncommitted
  viewer-triage work is a separate track; Pass I does not touch it
  and keeps it out of any commit.
- The greenfield code tree itself, created after the spec locks.
- The live poplar `make check`. The rebuild gate is a new artifact;
  the live gate's modern-go-check stays as is.

## Execution record

Ran 2026-05-29. Solo session, no subagents dispatched (the running process
had inherited the old `sonnet` env value, so dispatched agents would not have
honored the new frontmatter; the definitions resolve correctly next session).

What landed:

1. `~/.dotfiles/bash/.bashrc`: `CLAUDE_CODE_SUBAGENT_MODEL` set to `inherit`,
   restowed, confirmed in a fresh login shell. Two doc sources conflicted on
   whether `inherit` is a valid env value; resolved by reading the authoritative
   model-config page, which documents it as the switch for normal resolution.
2. Five user agents pinned to full model IDs (`cairn-implementer` to
   `claude-sonnet-4-6`; the four reviewers to `claude-opus-4-8` with
   `effort: high`). `effort` confirmed a documented frontmatter field. This
   restores the reviewers to Opus, which their frontmatter always declared and
   the env var was overriding down to Sonnet.
3. Three rebuild agents in `.claude/agents/`: `poplar-implementer`,
   `poplar-reviewer`, `poplar-go-reviewer`. The plan named four reviewers;
   built three agents total, merging spec and quality into one Opus reviewer
   per the research role table (spec + quality is one role).
4. Gate artifacts in `docs/superpowers/specs/rebuild-gate/`: `Makefile.gate`,
   `.golangci.yml` (verified with `golangci-lint config verify`, v2.12.2),
   `README.md`. Installed golangci-lint v2.12.2 and gofumpt v0.10.0.
5. Catalogue: T41 (SPDX, narrating the existing gate tag), T42, T43 added to
   `~/.claude/docs/go-comment-voice.md`; T42/T43 plus the `cmp.Or` note
   mirrored in the go-conventions skill.

Deviations from the plan:

- Reviewers collapsed from three to two (spec + quality merged). Matches the
  research role table and the per-task review cost model.
- Added T41 (SPDX) to the doc to close the numbering gap, since voice-check.sh
  already tags it. Beyond the plan's three additions, inline per pre-beta.
- `make install` skipped: Pass I changed no Go in the live tree, so there is
  nothing to rebuild. The uncommitted `internal/ui/` viewer-triage work is a
  separate track and was left untouched and out of the commit.

Verification: `golangci-lint config verify` exit 0; fresh-shell env check
shows `inherit`; gofumpt and golangci-lint report their versions.
