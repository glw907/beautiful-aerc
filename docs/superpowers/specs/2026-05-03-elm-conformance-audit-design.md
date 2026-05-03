# Pass 8.5b — Elm conformance audit (spec)

**Date:** 2026-05-03
**Status:** approved
**Pass:** 8.5b
**Predecessor:** Pass 8.5 (overengineering audit; ADR-0125/0126/0127)

## Goal

Audit `internal/ui/` for conformance to the elm-conventions skill
and surface substantial refactor opportunities. Apply fixes inline
per the pre-beta posture (CLAUDE.md "Release stance"): clean code
outweighs stability, schema/interface/migration changes are
welcomed, no compat shims, no churn-cost framing. This is one of
the last passes where breakage in service of correctness is
explicitly endorsed before the v0.9.0 → v1.0.0 stabilization.

## Charter

The audit is broader than rule-checklist conformance. Each finding
falls into one of two streams:

1. **Conformance** — violations of the seven elm-conventions rules
   plus the cmd-closure capture rule.
2. **Refactor** — type-design weaknesses, state-shape problems,
   component-boundary problems, Update-method shape problems,
   duplicated idioms, and dead/vestigial code.

Both streams produce findings in the same audit document. Both
streams flow into the same fix sweep. Conformance findings are
fixed without prior approval (the rules are settled). Refactor
proposals are presented for go/no-go before execution; those
declined or deferred are recorded with rationale and either queued
in STATUS or dropped.

## Scope

- **In:** every non-test `.go` file under `internal/ui/`.
- **Out:** `*_test.go` files, every package outside `internal/ui/`.

Test files are excluded because they're not part of the tea loop
and their patterns (helpers, golden capture) follow different
norms. If a fix to production code requires a test update, the
test changes go with the production change.

## Phase 1 — Parallel discovery

Dispatch four `Explore` subagents over disjoint file groups. Each
receives the same charter prompt and returns a structured findings
list.

### File groups

- **Group A — root + chrome:** `app.go`, `account_tab.go`,
  `top_line.go`, `footer.go`, `status_bar.go`, `layout.go`,
  `error_banner.go`, `dim.go`, `overlay.go`
- **Group B — mail surfaces:** `sidebar.go`, `sidebar_search.go`,
  `msglist.go`, `viewer.go`
- **Group C — modals & pickers:** `help_popover.go`,
  `linkpicker.go`, `movepicker.go`, `confirm_modal.go`, `toast.go`
- **Group D — helpers & cross-cutting:** `cmds.go`, `keys.go`,
  `styles.go`, `icons.go`, `iconwidth.go`, `date_format.go`

### Subagent prompt template

Each subagent gets:

- The seven elm-conventions rules verbatim from
  `~/.claude/skills/elm-conventions/SKILL.md`, plus the
  cmd-closure capture rule.
- The expanded refactor charter (type design, state shape,
  component boundary, Update shape, duplicated idioms, dead code).
- Its assigned file list.
- Output format: a YAML or markdown-table block with one row per
  finding: `file`, `line` (or line range), `stream`
  (`conformance` / `refactor`), `rule_or_category`, `severity`
  (`architectural` / `mechanical` / `cosmetic` for conformance;
  `major` / `minor` for refactor), `description`, `proposed_fix`,
  optional `notes`.
- Read-only: subagents do not edit files.

## Phase 2 — Consolidation

The audit document is written to
`docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md`
(this file) under a "Findings" section appended after Phase 1
completes. Structure:

- **Conformance findings table** (severity-ordered).
- **Refactor proposals** (one subsection per proposal: motivation,
  files touched, risk, rough size, recommendation).
- **Cross-cutting themes** — patterns that recur across two or
  more components and want a single shared fix.
- **Negative results** — explicit "no findings of category X"
  statements where they apply, so the audit is provably exhaustive
  rather than ambiguously empty.

Severity definitions:

- **Architectural** (Rules 1-5): package-level mutable state,
  mutation in `View`/`Init`/Cmd-closure, blocking I/O in `Update`,
  callback-based child→parent communication, duplicated ownership
  of shared state. Mandatory fix.
- **Mechanical** (Rules 6-7 + cmd closure): defensive parent-side
  `MaxWidth`/clip, missing wordwrap+hardwrap pairing, `len()` in
  width math on icon-bearing strings, `switch msg.String()` for
  KeyMsg dispatch, Cmd closures capturing `*Model` pointers
  instead of extracted values. Mandatory fix.
- **Cosmetic**: nit-level (e.g., `len()` on guaranteed-ASCII where
  correctness isn't at risk). Fix only if trivial.
- **Major refactor**: changes a type signature, splits a
  component, or moves state across the model tree.
- **Minor refactor**: localized rename, helper extract/inline,
  field re-typing.

## Phase 3 — Fix sweep

Two sub-phases, in order:

1. **Conformance fixes** — apply all architectural and mechanical
   findings. Cosmetic fixes only when trivial. Commit-per-theme so
   diffs stay reviewable. `make check` green between commits.
2. **Refactor execution** — present the proposal list for
   go/no-go, then apply approved proposals. Each proposal is its
   own commit.

If a proposal is large enough to warrant its own pass, it gets
queued in STATUS (e.g., "Pass 8.5c — viewer link mode extraction")
rather than stuffed into 8.5b.

Per pre-beta posture: no compat shims, no `// removed`
breadcrumbs, no preserved-for-future fields. Breaking renames are
fine; the commit message and (if behavior changed) an ADR carry
the rationale.

## Phase 4 — Pass-end ritual

Standard `poplar-pass` end-of-pass checklist:

- `simplify` skill on the cumulative diff.
- ADR(s) only when a *new* binding decision emerges. Conformance
  fixes do not need ADRs — the existing Elm-architecture ADRs
  already cover them. A refactor that codifies a previously-
  unspecified pattern earns one.
- `invariants.md` edit only if a fact changed.
- `STATUS.md`: flip 8.5b → done; queue 8.4c (already drafted) as
  next.
- Archive plan + spec via `git mv` to
  `docs/superpowers/archive/{plans,specs}/`.
- `make check` → commit → push → `make install`.

## Success criteria

- Every file in scope has been audited (positively or negatively).
- All architectural and mechanical conformance findings are fixed
  on master.
- Every refactor proposal has a disposition: applied, deferred
  (queued in STATUS with a starter prompt), or declined (with
  one-line rationale in the audit doc).
- `make check` green.
- The audit doc, archived under `docs/superpowers/archive/specs/`,
  stands as the historical record of the pass's findings.

## Non-goals

- Adding regression-prevention infrastructure (grep-based hooks,
  `make elm-check` target). The pass-end checklist (step 1b in
  `poplar-pass`) plus reviewer discipline already cover this.
- Auditing test files.
- Auditing packages outside `internal/ui/`.
- Touching bubbletea-conventions §10 review checklist items
  beyond what overlaps with Rules 6-7 (size contract, key
  bindings). Those have their own checklist baked into every UI
  pass-end ritual.

## Risks

- **Subagent finding overlap or contradiction.** Mitigated by
  disjoint file groups and a single consolidation step where
  conflicts are resolved by re-reading the file.
- **Refactor scope creep.** Mitigated by the go/no-go gate before
  Phase 3.2 and the "queue in STATUS as own pass" escape hatch.
- **Test fragility from architectural refactors.** Acceptable per
  pre-beta posture. Tests that break get rewritten in the same
  commit as the production change.
