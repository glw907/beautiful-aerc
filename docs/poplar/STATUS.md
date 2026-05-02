# Poplar Status

**Current pass:** Pass 7 next — Polish I: help popover narrow-
terminal layout (#15) + small render drift cleanup. Pass 6.7 done
— retention/empty verification (ConfirmModal width-drift fix +
strengthened test) and reference-app research ratifying
ADR-0092/0093/0094.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 5 (incl. SPUA-policy, 2.5b-4b) | Scaffold → backend → UI → bubbletea cleanup (see git log) | done |
| 6 / 6.5 | Triage + undo bar (ADR-0089/0090); move picker (ADR-0091) | done |
| 6.6 | Trash retention + manual empty (Destroy primitive, sweep, ConfirmModal) | done — ADR-0092/0093/0094 |
| 6.7 | Tmux verify retention/empty + reference-app research | done — ratifies 0092/0093/0094, ConfirmModal width-drift fix |
| 6.8 | Docs refactor: path-scoped UI rule, system-map reconcile, wireframes strong-trim, keybindings single-source | done — ADR-0095 |
| 7 | Polish I — popover narrow-terminal (#15) + small render drift cleanup | next |
| 8 | Gmail IMAP (direct-on-emersion rewrite) | pending |
| 9 | Compose framing: `Editor` interface, neovim `--embed` adapter, send via go-smtp | pending |
| 9.5 | Compose enhancements: Catkin native editor, tidytext (#12), content cleanup (#13) | pending |
| 10 | Config polish | pending |
| 11 | Final polish + 1.0 prep | pending |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |
| 1.1 | Neovim companion plugin (post-v1, #6) | post-v1 |

## Next starter prompt (Pass 7)

> **Goal.** Polish I: fix the help popover's narrow-terminal
> layout (BACKLOG #15) and clean up any small render-drift bugs
> surfaced incidentally.
>
> **Scope.** Help popover currently has a fixed natural width
> (~62 cols account context, ~58 viewer). On terminals narrower
> than that, `lipgloss.Place` clips gracefully but the layout
> breaks. Reflow strategies on the table: single-column stacking,
> dropping the right column at width < threshold, or content-
> aware wrapping. Live tmux verification at 80×24 and 60×24 is
> required. Capture before/after pane dumps.
>
> **Settled:** Help popover is App-owned with `helpOpen` +
> `viewerOpen`-driven context (ADR-0072, ADR-0082, ADR-0087).
> Wired/unwired styling stays — popover advertises planned
> vocabulary. No keybinding changes.
>
> **Still open — brainstorm before coding:**
> - Threshold(s) at which layout should switch — single
>   breakpoint or progressive?
> - Single-column reflow vs dropping a column — does dropping
>   misrepresent the planned vocabulary?
> - How does narrow layout interact with the `wired bool` dim
>   styling?
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-popover-narrow.md`, then
> implement. UI work — invoke `elm-conventions` and read
> `docs/poplar/bubbletea-conventions.md` before coding. Standard
> pass-end checklist applies.
