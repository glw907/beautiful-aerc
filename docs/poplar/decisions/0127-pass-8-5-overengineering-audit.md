---
title: Pass 8.5 overengineering audit summary
status: accepted
date: 2026-05-03
---

## Context

Sweep of the entire poplar codebase (~34 kLOC) for speculative
abstractions, dead scaffolding, defensive code for impossible paths,
single-impl preemptive interfaces, one-call-site helpers,
zero-value-only config knobs, and Middle Man wrappers. Run as a
four-phase audit (static analysis → eight parallel per-package review
agents → cross-cutting re-read → triage and apply) per
`docs/superpowers/specs/2026-05-03-overengineering-audit-design.md`.

## Decision

Apply 45 of 47 actionable findings. Two skipped:

1. `cache.SendArgs` / `cache.AppendArgs` sealed-sum members and their
   `KindSend` / `KindAppend` constants are kept. Pass 8.4c (Cache III
   — outbox + offline) is named in `STATUS.md` and ADR-0117 (typed op
   events) treats them as the canonical carriers Pass 9 (compose) will
   enqueue. Valid speculative-consumer skip per the pre-beta posture's
   skip-rationale guard.
2. `cache.Account.AccountName` and `AccountEmail` Middle Man methods
   are kept. The nil-backend path is load-bearing for the read-only
   CLI introspection routes (`poplar cache evict --older-than` opens
   an `Account` with `Backend == nil`). Removing the proxies would
   force the UI to pierce to `a.Backend` and add its own nil-check at
   every call site — strictly worse.

Architecturally significant deletions are documented in companion
ADRs:

- ADR-0125: trim `Search` and `Copy` from `mail.Backend`.
- ADR-0126: drop `config.Source` enum and Provider doc fields.

Other notable deletions, all module-internal:

- `cache.Cache` aggregator + `NewCache` constructor (unused — App
  wires `*cache.Account` directly). Drops the unreferenced `mu`
  `sync.RWMutex` field that golangci-lint flagged.
- `cache.errClosed` (scaffolded for a `Close`-guard never wired up).
- `mail.FlagRecent` constant. The bit position is preserved as `_` in
  the iota so cache-stored numeric `ui_flags` values do not shift.
  RFC 3501 `\Recent` is server-set and never client-passed.
- `theme.PaletteHex` plus three palette slots (`FgBrightest`,
  `ColorInfo`, `ColorSpecial`) populated in all 15 themes but never
  consumed by any composed `Style`. Cascade: 15 hex literals removed
  from `themes.go`.
- `term.measureSPUACells` one-call wrapper inlined into
  `MeasureSPUACells`; the unused `t *testing.T` parameter on
  `(*fakeTerminal).run` and the hand-rolled `intToStr` test helper
  (replaced by `strconv.Itoa`) also dropped.
- Helper inlines across `mailjmap`, `mailimap`, `cache`,
  `internal/ui/`, and `cmd/poplar/`: roughly 15 single-call-site
  helpers absorbed into their callers, plus several write-only struct
  fields removed (`mailjmap.Backend.current`, `mailimap.capSet.XGM`,
  `mailimap.listEntry.HasChildren`).
- IMAP `(*Backend).finishConnect`'s `ctx` parameter is now threaded
  into `idleLoop` via `context.WithCancel(ctx)` instead of being
  silently discarded into a fresh `context.Background()`. Closes the
  unparam finding.
- `--config` persistent flag now propagates through `poplar config
  init`, `poplar config path`, and `poplar config check`. Previously
  these subcommands hardcoded `""` to `config.Resolve`/`config.Load`
  so `poplar --config /path config check` silently used the default
  path.
- UI: `lipgloss.NewStyle()` moved out of `help_popover.go` into a new
  `Styles.HelpBoxBorder` field, restoring the styles-discipline
  invariant that `NewStyle()` lives only in `internal/ui/styles.go`
  and `internal/theme/palette.go`.

Tooling additions: `make audit` runs `deadcode`, `unparam`, and
`golangci-lint`, dropping raw output under `docs/poplar/audits/`.
`.golangci.yml` enables `unparam` and `unused` with
`exported-fields-are-used: false`.

## Consequences

The codebase shrinks by roughly 700 LOC net. The `mail.Backend`
contract and the `internal/config` public API are tighter and reflect
their actual consumers. Future passes can use `make audit` as a
recurring gate.

The two skipped findings establish the exception template: a skip is
valid only when the speculative consumer is named and scheduled in
`STATUS.md` (cache OpArgs ↔ Pass 8.4c) or when the apparent
overengineering is load-bearing for a non-obvious caller (cache
nil-backend CLI path).
