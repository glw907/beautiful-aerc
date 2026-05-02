# Poplar Status

**Current pass:** Pass 8.1 next — Gmail preset on top of the
generic IMAP backend. Pass 8 done — generic IMAP via
`emersion/go-imap` v2 with provider registry (yahoo/icloud/zoho),
two-connection model, 9-min IDLE refresh, and `Destroy` mapping
(ADR-0098/0099/0100/0101). Live verification ran end-to-end
against local Dovecot.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 5 (incl. SPUA-policy, 2.5b-4b) | Scaffold → backend → UI → bubbletea cleanup (see git log) | done |
| 6 / 6.5 | Triage + undo bar (ADR-0089/0090); move picker (ADR-0091) | done |
| 6.6 | Trash retention + manual empty (Destroy primitive, sweep, ConfirmModal) | done — ADR-0092/0093/0094 |
| 6.7 | Tmux verify retention/empty + reference-app research | done — ratifies 0092/0093/0094, ConfirmModal width-drift fix |
| 6.8 | Docs refactor: path-scoped UI rule, system-map reconcile, wireframes strong-trim, keybindings single-source | done — ADR-0095 |
| 7 | Polish I — responsive sidebar + 80×24 polish bar (#15 closed) | done — ADR-0096/0097 |
| 8 | Generic IMAP backend (provider registry, two-connection, 9-min IDLE, Destroy) | done — ADR-0098/0099/0100/0101 |
| 8.1 | Gmail preset: X-GM-EXT-1, Trash precondition, label-aware fallbacks | next |
| 9 | Compose framing: `Editor` interface, neovim `--embed` adapter, send via go-smtp | pending |
| 9.5 | Compose enhancements: Catkin native editor, tidytext (#12), content cleanup (#13) | pending |
| 10 | Config polish | pending |
| 11 | Final polish + 1.0 prep | pending |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |
| 1.1 | Neovim companion plugin (post-v1, #6) | post-v1 |

## Next starter prompt (Pass 8.1)

> **Goal.** Add a `gmail` provider preset adapting the generic IMAP
> backend to Gmail's quirks so Gmail accounts work in v1.
>
> **Scope.** New `gmail` entry in `config.Providers` with
> `GmailQuirks: true`. Gate Gmail-specific behavior in
> `internal/mailimap/` on the flag: assert `X-GM-EXT-1` at Connect;
> Move-to-Trash must select a non-Trash folder before EXPUNGE so
> Gmail actually deletes; X-GM-LABELS as classification fallback if
> SPECIAL-USE is missing. Wire the `internal/mailauth/` XOAUTH2
> refresh flow into `dialCommand`/`dialIdle`.
>
> **Settled:** Generic IMAP backend (ADR-0099/0100/0101). Provider
> registry (ADR-0098). XOAUTH2 helpers in `internal/mailauth/`.
>
> **Still open — brainstorm these:**
> - XOAUTH2 refresh ownership (cache + 401-watch vs pre-refresh).
> - X-GM-LABELS fallback necessity in 2026 Gmail (likely dead-code
>   defense — confirm).
> - Trash-precondition: generic `mail.Backend` contract or Gmail
>   branch on `b.caps.GmailQuirks`?
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-gmail-preset.md`, then
> implement. Standard pass-end checklist applies.
