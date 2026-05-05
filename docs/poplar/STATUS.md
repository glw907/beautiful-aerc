# Poplar Status

**Current pass:** Pass 9c next — Catkin power-user QoL.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.3 | Scaffold → backends → UI → triage → config v1 → Gmail preset → polish I (see git log; ADRs 0001–0109) | done |
| 8.4 – 8.4b | Cache 0–II — design, foundation, UI cutover, body cache + CLI (ADR-0110–0124) | done |
| 8.4c | Cache III — outbox + offline + `Q`/`!` overlays + status badge (ADR-0132–0134) | done |
| 8.5 – 8.5d | Overengineering audit, Elm conformance, UI structural cleanup, content/filter cleanup (ADR-0125–0131) | done |
| 8.6 – 8.7 | Attachments I (backend) + II (viewer) (ADR-0135–0140) | done |
| 8.8 – 8.9 | Human-voice audit I (string-only) + II (structural) | done |
| 8.10 | First-sync header population — JMAP per-folder baseline pull (ADR-0143) | done |
| 9 | Catkin core (ADR-0144) | done |
| 9a | Catkin live markdown styling (ADR-0145) | done |
| 9b | Catkin command vocabulary (ADR-0146) | done |
| 9c | Catkin power-user QoL — undo/redo, find/replace, auto-pair, smart paste, bracket match, typewriter+focus modes | pending |
| 9d | Annotation pipeline + spellcheck (Track 1 SymSpell + bundled lists; Track 2 hunspell subprocess) | pending |
| 9e | `internal/compose/` — Editor interface, CatkinEditor adapter, Draft, AssembleMIME, Seed{Reply,ReplyAll,Forward} | pending |
| 9f | Mail backend Send + Append — JMAP submission, IMAP+SMTP, `[account.smtp]` config | pending |
| 9g | Cache outbox Send/Append dispatch | pending |
| 9h | ComposeTab UI + `c` wiring + tidy seam | pending |
| 9i | Claude Tidy implementation | pending |
| 9.5 | Attachments-richer compose UI (#24) — multi-attach, attach-from-cache | pending (after 9i) |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9c)

> **Goal.** Catkin power-user QoL: undo/redo (50-step ring buffer
> of buffer + cursor snapshots), find/replace overlay (literal +
> case-insensitive toggle, `y`/`n`/`a` per match), markdown
> auto-pair (six pairs, disabled inside code spans/fences),
> smart URL paste → `[word](url)`, bracket/span match highlight,
> typewriter scroll mode, focus mode, `Ctrl+\` mode-cycle key.
>
> **Scope.** All inside `internal/catkin/`. Library-purity
> preserved. No host-side wiring.
>
> **Settled:** Spec at
> `docs/superpowers/specs/2026-05-04-compose-design.md` § Catkin
> QoL additions (#1–#11). Smart quotes deferred per spec.
>
> **Still open — brainstorm:** none — pure implementation.
>
> **Approach.** Plan at
> `docs/superpowers/plans/YYYY-MM-DD-catkin-qol.md`, then
> implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
