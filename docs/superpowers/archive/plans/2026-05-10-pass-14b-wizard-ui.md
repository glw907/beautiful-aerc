# Pass 14b — Wizard domain + huh UI

**Goal.** Build the wizard on top of 14a's substrate: the
UI-free `internal/wizard/` domain, the bubbletea+huh
`internal/ui/wizard/` surface, the section registry (account +
theme; stubs for contacts/signatures/tidy), and the
`poplar config init --interactive` subcommand.

**Prereq.** Pass 14a is `done` — `mail.ProbeResult`,
`mailimap.Probe`, `mailjmap.Probe`, `Provider.CredentialStrategy`,
`config.ConfigError`, `config.Render` must all exist.

**Master plan + spec.** Detailed task steps in
`docs/superpowers/plans/2026-05-09-pass-14-firstrun.md` (Pre-task
setup steps 0.1–0.4; tasks 6, 7, 10, 11, 12, 14). Design rationale
in `docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md`
(Architecture > New packages; Wizard flow; Wireframes).

**Skills.** Invoke `go-conventions` before any Go file;
`elm-conventions` before any `internal/ui/` file; load
`docs/poplar/bubbletea-conventions.md` before UI work.

## Tasks

- [ ] **Step 0.3–0.4.** `go get charm.land/huh/v2@latest`,
  commit dep addition. Master plan §0.3–0.4.
- [ ] **Task 6.** `internal/wizard/` skeleton — `Model`, `Step`,
  `Section`, `Strategy` interface, non-OAuth implementations
  (`appPasswordStrategy`, `apiTokenStrategy`, `plainIMAPStrategy`,
  `plainJMAPStrategy`), `Apply(model) (config.AccountConfig,
  error)`. Master plan §Task 6.
- [ ] **Task 7.** `internal/wizard/probe.go` — `Probe(ctx, cfg)
  ProbeResult` dispatcher routing on `cfg.Provider` to
  `mailimap.Probe` or `mailjmap.Probe`. Master plan §Task 7.
- [ ] **Task 10.** `internal/ui/wizard/` skeleton — `Model`,
  `Styles`, `huh.Theme` adapter from `theme.CompiledTheme`,
  typographic wordmark logo. Embed `art/poplar-logo.ans` via
  `//go:embed` (committed but unused at startup). Master plan
  §Task 10.
- [ ] **Task 11.** Account section — provider picker → email →
  credentials (one huh group per `CredentialStrategy`) → probe
  screen (custom `tea.Model` with live spinner + transcript) →
  identity → label. Master plan §Task 11.
- [ ] **Task 12.** Theme section with live preview + section
  registry + `wizard.Run(mode)` orchestrator. Stub
  `contactsSection`, `signatureSection`, `tidySection` with
  `Hide() == true`. Master plan §Task 12.
- [ ] **Cobra subcommand.** `poplar config init --interactive`
  (and `--section=<name>` subset mode). Stop short of the
  first-run auto-launch + `--repair` — those are 14c. Subset of
  master plan §Task 13.
- [ ] **Task 14.** Live tmux smoke at 80×24 and 120×40 — visit
  every section, capture probe-success + probe-failure screens.
  Master plan §Task 14.

## Pass-end ritual

1. `make check` green.
2. ADR-0191 — "First-run wizard architecture: sections,
   strategies, huh adoption." Covers the `Section`/`Strategy`
   seams, huh as the form library (deviation from
   bubbles-default per `bubbletea-conventions.md`; record the
   rationale), the per-subpackage `Styles` convention, the
   typographic logo / embedded `.ans` artifact.
3. `docs/poplar/invariants.md` updates:
   - **Architecture > Elm architecture & idiomatic bubbletea**:
     add `internal/ui/wizard/` to the bubbles-shaped subpackage
     list; note `Styles` ownership; note huh as the wizard
     form library.
   - **Config & theming**: note the `poplar config init
     --interactive` surface; note `wizard.Apply` as the
     `AccountConfig` constructor for wizard flows.
4. `docs/poplar/decisions/INDEX.md` — add ADR-0191.
5. `docs/poplar/bubbletea-conventions.md` — note huh's idioms
   if 14b surfaces new patterns the conventions doc should
   codify.
6. `git mv docs/superpowers/plans/2026-05-10-pass-14b-wizard-ui.md docs/superpowers/archive/plans/`.
7. STATUS.md — flip 14b to `done`, pivot to 14c.
8. `make install`, commit, push.

## Out of scope

- First-run auto-launch + `--repair=<name>` + `--no-wizard` /
  `POPLAR_NO_WIZARD=1` (14c).
- OAuth (Pass 14.1).
