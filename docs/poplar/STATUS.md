# Poplar Status

**Current pass:** Pass 9h.1 next — core organizational sweep before v1.0.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9g | Scaffold → backends → UI → triage → config → Gmail → polish I → Cache 0–III → audits → Attachments I+II → voice → JMAP baseline → Catkin core/QoL/annotations → render fixes → invariants split → catkin lint sweep → popover overlay padding → compose foundation → backend Send + Append → cache outbox Send/Append dispatch (ADRs 0001–0158) | done |
| 9h | ComposeTab + `c`/`r`/`R`/`f` wiring + tidy seam (ADRs 0159–0160) | done |
| 9h.1 | Core organizational sweep — naming, package boundaries, msg taxonomy | pending |
| 9h.5 | Drafts persistence (#33) | pending (after 9h.1) |
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

## Next starter prompt (Pass 9h.1)

> **Goal.** Lock in a clean organizational shape before v1.0 —
> naming, package boundaries, msg taxonomy, file layout. Must
> accommodate post-1.0 contacts (CardDAV) and richer calendar
> surfaces beyond 9.7's .ics viewer.
>
> **Scope.** All packages fair game. Up for review: the
> `AccountTab` / `ComposeTab` "Tab" suffix; the `cmds.go`
> kitchen-sink; subpackage split for reader and compose; whether
> any of `internal/mail|cache|compose|content|filter|tidy` have
> names that won't survive contact with future consumers; where
> `internal/contacts/` and `internal/calendar/` live. **Out:**
> any feature work — pure structural pass.
>
> **Settled.** "Tab" suffix is misleading; bubbles-style
> `package.Model` is the strongest external convention; pre-beta
> posture endorses breaking renames; on-disk data is untouched.
>
> **Still open — brainstorm:** subpackage split for compose now
> or at first substate (9.1)? Same for reader? Where do shared UI
> types (Styles, IconSet, theme) live once subpackages exist? How
> do future contacts/calendar surfaces compose with the reader —
> embedded panels, overlays, full screens? Msg-naming convention
> across the App ↔ child-package boundary? Does `cmds.go` survive
> or fragment per-screen?
>
> **Approach.** Brainstorm openly first; plan at
> `docs/superpowers/plans/YYYY-MM-DD-core-reorg.md`. Likely 2–3
> ADRs (naming, package boundary, msg-namespace policies).
> Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
