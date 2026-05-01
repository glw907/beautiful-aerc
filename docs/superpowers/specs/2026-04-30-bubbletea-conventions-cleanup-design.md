---
title: Pass 5 — Bubbletea conventions cleanup
date: 2026-04-30
pass: 5
addresses:
  - audit/2026-04-26-bubbletea-conventions.md A3 (#17)
  - audit/2026-04-26-bubbletea-conventions.md A9 (#18)
  - audit/2026-04-26-bubbletea-conventions.md A10 (#19)
---

# Bubbletea Conventions Cleanup

Pay down the structural debt called out by the Pass 4 audit. Three
changes in `internal/ui/`, all mechanical or near-mechanical: nothing
about the user-visible behavior changes.

## Goals

1. Every key dispatch in `internal/ui/` goes through `key.Matches`
   against a `key.Binding` field. No `msg.String()` switches survive
   in `app.go`, `account_tab.go`, or `viewer.go`.
2. Parent-child signaling no longer round-trips through zero-latency
   `tea.Cmd` wrappers. After delegating a message to AccountTab, App
   reads the child's new state synchronously via accessors and
   rebuilds its own chrome inline.
3. `AccountTab.View()` honors its assigned width strictly. `App.View`
   appends the right border without per-line measurement or padding.

## Non-goals

- No new components, no new keybindings, no behavior changes.
- No help-popover wiring of new keys (that ladder lives in later
  passes; this pass only ensures the binding declarations are in
  place to wire to).
- No backend changes. No mail-stack changes.
- No reshuffling of which component owns which state. The
  delegation-vs-Cmd change is purely about the *transport* between
  child and parent, not the *ownership* of state.

## Background

ADR-0080 anchors poplar to research-grounded idiomatic bubbletea.
The Pass 4 audit (`docs/poplar/audits/2026-04-26-bubbletea-conventions.md`)
flagged three structural deviations from that anchor. A3, A9, and
A10 were all backlogged at the time as a dedicated structural-cleanup
pass to keep Pass 4 scoped. This is that pass.

A3: every dispatch in `app.go`, `account_tab.go`, `viewer.go` uses
raw `msg.String()` switches. `GlobalKeys` is declared but never
matched against. The bindings are decorative.

A9: `AccountTab` and `Viewer` signal upward via `tea.Cmd` closures
that return typed messages — `FolderChangedMsg`, `ViewerOpenedMsg`,
`ViewerClosedMsg`, `ViewerScrollMsg`, `LinkPickerOpenMsg`,
`LinkPickerClosedMsg`. Each round-trips through the bubbletea event
loop with no asynchrony, no batching, and no I/O. The framing is
off-norm: state changes within one model tree should be expressed
as direct reads after delegation, not as messages.

A10: `App.View` performs parent-side per-line layout adjustment on
`AccountTab.View`'s output — measuring each line, padding to
`width-1`, then appending the right border. The right edge being
the chrome's responsibility is fine, but the per-line measurement
indicates `AccountTab` isn't honoring its width contract.

## Architecture

### Key bindings

Add `AccountKeys` and `ViewerKeys` to `internal/ui/keys.go`,
following the same shape as the existing `GlobalKeys`.

```go
type AccountKeys struct {
    OpenSearch     key.Binding  // "/"
    ClearSearch    key.Binding  // "esc" (when search is active)
    OpenMessage    key.Binding  // "enter"
    SidebarDown    key.Binding  // "J"
    SidebarUp      key.Binding  // "K"
    JumpInbox      key.Binding  // "I"
    JumpDrafts     key.Binding  // "D"
    JumpSent       key.Binding  // "S"
    JumpArchive    key.Binding  // "A"
    JumpSpam       key.Binding  // "X"
    JumpTrash      key.Binding  // "T"
    MsgListTop     key.Binding  // "g"
    MsgListBottom  key.Binding  // "G"
    MsgListDown    key.Binding  // "j", "down"
    MsgListUp      key.Binding  // "k", "up"
    ToggleFold     key.Binding  // " "
    ToggleFoldAll  key.Binding  // "F"
    NextMessage    key.Binding  // "n" (viewer-open only)
    PrevMessage    key.Binding  // "N" (viewer-open only)
}

type ViewerKeys struct {
    Close       key.Binding  // "q", "esc"
    OpenPicker  key.Binding  // "tab"
    BodyTop     key.Binding  // "g"
    BodyBottom  key.Binding  // "G"
    Link1       key.Binding  // "1"
    // ... Link2 through Link9
}
```

The viewport's own `j/k/space/b` keymap stays inside the viewport's
config — those keys are not part of `ViewerKeys` because the
component delegates them through the bubbles `viewport.Update`
path, which already uses `key.Matches` against `viewport.KeyMap`.

`g`/`G` are duplicated between `AccountKeys.MsgListTop`/`Bottom` and
`ViewerKeys.BodyTop`/`Bottom`. This duplication is intentional: each
binding's `WithHelp` text differs ("top of list" vs "top of body"),
and ADR-0072's wired-flag rendering needs distinct help strings.
Sharing one binding would force one help string and lose information
at the popover.

`AccountKeys.NextMessage`/`PrevMessage` live on AccountKeys (not
ViewerKeys) because AccountTab is the dispatch site — it consumes
the keypress when the viewer is open and translates it into a
message-list cursor move plus a fresh viewer load. The viewer never
sees `n`/`N`.

`q` is on `GlobalKeys.Quit` and stays there. The context-sensitivity
("`q` exits the viewer when the viewer is open") is implemented in
`App.Update` by inspecting child state after match — not by giving
`q` multiple binding values.

Constructors: `NewAccountKeys()` and `NewViewerKeys()` return the
default bindings, mirroring `NewGlobalKeys`.

### Delegation accessors

Add the following methods to `AccountTab`. All are pure reads of
existing state — no caching, no derived computation that isn't
already done.

- `ViewerOpen() bool` — returns `m.viewer.IsOpen()`.
- `SelectedFolderCounts() (exists, unseen int)` — returns the
  current counts for the selected folder; same source the
  `FolderChangedMsg` payload reads from today.
- `ViewerScrollPct() float64` — returns `m.viewer.ScrollPct()` if
  the viewer is open, else 0.
- `SearchState() SearchState` — returns `m.sidebarSearch.State()`.
  Replaces the `m.acct.sidebarSearch.State()` reach-in at
  `app.go:155`.
- `LinkPickerRequest() ([]content.Link, bool)` — one-shot.
  Returns `(links, true)` if the viewer's last keypress requested
  the link picker; `(nil, false)` otherwise. Reading clears the
  request.

`WindowCounter()` already exists and is preserved.

The link-picker request is the one piece of transient state added
by this pass. The pattern mirrors the App↔AccountTab relationship
one level down: the **Viewer** carries an unexported field
`pendingLinkPicker []content.Link`. Viewer's handleKey, on matching
`Tab` with non-empty links, sets the field on `v` before returning.
AccountTab.handleKey, after delegating to `v.Update`, reads
`v.LinkPickerRequest()` (a one-shot accessor on Viewer that clears
the field on read) and stores the value in its own
`pendingLinkPicker` field. App.Update then reads
`m.acct.LinkPickerRequest()` after its own delegation.

Two layers of "child mutates own state, parent reads after
delegation" — no Cmd round-trip, no callbacks, no parent pointers.
Tests assert that the accessor clears on read at both layers.

Trade-off note: the alternative is keeping `LinkPickerOpenMsg` as
the single Cmd-wrapped signal that survives. That's defensible —
the link picker is App-owned overlay state, so the message is
crossing a real ownership boundary, not just a parent-child
boundary inside one model tree. If the implementation reveals that
the pending-field approach is awkward, fall back to keeping
`LinkPickerOpenMsg` and mark the deviation in the pass-end ADR.

### App.Update rewrite

Today, App.Update has separate `case` arms for
`FolderChangedMsg`, `ViewerOpenedMsg`, `ViewerClosedMsg`,
`ViewerScrollMsg`, `LinkPickerOpenMsg`, `LinkPickerClosedMsg`. Each
reads payload data and updates `m.statusBar`, `m.footer`,
`m.viewerOpen`, `m.linkPicker` accordingly.

After Pass 5: those `case` arms are deleted. The single
`tea.KeyMsg` arm (and the `tea.WindowSizeMsg` arm where relevant)
delegates to AccountTab and then re-derives the chrome from the
new child state:

```go
m.acct, cmd = m.acct.Update(msg)
m.viewerOpen = m.acct.ViewerOpen()
exists, unseen := m.acct.SelectedFolderCounts()
m.statusBar = m.statusBar.SetCounts(exists, unseen)
if m.viewerOpen {
    m.footer = m.footer.SetContext(ViewerContext)
    m.statusBar = m.statusBar.SetMode(StatusViewer).
        SetScrollPct(m.acct.ViewerScrollPct())
} else {
    m.footer = m.footer.SetContext(AccountContext)
    m.statusBar = m.statusBar.SetMode(StatusAccount)
}
if links, ok := m.acct.LinkPickerRequest(); ok {
    m.linkPicker = m.linkPicker.Open(links)
}
```

This block runs after every delegation. The chrome is always
consistent with child state because it's derived, not signaled.

`backendUpdateMsg` and `ErrorMsg` continue to flow through their
own `case` arms — those are real external signals, not intra-model
parent-child wiring. `LinkPickerClosedMsg` is consumed by
`linkPicker.Update` itself; nothing crosses the App boundary.

### Width contract

`AccountTab.View()` must produce lines that are exactly its
assigned width in cells (not bytes; not visual + ANSI; not
padded-most-of-the-time). The plumbing already exists: App passes
`tea.WindowSizeMsg{Width: m.width - 1, Height: m.contentHeight()}`
into the child. The child must honor it for every line of every
View output, including:

- the loading placeholder spinner row,
- the error-banner row when present,
- the search shelf (3-row block at the bottom of the sidebar
  column),
- the status / footer composition done elsewhere (already honored).

Every renderer that produces a row pads with `lipgloss.PlaceHorizontal`
or equivalent to the assigned width. Truncation of icon-bearing
strings goes through `displayTruncate` per ADR-0084.

After this contract holds, `App.View` becomes:

```go
rawContent := m.acct.View()
lines := strings.Split(rawContent, "\n")
for i, line := range lines {
    lines[i] = line + borderChar
}
return strings.Join(lines, "\n")
```

No measuring, no padding. As a transitional safety net the first
implementation may keep an `ansi.Truncate(line, width-1)` guard
inside the loop with a `// TODO(pass5): remove once contract holds`
comment, then strip it once the tmux capture verifies clean
behavior.

## Commit shape

Three commits, in this order. Each leaves the tree green and
behaviour identical.

### Commit 1 — `#17` `key.Matches` migration

- Add `AccountKeys`, `ViewerKeys` to `keys.go`, with constructors.
- Wire them into `App` (already holds `GlobalKeys`), `AccountTab`,
  `Viewer`. Threaded as part of each model's struct, constructed
  in the existing `New*` constructors.
- Convert `App.Update`'s residual string compares (the search-
  active branch at `app.go:155` and the `ctrl+c`/`q` mix) to
  `key.Matches`.
- Convert `AccountTab.handleKey` (every `case` in the search-idle
  switch and the viewer-open `n`/`N` branch) to `key.Matches`.
- Convert `Viewer.handleKey` (q/esc, tab, 1-9 via a digit helper,
  g/G) to `key.Matches`.
- The `1`–`9` link bindings get their own helper because `key.Matches`
  doesn't match a parsed-int form. Either nine fields with one
  binding each, or one helper `linkIndexFromKey(msg, vk) (int, bool)`
  that walks `Link1`-`Link9`. The helper form is preferable.
- Tests: existing dispatch tests are rewritten to construct
  `tea.KeyMsg` values and assert on resulting model state. No
  test asserts on `msg.String()` paths.

### Commit 2 — `#18` direct delegation

- Add the accessors listed above to AccountTab.
- Rewrite `App.Update` so that `tea.KeyMsg` delegation always
  flows through the delegate-then-read shape sketched above.
- Delete `FolderChangedMsg`, `ViewerOpenedMsg`, `ViewerClosedMsg`,
  `ViewerScrollMsg` from `cmds.go`, and remove the `*Cmd`
  constructors that emit them. Their call sites inside child
  models are removed; no replacement is needed because the parent
  reads the new state directly.
- For `LinkPickerOpenMsg`: implement the pending-field approach
  in AccountTab, with a fallback to keeping the message wired if
  the field approach proves awkward. Decision recorded in the
  pass-end ADR.
- Update all tests that asserted on Cmd round-trips. Tests of
  AccountTab dispatch now construct a `tea.KeyMsg`, call Update,
  and assert on the resulting model state via accessors. Tests
  of App rebuild now run a delegation and assert that
  `m.statusBar`, `m.footer`, `m.viewerOpen` have the expected
  values.

### Commit 3 — `#19` App.View trust

- Audit every `View()` method below `AccountTab` for strict width
  honoring. Identify the actual sites (loading row, error banner
  row, search shelf, status/footer interplay).
- Pad each non-conforming row with `lipgloss.PlaceHorizontal` (or
  `displayCells`-aware equivalent for icon-bearing rows).
- Strip the per-line padding loop from `App.View`. Append the
  border char with a plain `+`.
- Capture a tmux render at 120×40 and at the minimum-viable width
  (currently 80×24 per the conventions doc). Save the captures
  to `docs/poplar/testing/icon-modes.md`-adjacent location.

## Testing

- `make check` (vet + test) passes after each of the three
  commits. Each commit is individually revertable.
- Unit tests in `*_test.go` cover the new key dispatch shape, the
  new accessors, and the delegate-then-read flow.
- Live tmux capture verifies the width contract and proves the
  per-line padding loop is not load-bearing. Captures saved
  alongside existing test artefacts.
- Pass-end §10 conventions checklist run against the cumulative
  diff of the pass.

## Risk and rollback

- Commit 1 is mechanical and individually revertable. Risk: a
  binding's `WithKeys` set differs from the prior `case` set —
  caught by tests.
- Commit 2 is the riskiest because it touches the wiring between
  child and parent. If a regression slips through (e.g., a chrome
  field stays stale because App forgot to re-read after one
  delegation path), reverting commit 2 alone restores the
  Cmd-wrapped signals.
- Commit 3 is small and visible. Regression manifests as visible
  layout breakage and is caught by the tmux capture.

## ADR plan

One ADR will be written at pass end:

- **Per-component KeyMap structs and direct delegation.** Records
  the (A + i) decision from the brainstorm, the accessor surface
  on AccountTab, the deletion of the Cmd-wrapped signal types,
  and the resolution of the LinkPickerOpenMsg fork. Updates
  invariants.md to reflect the new dispatch and signaling
  conventions in `internal/ui/`.

If the LinkPicker pending-field approach is rejected during
implementation, the ADR documents the deviation and the residual
`LinkPickerOpenMsg` is kept with a written rationale.
