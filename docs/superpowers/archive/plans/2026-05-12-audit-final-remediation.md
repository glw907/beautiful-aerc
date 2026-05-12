# Pass 41.1 — Audit Final remediation

## Goal

Land the P1 findings queued by Pass 41 (ADR-0238) so an audit
re-skim returns empty and beta soak opens.

## Scope

Mechanical fixes from
`docs/superpowers/archive/plans/2026-05-12-audit-final.md` findings
table.

## Tasks

1. **Security — temp-file races.** Replace `os.CreateTemp` +
   follow-up `Chmod` with `os.OpenFile(..., O_RDWR|O_CREATE|O_EXCL,
   0o600)` (or `f.Chmod(0o600)` on the freshly-created handle
   before any write) at:
   - `internal/ui/wizard/section_confirm.go:118-136`
   - `cmd/poplar/root.go:262-278` (repair write)
   - `cmd/poplar/config_discover_folders.go:80-97`
   Pattern reference: `internal/mailauth/tokenstore_age.go`.

2. **Test infra — `*Err` injection seams.** Add per-method error
   fields to:
   - `internal/cache/cache_test.go` `fakeBackend.Connect` →
     `connectErr`
   - `internal/cache/cache_test.go` `fakeBackend.QueryFolder` →
     `queryErr`
   - `internal/cache/bodies_test.go` `fakeBackendWithBody.FetchBody`
     → `bodyErr`
   - `internal/mailimap/fake_test.go` `fakeClient` →
     `selectErr` and `loginErr`
   Cover each new seam with a table-row test that asserts the error
   wraps through.

3. **End-to-end IMAP cmd-path `ErrAuth` → drainer.** Add a test
   that injects `loginErr = mail.ErrAuth` on cmd-path dial and
   asserts the drainer records `OpConflict auth-failure`. Mirror
   the SMTP analogue at `internal/cache/smtp_test.go:103`.

4. **Voice — `cache.CacheEvent → cache.Event` (T15).** Rename
   across `internal/cache/account.go`, all references, tests, and
   the binding facts in `.claude/rules/cache-invariants.md` and
   `docs/poplar/invariants.md` that name the type.

5. **Voice — `compose.CacheStore → compose.Store` (T15).** Rename
   at `internal/ui/compose/model.go:28` plus call sites.

6. **Voice — ADR em-dash density (0233–0237).** Replace
   clause-joining `—` with `.` where halves are independent
   clauses; keep parenthetical em-dashes. Target: handful per ADR.

7. **Voice — six line-level fixes (P2).**
   - `internal/cache/account.go:97`
   - `internal/wizard/apply.go:44`
   - `internal/mail/types.go:96`
   - `internal/catkin/catkin.go:8`
   - `internal/catkin/popover.go:168`
   - `internal/term/probe.go:17`
   Audit T-tells listed in the archived plan; apply per
   `~/.claude/docs/go-comment-voice.md`.

8. **Doc drift — `invariants.md:31` ADR refs.** Correct backend
   paragraph refs (0178/0191 → 0193/0220/0227 as relevant).

9. **Doc drift — internal-package list.** Annotate
   `invariants.md:24-25` brace-list as "load-bearing subset" or
   expand to all 20 packages. Pick the cheaper option.

10. **Doc drift — catkin reflow provenance.**
    `.claude/rules/catkin-invariants.md` must drop the
    `muesli/reflow` provenance claim (no such dep; clean-room).

11. **Pass-end consolidation.** Write ADR-0239, update
    `invariants.md` for type renames, update `STATUS.md`, archive
    this plan, `/simplify`, `make check`, commit, push, install.

## Soak gate

After the pass lands, run a quick re-skim against the four agent
lenses. Empty table → beta soak opens.
