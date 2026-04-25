# Pass 2.5b-4 — Message Viewer Prototype

Design spec for the viewer prototype pass. Wireframe reference:
`docs/poplar/wireframes.md` §4 (Viewer). ADR precedents: 0025
(three-sided frame), 0022 (per-screen prototype passes).

## Goal

Open a message in the right panel when the user presses `Enter` on
the message list. Render header block + body via the existing
`content.ParseBlocks` / `content.RenderBody` pipeline. Return to the
list on `q` / `esc`. The sidebar and chrome stay visible throughout
— no full-screen takeover.

This pass is a prototype on the mock backend. Pass 3 replaces the
synthesized headers with real backend addresses and wires live JMAP
/ IMAP body fetch.

## Scope

In scope:

- New `Viewer` tea.Model in `internal/ui/viewer.go`.
- `AccountTab` integration (state, key routing, open/close flow).
- Footer context switch via `ViewerOpenedMsg` / `ViewerClosedMsg`.
- Status-bar mode switch (scroll % replaces message counts).
- Body fetch `tea.Cmd` that parses blocks and delivers a
  `bodyLoadedMsg`.
- Loading placeholder using `bubbles/spinner`.
- Mark-read flow (optimistic local flip + fire-and-forget Cmd).
- Mock backend enrichment: per-UID realistic markdown bodies
  chosen to stress the wrap path.
- Footnote harvesting: inline `[^N]` markers + bottom list, dedupe
  URLs, skip auto-linked bare URLs, styled via `t.Link`.
- Quick-open link keys `1`-`9` (fire `xdg-open`, fire-and-forget).
- Body width cap correction: `maxBodyWidth` 78 → 72.
- Keybinding cleanup: remove `ctrl+d/u/f/b` from msglist and
  wireframes to enforce the modifier-free rule.
- Header address-unit atomic wrap regression test.
- Adversarial wrap stress tests in `content/render_test.go`.
- Wireframes (§4, §5) and `docs/poplar/keybindings.md` updated to
  match the new bindings.

Out of scope (deferred):

- Link picker modal (Pass 2.5b-4b). `Tab` is inert this pass.
- Triage actions (`d/a/s/.`) — Pass 6. Footer still advertises.
- Reply / compose (`r/R/f/c`) — Pass 9. Footer still advertises.
- Help popover — Pass 2.5b-5. `?` is still a stub.
- Status toast / error surfacing — Pass 2.5b-6. Body fetch errors
  drop silently in this pass.
- Viewer `n/N` walks filtered row set — backlog #9, bundled with
  Pass 3 (prefetch semantics only matter with real latency).
- Real `ParsedHeaders` from backend (full addresses, Cc/Bcc, etc.)
  — Pass 3, when `Backend.FetchHeaders(uid)` gets extended.

## Architecture

### Viewer tea.Model

```go
type viewerPhase int

const (
    viewerClosed viewerPhase = iota
    viewerLoading
    viewerReady
)

type Viewer struct {
    open     bool
    phase    viewerPhase
    msg      mail.MessageInfo
    blocks   []content.Block
    headers  content.ParsedHeaders
    links    []string // harvested URLs in first-seen order, deduped
    viewport viewport.Model
    spinner  spinner.Model
    styles   Styles
    theme    *theme.CompiledTheme
    width    int
    height   int
}
```

`Viewer` holds no `mail.Backend` reference. All body-fetch and
mark-read Cmds are constructed at the `AccountTab` level (which
already owns the backend). The viewer is pure state + rendering.

### State machine

```
closed ──Enter on msglist──► loading (spinner, body-fetch Cmd in flight)
loading ──bodyLoadedMsg──► ready (viewport populated, scroll % = 0%)
ready ──q/esc──► closed
loading ──q/esc──► closed (stale bodyLoadedMsg dropped via UID guard)
```

A stale `bodyLoadedMsg` (user closed and reopened on a different
UID before the Cmd resolved) is recognized by comparing
`msg.UID` in `AccountTab.Update` against `m.viewer.CurrentUID()`.
Mismatch → drop silently.

### AccountTab integration

- New field: `viewer Viewer` (held by value).
- `AccountTab.View` renders `viewer.View()` in place of
  `msglist.View()` when `m.viewer.IsOpen()` is true.
- `AccountTab.handleKey` checks `m.viewer.IsOpen()` first; when
  open, every key goes to the viewer handler.
- `AccountTab.updateTab` adds a `bodyLoadedMsg` case that calls
  `m.viewer.SetBody(msg.blocks)` iff UID matches.

### Cmds

```go
type bodyLoadedMsg struct {
    uid    mail.UID
    blocks []content.Block
}

func loadBodyCmd(b mail.Backend, uid mail.UID) tea.Cmd {
    return func() tea.Msg {
        r, err := b.FetchBody(uid)
        if err != nil {
            return backendErrMsg{err: err}
        }
        buf, err := io.ReadAll(r)
        if err != nil {
            return backendErrMsg{err: err}
        }
        return bodyLoadedMsg{uid: uid, blocks: content.ParseBlocks(string(buf))}
    }
}

func markReadCmd(b mail.Backend, uid mail.UID) tea.Cmd {
    return func() tea.Msg {
        if err := b.MarkRead([]mail.UID{uid}); err != nil {
            return backendErrMsg{err: err}
        }
        return nil
    }
}

func launchURLCmd(url string) tea.Cmd {
    return func() tea.Msg {
        // Fire-and-forget; errors drop silently this pass.
        _ = exec.Command("xdg-open", url).Start()
        return nil
    }
}
```

### Enter handler

```go
case "enter":
    if m.viewer.IsOpen() { return m, nil } // shouldn't happen; viewer consumes enter
    msg, ok := m.msglist.SelectedMessage()
    if !ok { return m, nil }
    m.viewer = m.viewer.Open(msg) // state transition into loading
    var cmds []tea.Cmd
    cmds = append(cmds, loadBodyCmd(m.backend, msg.UID))
    cmds = append(cmds, viewerOpenedCmd())
    if msg.Flags&mail.FlagSeen == 0 {
        m.msglist.MarkSeen(msg.UID)
        cmds = append(cmds, markReadCmd(m.backend, msg.UID))
    }
    cmds = append(cmds, m.viewer.spinner.Tick)
    return m, tea.Batch(cmds...)
```

### Viewer key routing

| Key | Action |
|---|---|
| `j/k` | viewport line up/down |
| `g/G` | top / bottom |
| `space` | page down |
| `b` | page up |
| `q`, `esc` | close viewer |
| `1`-`9` | launch link N via `xdg-open` (no-op if out of range) |
| `Tab` | no-op (Pass 2.5b-4b) |
| `?` | pass through for help (Pass 2.5b-5) |
| anything else | consumed, no-op |

Triage/reply keys are inert this pass but **not** passed through —
they land in Pass 6/9. Search shelf keys (`/`, mode cycle) are
inert while viewer is open. Folder jumps (`I/D/S/A/X/T`, `J/K`)
are inert while viewer is open.

### Msglist key cleanup (also in scope)

Remove `ctrl+d`, `ctrl+u`, `ctrl+f`, `ctrl+b`, `pgdown`, `pgup`
from `AccountTab.handleKey`. Msglist navigation is `j/k/g/G`
(plus `J/K` for the sidebar). The no-modifier rule from the
keybindings feedback memory applies uniformly across contexts.
Footer drop-rank tables in `footer.go` updated to remove any
references to Ctrl-bound hints.

### Footer context switch

`AccountTab` emits `ViewerOpenedMsg` and `ViewerClosedMsg` Cmds on
open/close. `App.Update` catches them and calls
`m.footer = m.footer.SetContext(ViewerContext | AccountContext)`.

### Status bar mode switch

`StatusBar` gains:

```go
type StatusMode int
const (
    StatusAccount StatusMode = iota
    StatusViewer
)
func (s StatusBar) SetMode(mode StatusMode) StatusBar
func (s StatusBar) SetScrollPct(pct int) StatusBar
```

`View` switches between counts-mode (existing) and viewer-mode
(`NN% · ● connected`). Viewer emits `ViewerScrollMsg{pct}` on
every scroll (pct recomputed from `viewport.YOffset`); App updates
the status bar.

### View composition (ready phase)

```
┌─ headers   (N rows, contentWidth, not capped)
├─ rule ─    (1 row, contentWidth, t.HorizontalRule)
└─ body      (fill rows, min(contentWidth, 72) wrap cap, viewport-scrolled)
```

- `contentWidth = width - sidebarWidth - 1` (divider).
- `headerWidth = contentWidth` — `RenderHeaders(h, t, contentWidth)`.
- `bodyWidth = contentWidth` — `RenderBody` applies its own 72 cap
  internally.
- `bodyHeight = height - headerHeight - 1` (rule line).
- On `tea.WindowSizeMsg` viewer re-renders headers to re-measure,
  then recomputes body height and `viewport.SetContent`.

Headers are synthesized from `msg` + `backend.AccountName()`:

```go
hdrs := content.ParsedHeaders{
    From:    []content.Address{{Name: msg.From}},
    To:      []content.Address{{Email: accountName}},
    Date:    msg.Date,
    Subject: msg.Subject,
}
```

Pass 3 replaces this with a backend-sourced `ParsedHeaders` once
`FetchHeaders` is extended to return addresses.

### View composition (loading phase)

Centered `spinner.View() + " Loading message…"` in `Styles.Dim`,
horizontally and vertically centered across the full content area
(no header region rendered — header heights are unknown until
`bodyLoadedMsg` arrives). Matches wireframes §6 "Viewer (fetching
body)".

### View composition (closed phase)

`View()` returns `""`. `AccountTab.View` checks `IsOpen()` and
renders `msglist.View()` instead; the viewer never draws a blank
frame.

## Content rendering

### Body width correction

`internal/content/render.go`:

```go
const maxBodyWidth = 72 // was 78, corrected to match mailrender baseline
```

Existing `render_test.go` assertions that expected 78 are updated.

### Footnote harvesting

New function in `internal/content/render.go` (or a sibling file):

```go
func RenderBodyWithFootnotes(blocks []Block, t *theme.CompiledTheme, width int) (string, []string)
```

Returns the rendered body and the ordered list of harvested URLs
(indexed 1..N from the viewer's perspective). The second return
feeds the `1`-`9` quick-open keys.

Behavior:

1. Walk blocks/spans once. For each `Link` span:
   - If `span.Text == span.URL` → **skip** (auto-linked bare URL;
     render inline as the URL in link style, no marker).
   - If `span.URL` already seen → reuse its existing footnote
     number.
   - Otherwise append to URL list, assign next number.
2. In-place rewrite of the span's rendered output: the last word
   of the link text gets glued to `[^N]` with `\u00a0` (no-break
   space). Earlier words use normal spaces. Entire unit — text +
   marker — rendered through `t.Link`. `ansi.Wordwrap` treats
   `\u00a0` as non-breaking, so `lastWord[^N]` stays atomic.
3. After rendering all blocks, if `len(urls) > 0` append a
   horizontal `Rule` then one line per URL: `t.Link.Render("[^N]: ") + t.Link.Render(url)`.

### Header wrap rule

`RenderHeaders(h, t, width)` already takes width as an argument and
doesn't use `maxBodyWidth`. Viewer passes `contentWidth`, so
headers wrap at the full content column width, not 72.

`renderHeaderAddresses` already wraps between addresses, never
inside a `Name <email>` unit. A regression test in
`headers_test.go` locks this property in.

## Mock backend enrichment

`internal/mail/mock.go` adds a package-level
`map[mail.UID]string` of realistic markdown bodies. `FetchBody`
looks up the UID and returns the matching string, falling back to
a short default. Body content chosen to exercise:

- Long paragraph with inline `**bold**`, `*italic*`, `` `code` ``
  spans straddling column 72.
- Multiple reply levels (`> > `) to exercise nested quote wrap.
- `QuoteAttribution` ("On ... wrote:") followed by unquoted
  content to exercise `wrapImpliedQuotes`.
- Long list item that wraps to exercise hanging indent.
- Code block wider than 72 (should render verbatim, no wrap).
- Inline links mixed with prose — some bare URLs (should skip
  footnote), some `[text](url)` form (should footnote).
- URL longer than 72 chars that must not break mid-token.
- A line where a `[^1]` marker lands exactly at col 72.
- At least one message with **no** links (footnote list absent).
- One trivial one-line body.

At least 6 distinct sample bodies. Mapped onto the existing UIDs
so the first few rows of the inbox showcase the variety.

## Testing

### Unit tests

`internal/content/render_test.go` — new cases:

- `TestRenderBodyWidthCap` — update expected from 78 to 72.
- `TestRenderBodyWrapStressParagraph` — long paragraph with
  styled spans, assert `max(lipgloss.Width(line)) <= 72`.
- `TestRenderBodyLongURL` — URL > 72 chars, assert URL stays
  atomic.
- `TestRenderBodyNestedQuoteWrap` — deep quote, inner wrap at 68.
- `TestRenderBodyListHangingIndent` — long list item, second
  visible line starts at hanging indent.
- `TestRenderBodyFootnoteEdge` — content ending at col 72 with
  next token `[^1]`; marker joins last word via `\u00a0`, no
  orphan.

`internal/content/render_footnote_test.go` — new file:

- `TestFootnoteHarvestBasic` — two distinct links → `[^1]`, `[^2]`
  inline and in list.
- `TestFootnoteDedupe` — same URL twice → only `[^1]`, one entry.
- `TestFootnoteSkipAutoLinked` — bare URL → no marker.
- `TestFootnoteLastWordAtomic` — link text wraps; last word +
  marker stay together via `\u00a0`.

`internal/content/headers_test.go` — new case:

- `TestRenderHeadersAddressUnitAtomic` — long `Name <email>`
  alongside a second address, assert unit never splits.

`internal/ui/viewer_test.go` — new file:

- `TestViewerOpenTransitionsToLoading`
- `TestViewerBodyLoadedSetsReady`
- `TestViewerStaleBodyLoadedIgnored`
- `TestViewerQClosesFromAnyPhase`
- `TestViewerScrollEmitsScrollMsg`
- `TestViewerNumericLaunchesURL` (with mock opener hook)
- `TestViewerNumericNoOpOutOfRange`

`internal/ui/account_tab_test.go` — extensions:

- `TestEnterOpensViewer`
- `TestEnterMarksRead`
- `TestEnterEmptyFolderNoOp`
- `TestSearchKeysInertWhileViewerOpen`
- `TestFolderJumpInertWhileViewerOpen`

`internal/ui/app_test.go` — extensions:

- `TestViewerOpenedSwitchesFooterContext`
- `TestViewerClosedRestoresFooterContext`
- `TestViewerScrollUpdatesStatusBar`

### Live tmux verification

End-of-pass ritual before commit:

1. `make install`.
2. `tmux new-session -d -s poplar-verify` sized 120×40.
3. Capture inbox; diff against wireframe §1 visually.
4. Open each stress-test mock message with `Enter`, capture,
   verify: no overflow, headers correct, footnotes correct,
   markers never orphaned.
5. Press `1` on a message with a link; verify `xdg-open` fires.
6. Press `q`, capture, verify list returns with read-state flip.
7. Resize tmux window narrower; confirm viewer re-wraps body at
   `min(width, 72)` and headers at `width`.

### Success criteria

- `make check` green.
- Every stress-test mock message renders with zero overflow at
  72 cells.
- Quick-open `1`-`9` works; `Tab` inert but advertised.
- Mark-read flip visible on return to list.
- Search filter survives viewer round-trip (open + close with
  filter committed → filter still active, same cursor row).
- No `Ctrl+`-modifier bindings anywhere in `internal/ui/`.

## Consequences

- Viewer establishes the pattern for subsequent modal/overlay
  prototypes (help popover, folder picker, link picker): a
  sub-model owned by `AccountTab`, keys routed viewer-first when
  open, context-bubbled Cmds for chrome (footer, status bar).
- `maxBodyWidth` correction (78 → 72) affects every future content
  render. Any existing consumer that expected 78 is fixed in this
  pass's test updates.
- Modifier-free keybinding rule is now enforced repo-wide.
  `ctrl+d/u/f/b` gone from msglist; no new `Ctrl+` introduced.
  Wireframes and keybindings doc brought in line.
- Mock backend now carries realistic content. Downstream prototype
  passes (viewer, picker, help popover) can rely on this content
  existing rather than re-adding ad-hoc test data.
- `1`-`9` link launch via `xdg-open` ships a user-facing feature
  ahead of the picker modal. The picker (2.5b-4b) becomes purely
  a discoverability / >9-coverage affordance.

## References

- Wireframes §4 (viewer), §6 (loading spinner), §7 (msglist
  interaction) — `docs/poplar/wireframes.md`.
- Keybindings — `docs/poplar/keybindings.md` (updated in this pass).
- ADR 0022 — per-screen prototype passes.
- ADR 0025 — three-sided frame, sidebar always visible.
- Invariants — `docs/poplar/invariants.md` (elm architecture, no
  multi-key sequences, modifier-free rule via memory).
- Backlog #9 — viewer `n/N` filtered navigation, bundled with
  Pass 3.
