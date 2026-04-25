# Pass 2.5b-4 — Message Viewer Prototype: Execution Plan

Companion to spec `docs/superpowers/specs/2026-04-14-viewer-prototype-design.md`.
The spec resolved every open question from the STATUS starter prompt
(async fetch with spinner; link picker out / `1`-`9` inline in;
auto-mark-read in; search keys inert while viewer open + filter
survives round-trip). This doc sequences the spec into executable
phases.

## Strategy

Phases are ordered by dependency. Inside a phase, tasks run serially
unless explicitly noted as parallel-safe (no overlapping files).
Each phase ends with `make check` before moving on so failures stay
local. Live tmux verification only runs after all phases pass.

## Phase 1 — Content layer foundation

Independent of UI. Land first so viewer can pull from a stable
content pipeline and a richer mock backend.

1. **Width cap correction** — `internal/content/render.go:12` change
   `maxBodyWidth = 78` → `72`. Update any test in
   `render_test.go` that asserted 78. (ADR pulled forward to record
   the rationale: matches mailrender baseline.)
2. **Footnote harvesting** — add `RenderBodyWithFootnotes(blocks,
   theme, width) (string, []string)` per spec §"Footnote
   harvesting". Skip auto-linked bare URLs; dedupe; glue last word
   to `[^N]` with ` `; append a horizontal rule + per-URL list
   when count > 0. New file `internal/content/render_footnote.go`
   to keep `render.go` from bloating; tests in new
   `render_footnote_test.go`.
3. **Header address-unit regression test** — `headers_test.go` add
   `TestRenderHeadersAddressUnitAtomic` locking the existing
   "wrap between addresses, never inside `Name <email>`" property.
4. **Adversarial wrap tests in `render_test.go`** — `TestRenderBody
   {WrapStressParagraph, LongURL, NestedQuoteWrap, ListHangingIndent,
   FootnoteEdge}` per spec §"Unit tests".

Gate: `make check`.

## Phase 2 — Mock backend enrichment

5. **Realistic markdown bodies in `internal/mail/mock.go`** — add a
   package-level `map[mail.UID]string` of ≥6 stress-test bodies
   per spec §"Mock backend enrichment". `FetchBody` returns the
   matching string; falls back to a short default for unmapped
   UIDs. Update `mock_test.go` if it asserts the fallback shape.

Gate: `make check`.

## Phase 3 — Viewer model

6. **`internal/ui/viewer.go`** — `Viewer` struct + `viewerPhase`
   + `Open(msg)`, `Close()`, `IsOpen()`, `CurrentUID()`,
   `SetBody(blocks, headers)`, `View()`, `Update(msg, theme,
   styles, w, h)`. No backend reference — pure state + render.
   `Update` returns `(Viewer, tea.Cmd)`. Loading phase shows
   centered `spinner.View() + " Loading message…"`. Ready phase
   composes headers (full content width) + rule + body
   (viewport-scrolled, `min(contentWidth, 72)`). Closed phase
   returns `""`. Emits `ViewerScrollMsg{pct}` when scroll position
   changes.
7. **`internal/ui/viewer_test.go`** — every viewer test from spec
   §"Unit tests", including stale `bodyLoadedMsg` UID guard,
   numeric link launch with mock opener hook (export an
   `openerFunc` package var the test swaps).

Gate: `make check`.

## Phase 4 — AccountTab integration

8. **State, key routing, Cmds in `internal/ui/account_tab.go`** —
   add `viewer Viewer` field. `View` swaps between `msglist.View()`
   and `viewer.View()` based on `viewer.IsOpen()`. `handleKey`
   routes viewer-first when open. New cases on the msglist side:
   `enter` opens viewer, fires `loadBodyCmd`, fires `markReadCmd`
   if unread, optimistically flips local seen flag. New
   `bodyLoadedMsg` case in `updateTab` calls `viewer.SetBody` iff
   UID matches. Search keys (`/`, mode cycle, history) inert while
   viewer open. Folder jumps (`I/D/S/A/X/T`, `J/K`) inert while
   viewer open. Cmds (`loadBodyCmd`, `markReadCmd`, `launchURLCmd`)
   live in `cmds.go`.
9. **`internal/ui/account_tab_test.go`** — extensions per spec
   §"Unit tests": `TestEnterOpensViewer`, `TestEnterMarksRead`,
   `TestEnterEmptyFolderNoOp`, `TestSearchKeysInertWhileViewerOpen`,
   `TestFolderJumpInertWhileViewerOpen`.

Gate: `make check`.

## Phase 5 — Chrome wiring

10. **Status bar mode switch** — `internal/ui/status_bar.go` adds
    `StatusMode` (`StatusAccount`, `StatusViewer`), `SetMode`,
    `SetScrollPct`. View switches between counts (existing) and
    `NN% · ● connected`. Tests in `status_bar_test.go`.
11. **Footer context switch** — `ViewerOpenedMsg`/`ViewerClosedMsg`
    Cmds in `cmds.go`. `App.Update` catches them and calls
    `footer.SetContext(...)` + `statusBar.SetMode(...)`.
    `ViewerScrollMsg` updates status bar scroll pct. Extensions in
    `app_test.go`: `TestViewerOpenedSwitchesFooterContext`,
    `TestViewerClosedRestoresFooterContext`,
    `TestViewerScrollUpdatesStatusBar`.
12. **Footer viewer context content** — `internal/ui/footer.go`
    add the viewer context binding list per wireframes §4 footer:
    `d:del a:archive s:star ┊ r:reply R:all f:fwd ┊ Tab:links
    q:close ?:help`. Drop ranks per the spec's "rank 0 never
    drops" rule.

Gate: `make check`.

## Phase 6 — Modifier-free cleanup

13. **Msglist key cleanup** — `internal/ui/account_tab.go` remove
    the `ctrl+d/u/f/b` and `pgdown/pgup` cases (lines 168-174 in
    current source). Quit binding: `keys.go` line 17 keeps
    `ctrl+c` as the conventional terminal-kill alias (not a
    user-facing binding) but the help/footer never advertises it.
14. **Footer drop-rank tables** — `footer.go` remove any hint
    referencing Ctrl-bound nav.
15. **Wireframes update** — `docs/poplar/wireframes.md` §4 footer
    line + §5 viewer/account help popovers: strip `C-d/C-u/C-f/C-b`
    rows.
16. **Keybindings doc update** — `docs/poplar/keybindings.md` strip
    rows 23-26 (the four C- nav rows).

Gate: `make check`.

## Phase 7 — Live verification

17. **`make install` + tmux render walk** per spec §"Live tmux
    verification" steps 1-7. Capture before/after; fix any visual
    regression before commit.

## Phase 8 — Pass-end ritual

18. `/simplify`, ADRs (viewer model, footnote harvesting, width
    correction, modifier-free enforcement), invariants update,
    move this plan + the viewer spec into `archive/`, `make check`,
    commit, push, `make install`.

## Risk notes

- **Viewport height churn** — `WindowSizeMsg` arrives before
  `bodyLoadedMsg`. Loading phase ignores stale height and
  recomputes when ready. Test covers this via
  `TestViewerScrollEmitsScrollMsg` after a `WindowSizeMsg` mid-load.
- **Stale body race** — UID guard in `updateTab` is the only
  safety net. The test for `TestViewerStaleBodyLoadedIgnored`
  must exercise the close+reopen-on-different-UID path explicitly.
- **`xdg-open` portability** — fire-and-forget per spec; errors
  drop silently. Production hardening waits for Pass 2.5b-6
  (toast/error system).
- **Search ↔ viewer cursor coupling** — out of scope (backlog #9
  bundled with Pass 3). When viewer opens, msglist cursor stays
  put; close returns to the same row.

## File touch list

New:
- `internal/content/render_footnote.go`
- `internal/content/render_footnote_test.go`
- `internal/ui/viewer.go`
- `internal/ui/viewer_test.go`

Modified:
- `internal/content/render.go` (width cap)
- `internal/content/render_test.go` (width assertion + adversarial)
- `internal/content/headers_test.go` (atomic-unit test)
- `internal/mail/mock.go` (body bodies)
- `internal/mail/mock_test.go` (fallback shape if asserted)
- `internal/ui/account_tab.go` (viewer integration + ctrl removal)
- `internal/ui/account_tab_test.go` (extensions)
- `internal/ui/app.go` (context msg routing)
- `internal/ui/app_test.go` (extensions)
- `internal/ui/cmds.go` (load/mark/launch + open/close/scroll msgs)
- `internal/ui/footer.go` (viewer context + drop ranks)
- `internal/ui/footer_test.go` (rank coverage)
- `internal/ui/status_bar.go` (mode switch)
- `internal/ui/status_bar_test.go` (mode coverage)
- `docs/poplar/wireframes.md` (§4 + §5 ctrl strip)
- `docs/poplar/keybindings.md` (rows 23-26 strip)
