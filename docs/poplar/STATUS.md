# Poplar Status

**Current pass:** Pass 9a next — Catkin live markdown styling.

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
| 8.10 | First-sync header population — JMAP per-folder baseline pull (Email/query + sentinel-id state probe in one roundtrip); FetchHeaders chunked at 500 (ADR-0143) | done |
| 9 | Catkin core — package, classifier, reflow, plain render, word nav, scroll-off (ADR-0144) | done |
| 9a | Catkin live markdown styling — render-time lipgloss overlay | pending |
| 9.5 | Compose enhancements — #5 #12 #24 | pending |
| 9.6 | First-run wizard (#27) + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.6 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9a)

> **Goal.** Layer live markdown styling onto Catkin's renderer.
> Bold/italic/code spans render with their delimiters visible
> (iA-Writer-shaped: `**bold**` shows the asterisks AND a bold
> face). Headings, list markers, quote prefixes, and code blocks
> render with style classes wired to `theme.CompiledTheme`. The
> raw buffer is unchanged — styling is purely a render-time
> overlay of lipgloss styles on top of the plain output Pass 9
> ships today.
>
> **Scope.** `internal/catkin/render.go` (extend), new
> `internal/catkin/style.go` (theme-driven style table), no
> changes to blocks.go or reflow.go. Cursor block keeps its
> current shape; styling does not change displayCells per row.
> ADR-0144 invariants hold.
>
> **Settled:** Styling reads from `*theme.CompiledTheme` passed
> through `Model.SetTheme`. Inline span detection runs after the
> block classifier and uses the same regex toolkit (no full
> markdown parser).
>
> **Still open — brainstorm:** code-fence syntax-highlighting
> scope (off / chroma / treesitter); inline span detection
> precedence (e.g., `***triple***` interleaving rules); whether
> link `[text](url)` shows the URL inline.
>
> **Approach.** Brainstorm the open questions, write a plan at
> `docs/superpowers/plans/YYYY-MM-DD-catkin-styling.md`, then
> implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
