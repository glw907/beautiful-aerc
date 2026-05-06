# Poplar Status

**Current pass:** Pass 9.1 next — address autocomplete from CardDAV (#34).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9g | Scaffold → backends → UI → triage → config → Gmail → polish I → Cache 0–III → audits → Attachments I+II → voice → JMAP baseline → Catkin core/QoL/annotations → render fixes → invariants split → catkin lint sweep → popover overlay padding → compose foundation → backend Send + Append → cache outbox Send/Append dispatch (ADRs 0001–0158) | done |
| 9h | ComposeTab + `c`/`r`/`R`/`f` wiring + tidy seam (ADRs 0159–0160) | done |
| 9h.1 | Core reorg leaves — extract compose / movepicker / helppopover / messagelist / sidebar / reader subpackages + uicore sibling; hoist mail.FolderEntry (ADRs 0161–0162) | done |
| 9h.2 | Core reorg parent — extract account/, hoist ErrorMsg/TriageOp/ComputeLayout/NewSpinner to uicore, lift account-scoped cmds (ADR-0163) | done |
| 9h.5 | Drafts persistence (#33) — schema v7, JMAP PushDraft, compose lifecycle, Drafts-folder routing (ADR-0164) | done |
| 9h.6 | IMAP PushDraft impl + conflict banner + drafts discard race (#39) | pending |
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

## Next starter prompt (Pass 9h.6)

> **Goal.** IMAP `PushDraft` impl, lift the `IsJMAP()` gate, fix
> the discard race (#39), add the conflict-superseded banner.
>
> **Scope.** APPEND `\Draft` then `UID STORE +FLAGS \Deleted` +
> `UID EXPUNGE` on `prevUID` (UIDPLUS-scoped). New UID from the
> APPENDUID response. Drop `IsJMAP()` branches in `c`/Drafts-Enter.
> Discard fix: cancel autosave/push tickers before DeleteDraft (or
> row-version check). Conflict banner: when MarkDraftPushed targets
> a deleted row, surface "draft superseded by another client" once.
> **Out:** signatures (9.4), autocomplete (9.1), schedule send
> (9.2), attachments-rich compose (9.5).
>
> **Settled.** Schema v7 stays. Last-write-wins (ADR-0164). APPEND
> on cmd connection per IMAP invariants.
>
> **Open.** APPENDUID parsing in emersion/go-imap v2. UID EXPUNGE
> single-UID semantics. Ticker-cancel vs. row-version for the race.
>
> **Approach.** Brainstorm the open questions, write
> `docs/superpowers/plans/YYYY-MM-DD-drafts-imap.md`, implement.
> Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
