# Poplar Status

**Current pass:** Pass 9o next — Claude Tidy implementation.

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
| 9n | Email signatures + multiple identities (ADR-0177) | done |
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

## Next starter prompt (Pass 9o)

> **Goal.** Claude Tidy implementation. Replace the identity
> `TidyFn` seam in `internal/ui/app.go` with a Claude-API-backed
> rewrite that runs before MIME assembly on send.
>
> **Scope.** `internal/tidy/` already exists as the home for the
> implementation. Wire it as the default `TidyFn` (replacing
> `identityTidy`), gated by config so users can opt in. The body
> goes through tidy → goldmark → MIME. Failure modes: tidy
> errors surface inline as `c.err` and block the send (the user
> can disable tidy and resend).
>
> **Settled.** TidyFn is a function-pointer seam (ADR-0159).
> Claude API access uses the existing `claude-api` skill / SDK
> shape. Tidy runs on send, not on autosave or draft push.
>
> **Still open — brainstorm:** Config gate (per-account opt-in
> vs global); model selection (Haiku vs Sonnet); prompt shape
> (rewrite vs suggest-and-confirm); error UX (inline banner vs
> overlay); whether to dry-run with a diff preview before send.
>
> **Approach.** Brainstorm the open questions, write
> `docs/superpowers/plans/YYYY-MM-DD-claude-tidy.md`, then
> implement. Standard pass-end ritual via `poplar-pass`.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
