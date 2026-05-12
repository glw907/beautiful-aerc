# Poplar Status

**Current pass:** Pass 34 — mouse support (sidebar + cross-pane).
Pass 33 closed ADR-0218: `tea.View.MouseMode = CellMotion`
declared every frame; `App.updateMouse` cascade absorbs over
overlays, forwards to reader when viewer-ready; reader hit-tests
attachment chips and `[N]: <url>` ribbon rows; wheel forwards
to the body viewport.

**Beta soak deferred.** Pre-beta rules apply; soak entry gated
on a full audit cycle returning no findings.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 32 | Scaffold through v2 declarative chrome (ADRs 0001–0217) | done |
| 33 | Mouse support — reader + attachments + scroll (ADR-0218) | done |
| 34 | **Mouse support** — sidebar + cross-pane | next |
| 35 | Native OAuth for Gmail / Outlook IMAP (#42, BYO client ID) | gated |
| 36 | **Audit C** — feature surface | gate |
| 37 | **Audit D** — database (schema ladder, tx boundaries, FTS5, UIDVALIDITY, on-disk shape) | gate |
| 38 | **Audit Final** — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |
| post-1.0 | Neovim companion (#6), raw RFC822 (#21), beyond | future |

### Next starter prompt (Pass 34)

> **Goal.** Extend the Pass 33 mouse dispatch to the sidebar
> and message list so a click moves the folder cursor / message
> cursor, and pane-crossing clicks behave intuitively (e.g.
> clicking a message in the list while the viewer is open opens
> that message in the viewer).
>
> **Scope.** Sidebar folder rows (single-click selects + loads),
> message-list rows (single-click moves cursor + opens), tree
> expand/collapse on parent rows, scroll wheel in sidebar and
> message list. No keybinding changes; mouse is additive.
>
> **Settled (do not re-brainstorm):** `App.updateMouse` is the
> dispatch arm (ADR-0218); cascade absorbs while an overlay is
> open; mouse never closes overlays via outside-click. Sidebar
> width math threads `uicore.ComputeLayout`.
>
> **Still open — brainstorm these:**
> - Double-click vs single-click semantics for message-list
>   rows. Trade-off: discoverability vs accidental viewer open.
> - Sidebar tree expand-on-click — same row as selection or a
>   gutter target? Account for synthesized intermediate nodes.
> - Scroll-wheel inside the message list — own viewport or
>   bubble up to a future global scroll model?
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-mouse-sidebar.md`, then
> implement. Standard pass-end checklist applies.
