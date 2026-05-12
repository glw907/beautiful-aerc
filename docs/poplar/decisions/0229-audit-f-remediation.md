---
title: Audit F remediation
status: accepted
date: 2026-05-13
---

## Context

Pass 39 (ADR-0228) ran Audit F and landed 8 P1 items in the
queue. All surface-level; no design alternatives to weigh, no
binding-fact changes.

## Decision

Eight fixes shipped together in Pass 39.1:

- **F-F-1.** `internal/mailauth/loopback.go` sets
  `ReadHeaderTimeout: 10s` and `WriteTimeout: 5s` on the OAuth
  consent `http.Server`. A stalled local connection now closes
  on its own rather than holding the consent goroutine open
  until `consentTimeout`.
- **F-F-2.** `internal/mailimap/smtp.go` threads `ctx` through
  the `smtpDial` seam (`func(ctx, *Backend)`) and
  `Backend.smtpClientLocked(ctx)`. `ProbeSMTP` grew a leading
  `ctx context.Context` parameter, threaded from
  `wizard.Probe`'s ctx and `config check`'s `cmd.Context()`.
  The test fake's two-arg signature follows. `Backend.Send`
  still passes `context.Background()`; the outbox drainer has
  no caller ctx yet, and changing the `mail.Backend.Send`
  signature is a separate decision.
- **F-F-3.** `internal/mailauth/devicecode.go:RequestDeviceAuth`
  floors `dar.ExpiresIn` to 300 (5 min) when the server returns
  a missing or non-positive value, before constructing
  `DeviceAuth`. `PollDeviceCode` now runs at least one
  iteration instead of tripping `ErrConsentTimeout` immediately
  on a quirky server.
- **F-batch2-1.** `cache/reads.go:FetchBody` and
  `cache/attachments.go:FetchAttachment` replace `_ = storeErr`
  with `a.log.Warn`. `cache/attachments.go:Attachments`
  propagates `storeAttachments` errors; a missing metadata row
  would otherwise surface later as "unknown row (call
  Attachments first)" in `FetchAttachment`.
- **F-batch2-2.** `cache/drainer.go:executeOne` routes every
  `finalizeSuccess`/`finishOp` result through a new
  `Account.logTerminal` helper. A non-nil error there leaves
  the row stuck in `OpExecuting`, blocking sibling-op pickup;
  now logged at `Error` level via the drainer's logger.
- **F-batch3-1.** `internal/ui/account/cmds.go:queryFolderCmd`
  surfaces `SyncFolder` errors as
  `uicore.ErrorMsg{Op: op, Err: err}`, matching
  `loadFoldersCmd`'s sibling pattern. Stale-folder failures no
  longer disappear silently.
- **F-batch4-1.** `mailauth.OpenStore` grew a third `preferred
  Backend` parameter. `BackendKeyring` / `BackendAgeFile` skip
  the keyring probe and return the requested store; empty
  preserves the historical probe-and-fallback path.
  `cmd/poplar/backend.go:buildOAuthClient` and
  `internal/ui/wizard/section_oauth.go:buildOAuthClient` pass
  `acct.OAuthStore` / `state.OAuthStore` respectively, so the
  user's explicit `oauth-store` config wins over keyring-probe
  drift.
- **F-batch4-2.** `cmd/poplar/config_discover_folders.go:
  writeAtomically` calls `os.Chmod(tmpPath, 0o600)` after the
  temp close and before the rename, mirroring
  `cmd/poplar/root.go:writeConfigAtomic`. `config.toml` can
  carry embedded credentials.

No invariant changes — these are bug fixes, not new binding
facts. The `OpenStore` signature is not enshrined as an
invariant.

## Consequences

- Audit F P1 queue closed; Audit G (Pass 40, test-assertion
  meaningfulness) unblocked.
- The drainer's terminal-state logging closes a discipline gap
  ADR-0228 called out: silent error suppression on cache
  write-back paths was the project's dominant sharp-edge
  pattern. Future audits grep for `_ = err` first when the
  rubric is calibrated.
- `mailauth.OpenStore`'s signature change is a one-pass break —
  every call site was updated in this commit. No back-compat
  shim.
- `mailimap.ProbeSMTP` gained a leading `ctx` parameter; the
  wizard and `config check` paths already had one to thread.
