---
title: Audit C remediation — SMTP OAuth retry + viewer-under-compose
status: accepted
date: 2026-05-11
---

## Context

Audit C (ADR-0221) queued two P1 findings before Audit D could
proceed:

- **A.** `mailimap.smtpAuth` ran `cli.Auth` exactly once. Within
  the 5-minute `mailauth.Client.Token` cushion, a server-side-
  rotated or revoked access token surfaces as auth-failure with no
  recovery path; the cached SMTP client gets dropped on send error
  but the next dial pulls the same stale token, racking outbox
  attempts toward `OpConflict max-attempts-exceeded`.
- **B.** `App.updateMouse` keyed right-pane routing on
  `m.viewerOpen`, but `renderFrame` shows `m.compose.View()`
  whenever `m.compose != nil` regardless of viewer state. A user
  hitting `r`/`R`/`f` from a ready viewer (via
  `composeSeedCmd → SeededMsg → openNewCompose`) or `c` from
  `updateGlobalKey` mounts compose without calling
  `m.acct.CloseViewer()`. Mouse clicks then routed to the
  invisible viewer.

While preparing the SMTP fix, the audit's assumption that the
existing IMAP `dial` retry block (`auth.go:97-113`) actually fires
turned out to be incorrect. `cli.Authenticate` returns raw
`*imap.Error` and `errors.Is(authErr, mail.ErrAuth)` never
matched. The remediation extends `classifyErr` to recognize SMTP
`*gosmtp.SMTPError` codes (530/535/538) and wraps the dial-side
`authenticate` calls through `classifyErr` so both the existing
IMAP retry and the new SMTP mirror trigger correctly.

## Decision

1. **`classifyErr` recognizes SMTP auth codes.** SMTP error codes
   530, 535, and 538 wrap to `mail.ErrAuth`. `mailimap/errors.go`
   imports `gosmtp` for the `*SMTPError` type assertion.

2. **IMAP `dial` wraps `authenticate` through `classifyErr`.** The
   call sites at `auth.go:97` and `auth.go:105` now pass through
   `classifyErr(authenticate(...))`; the existing
   `errors.Is(_, mail.ErrAuth)` retry branch fires on any IMAP
   AUTHENTICATE NO with the auth-failure response code.

3. **`smtpAuth` retries once on stale-token `ErrAuth`.** Mirrors
   the IMAP pattern: classify `cli.Auth`, on `mail.ErrAuth` with
   `b.oauth != nil` call `b.oauth.ForceRefresh(ctx)` and retry
   once. A new `smtpAuther` interface (`Auth(c sasl.Client) error`)
   replaces the concrete `*gosmtp.Client` parameter so tests can
   substitute a fake without dialing.

4. **`Send` lifecycle unchanged.** Send already drops the cached
   SMTP client on every error (smtp.go:155). The next Send
   re-dials, re-runs `smtpAuth`, which now ForceRefreshes on
   `ErrAuth`. No explicit ErrAuth branch needed in `Send`.

5. **Compose closes the viewer on open.** Both compose-mount
   sites — `openNewCompose` (SeededMsg / RestoreFromDraftMsg /
   reply / forward) and the `m.keys.Compose` branch of
   `updateGlobalKey` — now call `m.acct = m.acct.CloseViewer()`
   followed by `m = m.deriveChromeFromAcct()` when
   `m.viewerOpen == true`. `updateMouse`'s right-pane predicate
   stays keyed on `m.viewerOpen` and now matches `renderFrame`'s
   surface choice.

## Consequences

- The 5-minute Token cushion no longer strands outbox sends on a
  rotated access token. SMTP and IMAP share the same
  ForceRefresh-on-ErrAuth contract.
- Mouse routing on the right pane is single-sourced: the same
  predicate (`m.viewerOpen` for viewer routing, `m.compose != nil`
  implicitly via the close-on-open invariant) governs both render
  and dispatch.
- `mailimap.classifyErr` is now a unified IMAP+SMTP classifier;
  `internal/mailimap/errors.go` carries the `gosmtp` import.
  Future SMTP error mapping (e.g. 552 oversize → its own sentinel)
  has an obvious home.
- `smtpAuther` is a single-impl interface in production code, but
  it has a real test seam (`fakeAuther` in `smtp_test.go`) — the
  ADR-0141 single-impl-interface bar is met.
- Audit D (Pass 37, database) is now unblocked. Pass 35.1 (live
  Gmail + Outlook OAuth verification) remains queued, gated on
  client IDs.
