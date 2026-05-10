# Pass 14a — Probe + config substrate

**Goal.** Land the non-UI substrate the first-run wizard (Pass 14b)
rides on, plus the standalone fixes from BACKLOG #29.

**Master plan + spec.** Detailed task steps live in
`docs/superpowers/plans/2026-05-09-pass-14-firstrun.md` (tasks
1, 2, 3, 4, 5, 8, 9 — Pre-task setup steps 0.1–0.4 apply to 14b;
skip them here, no huh dep yet). Design rationale lives in
`docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md`
(read the "Architecture > Extensions to existing packages",
"Error handling", and "Config writing" sections).

**Skills.** Invoke `go-conventions` before any Go file edit;
no `elm-conventions` work in this pass (UI lands in 14b).

## Tasks

Execute in order. After each task, run the master plan's
verification steps (test → implement → vet → simplify) before
moving to the next.

- [ ] **Task 1.** `internal/mail/probe.go` — shared
  `ProbeResult`, `ProbeStep`, `ProbeStatus` value types. Lives in
  `internal/mail` so both backend packages import without cycle.
  Master plan §Task 1, lines 101–223.

- [ ] **Task 2.** `internal/mailimap/probe.go` — step-by-step
  IMAP/SMTP connect transcript. Wraps existing `Probe` /
  `ProbeSMTP` paths into the new `mail.ProbeResult` shape.
  Master plan §Task 2, lines 224–449.

- [ ] **Task 3.** `internal/mailjmap/probe.go` — step-by-step
  JMAP session transcript (session URL → TLS → bearer auth →
  `Session/get` → `Account/get`). Master plan §Task 3, lines
  450–624.

- [ ] **Task 4.** `internal/config/providers.go` — add
  `Provider.CredentialStrategy` enum
  (`StrategyAppPassword`/`APIToken`/`OAuth`/`PlainIMAP`/`PlainJMAP`)
  and `HelpURL` field. Wire every preset. Master plan §Task 4,
  lines 625–827.

- [ ] **Task 5.** `internal/config/errors.go` — new typed
  `ConfigError{Path, Line, Field, Message, Suggest}`; existing
  validators wrap their failures. Drop the empty-name check in
  `accounts.go`; default `name` to `email` when omitted (#29
  fix part 1). Master plan §Task 5, lines 828–1007.

- [ ] **Task 8.** `internal/config/writer.go` — implement
  `Render(accts, ui, cache) []byte` emitting canonical TOML.
  Idempotent round-trip with `Load`. Atomic write helper
  remains the standard `config.toml.tmp` + fsync + rename
  pattern. Master plan §Task 8, lines 1483–1625.

- [ ] **Task 9.** `internal/config/template.go` (+
  `template.golden`) — rewrite to drop "until poplar's first-run
  wizard ships…" comments, rework the OAuth section ("Gmail and
  Outlook are configured by the wizard"), reflect the
  name-optional change (#29 fix part 2). Master plan §Task 9,
  lines 1626–1683.

## Pass-end ritual

1. `make check` green.
2. ADR-0189a — "Probe transcripts + typed `ConfigError` +
   canonical `config.Render`". One ADR for the three contracts
   this pass introduces (probe shape, error shape, render
   shape). Note that ADRs 0189a/0189b are already taken by the
   charm.land/v2 migration; this becomes **ADR-0190** (verify
   against `docs/poplar/decisions/INDEX.md`).
3. `docs/poplar/invariants.md` updates:
   - **Architecture > Repo & libraries**: add
     `internal/mail/ProbeResult` to the shared-types list.
   - **Config & theming**: note `Provider.CredentialStrategy` +
     `HelpURL`; note `config.ConfigError` validators; note
     `config.Render` round-trip contract; flip
     `config.AccountConfig.Name` from required to "defaults to
     `Email` when empty."
4. `docs/poplar/decisions/INDEX.md` — add the new ADR row.
5. `git mv docs/superpowers/plans/2026-05-10-pass-14a-probe-substrate.md docs/superpowers/archive/plans/`.
   (Master plan + spec stay in place — 14b and 14c still need
   them.)
6. STATUS.md — flip 14a to `done`, pivot starter prompt to 14b.
7. `make install`, commit, push.

## Out of scope

- `internal/wizard/` (14b).
- `internal/ui/wizard/` + huh + theme adapter + logo (14b).
- `wizard.Probe` dispatcher (14b — it routes between
  `mailimap.Probe` and `mailjmap.Probe` which this pass lands).
- `config init --interactive` cobra subcommand (14b).
- `runRoot` first-run auto-launch + `--repair=<name>` (14c).
- OAuth (Pass 14.1).
