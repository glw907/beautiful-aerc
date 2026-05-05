# poplar

A bubbletea terminal email client. Single binary, built from one Go
module. Opinionated, vim-first, showcase-quality — "better Pine,"
not "better mutt."

@docs/poplar/invariants.md

## Release stance

The project moves through three phases with different rules
(ADR-0105). Check `STATUS.md` to see which phase is active.

**Pre-beta** (now → Pass 10): **clean code outweighs stability.**

- Refactor and rename freely. No compat shims, no "churn cost"
  framing, no dead fields preserved "in case Pass N needs them" —
  strip them; the next pass re-adds with its consumer.
- When you notice a small adjacent issue while finishing other
  work — a bug, an awkward API, dead code, duplicated logic, a
  named type that no longer fits — **fix it inline** as part of
  the current pass. Do **not** offer to log it, do **not** offer
  to /schedule it, do **not** ask permission, do **not** add it
  to the backlog. The only valid reasons to defer are: (a) the
  fix needs research that would derail the current pass, or
  (b) the fix touches a file the current pass deliberately
  shouldn't touch for review-scope reasons. "It's not in the
  plan" is not a reason. "It's adjacent but unrelated" is not a
  reason — adjacency is the whole point. (Distinction vs
  `/simplify`: that reviews code *you just wrote*; this rule
  covers pre-existing ugliness you ran into while passing
  through. Both flow into the current pass.) Reserve /schedule
  for genuinely time-gated follow-ups (soak windows, metric
  checks, recurring sweeps).
- Migrations and breaking changes are first-class — explain in
  the commit message and ADR; don't engineer around them.
- The only sacred thing is data on disk the user can't easily
  regenerate (mail caches, OAuth refresh tokens).
- **Anti-pattern when triaging review findings:** never skip with
  framings like "cross-package," "schema change," "non-trivial
  refactor," "would require interface change," "churn cost,"
  "out of scope." Every one of those describes work this pre-beta
  posture explicitly endorses, so they cannot be used as defer
  rationales. Schema work in particular is *welcomed* now (v1.0
  freeze is the trigger to land schema improvements, not the
  reason to defer them). The only valid skip rationales are:
  (a) **speculative future consumer** — the finding adds a
  field/type/hook with no current call site and no immediate
  need; (b) **upstream-blocked** — requires a third-party change
  the project doesn't control or vendor; (c) **premature
  optimization without measurement** (efficiency findings only,
  bounded current shape). Anything else: apply it.

**Beta soak** (Pass 11 ships `v0.9.0` → `v1.0.0`): **stability first.**

- Master accepts bug fixes only. No new features on master.
- On-disk data formats frozen. Schema versions + automatic
  lossless migrations across beta releases.
- Refactors that don't touch user-visible behavior are OK if
  small, reviewable, and tested.
- New features queue on the `1.1` branch.

**Post-1.0** (`v1.0.0` ships): standard SemVer. `v1.x.y`
backwards-compatible; breaking changes wait for `v2.0.0`.

## Conventions

Three global skills hold the rules. Invoke the relevant one before
writing code.

- **`go-conventions`** — mandatory for every Go file. Anti-patterns,
  project structure, cobra shape, error wrapping, tests, Makefile,
  naming.
- **`elm-conventions`** — mandatory before touching `internal/ui/`.
  Elm architecture rules: state in models, mutations in Update, I/O
  in tea.Cmd, Msg-driven communication, state ownership at the root.
  Pairs with `docs/poplar/bubbletea-conventions.md` (idiomatic
  bubbletea: size contract, self-guarded `View()`, JoinHorizontal
  trust). UI/UX work tries the bubbles/glamour analogue first;
  deviations are named in the plan and confirmed in review.
- **`poplar-pass`** — pass-end consolidation ritual (ADRs, invariants
  update, plan archival, commit + push + install) and the starter-
  prompt format for the next pass.

## Human voice

Code must read as if one experienced Go developer wrote it.
Contributors recognize AI-generated Go on sight and disengage —
that's the threat model. Apply as you write, not as cleanup.

The full style guide is `~/.claude/docs/go-comment-voice.md`:
decision rubric, voice palette (poplar = stdlib-formal base,
Gerrand-welcoming for package docs, Pike-aphoristic for errors),
phrasing patterns, error-string rules, and the 32-tell catalogue
with mechanical avoidance rules per tell. The `go-conventions`
skill loads the same catalogue inline and is invoked before any Go
file edit. `/simplify`'s voice lens (Agent 4) scans diffs against
the catalogue by tell number.

Poplar-specific framing — beyond the universal rules in the guide:

- **Comments default to none.** WHY-comments only when the why is
  non-obvious. Never restate code. Skip godoc on unexported symbols
  unless the doc adds information beyond the name (Google's
  "unobvious" bar — see guide §10).
- **No defensive checks on internal callers.** Validate at
  boundaries (user input, config load, external APIs), not between
  two functions in the same package.
- **No single-impl interfaces, no zero-line wrappers.** An
  interface with one impl is a tell unless a real seam (test fake,
  DI point) is named in the code or in an ADR. Inline.

ADR-0141 codifies the policy and points at the guide as the
binding artifact.

## Path-scoped rules

`.claude/rules/ui-invariants.md` auto-loads when editing
`internal/ui/`, planning a UI pass (plan or spec docs under
`docs/superpowers/`), or reading `docs/poplar/wireframes.md` /
`docs/poplar/keybindings.md`. It carries the component + UX
binding facts that are not universal.

## On-demand reading

- `docs/poplar/system-map.md` — package layout, data flow, hook and
  skill inventory. Load when you need to find where something lives.
- `docs/poplar/styling.md` — palette-to-surface map. **Load before
  touching any color.**
- `docs/poplar/bubbletea-conventions.md` — idiomatic bubbletea
  reference (size contract, wordwrap+hardwrap, planning + review
  checklists). **Load before any UI planning or review.**
- `docs/poplar/responsive-design.md` — three-tier responsive
  model + the data-driven-cliff methodology for adding
  responsive behavior to new UI components. Load before
  planning any new responsive surface.
- `docs/poplar/research/2026-04-26-bubbletea-norms.md` and
  `docs/poplar/research/2026-04-26-reference-apps.md` — the
  authority-of-last-resort for bubbletea conventions. If the
  conventions doc and the source code (or a reference app) appear
  to disagree, the research docs cite the primary source — they
  win. Load when chasing a conflict, not on every UI pass.
- `docs/poplar/wireframes.md` — reference wireframes for every screen.
- `docs/poplar/keybindings.md` — authoritative key map.
- `docs/poplar/STATUS.md` — current pass + next starter prompt.
- `docs/poplar/decisions/` — ADR archive. Load a specific ADR when
  you need the rationale behind an invariant.

## Development workflow

Pass-driven. Each pass has a starter prompt in `STATUS.md`, a plan
doc under `docs/superpowers/plans/`, and usually a spec under
`docs/superpowers/specs/`.

Trigger phrases — "continue development," "next pass," "finish
pass," "ship pass" — invoke the `poplar-pass` skill. That skill
covers both starting a pass (read STATUS, read invariants, read
plan, execute) and ending one (the consolidation ritual).

**Pass size budget.** A pass should fit in roughly **8–12 tasks**
and one ADR (or two tightly-coupled ADRs). When planning, if the
task list grows past 12 or the ADR splits into two unrelated
subsystems, **split the pass before coding** — the second
subsystem becomes its own pass with its own plan, even when both
are part of the same nominal feature. Symptoms of an oversized
pass that already started: per-task review fatigue, plan
deviations the controller waves through, integration bugs that
only surface at consolidation, ADR-writing that asks "what was
this pass even about?" When you notice these, split inline:
land what's done, queue the rest as `<n>.1` / `<n>.2` follow-up
passes in STATUS, and stop. Pass 9d (Catkin annotations + 14
tasks + two subsystems in one ADR) is the canonical too-large
example; its post-hoc 9d.1–9d.4 audits exist *because* the pass
ran long. Avoid that shape.

## Build

```
make build     # go build -o poplar ./cmd/poplar
make test      # go test ./...
make check     # vet + test (commit gate)
make install   # install poplar into ~/.local/bin/
```

## Testing

- Unit tests alongside source, table-driven, no assertion libraries.
- Live UI verification uses tmux — see `.claude/docs/tmux-testing.md`.
- Install and verify real renders before claiming a rendering task
  is done.

## Backlog

`BACKLOG.md` is the project issue tracker. Log with `/log-issue`.
Check it before starting work — may contain known limitations or
upstream blockers relevant to the task.
