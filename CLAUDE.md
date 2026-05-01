# poplar

A bubbletea terminal email client. Single binary, built from one Go
module. Opinionated, vim-first, showcase-quality — "better Pine,"
not "better mutt."

@docs/poplar/invariants.md

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
