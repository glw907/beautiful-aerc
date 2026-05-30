# Poplar Rebuild: Gate and Orchestration

Drop-in artifacts and decisions from Pass I (Claude infrastructure
refresh). The greenfield code tree does not exist yet, so these are
definitions, not a live gate. When the tree is created, the Makefile
fragment and `.golangci.yml` move into it and the decisions below govern
how the domain passes run.

## Files here

- `Makefile.gate`: the commit-gate targets. Append to the greenfield
  `Makefile`.
- `.golangci.yml`: golangci-lint v2 config. Copy to the tree root.
  Verified with `golangci-lint config verify` against v2.12.2.

## The gate

`make check` is the commit gate. It carries the current poplar gate
forward with three changes:

1. **gofumpt replaces gofmt.** gofumpt is a backward-compatible superset
   that also catches grouping and blank-line tells. `fmt-check` fails on
   any file gofumpt would rewrite.
2. **modern-go-check runs strict.** The current tree runs it soft-warn
   (exit 0) so a cleanup pass can land incrementally. The greenfield tree
   is clean from day one, so the rebuild runs it with `MODERN_GO_STRICT=1`
   and a pre-1.21 idiom fails the gate.
3. **golangci-lint joins the gate.** A new `lint` target runs the v2
   linters. The standard default set (errcheck, govet, ineffassign,
   staticcheck, unused) matches the current tree. The enabled extras catch
   the structural LLM-isms the grep-tier voice gate cannot see:

   - `iface`: single-implementation, unused, and identical interfaces.
     This was the biggest gap, caught before only by `/simplify`.
   - `containedctx`: a `context.Context` stored in a struct.
   - `contextcheck`: a context not propagated through a call chain.
   - `nonamedreturns`: named returns read explanatory and cause
     naked-return bugs.
   - `godot`: doc comments end in a period.
   - `unparam`: unused function parameters (carried from the v1 config).

The three companion scripts (`scripts/voice-check.sh`,
`scripts/modern-go-check.sh`, `scripts/skipcheck/`) carry forward verbatim
from the current tree. Only the modern-go-check invocation changes.

## Model selection per role

Pass I set `CLAUDE_CODE_SUBAGENT_MODEL=inherit` so each agent's frontmatter
`model:` and the per-dispatch `model` parameter resolve normally. The
project default model stays `claude-opus-4-8` in `settings.json`. Full
model IDs are pinned in agent files so they hold when the next model ships.

| Role | Model | Agent |
|---|---|---|
| Orchestrator | claude-opus-4-8 | the main session |
| Implementer, mechanical | claude-sonnet-4-6 | `poplar-implementer` |
| Implementer, judgment-heavy | claude-opus-4-8 | `poplar-implementer`, dispatched `model:opus` |
| Reviewer, spec + quality | claude-opus-4-8 | `poplar-reviewer` (effort: high) |
| Reviewer, convention | claude-sonnet-4-6 | `poplar-go-reviewer` |
| Explore / search | claude-haiku-4-5 | built-in Explore |

The agents live in this repo's `.claude/agents/`. Each implementer task is
test-first and clears `make check` before reporting done, with pasted
evidence. Each task gets a `poplar-reviewer` pass (spec plus quality) and,
when it touches Go, a `poplar-go-reviewer` pass (conventions).

## Orchestration default

The domain passes (1 through 7) run under `subagent-driven-development`.
The passes are 8 to 12 tasks with sequential dependence and mid-pass
judgment, which is exactly that skill's shape. The orchestrator dispatches
one implementer per task, reviews, and moves on.

Reach for the Workflow harness when a pass has wide independent fan-out
(many files transformed the same way, or an audit that sweeps the tree) or
when the scale exceeds what one context tracks. Reach for `ultracode` as a
middle path on judgment-heavy passes where Claude stays conversational and
auto-generates a workflow per substantive task.
