# Poplar Status

**Current pass:** Pass 9w next — File `clipperhouse/displaywidth`
upstream PR for `Options.Overrides`. Pass 9w.0 (SPUA seam
investigation) closed — path (a) one-line override is dead;
path forward is upstream PR-first. See
`docs/superpowers/specs/2026-05-08-spua-width-upstream-investigation.md`
and `docs/superpowers/specs/2026-05-08-displaywidth-issue-draft.md`.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9p | Scaffold through attachments compose UI (ADRs 0001–0179) | done |
| 9v | Bubble Eval A (strong matches) — triage spec; no swaps | done |
| 9w.0 | SPUA seam investigation — path (a) dead; PR-first chosen | done |
| 9w | File `clipperhouse/displaywidth` PR for `Options.Overrides` | pending |
| 9q | Outbox delivery controls — undo + schedule send (#35) | pending |
| 9r–9t | List-Unsubscribe (#36), .ics viewer (#37), search (#38) | pending |
| 9u | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 10 | Polish II — popover dim (#14) + items surfaced during 9j–9u | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 9w)

> **Goal.** File a PR at `clipperhouse/displaywidth` adding
> `Options.Overrides []OverrideRange` — declarative range-table
> for runtime width overrides. Unblocks ADR-0084 collapse once
> displaywidth + `charmbracelet/x/ansi` ship the change.
>
> **Scope.** External fork only. No poplar code changes. x/ansi
> plumbing PR and poplar-side cleanup are later passes.
>
> **Settled (do not re-brainstorm):** API shape, PR-first
> sequencing, displaywidth-only filing, workflow-matching as the
> load-bearing concern. Full settled list and rationale in
> `docs/superpowers/specs/2026-05-08-spua-width-upstream-investigation.md`
> and `docs/superpowers/specs/2026-05-08-displaywidth-issue-draft.md`.
> PR description body kernel lives in the latter — adapt for PR.
>
> **Still open — brainstorm these:** None. Pure implementation.
>
> **Approach.** Read displaywidth top-to-bottom and match its
> conventions precisely (PRs #20–#22 for tone, test structure,
> GoDoc voice; AGENTS.md for review workflow). Implement
> `Overrides` consultation in `graphemeWidth` slow path; wire
> into all four entry points (String/Bytes/TruncateString/
> TruncateBytes). Tests, fuzz test, GoDoc, README — all
> first-class deliverables. Pass output: filed PR URL recorded
> here. Standard pass-end ritual via `poplar-pass`.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
