# Poplar Status

**Current pass:** Pass 8 next — Gmail IMAP backend on
`emersion/go-imap` v1. Pass 7 done — responsive sidebar
(`sidebarWidthFor`) + 80×24 design polish bar (ADR-0096/0097),
closes BACKLOG #15.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 5 (incl. SPUA-policy, 2.5b-4b) | Scaffold → backend → UI → bubbletea cleanup (see git log) | done |
| 6 / 6.5 | Triage + undo bar (ADR-0089/0090); move picker (ADR-0091) | done |
| 6.6 | Trash retention + manual empty (Destroy primitive, sweep, ConfirmModal) | done — ADR-0092/0093/0094 |
| 6.7 | Tmux verify retention/empty + reference-app research | done — ratifies 0092/0093/0094, ConfirmModal width-drift fix |
| 6.8 | Docs refactor: path-scoped UI rule, system-map reconcile, wireframes strong-trim, keybindings single-source | done — ADR-0095 |
| 7 | Polish I — responsive sidebar + 80×24 polish bar (#15 closed) | done — ADR-0096/0097 |
| 8 | Gmail IMAP (direct-on-emersion rewrite) | next |
| 9 | Compose framing: `Editor` interface, neovim `--embed` adapter, send via go-smtp | pending |
| 9.5 | Compose enhancements: Catkin native editor, tidytext (#12), content cleanup (#13) | pending |
| 10 | Config polish | pending |
| 11 | Final polish + 1.0 prep | pending |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |
| 1.1 | Neovim companion plugin (post-v1, #6) | post-v1 |

## Next starter prompt (Pass 8)

> **Goal.** Add Gmail IMAP backend on `emersion/go-imap` v1,
> paralleling the JMAP backend in `internal/mailjmap/` so Gmail
> accounts are usable in v1.
>
> **Scope.** New package `internal/mailimap/` implementing the
> `mail.Backend` interface against `emersion/go-imap` v1 with
> vendored XOAUTH2 + Gmail X-GM-EXT helpers from
> `internal/mailauth/`. Folder classification reuses the existing
> `mail.Classify`. Keep changes within `internal/mailimap/`,
> `internal/config/` (account decode), and `cmd/poplar/` (account
> dispatch).
>
> **Settled:** mail.Backend stays synchronous (ADR-0075). Direct-on-
> emersion (no aerc fork). XOAUTH2 + X-GM-EXT vendored helpers
> already in `internal/mailauth/`. 80×24 polish bar (ADR-0097)
> applies to any new UI surface this pass introduces.
>
> **Still open — brainstorm before coding:**
> - Token refresh ownership: in `mailauth/` or `mailimap/`?
> - Idle/keepalive strategy for the IMAP connection.
> - How `Destroy` (ADR-0092) maps to IMAP UID EXPUNGE.
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-gmail-imap.md`, then implement.
> Standard pass-end checklist applies.
