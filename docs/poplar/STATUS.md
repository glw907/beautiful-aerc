# Poplar Status

**Current pass:** Pass 9n next — Email signatures + multiple
identities (#32).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9h.6 | Scaffold through drafts persistence (ADRs 0001–0165) | done |
| 9.1a/9.1b | Address book mockups + contact edit form (ADRs 0166, 0167) | done |
| 9j | Comment voice infrastructure — §0 rubric, T38–T40 (ADRs 0168, 0169) | done |
| 9k.1 | Comment sweep — mail wire + config; density-floor exemption (ADR-0170) | done |
| 9k.2 | Comment sweep — cache + outbound chain | done |
| 9k.3 | Comment sweep — UI core; T34 demoted to voice-lens (ADR-0173) | done |
| 9k.4 | Comment sweep — UI subpackages + catkin | done |
| 9l | Compose autocomplete dropdown — fixture-backed To/Cc/Bcc (ADR-0174) | done |
| 9m | CardDAV ingest — swap fixtures for real contacts cache (ADR-0175) | done |
| 9m.1 | CardDAV write-back — form save round-trip via outbox (ADR-0176) | done |
| 9n | Email signatures + multiple identities (#32) | pending |
| 9o | Claude Tidy implementation | pending |
| 9p | Attachments-richer compose UI (#24) | pending |
| 9q | Outbox delivery controls — undo + schedule send (#35) | pending |
| 9r–9t | List-Unsubscribe (#36), .ics viewer (#37), full-account search (#38) | pending |
| 9u | First-run wizard (#27) + OAuth refresh + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14) + items surfaced during 9j–9u | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 9n)

> **Goal.** Signatures + multiple identities (#32). Per-account
> configurable signatures; per-identity selection in compose;
> identity affects `From` header and the JMAP `Identity/get`
> submission identity.
>
> **Scope.** `[account.identities]` TOML block (display name +
> email + signature text or signature-file). Compose adds an
> identity cycler and a signature toggle. JMAP `Send` uses the
> selected identity's `IdentityID`; IMAP path threads the From
> override into the assembled MIME. Signature is appended below
> the user's body on send; the user-facing editor never sees it.
>
> **Settled.** One identity per `[[account]]` is default;
> `[account.identities]` opt-in. Markdown signatures go through
> the same goldmark pipeline as the body. Per-identity From at
> compose time.
>
> **Still open — brainstorm:** Signature delimiter (`-- \n` or
> none); whether to render signatures in the reader (probably yes,
> dimmed); identity selection UX (cycler vs picker).
>
> **Approach.** Brainstorm, write
> `docs/superpowers/plans/YYYY-MM-DD-signatures-identities.md`,
> implement. Standard pass-end ritual via `poplar-pass`.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
