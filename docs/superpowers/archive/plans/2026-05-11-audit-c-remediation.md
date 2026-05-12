# Pass 36.1 — Audit C remediation

Date: 2026-05-11
Pass: 36.1
Trigger: Audit C (Pass 36, ADR-0221) queued two P1 findings before
Audit D can run.

## Scope

Two findings from
`docs/superpowers/archive/plans/2026-05-13-audit-c.md`:

- **A.** SMTP `smtpAuth` lacks `ForceRefresh`-on-`ErrAuth` retry —
  stale cached access token within the 5-minute Token cushion
  surfaces as auth-failure with no recovery, racking outbox
  attempt counts toward conflict.
- **B.** Mouse routes to viewer when compose is open over a ready
  viewer — `App.updateMouse` keys on `m.viewerOpen` while
  `renderFrame` actually shows compose in the right pane.

## Settled decisions

- **A vs B in fix B — close the viewer on compose open.** Matches
  the rendered state, keeps the `updateMouse` predicate identical
  to `renderFrame`, and avoids the post-1.0 question of whether
  compose should claim mouse input. Apply at both compose-open
  sites: `openNewCompose` (SeededMsg / RestoreFromDraftMsg /
  reply / forward) and the `m.keys.Compose` branch of
  `updateGlobalKey`.
- The IMAP `dial` retry block (`auth.go:97-113`) is presumed
  effective by ADR-0221, but `cli.Authenticate` returns raw
  `*imap.Error` — `errors.Is(authErr, mail.ErrAuth)` never fires
  today. Fix by wrapping the `authenticate` return through
  `classifyErr` so both the existing IMAP retry and the new SMTP
  mirror trigger correctly. `classifyErr` extends to cover
  `*gosmtp.SMTPError` codes 535/530/538 → `mail.ErrAuth`.
- `Send` already drops the cached SMTP client on every error
  (smtp.go:155). No change needed there — the next Send re-dials,
  re-runs smtpAuth, which now ForceRefreshes on ErrAuth.

## Tasks

1. Extend `mailimap.classifyErr` to map `*gosmtp.SMTPError`
   codes 530/535/538 to `mail.ErrAuth`. Cover with a small unit
   test.
2. Wrap the `authenticate` return value through `classifyErr` in
   `mailimap/auth.go` so the existing IMAP retry actually fires
   on `cli.Authenticate` failures. (The wrap happens at the call
   site, not inside `authenticate`, so probe.go's transcript
   detail string stays unchanged.)
3. In `mailimap/smtp.go`, mirror the IMAP retry pattern in
   `smtpAuth`: classify `cli.Auth` error; on `mail.ErrAuth` with
   `b.oauth != nil`, `ForceRefresh` and retry once with a fresh
   token.
4. Test: extend `smtp_test.go` with a fakeSMTP-style auth path —
   first attempt returns `*gosmtp.SMTPError{Code: 535}`, second
   succeeds; assert ForceRefresh fired and Send completes. Cover
   both the OAuth-on path (retry) and OAuth-off path (no retry,
   surface ErrAuth).
5. Fix B — close viewer at both compose-open sites in
   `internal/ui/app_keys.go` and `internal/ui/app_compose.go`.
   Re-derive chrome via `m.deriveChromeFromAcct()` after
   `CloseViewer()` so the chrome row reflects the new state.
6. Test: extend `app_test.go` with a case that opens the viewer,
   then triggers Compose, and asserts `m.acct.ViewerOpen()` (or
   equivalent) is false post-dispatch.
7. Pass-end: `/simplify`, ADR-0222 (records both fixes + the
   `classifyErr` extension), invariants update if any binding
   fact changed (the SMTP Send/Append + auth language in the
   "Send + Append" section may need a one-line touch-up), STATUS
   bump to Pass 37 (Audit D), archive plan, `make check`,
   commit + push + install.

## Out of scope

- ADR-0221 Finding D follow-on (audit-plan ReportFocus item) was
  already applied in Pass 36.
- Pass 35.1 live OAuth verification — still gated on Gmail +
  Outlook client IDs.
- Mid-session IMAP cmd-path SASL re-auth (P2 adjacent
  observation in Audit C). No current consumer.
