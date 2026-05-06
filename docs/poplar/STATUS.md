# Poplar Status

**Current pass:** Pass 9h next — ComposeTab UI + `c` wiring + tidy seam.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9g | Scaffold → backends → UI → triage → config → Gmail → polish I → Cache 0–III → audits → Attachments I+II → voice → JMAP baseline → Catkin core/QoL/annotations → render fixes → invariants split → catkin lint sweep → popover overlay padding → compose foundation → backend Send + Append → cache outbox Send/Append dispatch (ADRs 0001–0158) | done |
| 9h | ComposeTab UI + `c` wiring + tidy seam (drafts deferred to 9h.5) | pending |
| 9h.5 | Drafts persistence (#33) | pending (after 9h) |
| 9.1 | Address autocomplete from CardDAV (#34) | pending (after 9h) |
| 9.4 | Email signatures + multiple identities (#32) | pending (after 9.1; brainstorm first) |
| 9i | Claude Tidy implementation | pending |
| 9.5 | Attachments-richer compose UI (#24) | pending (after 9i) |
| 9.2 | Outbox delivery controls — undo + schedule send (#35) | pending (after 9.5) |
| 9.3 | List-Unsubscribe one-click, RFC 8058 (#36) | pending |
| 9.7 | Calendar invite (.ics) viewer (#37) | pending |
| 9.8 | Full-account / cross-folder search (#38) | pending |
| 9.6 | First-run wizard (#27) + OAuth refresh + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.8 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9h)

> **Goal.** Land ComposeTab — a new tab wrapping a Catkin body
> editor + header fields (To/Cc/Bcc/Subject). `c` opens new; `r`
> / `R` / `f` open pre-seeded via `compose.Seed*`. Send assembles
> via `compose.AssembleMIME` and queues through `cache.QueueSend`
> (+ `QueueAppend(Sent, FlagSeen)` on IMAP). Land a no-op tidy
> seam for Pass 9i.
>
> **Scope.** `internal/ui/compose_tab.go`; key wiring; plain
> `textinput` address fields (autocomplete is 9.1); discard-
> confirm via `ConfirmModal` on `Esc`. **Out:** drafts persistence
> (9h.5), autocomplete (9.1), signatures (9.4), undo-send (9.2),
> attach UI (9.5).
>
> **Settled.** Editor seam + AssembleMIME + Seed* (ADR-0156);
> Backend Send/Append (ADR-0157); cache QueueSend/QueueAppend +
> payload column (ADR-0158); two-step IMAP, JMAP atomic Sent.
>
> **Still open — brainstorm:** ComposeTab as top-level tab vs.
> AccountTab mode (vim-norm: buffer, not popup); header layout
> (stacked vs. single-line cycling); `Tab`/`Shift+Tab` cycling
> across address fields ↔ body; tidy seam shape (`Tidy interface`
> vs. function pointer).
>
> **Approach.** Read `docs/poplar/research/2026-05-05-compose-cluster.md`
> for the feature-cluster context. Brainstorm, write plan at
> `docs/superpowers/plans/YYYY-MM-DD-compose-tab.md`, implement.
> UI pass: read `docs/poplar/bubbletea-conventions.md`; plan must
> name bubbles analogues. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
