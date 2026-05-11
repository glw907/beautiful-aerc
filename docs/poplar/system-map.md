# Poplar System Map

On-demand reference. Load when you need the package layout or when
you're looking for where a piece of code lives. Kept lean — no
rationale, no decision history (see `decisions/` for that).

## Binary

Single binary: `cmd/poplar/`. Build with `make build`. Install with
`make install` (drops into `~/.local/bin/poplar`).

## Package layout

| Package | Role |
|---------|------|
| `cmd/poplar/` | Cobra CLI wiring, `main`, `root`, subcommands (`themes`, `config init`); resolves icon mode at startup |
| `internal/ui/` | Bubbletea root: `App`, App-level chrome (`TopLine`, `StatusBar`, `Footer`, `Toast`, `ErrorBanner`, `ConfirmModal`, `OutboxOverlay`, `ConflictOverlay`), shared `styles.go` + `icons.go` + `keys.go`, residual `cmds.go` (App-scoped pumps, URL launcher, outbox, reader/compose orchestration). Bubbles-shaped subpackages live below |
| `internal/ui/account/` | `account.Model` — composes sidebar/messagelist/reader, threads outbox/triage/sweep state. Folder + cache cmds and msgs (`LoadFoldersCmd`, `OpenFolderCmd`, `CacheEventMsg`, `TriageStartedMsg`, `OpenConfirmEmptyMsg`, …) |
| `internal/ui/sidebar/` | `sidebar.Model` + `sidebar.Column` + `sidebar.Search` |
| `internal/ui/messagelist/` | `messagelist.Model` — thread grouping, fold state, render |
| `internal/ui/reader/` | `reader.Model` — viewer, plus `reader.LinkPicker` and `reader.AttachPicker` sub-overlays |
| `internal/ui/compose/` | `compose.Model` — App-owned inline compose surface (imported as `uicompose` to disambiguate from the `internal/mailcompose` domain package) |
| `internal/ui/movepicker/` | `movepicker.Model` |
| `internal/ui/helppopover/` | `helppopover.Model` |
| `internal/ui/uicore/` | Shared chrome library: `LayoutMode`, `IconSet`, `ComputeLayout`, `NewSpinner`, `ErrorMsg`, `TriageOp`, `ModalShell`, `PlaceOverlay`, render primitives. Subpackages and `internal/ui/` both import; subpackages cannot import `internal/ui/` |
| `internal/mail/` | `Backend` interface, poplar-native types (`Folder`, `MessageInfo`, `UID`), `Classify([]Folder) []ClassifiedFolder`, mock backend |
| `internal/mailjmap/` | Fastmail JMAP backend, direct on `git.sr.ht/~rockorager/go-jmap` (synchronous) |
| `internal/mailauth/` | Vendored MIT-licensed snippets: XOAUTH2 against `go-sasl`, Gmail X-GM-EXT keepalive against `go-imap`. Top-of-file provenance comments |
| `internal/term/` | Terminal capability detection: `HasNerdFont`, `MeasureSPUACells`, `Resolve` → `(IconMode, spuaCellWidth)` |
| `internal/config/` | `accounts.toml` parsing: `AccountConfig`, `ParseAccounts`, `UIConfig`, `LoadUI`, config init writer |
| `internal/theme/` | Compiled lipgloss themes (15 themes, `Palette` → `NewCompiledTheme` → `*CompiledTheme`) |
| `internal/filter/` | Email cleanup pipeline (`CleanHTML`, `CleanPlain`). Library — awaits viewer consumer |
| `internal/content/` | Block model + lipgloss renderer (`ParseBlocks`, `RenderBody`, `RenderBodyWithFootnotes`, `ParseHeaders`, `RenderHeaders`) |
| `internal/tidytext/` | Claude API prose tidier (config, prompt, API call). Library — awaits Pass 9.5 compose consumer |

`internal/mailimap/` (Gmail IMAP, direct on `emersion/go-imap` v1)
arrives in Pass 8.

## Data flow

```
accounts.toml ─► config.ParseAccounts ─┐
                                        ├─► cmd/poplar wires ─► tea.NewProgram
accounts.toml ─► config.LoadUI ────────┘

mail.Backend (interface, synchronous)
    ├── internal/mailjmap (Fastmail JMAP, direct on go-jmap)
    ├── internal/mailimap (Gmail IMAP, direct on go-imap v1 — Pass 8)
    └── internal/mail/mock.go (development / tests)

internal/ui/App
    ├── AccountTab
    │   ├── Sidebar (+ SidebarSearch shelf)
    │   ├── MessageList
    │   └── Viewer
    ├── overlays: HelpPopover, ConfirmModal, LinkPicker, MovePicker
    └── chrome: TopLine, StatusBar, Footer, Toast, ErrorBanner
```

## Testing

- Unit tests alongside source files: `*_test.go` in the same package.
- Table-driven pattern with `[]struct{ name, input, expected }`.
- No third-party assertion libraries.
- Live UI verification uses the tmux testing workflow in
  `.claude/docs/tmux-testing.md`.

## Build gates

| Command | Purpose |
|---------|---------|
| `make vet` | `go vet ./...` |
| `make test` | `go test ./...` |
| `make check` | vet + test — the commit gate |
| `make build` | `go build -o poplar ./cmd/poplar` |
| `make install` | `go install` into `~/.local/bin/` |
| `make clean` | remove built binary |

## Hooks

- `.claude/hooks/claude-md-size.sh` — caps `CLAUDE.md` (200) and
  `docs/poplar/invariants.md` (400)
- `.claude/hooks/elm-architecture-lint.sh` — guards `internal/ui/`
  against common Elm violations
- `.claude/hooks/bubbletea-conventions-lint.sh` — bubbletea
  size/width-math/key-dispatch checks per ADR-0078

## Docs

- `CLAUDE.md` — project identity + convention pointers; one
  `@`-import (`invariants.md`).
- `docs/poplar/invariants.md` — universal binding facts,
  always auto-loaded via the `@`-import.
- `.claude/rules/ui-invariants.md` — component + UX invariants;
  path-scoped (loads when editing `internal/ui/`, planning a UI
  pass, or reading wireframes/keybindings).
- `.claude/rules/poplar-development.md` — trigger-phrase rule
  pointing at the `poplar-pass` skill.
- `docs/poplar/styling.md` — palette-to-surface map. Load before
  touching any color.
- `docs/poplar/bubbletea-conventions.md` — idiomatic bubbletea
  reference (size contract, wordwrap+hardwrap, planning + review
  checklists).
- `docs/poplar/wireframes.md` — UI reference for every screen.
- `docs/poplar/keybindings.md` — authoritative key map.
- `docs/poplar/STATUS.md` — current pass + next starter prompt.
- `docs/poplar/decisions/` — ADR archive.
- `docs/poplar/research/` — research notes (mail libs, bubbletea
  norms, reference apps).
- `docs/poplar/system-map.md` — this file.
- `docs/superpowers/plans/` — active plan files.
- `docs/superpowers/specs/` — active spec files.
- `docs/superpowers/archive/` — completed plans and specs.

## Global Claude infrastructure

- `~/.claude/skills/go-conventions/` — mandatory Go rules
- `~/.claude/skills/elm-conventions/` — mandatory `internal/ui/`
  rules
- `.claude/skills/poplar-pass/` — pass-end ritual, starter-prompt
  format
