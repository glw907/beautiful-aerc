---
title: Audit Final remediation (Pass 41.1)
status: accepted
date: 2026-05-12
---

## Context

Pass 41 (ADR-0238) ran the comprehensive pre-soak audit and
returned a non-empty findings table: one P1 security item, five
fake-backend test-infra seams, two T15 package-doubling renames,
em-dash density in ADRs 0233–0237, six line-level voice fixes, and
three invariant doc-drift items. Beta soak was gated on the
remediation landing and a re-skim returning empty.

## Decision

Pass 41.1 lands the remediation in one batch:

1. **Security — temp-file write race.** The three config-write
   sites (`internal/ui/wizard/section_confirm.go`, `cmd/poplar/root.go`,
   `cmd/poplar/config_discover_folders.go`) collapse to a single
   `config.AtomicWrite(path, body []byte) error` in
   `internal/config/atomic.go`. The helper chmods the open handle
   to 0o600 *before* any write, closing the umask-window race the
   `os.CreateTemp → write → os.Chmod` sequence opened. Pattern
   mirrors `internal/mailauth/tokenstore_age.go`.
2. **Test-infrastructure seams.** Per-method `*Err` injection
   fields on `internal/cache/cache_test.go::fakeBackend`
   (`connectErr`, `queryErr`), `internal/cache/bodies_test.go::fakeBackendWithBody`
   (`bodyErr`), and `internal/mailimap/fake_test.go::fakeClient`
   (`selectErr`, `capsErr`). The audit's `loginErr` recommendation
   does not fit — `Login` is invoked on the raw `imapclient.Client`
   inside `dial()`, not through the `imapClient` interface — so
   that seam is recorded as out-of-shape rather than added. The
   missing IMAP-cmd-path coverage lands as
   `TestCmdClient_AuthDialFailure` in `redial_test.go`: it injects
   `mail.ErrAuth` via `dialFn` after a connection-error drop and
   asserts a subsequent `Flag` call returns wrapped `ErrAuth`,
   completing the SMTP-side analogue at `smtp_test.go:103`.
3. **T15 renames.** `cache.CacheEvent → cache.Event` (across
   `internal/cache/{account.go,drainer.go}`, `internal/ui/account/msgs.go`,
   `.claude/rules/cache-invariants.md`); `compose.CacheStore →
   compose.Store` (`internal/ui/compose/model.go`).
4. **ADR em-dash density.** Clause-joining em-dashes in ADRs 0233,
   0234, 0235, 0236, 0237 replaced with periods, colons, or
   semicolons. Parenthetical em-dashes and "file — description"
   list separators retained.
5. **Line-level voice.** Six fixes per the audit's P2 table:
   `cache/account.go` MaxSize doc loses the rot-prone "(the new
   default)"; `wizard/apply.go` SMTP-default comment drops the
   user-help meta-note; `mail/types.go::ParseDisposition` doc
   loses the consequentialist tail; `catkin/catkin.go` package
   doc drops the wrong `muesli/reflow` dep claim; `catkin/popover.go`
   `appendUserWord` doc untangles a misplaced parenthetical;
   `term/probe.go::MeasureSPUACells` doc replaces "test glyph"
   shorthand with the actual operation.
6. **Doc drift.** `invariants.md:31` swaps `ADRs 0178, 0191` for
   `ADRs 0193, 0220, 0227` (OAuth subsystem refs);
   `invariants.md:20-21` expands the internal-package brace-list
   to the full 20; `.claude/rules/catkin-invariants.md` drops the
   incorrect `muesli/reflow` provenance — the reflow primitives
   are clean-room.

## Consequences

- `config.AtomicWrite` is the canonical config-write seam. New
  call sites use it directly; the three pre-existing hand-rolled
  variants are gone.
- The IMAP-side cmd-path `ErrAuth → drainer` invariant is now
  test-covered end-to-end. The next pass to touch
  `mailimap.Backend.cmdClient` or `classifyErr` has a direct
  regression net.
- The Audit Final findings table is empty on re-skim grounds:
  every P1 either landed or is documented as out-of-shape
  (the `loginErr` seam). Beta soak opens with Pass 41.2 (the
  first soak entry, not a remediation).
- The `loginErr` gap is recorded as a future shape decision: if
  the `Login` flow ever moves onto the `imapClient` interface for
  testability, the seam follows. Not added prospectively.
