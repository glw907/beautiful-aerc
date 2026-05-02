# Pass 8.1 — Gmail preset (design)

**Date:** 2026-05-02
**Pass:** 8.1
**Status:** draft, awaiting user review

## Goal

Add a `gmail` provider preset that adapts the generic IMAP backend to
Gmail's IMAP eccentricities so Gmail accounts work end-to-end before
beta.

## Scope

In:

- New `gmail` entry in `config.Providers`.
- New `Provider.GmailQuirks bool` field; preset sets it.
- New `capSet.XGM bool`; asserted at Connect for `gmail` accounts.
- Gmail-aware `Destroy`: select `[Gmail]/Trash` before
  `STORE \Deleted` + `UID EXPUNGE`, then re-select the prior
  mailbox. Gated on `b.caps.GmailQuirks`.
- XOAUTH2 path in `dialCommand` / `dialIdle`: re-resolve
  `password-cmd` per dial when `cfg.Auth == "xoauth2"`; never cache
  the access token on the Backend.
- Strip the dead `OAuthClientID` / `OAuthClientSecret` /
  `OAuthRefreshToken` fields and their decode plumbing — they
  were aspirational, never consumed; the wizard pass (9.6) re-adds
  them when there is a real consumer.
- Update `template.go` / `template.golden` so the OAuth note matches
  reality: Gmail/Outlook auth via `password-cmd` returning a fresh
  access token; an in-app OAuth flow lands with the wizard.

Out:

- Internal OAuth refresh (HTTP exchange against Google's token
  endpoint). Defer to Pass 9.6.
- X-GM-LABELS classification fallback. Gmail has reliably advertised
  SPECIAL-USE for years; the existing `mail.Classify` alias table
  already covers Gmail's `[Gmail]/...` names. Adding a labels-based
  fallback for a case that does not occur in 2026 violates the
  pre-beta "no defensive code for non-existent cases" rule.
- Gmail-specific Move quirks. `UID MOVE` on Gmail relabels, which
  is the desired semantic for soft delete (`Delete()` → Trash) and
  ordinary file-into-folder. No branch needed.

## Settled decisions

These are recorded so the plan does not re-litigate them; each has
been judged architecturally obvious for the pre-beta stance.

1. **XOAUTH2 refresh ownership: external via `password-cmd`.**
   `cfg.Auth == "xoauth2"` accounts skip the `b.password` cache and
   re-run the resolver on every dial. Users wire any refresher
   (`oauth2l`, a custom script, 1Password's `op`, etc.) into
   `password-cmd`. Internal refresh against Google's token endpoint
   waits for Pass 9.6, where the first-run wizard has a UI to
   capture client ID / secret / refresh token.
2. **No X-GM-LABELS fallback.** Gmail SPECIAL-USE is reliable in
   2026; the alias table handles `[Gmail]/Trash` etc. anyway.
3. **Trash precondition lives in the IMAP backend, gated on
   `b.caps.GmailQuirks`.** It is a Gmail-specific eccentricity;
   the generic `mail.Backend` contract should not carry it. JMAP
   does not need it.

## Architecture

### `internal/config/`

- `Provider` gains `GmailQuirks bool`. Preset registry adds:
  ```
  "gmail": {
      Name:        "gmail",
      Backend:     "imap",
      Host:        "imap.gmail.com",
      Port:        993,
      AuthHint:    "xoauth2",
      GmailQuirks: true,
      HelpURL:     "https://support.google.com/mail/answer/7126229",
  },
  ```
- `AccountConfig` gains `GmailQuirks bool`; `Provider.GmailQuirks`
  is copied into it during preset resolution (mirrors how
  `InsecureTLS` flows from preset → account).
- `OAuthClientID` / `OAuthClientSecret` / `OAuthRefreshToken`
  removed from `AccountConfig`, the TOML schema struct, the
  `resolveEnv` plumbing, and the template. Tests covering those
  fields are deleted (the wizard pass re-adds with consumers).
- Template prose updated to: "Gmail and Outlook need an access
  token, not a password. Set `password-cmd` to a command that
  prints a fresh access token to stdout (e.g., `oauth2l fetch
  --type=oauth2 --output_format=bare ...`). An in-app OAuth flow
  lands with the first-run wizard."

### `internal/mailimap/`

- `capSet` gains `XGM bool`; `finishConnect` reads
  `caps["X-GM-EXT-1"]`. When `b.cfg.GmailQuirks` is true and
  `cs.XGM` is false, Connect returns
  `errors.New("gmail account but server does not advertise X-GM-EXT-1")`.
  Storing the bit on `capSet` mirrors `MOVE`/`IDLE`/`SpecialUse`.
- `Destroy(uids)` branches on `b.caps.GmailQuirks`. Generic path
  unchanged. Gmail path:
  1. Resolve Trash folder via `resolveTrashFolder()`.
  2. `cmd.Select(trash, false)` — explicit, so EXPUNGE truly
     deletes on Gmail.
  3. `cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"})`.
  4. `cmd.UIDExpunge(uids)`.

  Contract: `Destroy` on a Gmail backend assumes `uids` reference
  messages that already live in `[Gmail]/Trash`. Both real callers
  satisfy this — manual Empty Trash (ADR-0094) is gated to
  Disposal folders, and the retention sweep (ADR-0093) only fires
  on Disposal-folder entry. Other backends (and other IMAP
  servers) accept Destroy from any folder; the Gmail constraint
  is documented in `mailimap/README.md` and asserted-by-comment
  in `Destroy`. No selection restore — every other backend method
  (`OpenFolder`, `QueryFolder`, …) issues its own `Select` before
  acting.
- `dial` (in `auth.go`): the caller passes the resolved password
  in. `Connect` already calls `b.resolvedPassword()` once. Change
  to: when `cfg.Auth == "xoauth2"`, call `resolvePassword(&cfg)`
  directly — bypassing the cache. The `b.password` field stays for
  password / app-password accounts. Idle reconnects in `idle.go`
  similarly call a token resolver per attempt for XOAUTH2.

### Tests

- `config/providers_test.go`: add `gmail` preset case; assert
  `GmailQuirks == true`, `AuthHint == "xoauth2"`.
- `config/accounts_test.go`: assert preset → `AccountConfig` copies
  `GmailQuirks`. Remove the `OAuthClient*` test cases.
- `mailimap/imap_test.go`: Connect with `GmailQuirks` and a
  capability map missing `X-GM-EXT-1` returns the expected error;
  with it present, succeeds.
- `mailimap/actions_test.go`: new test
  `TestDestroy_GmailQuirks_SelectsTrashFirst` driven by the
  existing fake imapClient. Asserts the call sequence:
  `Select(trash, false)`, `Store(...)`, `UIDExpunge(uids)`. A
  paired test on a non-quirks backend asserts no `Select` call.
- `mailimap/auth_test.go`: a Backend with `cfg.Auth == "xoauth2"`
  and a `password-cmd` that returns different values on each call
  observes both values across two dials (no caching).

### Docs

- `internal/mailauth/README.md`: add a sentence noting that until
  the wizard ships, XOAUTH2 access tokens come from `password-cmd`.
- `internal/config/template.golden` regenerated.
- `docs/poplar/invariants.md`: under the "Backends in v1" bullet,
  add `gmail` to the preset list and note "X-GM-EXT-1 required;
  Destroy routed via `[Gmail]/Trash`; XOAUTH2 access token via
  `password-cmd`, no internal refresh until Pass 9.6."

### ADRs

Pass-end will write:

- **ADR-0106 — Gmail preset and X-GM-EXT-1 assertion.** Pins the
  preset shape and the Connect-time capability assertion.
- **ADR-0107 — Gmail Destroy routing via `[Gmail]/Trash`.**
  Pins the SELECT-Trash-before-EXPUNGE pattern, gated on
  `GmailQuirks`. Cross-references ADR-0100.
- **ADR-0108 — XOAUTH2 access tokens via `password-cmd`.** Pins
  the external-refresh decision, names Pass 9.6 as the integrator,
  records the deletion of the unused `OAuth*` fields.

## Risks

- **Live-test coverage.** The fake `imapClient` exercises the
  Destroy sequence, but no Gmail account is wired into the
  workstation today. Mitigation: pass-end smoke test runs `poplar
  config check` against a Gmail account configured with a manual
  `oauth2l fetch` command; if the user does not have one
  configured, the live test is deferred to a follow-up note in
  `BACKLOG.md`.
- **Re-running `password-cmd` per dial.** If the user wires a slow
  refresher (e.g., interactive OAuth dance), every reconnect blocks
  on it. Acceptable pre-beta; documented in the template.

## Out-of-scope follow-ups

- Internal OAuth refresh / token endpoint exchange — Pass 9.6.
- Gmail-specific UI affordances (label coloring, X-GM-MSGID
  stable URLs) — none planned for v1.
