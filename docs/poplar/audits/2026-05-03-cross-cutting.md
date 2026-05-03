# Pass 8.5 Cross-Cutting Findings

Phase C output — single-reviewer manual re-read of high-yield
packages and named seams (per
docs/superpowers/specs/2026-05-03-overengineering-audit-design.md).

Top-yield packages from Phase B counts: leaves(9), ui(8), mailimap(8),
cache(8), mailjmap(7). Per-package agents already exhausted obvious
intra-package candidates within ceiling. Phase C focuses on cross-
package seams.

## Named seam re-checks

**Seam 1 — `mail.ChangeTracker` ↔ backend impls.** Two distinct
production impls (`mailjmap.Backend`, `mailimap.Backend`); cache
constructor accepts it as a separate parameter from `mail.Backend`,
enabling independent fakes. Runtime assertion at
`cmd/poplar/root.go:115` guards the seam. Earning its place — keep.

**Seam 2 — `mail.Backend` post-cutover.** Confirmed:
- `mail.Backend.Search` — zero call sites through the interface (UI's
  `SearchState` is unrelated; sidebar search is in-memory).
- `mail.Backend.Copy` — zero call sites through the interface; only
  `mailimap` calls its own `Move`'s internal `copyUIDs` helper.
Both already flagged by mail agent (see
`2026-05-03-pkg-mail.md`). The deletion cascades across mailjmap,
mailimap, and `MockBackend`. Counted under that finding — no
separate cross-cutting row.

**Seam 3 — `cache.OpArgs` reserved sum (`SendArgs`/`AppendArgs`).**
STATUS.md lists Pass 8.4c (Cache III — outbox + offline) with a
starter prompt explicitly citing ADR-0117 (typed op events). That is
a named scheduled pass — valid speculative-consumer skip. Keep
(matches cache agent decision).

**Seam 4 — `cmd/poplar/` ↔ `internal/config/` flag wiring.**
Confirmed: `--config` persistent flag is wired on root but only
`config discover-folders` reads it via `cmd.Root().PersistentFlags()`.
The peer subcommands `config init`, `config init-template`,
`config path`, and `config check` hardcode `""` to `config.Resolve`
/ `config.Load`. Already flagged by cmd agent
(`2026-05-03-pkg-cmd-poplar.md`). The fix is local to
`cmd/poplar/config_cmd.go` (not cross-package), so counted under
that finding.

## Cross-package findings (additive)

- internal/config/loader.go:14-19 ↔ cmd/poplar/root.go — 4 — `config.Source` typed enum is exported but every cmd-layer consumer of `Resolve` discards the second return with `_` (cmd/poplar/root.go and cmd/poplar/config_cmd.go).
  Action: refactor (cascades from internal/config finding into cmd/poplar)
  Rationale: Already noted in config findings; flagged here so the cascading edits in cmd/poplar are not missed during apply.

- internal/theme/palette.go ↔ internal/theme/themes.go — 3 — Deleting `FgBrightest`, `ColorInfo`, and `ColorSpecial` from `Palette` requires removing the corresponding hex assignments from all 15 themes in `themes.go`. Treat as one bundled change.
  Action: delete (cascading)
  Rationale: Already implicit in leaves findings; called out here so the apply task knows to touch both files.

- internal/mail/backend.go ↔ internal/mailjmap/jmap.go ↔ internal/mailimap/imap.go ↔ internal/mail/mock.go — 1 — Removing `Search` and `Copy` from the `Backend` interface (mail finding) cascades to:
  - mailjmap: keep both methods on the concrete `*Backend` (Move's internal copy uses Copy in the IMAP backend, but jmap's own internal Move via Email/set's `mailboxIds` patch does not need Copy at all — verify before deleting jmap's Copy method).
  - mailimap: keep `copyUIDs` as an internal helper consumed by Move; delete the public `Copy` wrapper if no external caller remains after the interface trim.
  - MockBackend: remove `Copy` and `Search` stubs.
  Action: refactor (cascading)
  Rationale: Tracking the cross-package edit graph so the apply task knows the scope.

## Summary

The eight per-package agents covered the named seams from the spec.
This Phase C pass adds three cross-cutting cascade notes (config
Source, theme palette deletions, mail.Backend interface trim) to
ensure the apply phase touches every implicated file.
