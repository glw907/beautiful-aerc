# Poplar Status

**Current pass:** Pass 9h.5 next — drafts persistence (issue #33).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9g | Scaffold → backends → UI → triage → config → Gmail → polish I → Cache 0–III → audits → Attachments I+II → voice → JMAP baseline → Catkin core/QoL/annotations → render fixes → invariants split → catkin lint sweep → popover overlay padding → compose foundation → backend Send + Append → cache outbox Send/Append dispatch (ADRs 0001–0158) | done |
| 9h | ComposeTab + `c`/`r`/`R`/`f` wiring + tidy seam (ADRs 0159–0160) | done |
| 9h.1 | Core reorg leaves — extract compose / movepicker / helppopover / messagelist / sidebar / reader subpackages + uicore sibling; hoist mail.FolderEntry (ADRs 0161–0162) | done |
| 9h.2 | Core reorg parent — extract account/, hoist ErrorMsg/TriageOp/ComputeLayout/NewSpinner to uicore, lift account-scoped cmds (ADR-0163) | done |
| 9h.5 | Drafts persistence (#33) | pending |
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

## Next starter prompt (Pass 9h.5)

> **Goal.** Persist compose drafts so closing the compose surface
> (Esc / Ctrl+C with discard / app quit) preserves the draft and
> reopening (`c` from anywhere, or selecting a Drafts row) restores
> it. GitHub issue #33.
>
> **Scope.** Cache schema bump for a `drafts` table keyed by
> draft-id (UUID), columns mirroring `compose.Draft` (To/Cc/Bcc/
> Subject/Body/From + attachment paths) plus `created_at` /
> `updated_at`. Auto-save on dirty during compose. New
> `compose.Model` lifecycle: opening with no draft-id creates one,
> opening with an id seeds from the cache. App routes Drafts-folder
> Enter to compose-with-id. Discard removes the row; Send removes
> the row after queuing (cache-tx). **Out:** signatures (9.4),
> address autocomplete (9.1), schedule send (9.2), attachments-rich
> compose UI (9.5).
>
> **Settled (do not re-brainstorm).**
> - Cache schema migration is welcomed pre-beta (ADR-0125 posture).
> - Cache schema currently at v6 (ADR-0158). Bump to v7 with the
>   drafts table.
> - `compose.Model` is App-owned; lifecycle stays in App.
>
> **Still open — brainstorm:**
> - Auto-save cadence — every Update tick, debounced, or on focus
>   change between fields? Trade-off: write amplification vs. lost
>   characters on crash.
> - Drafts-folder UX: does the Drafts row show the cached draft
>   contents, or does it still go through `Append`-to-Sent-style
>   server round-trip? IMAP/JMAP behavior differs here.
> - Multi-draft model: one open compose at a time stays the rule
>   (ADR-0159). Confirm draft-id assignment doesn't change that.
>
> **Approach.** Brainstorm the open questions first, write a plan
> doc at `docs/superpowers/plans/YYYY-MM-DD-drafts-persistence.md`,
> then implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
