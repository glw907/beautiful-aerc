# Pass 8.5 B — Overengineering Audit: `cmd/poplar/`

**Date:** 2026-05-03
**Lens:** CLI seam — config touchpoints, command wiring, flag/env-var pairs wired but unused at the consumer.

## Findings

- cmd/poplar/cache.go:72 — 2 — `gatherStats` has one call site (line 57, inside `newCacheStatsCmd`'s RunE closure).
  Action: inline
  Rationale: 14-line body adds naming indirection with no reuse value.

- cmd/poplar/config_discover_folders.go:85 — 2 — `openBackendForDiscoverFolders` has one call site (line 55).
  Action: inline
  Rationale: 4-line wrapper around `openBackend` + `b.Connect`.

- cmd/poplar/diagnose.go:28 — 2 — `runDiagnose` has one call site (line 23, inside `newDiagnoseCmd`'s RunE closure).
  Action: inline
  Rationale: No test coverage and a single caller — extract-method split is scaffolding residue.

- cmd/poplar/cache.go:335 — 5 — `fileSize` returns an error that both callers (lines 307, 325) silently discard with `_`.
  Action: delete
  Rationale: Both sites treat error as non-fatal and print `0 B`; inline `os.Stat(...).Size()` with explicit zero-fallback.

- cmd/poplar/config_cmd.go:23,54,72 — 4 — `newConfigInitTemplateCmd`, `newConfigPathCmd`, `newConfigCheckCmd` hardcode `""` to `config.Resolve`/`config.Load`, ignoring `--config` persistent flag.
  Action: refactor
  Rationale: `discover-folders` correctly reads the flag (config_discover_folders.go:35); peer subcommands do not, so `poplar --config /path config check` silently uses default — wired flag has no effect at consumers.

- cmd/poplar/config_discover_folders.go:16 — 8 — `configDiscoverFoldersFlags` is a single-field struct (`write bool`) used only as local var.
  Action: delete
  Rationale: Replace with bare `var write bool` bound directly via `BoolVar`; struct carries no grouping value with one field and one consumer.
