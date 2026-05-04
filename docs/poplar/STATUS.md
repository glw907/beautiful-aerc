# Poplar Status

**Current pass:** Pass 9 next — Compose framing.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 8.3 | Scaffold → backends → UI → triage → config v1 → Gmail preset → polish I (see git log; ADRs 0001–0109) | done |
| 8.4 – 8.4b | Cache 0–II — design, foundation, UI cutover, body cache + CLI (ADR-0110–0124) | done |
| 8.5 | Overengineering audit — ADR-0125/0126/0127; ~700 LOC net deletion | done |
| 8.5b | Elm architecture conformance audit (`internal/ui/`) — ADR-0128 | done |
| 8.5c | UI structural cleanup — ModalShell + SidebarColumn + overlay render caching (ADR-0129, 0130) | done |
| 8.5d | Content/filter cleanup — `Block`/`Span` marker simplification (ADR-0131); #23 already shipped in 9174f85 | done |
| 8.4c | Cache III — outbox + offline + `Q`/`!` overlays + status badge (ADR-0132, 0133, 0134) | done |
| 8.6 | Attachments I — backend (#24) (ADR-0135, 0136, 0137) | done |
| 8.7 | Attachments II — viewer (#24) (ADR-0138, 0139, 0140) | done |
| 8.8 | Human-voice audit I — research-grounded style guide, persona, ADR-0141, skill + `/simplify` updates; string-only fixes (C1 comments, C7 errors, C4 prose verbosity) | done |
| 8.9 | Human-voice audit II — structural fixes (C2/C5/C6/C8/C4-structural) + dedupe of resolvePassword | done |
| 9 | Compose framing — Editor interface, neovim adapter, `go-smtp` | active |
| 9.5 | Compose enhancements — #5 #12 #24 | pending |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9)

> **Goal.** Land the compose framing: pluggable `Editor` interface,
> Catkin (native bubbletea editor) v1, scaffolding for the v1.1
> neovim adapter, and an SMTP send path via `emersion/go-smtp`.
>
> **Scope.** New: `internal/compose/` (Editor interface, Catkin
> impl), `internal/mailsmtp/` (or extend the existing backends to
> send via SMTP). Touches: viewer reply keys (`r`/`R`/`f` per
> ADR-0033), App-level compose overlay, status bar mode for
> compose. Inline rendering — sidebar + chrome stay visible;
> `tea.ExecProcess` is forbidden.
>
> **Settled (do not re-brainstorm):** ADR-0031/0032/0033/0076 set
> the compose framing — `Editor` interface, native Catkin in v1,
> neovim via `--embed` RPC in v1.1, single-key keybindings only
> on text-entry surfaces too. ADR-0058 monorepo. `mailauth`
> already vendors the SASL helpers send will need.
>
> **Still open — brainstorm these:** SMTP backend shape (one new
> package or extend mailimap/mailjmap), Catkin's text-entry input
> model (textarea vs. raw KeyMsg), draft persistence (cache
> outbox `SendArgs`/`AppendArgs` were reserved at Pass 8.4).
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-compose-framing.md`,
> then implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
