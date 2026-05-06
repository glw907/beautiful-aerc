---
title: Backend Send + Append
status: accepted
date: 2026-05-05
---

## Context

Pass 9e landed the compose foundation (`compose.AssembleMIME` and the
Editor/Draft/Seed types) but left submission unwired. The two v1
backends each need to deposit a finished MIME blob: JMAP via
`Email/import` + `EmailSubmission/set`, IMAP via SMTP for transmission
and APPEND for the Sent copy. The previous `mail.Backend.Send(from
string, rcpts []string, body io.Reader)` shape predates the compose
package and threads bytes the wrong way for IMAP APPEND (which needs
a known length up front) and JMAP (which uploads a blob, not a stream).

## Decision

Reshape `mail.Backend.Send` to `Send(env Envelope, mime []byte) error`
where `Envelope = { From string; Rcpts []string }`. Add
`Append(folder string, mime []byte, flags Flag) error`. Both take
pre-assembled MIME bytes from `compose.AssembleMIME`; the layering
rule stays one-way (`internal/mail/` does not import
`internal/compose/`).

JMAP `Send` batches `Email/import` + `EmailSubmission/set` in one
request. The submission's `EmailID` uses the JMAP `#k1` creation
reference so server-side Sent placement is atomic with submission.
JMAP `Append` is the same shape minus the submission call. Identity
ID is resolved via `Identity/get` on first Send and cached on the
Backend (`b.identityID`).

IMAP `Append` runs `APPEND` on the cmd connection. `Send` dials SMTP
lazily on first call via `emersion/go-smtp` and caches the client; on
any send error the cached client is dropped so the next call redials.
Major clients (mutt/alpine/aerc) all dial SMTP lazily and SMTP
servers drop idle connections aggressively, so an always-on third
connection would buy nothing for the typical "read mostly, send
occasionally" pattern. SMTP creds default to mirroring the IMAP-side
`password`/`password-cmd`/`auth`; the `[account.smtp]` block
overrides only when SMTP differs.

`[account.smtp]` lands as a TOML sub-table on each `[[account]]`.
Provider presets gain `SMTPHost`/`SMTPPort`/`SMTPStartTLS`/`SMTPInsecureTLS`
fields filling the standard submission endpoints (gmail
`smtp.gmail.com:465`, fastmail `smtp.fastmail.com:465`, outlook
`smtp.office365.com:587/STARTTLS`, etc.). `poplar config check`
gains an SMTP probe sequential with the existing IMAP probe
(`mailimap.ProbeSMTP`).

## Consequences

ComposeTab (Pass 9h) and the cache outbox dispatch (Pass 9g) both
land on top of this surface without re-shaping the backend. The
JMAP path collapses Send + Sent-copy atomically; the IMAP path queues
two ops (Send via SMTP, then Append-to-Sent) — the cache drainer in
Pass 9g handles the two-step. The shared `dialRawTCP` helper extracted
from `mailimap/auth.go` covers both IMAP and SMTP TCP setup
(timeout + keepalive tuning). The shared `resolvePasswordCmd` helper
in `internal/config/` collapses the two near-identical password
resolvers.
