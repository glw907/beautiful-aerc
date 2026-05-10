# Pass 14c — First-run integration

**Goal.** Wire the wizard into `runRoot` so missing config
auto-launches it; add `--repair=<name>` and the two opt-outs
(`--no-wizard` flag, `POPLAR_NO_WIZARD=1` env var).

**Prereq.** 14a + 14b are both `done`. `poplar config init
--interactive` works as a standalone subcommand.

**Master plan + spec.** Master plan §Task 13 (line ~2736).
Design rationale in `docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md`
("Error handling" section, "Cancel mid-wizard").

## Tasks

- [ ] `cmd/poplar/root.go` — replace the `ErrFirstRun` → exit 78
  path with a wizard auto-launch. Respect `--no-wizard` and
  `POPLAR_NO_WIZARD=1` (both fall back to exit-78 for existing
  automation). Master plan §Task 13.
- [ ] `--repair=<name>` flag — jumps into the account section
  with the broken `[[account]]` block pre-populated. Wires
  through to the `wizard.Run` orchestrator landed in 14b.
- [ ] `cmd/poplar`: `ErrOldAccountsToml` path keeps exit-78.
  Stale-config auto-repair is post-1.0.
- [ ] Live tmux verification: missing-config first run,
  malformed-config `--repair` flow, `POPLAR_NO_WIZARD=1` exit-78
  passthrough.

## Pass-end ritual

1. `make check` green.
2. ADR-0192 — "First-run auto-launch + repair flow." Covers the
   `runRoot` decision tree (config missing → wizard; malformed
   → friendly error + `--repair` hint; opt-outs honored),
   exit-78 compatibility for automation.
3. `docs/poplar/invariants.md` updates:
   - **Config & theming**: replace the
     "missing config returns `ErrFirstRun`; root exits 78" line
     with the auto-launch behavior + opt-out env/flag. Note
     `--repair=<name>` entry point.
4. `docs/poplar/decisions/INDEX.md` — add ADR-0192.
5. `git mv docs/superpowers/plans/2026-05-10-pass-14c-firstrun-integration.md docs/superpowers/archive/plans/`.
6. `git mv docs/superpowers/plans/2026-05-09-pass-14-firstrun.md docs/superpowers/archive/plans/` — master plan retires here.
7. `git mv docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md docs/superpowers/archive/specs/`.
8. STATUS.md — flip 14c to `done`, pivot to 14.1 (OAuth) or 15.
9. `make install`, commit, push.
