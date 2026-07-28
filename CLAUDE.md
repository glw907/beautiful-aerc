# poplar

A bubbletea terminal email client. Single binary, built from one Go
module. Opinionated, vim-first, showcase-quality, aiming to be a
better Pine, not a better mutt.

The active track is the re-founding: one process, three layers (the
store, the background engines, the bubbletea UI), rebuilt from a
settled architecture rather than grown from the archived client. The
charter is
`docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md`.
`docs/superpowers/specs/poplar-refounding-STATUS.md` owns the phase
cursor; read it on "continue" or any phase or pass trigger.

## Binding docs

The re-founding set is the only binding documentation:

- The charter, vision, and requirements under `docs/superpowers/specs/`.
- The technical design and its ADRs under
  `docs/superpowers/specs/adr/`.
- The design language,
  `docs/superpowers/specs/2026-07-27-poplar-design-language.md`.
- The build machine design,
  `docs/superpowers/specs/2026-07-27-poplar-build-machine.md`.

`docs/superpowers/specs/poplar-refounding-STATUS.md` is what
"continue" reads first. Any other doc under `docs/` predates the
re-founding and binds nothing.

## The archived client

The previous client lives on branch `legacy` (tag `poplar-legacy`).
Salvage from it is copy-with-rewrite: bring code across, rewrite it
against the settled architecture, and review it like new work. Never
copy a file across unreviewed.

## Conventions

Invoke the relevant skill before writing code, including spike code.

- **`go-conventions`**, mandatory for every Go file.
- **`elm-conventions`**, mandatory before touching bubbletea UI code.

## Human voice

Code must read as if one experienced Go developer wrote it. Comments
default to none; WHY-comments only when the why is non-obvious. No
defensive checks on internal callers. No single-impl interfaces.

## Build

```
make build     # go build -o poplar ./cmd/poplar
make test      # go test ./...
make check     # tidy-check, build, fmt-check, lint, analyzers,
               # vale-comments, skipcheck, test, perf
make install   # install poplar into ~/.local/bin/
```

## Authoring

Claude's drafting on this repo follows the workstation authoring
charter at `~/.claude/docs/authoring-charter.md`. The Go comment
audience is wired: the in-tree `.vale.ini` lints `.go` comment prose
through the vendored `glw907` overlay in `.vale/styles/glw907`, and
`make check` runs the Vale comment gate to block a commit on an
error-level finding. Re-sync the overlay after a canonical change
with `~/.dotfiles/scripts/glw907-vendor.sh ~/Projects/poplar --sync`.

## Backlog

`BACKLOG.md` is the project issue tracker (`/log-issue`). Entries
predating the re-founding refer to the archived client; check the
charter before acting on one.
