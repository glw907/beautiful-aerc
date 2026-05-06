# Poplar Status

**Current pass:** Pass 9f next — mail backend Send + Append.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9e | Scaffold → backends → UI → triage → config → Gmail → polish I → Cache 0–III → audits → Attachments I+II → voice → JMAP baseline → Catkin core/QoL/annotations → render fixes → invariants split → catkin lint sweep → popover overlay padding → compose foundation (ADRs 0001–0156) | done |
| 9f | Mail backend Send + Append — JMAP submission, IMAP+SMTP, `[account.smtp]` config | pending |
| 9g | Cache outbox Send/Append dispatch | pending |
| 9h | ComposeTab UI + `c` wiring + tidy seam | pending |
| 9i | Claude Tidy implementation | pending |
| 9.5 | Attachments-richer compose UI (#24) | pending (after 9i) |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9f)

> **Goal.** Add `Send` and `Append` to `mail.Backend` and implement
> them on both v1 backends. JMAP uses `Email/submission` plus
> `Email/import`. Generic IMAP grows a third connection — an SMTP
> client via `emersion/go-smtp`. Add the `[account.smtp]` config
> block with provider-preset defaults.
>
> **Scope.** `internal/mail/backend.go` (interface), `internal/mailjmap/`,
> `internal/mailimap/` (SMTP sibling, third connection), `internal/config/`
> (`[account.smtp]`, provider preset SMTP defaults), `poplar config check`
> (SMTP probe). No cache outbox dispatch (Pass 9g). No ComposeTab UI
> (Pass 9h). The current `Backend.Send(from, rcpts, body io.Reader)`
> shape is reshaped to take pre-assembled MIME bytes from
> `compose.AssembleMIME`.
>
> **Settled.** Compose foundation landed (ADR-0156). SMTP via
> `emersion/go-smtp` with the same backoff/reconnect rules as the
> IMAP idle connection (ADR-0107 lineage). Auth via `password-cmd`,
> XOAUTH2 mirror for Gmail (ADR-0102 / ADR-0108). JMAP `Email/submission`
> collapses Send + Append-to-Sent atomically; IMAP path queues a
> separate `Append` for the Sent copy. Provider presets fill SMTP
> defaults at decode time (mirrors the IMAP preset path).
>
> **Still open — brainstorm before coding:** Backend.Send signature
> after the reshape (raw bytes plus envelope, or accept a `Draft`?);
> SMTP connection lifetime (lazy on first Send vs. eager on Connect);
> connect-test surface in `poplar config check` (sequential vs.
> parallel with IMAP/JMAP probes).
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-backend-send.md`, then implement.
> Pass size budget applies. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
