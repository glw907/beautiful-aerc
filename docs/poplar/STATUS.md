# Poplar Status

**Current pass:** Pass 9h.2 next — finish core reorg (account/ extract + cmds.go decomp).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9g | Scaffold → backends → UI → triage → config → Gmail → polish I → Cache 0–III → audits → Attachments I+II → voice → JMAP baseline → Catkin core/QoL/annotations → render fixes → invariants split → catkin lint sweep → popover overlay padding → compose foundation → backend Send + Append → cache outbox Send/Append dispatch (ADRs 0001–0158) | done |
| 9h | ComposeTab + `c`/`r`/`R`/`f` wiring + tidy seam (ADRs 0159–0160) | done |
| 9h.1 | Core reorg leaves — extract compose / movepicker / helppopover / messagelist / sidebar / reader subpackages + uicore sibling; hoist mail.FolderEntry (ADRs 0161–0162) | done |
| 9h.2 | Core reorg parent — extract account/ + decompose cmds.go + system-map refresh + live tmux verification | pending |
| 9h.5 | Drafts persistence (#33) | pending (after 9h.2) |
| 9.1 | Address autocomplete from CardDAV (#34) | pending |
| 9.4 | Email signatures + multiple identities (#32) | pending |
| 9i | Claude Tidy implementation | pending |
| 9.5 | Attachments-richer compose UI (#24) | pending |
| 9.2 | Outbox delivery controls — undo + schedule send (#35) | pending |
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

## Next starter prompt (Pass 9h.2)

> **Goal.** Finish the core reorg started in 9h.1 — extract the
> `AccountTab` parent into `internal/ui/account/` as
> `account.Model`, lift its folder/outbox/triage/sweep cmds and
> msgs out of `internal/ui/cmds.go`, audit the residual `cmds.go`
> down to App-only cross-cutting concerns, and refresh the
> system map.
>
> **Scope.** `AccountTab` → `account.Model` (largest extraction
> by responsibility — composes sidebar/messagelist/reader and
> threads outbox/triage/sweep state). Lift roughly 14 cmds and 12
> msgs from `internal/ui/cmds.go` per the original plan's Task 7
> table. App-level types (`URLOpener`, `TidyFn`, `Styles`,
> `IconSet`) stay in `internal/ui/` and pass through structural
> function-typed parameters where needed. Final `cmds.go` audit
> + `docs/poplar/system-map.md` refresh. Live tmux capture at
> 80×24 and 120×40 (the 9h.1 pass deferred capture pending the
> full reorg). **Out:** any feature work.
>
> **Settled (do not re-brainstorm).**
> - Naming: `package.Model` + `New(...)`; `*Tab` suffix dropped
>   (ADR-0162). The account subpackage will be `account.Model`.
> - Per-package Msg namespace: outbound msgs in `<subpkg>/msgs.go`,
>   App qualifies them at the switch arm (ADR-0162).
> - `uicore` is the shared-chrome home; subpackages cannot import
>   the parent (ADR-0161).
> - Cmds that emit `ErrorMsg` stay in `internal/ui/cmds.go` and
>   take App seams as function-typed parameters.
> - Per-subpackage `Styles` lives in `<subpkg>/styles.go` with
>   `NewStyles(*theme.CompiledTheme)`.
>
> **Still open — brainstorm:**
> - Cycle risk in the account extract: any seam that App owns
>   (`URLOpener`, `TidyFn`, the confirm-modal trio) and account
>   needs to consume during Update — confirm the function-typed
>   parameter pattern handles all of them, or flag a residual.
> - Final residual shape of `internal/ui/cmds.go`: what stays
>   App-level (pumps, ErrorMsg, LaunchURLMsg, confirm-modal)
>   vs. what hoists to account or splits per-screen.
>
> **Approach.** Read this pass's plan + spec in
> `docs/superpowers/archive/plans/` for the original Task 7
> table. Plan at `docs/superpowers/plans/YYYY-MM-DD-core-reorg-finish.md`.
> Likely one ADR (account extract details) plus an update to
> ADR-0161 if the residual `cmds.go` shape settles differently
> than predicted. Standard pass-end checklist applies, plus the
> live tmux verification deferred from 9h.1.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
