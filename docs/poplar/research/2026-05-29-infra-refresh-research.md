# Poplar Rebuild: Infrastructure-Refresh Research (Pass I groundwork)

**Date:** 2026-05-29
**Purpose:** Feed Pass I (the Claude infrastructure refresh). Two topics: model selection per role for the Opus 4.8 era, and improving the idiomatic-Go tuning. Sourced from current Claude Code / Anthropic docs and the Go linting community as of today.

---

## 1. Model selection and agent strategy (Opus 4.8)

### Finding: CONFIRMED (2026-05-29)

The official model-config doc confirms it. `CLAUDE_CODE_SUBAGENT_MODEL` is "The model to use for all subagents and agent teams. Overrides the per-invocation `model` parameter and the subagent definition's `model` frontmatter. Set to `inherit` to use normal model resolution instead."

It is set to `sonnet` at `~/.dotfiles/bash/.bashrc:157` (the source of the `~/.bashrc` stow symlink) and is live in the shell. So every subagent on this workstation currently runs Sonnet 4.6 regardless of any `model:` frontmatter or per-dispatch `model` parameter. Prior Opus-reviewer dispatches did run on Sonnet.

Fix: set `CLAUDE_CODE_SUBAGENT_MODEL=inherit` (the documented way to restore normal resolution: per-dispatch `model` beats frontmatter `model:` beats the main model). Then pin each role's model in agent frontmatter with full IDs. This is a workstation-wide change (it affects every project's subagents), so the cheap default shifts from "always Sonnet" to "inherit the main model unless frontmatter pins it"; audit user-level agents so the ones that should stay cheap pin `claude-sonnet-4-6` or `claude-haiku-4-5`. The built-in Explore agent keeps its Haiku default. Confirm with the user before editing dotfiles.

### Documented precedence (confirmed)

```
1. CLAUDE_CODE_SUBAGENT_MODEL env var      (highest; overrides the rest)
2. per-invocation model parameter (Agent tool / dispatch)
3. subagent frontmatter model:
4. main conversation model (inherit)
```

### Per-role assignment

| Role | Model | Why |
|---|---|---|
| Orchestrator | claude-opus-4-8 | Plans, sequences, judges gate output. Errors cascade. Opus 4.8 is ~4x less likely to overlook code flaws than 4.7. |
| Implementer, mechanical | claude-sonnet-4-6 | Well-specified task with a red/green test oracle. Cheaper, reliable. |
| Implementer, judgment-heavy | claude-opus-4-8 | Spec ambiguity, cross-package refactor, architecture calls. |
| Reviewer, spec + quality | claude-opus-4-8 | The role that gained most from 4.8's flaw-detection improvement. |
| Reviewer, convention (Go, Elm) | claude-sonnet-4-6 | Pattern-matching a documented ruleset. |
| Explore / search | claude-haiku-4-5 | Read-only, fast, cheap; the built-in Explore default. |

What changed with Opus 4.8: the "Sonnet implements, Opus reviews" split still holds for mechanical work, but Opus is now worth using for judgment-heavy implementers too, and fast mode is roughly 3x cheaper than on 4.7 (about $10/$50 per MTok at ~2.5x speed), which makes Opus viable under latency pressure.

### Mechanics

- Pin full model IDs (`claude-opus-4-8`), not aliases, in agent files run repeatedly over weeks; aliases move when 4.9 ships.
- Set `effort: high` explicitly rather than relying on defaults.
- A safe default model belongs in project settings, not the shell env var, so frontmatter can still win.

### Orchestration

- Start with `subagent-driven-development` for the 8 to 12 task passes (sequential dependence, mid-pass judgment, already codified as a skill).
- Reach for the Workflow harness when tasks are independent and fan out wide, or scale exceeds what one context tracks.
- `ultracode` is a middle path (Claude stays conversational, auto-generates a workflow per substantive task); try it on judgment-heavy passes.

### Rough cost

Opus 4.8 $5/$25, Sonnet 4.6 $3/$15, Haiku 4.5 $1/$5 per MTok in/out. A 10-task pass (Opus orchestrator, ~7 Sonnet implementers, ~2 Opus reviewers, ~2 Haiku explorers) lands near a couple of dollars at typical token counts; scales linearly with context size.

---

## 2. Idiomatic-Go tuning

### Existing artifacts are strong

The `go-conventions` skill, the `go-comment-voice.md` AI-tell catalogue (T1 to T41), and the `make check` grep-tier gates (`voice-check.sh`, `modern-go-check.sh`, `skipcheck`) already cover most mechanically-detectable tells and the modern-stdlib regressions. The industry's answer to non-idiomatic LLM Go (in-context specific rules plus a tight linter feedback loop, e.g. Gopher Guides' MCP plugin) is structurally what poplar already does. The gap is automated enforcement of the structural tells that are too semantic for grep.

### Recommended additions (golangci-lint and gate)

High value:
- **`iface`** (opaque/unused/identical analyzers): catches single-implementation interfaces, currently only caught by /simplify. The biggest gap.
- **`containedctx`** and **`contextcheck`**: `context.Context` stored in a struct, and context not propagated through a call chain. Both classic LLM-isms, neither currently checked.
- **`nonamedreturns`**: named returns are an LLM tell (look explanatory, cause naked-return bugs).
- **`godot`**: enforces the period-on-doc-comment rule mechanically.
- **Harden `modern-go-check.sh` to exit 1** once the tree is clean (currently soft-warn, so pre-1.21 idioms slip in silently).

Medium value:
- Consider **`gofumpt`** in place of `gofmt` (backward-compatible superset; catches grouping and blank-line tells).
- Add catalogue tells for **pointer-receiver-on-value-type** and **goroutine-per-task without errgroup/WaitGroup**; both go in the /simplify voice lens (semantic).
- Add an M5 note for `cmp.Or` nil-coalescing chains (voice-lens, not grep).

### Common LLM Go failure modes (for the catalogue and reviewers)

Over-interfacing (repository/service/handler triads), pointer receivers everywhere, `context.Context` in structs, builder patterns for invariant-free structs, goroutine-per-task without measurement, the `"failed to X: %w"` error chorus, singletons, and not reaching for modern stdlib (slices/maps/iter/slog). Detailed in-context rules beat "be idiomatic"; the compiler and linter loop catches the rest.

---

## 3. Provisional Pass I action list

1. Resolve the `CLAUDE_CODE_SUBAGENT_MODEL` precedence question; fix the shell/settings split so per-role pinning works.
2. Write the implementer agents (mechanical Sonnet, judgment-heavy Opus) and reviewer agents (Opus spec/quality, Sonnet convention), with pinned full model IDs and `effort: high`.
3. Decide orchestration default (subagent-driven-development) and document when to switch to Workflow.
4. Carry forward the go-conventions skill, voice catalogue, and gate; add the linters above; harden modern-go-check.
5. Author the rebuild's `make check` gate and `.golangci.yml`.

## 4. Open items to verify

- The env-var precedence claim (top priority; empirical check against current behavior).
- Whether `CLAUDE_CODE_SUBAGENT_MODEL=inherit` is exactly equivalent to unsetting it.
- Whether fast mode can be requested from agent frontmatter or is purely a serving tier.

Sources are listed in the Pass I research transcripts (Claude Code subagents/model-config/workflows docs, Anthropic models overview and Opus 4.8 announcement, golangci-lint and linter project pages, Gopher Guides and Addy Osmani write-ups).
