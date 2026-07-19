# poplar

A bubbletea terminal email client. Single binary, built from one Go
module. Opinionated, vim-first, showcase-quality — "better Pine,"
not "better mutt."

> **Active work: the re-founding.** Poplar is being re-founded under
> the charter at
> `docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md`:
> the rendering bet validated first, then vision, requirements, and a
> settled technical design before any build. On "continue" or any
> phase or pass trigger, read
> `docs/superpowers/specs/poplar-refounding-STATUS.md` for the current
> phase and its starter prompt. Two closed efforts serve as research
> inputs and salvage: the dogfood client (tag `poplar-legacy`, branch
> `legacy`) and the 2026-05-29 rebuild spec track.

## The tree today

`master` holds the archived dogfood client's code, kept intact until
the re-founding's build boundary. Spikes borrow from it freely; no
feature work lands on it. Its documentation — `docs/poplar/`
(invariants, STATUS, ADRs, wireframes, keybindings, system map) and
the `.claude/rules/*-invariants.md` files that auto-load on legacy
source paths — describes that archived client and binds nothing in
the re-founding. Treat all of it as reference.

## Conventions

Global skills hold the code rules. Invoke the relevant one before
writing code, including spike code.

- **`go-conventions`** — mandatory for every Go file.
- **`elm-conventions`** — mandatory before touching bubbletea UI code.

## Human voice

Code must read as if one experienced Go developer wrote it. The full
style guide is `~/.claude/docs/go-comment-voice.md`; the
`go-conventions` skill loads its tell catalogue inline. Comments
default to none; WHY-comments only when the why is non-obvious. No
defensive checks on internal callers. No single-impl interfaces.

## Build

```
make build     # go build -o poplar ./cmd/poplar
make test      # go test ./...
make check     # fmt, vet, voice, modern-go, skipcheck, vale-comments, test
make install   # install poplar into ~/.local/bin/
```

These run against the legacy tree in place and stay the gate for any
Go that lands on `master` (spike tooling included) until Phase 5
defines the re-founding's own gate.

## Authoring

Claude's drafting on this repo follows the workstation authoring
charter at `~/.claude/docs/authoring-charter.md`. The Go comment
audience is wired: the in-tree `.vale.ini` lints `.go` comment prose
through the vendored `glw907` overlay in `.vale/styles/glw907`, and
`make check` runs `scripts/vale-comments.sh` to block a commit on an
error-level finding. Re-sync the overlay after a canonical change
with `~/.dotfiles/scripts/glw907-vendor.sh ~/Projects/poplar --sync`.

## Backlog

`BACKLOG.md` is the project issue tracker (`/log-issue`). Entries
predating the re-founding refer to the archived client; check the
charter before acting on one.
