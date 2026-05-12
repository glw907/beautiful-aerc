# Poplar Status

**Current pass:** Pass 35 — native OAuth for Gmail / Outlook IMAP
(#42, BYO client ID). Pass 34 closed ADR-0219: mouse is
one-to-one keyboard shorthand across sidebar + right pane +
cross-pane, with `account.Model.CloseViewer()` as the new
viewer-close seam.

**Beta soak deferred.** Pre-beta rules apply; soak entry gated
on a full audit cycle returning no findings.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 32 | Scaffold through v2 declarative chrome (ADRs 0001–0217) | done |
| 33 | Mouse support — reader + attachments + scroll (ADR-0218) | done |
| 34 | Mouse support — sidebar + cross-pane (ADR-0219) | done |
| 35 | **Native OAuth** for Gmail / Outlook IMAP (#42, BYO client ID) | next |
| 36 | **Audit C** — feature surface | gate |
| 37 | **Audit D** — database (schema ladder, tx boundaries, FTS5, UIDVALIDITY, on-disk shape) | gate |
| 38 | **Audit Final** — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |
| post-1.0 | Neovim companion (#6), raw RFC822 (#21), beyond | future |

### Next starter prompt (Pass 35)

> **Goal.** Wire native OAuth (PKCE + refresh) for Gmail and
> Outlook IMAP so the user no longer needs to paste app-specific
> passwords or shell-out to `oauth2l`. BYO client ID via
> `[account.oauth] client-id` / `client-secret`.
>
> **Scope.** End-to-end consent + token cache for Gmail and
> Outlook. `mailauth.Authorize` opens a localhost loopback,
> drives the PKCE authorize endpoint, stores the refresh token
> via `oauth-store` (`keyring` or `age-file`), and
> `mailauth.Token(ctx)` resolves access tokens on every IMAP /
> SMTP dial. `poplar --reauth=<name>` re-runs consent.
>
> **Settled (do not re-brainstorm):** `[account.oauth]` schema
> (ADR-0193); `mailauth.Token(ctx)` is the read seam;
> XOAUTH2 wraps the access token (ADR-0208); IMAP backend
> already routes through `mailauth.Token` when
> `[account.oauth]` is present.
>
> **Still open — brainstorm these:**
> - Loopback redirect URI port — fixed vs ephemeral. Gmail
>   accepts `http://127.0.0.1:PORT` for native apps;
>   Outlook's policy may differ.
> - Browser-open seam. We have `URLOpener` for viewer links —
>   reuse, or split out a `mailauth.Opener` for testability?
> - First-run flow integration. Wizard hits `Authorize` during
>   the credentials stage; how does that compose with the
>   existing probe step (`wizard.Probe`)?
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-native-oauth.md`, then
> implement. Standard pass-end checklist applies.
