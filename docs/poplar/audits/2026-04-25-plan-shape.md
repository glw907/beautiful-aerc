# Audit: Plan Shape (Forward Look)

> Run **last**, after both prior audits have produced findings —
> their results may reshape what's worth asking here.

**Goal.** Review `STATUS.md`, `BACKLOG.md`, and the next 1–2
starter prompts as a single forward-looking package. Is the
sequencing right? Are passes scoped well? Is anything missing or
over-specified?

**Why now.** Pass 2.5b-4 just shipped (2026-04-25). The next
several passes (2.5b-5 help popover, 2.5b-6 status/toast, then
Pass 3 wire-to-live-backend) are the highest-leverage moment for
pre-flight plan review — cheaper to adjust scope now than after
another pass commits to the current direction.

## Scope

Forward-looking only. Look at:

1. **STATUS.md "Passes" table** — is the order still right? Are
   any pending passes implicitly blocking each other in unstated
   ways? Should anything queued be promoted or deferred?
2. **The "Next starter prompt" block in STATUS.md** — is the
   scope right for the next pass? Anything over- or
   under-specified? Are the open brainstorm questions the right
   ones?
3. **BACKLOG.md** — are any backlog items implicitly assuming a
   pass shape that's drifted? Are bundling decisions (e.g.,
   "bundle #9 with Pass 3") still right given current state? Are
   any items that should be promoted to a pass still sitting on
   the backlog?
4. **ADR-0022 (per-screen prototype passes)** — does the current
   sequence still match the ADR's intent, or has the prototype
   strategy diverged in practice?

## Out of scope

- Past passes. Done is done; this audit doesn't re-litigate
  shipped work.
- Plans for distant work (Pass 9, 10, 11, 1.1). Too far out for
  meaningful adjustment — the world will have changed by then.
- Implementation details inside any pass. Scope and order only.
- The library packages question — that's the prior audit's job.

## Inputs to load

- `docs/poplar/STATUS.md`
- `BACKLOG.md`
- `docs/poplar/decisions/0022-*.md` (per-screen prototype passes)
- Findings from the prior two audits, if they recommended scope
  changes that would affect upcoming passes.
- The most recent 2–3 plans under `docs/superpowers/plans/` for
  calibration on current pass-scope norms.

## Method

A single Claude session on Opus 4.7. The output is a
**recommendation document**, not a rewrite of STATUS.md. The user
applies (or rejects) recommendations as a separate small commit.

Approach: for each pass in the table from "next" forward (don't
re-evaluate done passes), ask one question — *is the scope still
right?* — and write a one-paragraph verdict. Don't try to redesign
the project. The audit's value is in noticing things that drifted,
not in proposing a new plan.

## Done

A findings document at
`docs/poplar/audits/2026-04-25-plan-shape-findings.md` with:

- Per-pass verdicts: **keep** / **re-scope** / **re-order** /
  **split** / **merge** — with one-paragraph reasoning each.
- Explicit **no change** verdicts where applicable (also valuable
  signal — they confirm the current shape is good).
- Recommended STATUS.md edits, if any, as a diff-style block at
  the end.
- A short summary section: which recommendations matter most, and
  why.

## Follow-up

- A small STATUS.md update commit if recommendations are accepted.
- No code changes.
- No action at all if the verdict is "current shape is good" —
  that result is the win.
