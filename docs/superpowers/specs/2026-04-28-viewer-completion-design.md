# Pass 2.5b-4b — Viewer Completion: Design Spec

**Date.** 2026-04-28
**Goal.** Complete the message viewer with three independent units:
long bare URL handling, filtered `n/N` navigation (BACKLOG #9), and
the `Tab` link picker overlay.

## Scope

In:

- `internal/content/render_footnote.go` and a new
  `internal/content/url_trim.go` — long bare URL detection, trim
  rule, footnote integration.
- `internal/ui/account_tab.go` and `internal/ui/msglist.go` —
  `n`/`N` key dispatch and a new `MessageList.MoveCursor(delta)`
  helper.
- `internal/ui/linkpicker.go` (new) and `internal/ui/app.go` —
  modal link picker overlay, App-level state and composition.
- `internal/ui/viewer.go` — `Tab` handler emits
  `LinkPickerOpenMsg{links}` instead of being a no-op.
- `internal/ui/help.go` — wire a `Tab` row in the viewer help
  vocabulary.

Out: triage actions (Pass 6), key.Matches migration of existing
AccountTab/Viewer dispatch (Pass 5, BACKLOG #17), App.View trust
refactor (BACKLOG #19), and any URL parsing changes beyond bare-URL
length-thresholding (`mailto:`, IDN, userinfo edge cases stay
out-of-scope).

## Unit 1 — Long bare URL handling

### Problem

Today `harvestFootnotes` skips auto-linked bare URLs (where
`Link.Text == Link.URL`): they render inline as the URL itself in
link style, do not occupy a footnote slot, and are not in the
launchable list. A long bare URL is then an unbreakable token —
wordwrap can't split it, hardwrap mangles it mid-URL.

### Decision

Bare URLs > 30 display cells get the long-URL path: footnote slot,
trimmed inline form with `…`, full URL in the harvest list and
footnote-list block. Bare URLs ≤ 30 cells stay unchanged.

### Trim rule

Pure function `trimURL(url string) string`:

1. Strip scheme (`https://`, `http://`, `mailto:`, etc.) — produces
   the "userinfo@host[:port]" + path + query + fragment remainder.
2. Take the host (everything up to the first `/`, `?`, `#`, or end).
3. If a path follows, append `/` plus the first path segment
   (chars after the first `/` until the next `/`, `?`, `#`, or
   `&`).
4. If the URL ends literally with the trailing `/` of step 3,
   preserve it. Otherwise drop any trailing `/`.
5. If anything was trimmed (the trim result is shorter, in display
   cells, than the original `url` minus its scheme), append `…`.

Examples:

| Original | trim |
|---|---|
| `https://example.com` | `example.com` |
| `https://example.com/` | `example.com/` |
| `https://example.com/foo` | `example.com/foo` |
| `https://example.com/foo/` | `example.com/foo/` |
| `https://example.com/foo/bar` | `example.com/foo…` |
| `https://example.com/foo?q=1` | `example.com/foo…` |
| `https://example.com/a/b/c?x=1#frag` | `example.com/a…` |

Out-of-scope edge cases (pass through unchanged): URLs with
userinfo (`https://u:p@host/...`), punycode hosts, raw IPv6
brackets. These do not appear in real bodies poplar surfaces; if
they do, the trim falls through with a slightly less elegant
inline form, never an incorrect URL.

### Integration

`harvestFootnotes` extension in `render_footnote.go`:

- The bare-URL branch (`link.Text == link.URL`) now splits on
  `displayCells(link.URL) > 30`.
- ≤ 30 cells: unchanged (skip).
- > 30 cells: assign a marker via `markerFor(url)`, append `url` to
  `urls`, replace the span with `Link{Text: trimURL(url) + nbsp +
  "[^N]", URL: url}`.

Dedupe semantics: a long bare URL that also appears as a
text-bearing link in the same body shares one footnote slot. The
inline form for the bare occurrence still uses `trimURL`; the
text-bearing occurrence still uses its anchor text. Both glue to
the same `[^N]`. The footnote block lists the URL once.

### Tests

New file `internal/content/url_trim_test.go`:

- `TestTrimURL` — table-driven across the table above plus
  `mailto:foo@example.com` → `foo@example.com` (no path, no `…`).

Extend `internal/content/render_footnote_test.go`:

- `TestLongBareURLFootnoted` — bare URL of 50 cells produces
  `[^1]` inline (with the trimmed form) and a footnote-list entry
  with the full URL.
- `TestShortBareURLPassThrough` — bare URL of 25 cells renders
  unchanged inline, no footnote, harvest list empty.
- `TestLongBareURLDedupedWithTextLink` — body contains both
  `https://x.com/very/long/path?q=1` (bare, ≥30) and
  `[click here](https://x.com/very/long/path?q=1)` — single
  footnote, single harvest list entry.

## Unit 2 — `n`/`N` filtered navigation

### Problem

Today the viewer is a single-message UI: open from the message
list with `Enter`, close with `q`/`Esc`. Walking through messages
requires close → move cursor → re-open, three keystrokes per step.
BACKLOG #9 specifies `n`/`N` for next/previous; implementation was
deferred until Pass 3 wiring landed (which is now done).

### Decision

While the viewer is open and ready (not loading), `n` advances and
`N` retreats over the message list's currently visible row set.
Whichever row set the message list is rendering — filtered when a
search filter is committed, full folder list otherwise — is what
`n`/`N` walks. Boundary: inert at first/last visible row, no wrap.
Thread children count as visible rows; `n`/`N` walks them like any
other row.

### Implementation shape

`MessageList.MoveCursor(delta int) (UID, bool)` — moves the
cursor by `delta` over the visible row set, returns the resulting
UID and whether the cursor moved. Existing visible-row machinery
(thread fold + filter) is the source of truth.

`AccountTab.handleKey` adds `n` and `N` cases when
`viewer.IsOpen()` and `viewer.phase == viewerReady`:

```go
case "n", "N":
    delta := 1
    if msg.String() == "N" {
        delta = -1
    }
    uid, moved := a.msglist.MoveCursor(delta)
    if !moved {
        return a, nil
    }
    info := a.msglist.MessageByUID(uid)  // already exists or trivial to add
    a.msglist = a.msglist.MarkSeen(uid)  // optimistic, reuses Enter path
    a.viewer = a.viewer.Open(info)
    return a, tea.Batch(
        a.viewer.SpinnerTick(),
        loadBodyCmd(a.backend, info),
        markReadCmd(a.backend, info),
    )
```

This mirrors the existing `Enter` handler. The stale-`bodyLoadedMsg`
guard (`viewer.CurrentUID()` UID match) handles rapid-`n` cases
where one fetch is in flight when the next advance fires.

While `viewer.phase == viewerLoading`, `n`/`N` are inert (avoids
queuing a second fetch on top of the first).

### Tests

Extend `internal/ui/account_tab_test.go`:

- `TestViewerNNAdvancesUnfiltered` — Enter on row 0, `n`, viewer
  shows row-1's body.
- `TestViewerNNAdvancesFiltered` — search committed to a 3-thread
  subset, Enter on first, `n` → next visible row (skipping
  filtered-out messages).
- `TestViewerNAtBoundaryInert` — Enter on last row, `n` returns
  no Cmd, viewer state unchanged.
- `TestViewerNDuringLoadInert` — Enter on row 0, `n` before
  `bodyLoadedMsg` arrives — second fetch not queued.
- `TestViewerNDropsStaleBody` — Enter on row 0, `n`, then the
  row-0 `bodyLoadedMsg` arrives (out of order) — viewer ignores it
  and keeps row-1's body.

## Unit 3 — Link picker overlay

### Problem

`1`–`9` quick-launch covers the first 9 harvested URLs only, and
even within the first 9 the user has no way to confirm an index
without scrolling the body. `Tab` is reserved in `viewer.go:165`
as a no-op for this overlay.

### Decision

Modal overlay launched by `Tab` while the viewer is open and ready,
showing every harvested URL as a single-column list with cursor
navigation, full-URL preview footer, and the existing `1`–`9`
shortcuts. App owns the open state and the overlay composition,
following the help popover precedent (ADR-0082). Always-on whenever
`len(viewer.Links()) ≥ 1`.

### Bubbles analogue + deviation

Natural fit: `bubbles/list`. Deviation: hand-rolled.

Reasons:

1. `bubbles/list` ships a default keymap (filter `/`, paging
   `pgup`/`pgdown`, help `?`) that collides extensively with
   poplar's modifier-free vocabulary and reading-surface
   conventions. Suppressing every binding leaves a thin shell that
   provides little over a hand-rolled list.
2. Row formatting needs precise alignment (right-aligned `[N]`
   index column with leading-space padding, `…`-truncation at a
   fixed inline width, full-URL preview footer below the list).
   This requires a custom delegate either way.
3. Help popover (ADR-0072) sets the precedent for hand-rolled
   transient modals. Same machinery applies here.

To be ADR'd this pass.

### State

```go
type LinkPicker struct {
    open   bool
    links  []string
    cursor int
    offset int    // top of visible row window when scrolling
    width  int
    height int
    styles Styles
    theme  *theme.CompiledTheme
    keys   LinkPickerKeys  // key.Binding fields
}
```

No Cmd ownership, no I/O, no spinner.

### Trigger

`Viewer.handleKey` `tab` case:

```go
case "tab":
    if len(v.links) == 0 {
        return v, nil
    }
    return v, linkPickerOpenCmd(v.links)
```

`linkPickerOpenCmd` returns `LinkPickerOpenMsg{links []string}`.
AccountTab forwards to App. App.Update:

```go
case LinkPickerOpenMsg:
    m.linkPicker = m.linkPicker.Open(msg.links)
    return m, nil
```

### Update (while open)

App.Update short-circuits: while `linkPicker.IsOpen()`, all keys
route to `linkPicker.Update(msg)` and nothing else. Same pattern
as `helpOpen`.

`LinkPicker.Update(tea.Msg) (LinkPicker, tea.Cmd)` dispatches via
`key.Matches`:

| Binding | Action |
|---|---|
| `j` / `down` | `cursor = min(cursor+1, len-1)`; scroll if past visible window |
| `k` / `up` | `cursor = max(cursor-1, 0)`; scroll if before visible window |
| `enter` | emit `LaunchURLMsg{links[cursor]}` + `LinkPickerClosedMsg` |
| `1`-`9` | if index in range: emit `LaunchURLMsg{links[idx]}` + `LinkPickerClosedMsg`; else inert |
| `esc` / `tab` | emit `LinkPickerClosedMsg` |
| `q` | swallowed (consistent with help) |
| any other | inert |

App handles the emitted Msgs:

```go
case LaunchURLMsg:
    return m, launchURLCmd(msg.url)
case LinkPickerClosedMsg:
    m.linkPicker = m.linkPicker.Close()
    return m, nil
```

`launchURLCmd` is the existing function in `viewer.go`; unify by
moving it to `cmds.go` so both the viewer's `1`–`9` handler and
the picker share it. The `openURL` package var (test hook) stays.

### Layout

Box width: `min(70, terminalWidth - 4)` natural. Index column
width: `2 + maxDigits` where `maxDigits = len(strconv.Itoa(
len(links)))` (the `2` is the `[` + `]`). Inline URL truncation
width: `boxContentWidth - indexColumnWidth - 1` (the space between
`[N]` and the URL), clipped at 50.

Row format:

```
<leading-space-pad>[<i>] <displayTruncate(url, urlInlineWidth)>
```

Single-digit indices in a ≥10-link list get a leading space before
`[` so the closing `]` aligns. Cursor row is painted with
`AccentPrimary` background (extends across the full row width, not
just the URL).

Preview footer: 2 rows, separated from the list by a thin
horizontal rule (`─` styled with `t.HorizontalRule`). The full URL
of the cursor row is wrapped via `wrap` to box content width.
If the wrapped result exceeds 2 rows, the second row is truncated
with `…`. (Wrap, then count rows, then truncate the second row if
needed.)

Box height:

```
top-border (1)
+ visible list rows (min(len(links), terminalHeight - 7))
+ rule (1)
+ preview rows (2)
+ bottom-border (1)
```

= `5 + visibleRows`. Visible rows bounded by terminal height
minus chrome.

Internal scroll: when the list overflows the visible window,
cursor movement past the window scrolls `offset` so the cursor
stays in view. No scrollbar — the list is short by nature
(harvested URLs in a single message). 1-line scroll on each
boundary cross.

Box title: `┌─ Links ─` ... `─┐` — same style as the help popover
top border.

### App.View composition

When `linkPicker.IsOpen()`:

```go
frame := /* compose the underlying frame (chrome + acct + status) as today */
dimmed := DimANSI(frame)
box := m.linkPicker.Box(m.width, m.height)
return PlaceOverlay(
    m.linkPicker.Position(box, m.width, m.height),
    dimmed, box,
)
```

Identical flow to help — only the box generator differs.

### Help vocabulary

Per ADR-0072, every key the picker exposes appears in the viewer
help popover with a wired/unwired flag. New row in `viewerGroups`:

```go
{Key: "Tab", Desc: "open link picker", Wired: true}
```

Picker's internal keys (`j/k`, `Enter`, `Esc`) don't get separate
rows — picker is transient and its keys are conventional list-nav
keys advertised globally.

### Tests

New file `internal/ui/linkpicker_test.go`:

- `TestLinkPickerOpenWithEmptyListNoOp` — Tab in viewer with
  zero harvested URLs returns no Cmd.
- `TestLinkPickerOpenSetsCursor` — Open(links) sets cursor=0,
  offset=0.
- `TestLinkPickerCursorBounds` — `j` past last row inert; `k` past
  first row inert.
- `TestLinkPickerEnterLaunches` — Enter on cursor row emits
  `LaunchURLMsg{links[cursor]}` + `LinkPickerClosedMsg`.
- `TestLinkPickerNumericLaunches` — `1`–`9` emit launch + close
  for in-range indices, inert for out-of-range.
- `TestLinkPickerEscCloses` / `TestLinkPickerTabCloses` — emit
  `LinkPickerClosedMsg` only.
- `TestLinkPickerQSwallowed` — `q` produces no Cmds, picker stays
  open.
- `TestLinkPickerScroll` — open with 30 links, `j` past visible
  window scrolls offset.
- `TestLinkPickerRowFormat` — visual: list of 12 URLs renders with
  ` [1]` … `[12]` (leading-space padding on single-digit indices).
- `TestLinkPickerPreviewWraps` — long URL on cursor row wraps in
  preview footer; URL exceeding 2 rows truncated with `…` on row 2.

App-level integration (`app_test.go`):

- `TestAppLinkPickerRoundTrip` — open viewer → `Tab` → assert
  `linkPicker.IsOpen()` → `Enter` → assert `openURL` hook called
  with correct URL → assert picker closed.

### Conventions checklist

Per `bubbletea-conventions.md` §10:

- Width math: `displayCells` / `displayTruncate` only; no `len()`
  on rendered strings.
- State only in `LinkPicker` struct; mutations only in
  `LinkPicker.Update` and `LinkPicker.Open` / `.Close` /
  `.SetSize`.
- No I/O in `View()`. No state mutation in tea.Cmd closures.
- Renderer honors width: list rows truncated via `displayTruncate`,
  preview wrapped via `wrap` (wordwrap + hardwrap).
- No defensive parent-side clipping in App.View.
- Keys declared as `key.Binding` in `LinkPickerKeys`; dispatched
  via `key.Matches`.
- `WindowSizeMsg`: App.Update calls `linkPicker.SetSize(w, h)`
  alongside the existing children.
- No deprecated APIs (`HighPerformanceRendering` etc.).

## ADRs to write at pass end

1. **Long bare URL footnoting** — > 30 cell threshold, `trimURL`
   rule, `…` only on actual trim, dedupe with text-bearing links.
2. **`n`/`N` viewer navigation semantics** — visible-row coupling,
   boundary inert, loading-phase inert, optimistic mark-seen
   reuse.
3. **Link picker overlay** — modal launched by `Tab`, hand-rolled
   (deviation from `bubbles/list` justified), key vocabulary,
   index-column right-alignment with leading-space pad, 50-cell
   inline URL truncation, 2-row preview footer with full-URL
   wrap+truncate.

## Pass-end ritual

Standard: simplify, conventions checklist, ADRs, invariants
update, STATUS bump, plan + spec archive, `make check`, commit,
push, install. Live tmux capture of the link picker at 120×40 and
at the minimum viable popover width to confirm layout.
