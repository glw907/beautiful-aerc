# Pass 8.5 B — Overengineering Audit: leaf packages

**Date:** 2026-05-03
**Packages:** `internal/theme/`, `internal/term/`, `internal/backoff/`

## internal/backoff/ — pre-check

`backoff.Exponential` is used by all three claimed consumers:
- Cache drainer — `internal/cache/drainer.go:262`
- JMAP push loop — `internal/mailjmap/push.go:46`
- IMAP idle loop — `internal/mailimap/idle.go:42`

Nothing found because `internal/backoff` is a 30-line pure function with
three genuine consumers and no dead code, interfaces, or abstractions.

## internal/theme/ — pre-check

`Palette` defines 17 color slots; three are populated in all 15 themes
but never referenced by any composed Style. `PaletteHex` is test-only.

## internal/term/ — pre-check

`IconMode`, `Resolve`, `HasNerdFont`, `MeasureSPUACells` correctly
encapsulated; `cmd/poplar` calls all three probes once.

## Findings

- internal/theme/palette.go:17,38,74,144 — 3 — `FgBrightest` field defined in `Palette` and `CompiledTheme`, populated in all 15 themes, but never referenced by any composed `Style`; only accessible via test-only `PaletteHex`.
  Action: delete
  Rationale: No style paints with `FgBrightest`; dead color data through 15 theme defs and the compiled struct.

- internal/theme/palette.go:27,41,82,83,160,162 — 3 — `ColorInfo` and `ColorSpecial` fields populated in all 15 themes but never referenced by any composed `Style`.
  Action: delete
  Rationale: Both slots dead; only reachable via test-only `PaletteHex`.

- internal/theme/palette.go:126-166 — 2 — `PaletteHex` is a string-lookup function with no production call site; only called from `theme_test.go`.
  Action: delete
  Rationale: Two callers in `theme_test.go`; replace with direct field access on `CompiledTheme` (e.g., `string(Nord.BgBase)`).

- internal/term/probe.go:35-41 — 2 — `measureSPUACells(timeout)` has one call site (`MeasureSPUACells` line 28); opens `/dev/tty` and delegates to `measureSPUACellsOn`.
  Action: inline
  Rationale: The intermediate function exists for a testable timeout, but tests bypass it by calling `measureSPUACellsOn` directly with a PTY; indirection serves no purpose.

- internal/term/probe_test.go:34 — 4 — `(*fakeTerminal).run` accepts `t *testing.T` which is never used (unparam).
  Action: delete (drop param)
  Rationale: Removing simplifies signature and eliminates unparam finding.

- internal/term/probe_test.go:73-90 — 2 — `intToStr` test helper with one call site (line 62); reimplements integer-to-string to avoid `strconv` import.
  Action: inline (use `strconv.Itoa`)
  Rationale: With `t *testing.T` removed from `run`, no reason not to use stdlib.
