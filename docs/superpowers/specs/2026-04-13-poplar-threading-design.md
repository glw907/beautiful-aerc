# Poplar Threading + Fold Design

**Pass:** 2.5b-3.6 (prototype: threading + fold)
**Date:** 2026-04-13
**Status:** approved, ready for implementation plan

## Goal

Complete the message list prototype with threaded display, per-thread
fold state, and bulk fold/unfold. After this pass, the index view
matches wireframes §3 (default threaded rendering) and §7 #14
(expanded, collapsed, and partially collapsed states), and the
`Space`/`F`/`U` keys move from "reserved" to "live" in the keybindings
doc and command footer.

## Settled (inherited)

- Threading is default-on globally, per-folder override (ADR 0045).
- `Space` is the thread-fold toggle outside visual-select mode; row
  toggle inside it (ADR 0052). `F`/`U` are bulk fold/unfold (ADR
  0053). No runtime threading toggle (ADR 0054).
- Thread prefixes render as `├─ └─ │` in `fg_dim` (wireframe §3).
- Per-folder `[ui.folders.<name>] threading` and `sort` config fields
  already parsed by `internal/config/ui.go` (Pass 2.5b-3.5) but
  currently unused. This pass makes them load-bearing.

## Architecture

Threading lives in two layers.

### Wire layer — `internal/mail/types.go`

`MessageInfo` gains two fields:

```go
type MessageInfo struct {
    UID       UID
    Subject   string
    From      string
    Date      string
    Flags     Flag
    Size      uint32

    ThreadID  UID // shared by all messages in a conversation
    InReplyTo UID // empty for thread roots
}
```

`ThreadID` and `InReplyTo` come straight from `Email.threadId` /
`Email.inReplyTo` on JMAP and from the `THREAD` extension on IMAP.
Depth is *not* a wire field — it's derived by the UI during the
build pipeline from the actual tree shape, alongside the prefix
string. Carrying depth on the wire would duplicate information the
prefix walk already produces and would risk drift if a backend
miscounted under an unusual reference chain.

A non-threaded message — i.e., a flat conversation of one — has
`ThreadID == UID` and `InReplyTo == ""`. This makes every message a
thread of size 1 by default, so the renderer doesn't need a special
"no thread" code path.

### Display layer — `internal/ui/msglist.go`

`MessageList` owns grouping, sorting, fold state, and rendering. The
key idea: the thread tree never exists as an owned struct. The flat
input slice is grouped + sorted + flattened into a private
`[]displayRow` on every `SetMessages` and on every fold mutation. This
is the **Camp 2 / Thunderbird-style** approach, chosen because:

- JMAP delivers `threadId` on the wire and `Email/changes` returns
  bounded deltas. Per-update flat re-flatten is O(deltas), not O(n).
- Bubbletea's render loop is fundamentally "iterate visible rows,
  produce strings." A flat slice fits the existing
  `selected int`/`offset int`/`clampOffset()` machinery directly.
- Aerc's tree-mutation approach is optimized for IMAP IDLE pushes
  on huge folders — a problem JMAP solves at the protocol layer.

`displayRow`:

```go
type displayRow struct {
    msg          mail.MessageInfo
    prefix       string // "", "├─ ", "└─ ", "│  └─ ", etc.
    isThreadRoot bool
    threadSize   int    // set on roots only; 1 for unthreaded
    hidden       bool   // true when collapsed under a folded root
    depth        uint8  // derived during prefix computation; 0 = root
}
```

Fold state:

```go
folded map[mail.UID]bool // keyed by thread root UID
```

Per-`MessageList`, reset on every `SetMessages`. No persistence.

## Build pipeline

`MessageList.SetMessages([]MessageInfo)` runs the build pipeline:

1. **Bucket** by `ThreadID`.
2. **Pick a root** for each bucket: the message with empty `InReplyTo`.
   If none exists (broken parent chain), pick the earliest by date as a
   synthetic root. If multiple messages have empty `InReplyTo`
   (forked conversation), the earliest is the root and the others
   become its top-level children.
3. **Sort children** chronologically ascending.
4. **Compute thread sort key**: latest-activity timestamp = max date
   across all messages in the thread (root + children).
5. **Sort threads** by latest-activity in the folder's configured sort
   direction (`date-desc` default; `date-asc` if the folder overrides).
6. **Flatten** thread-by-thread, root-then-children, computing each
   row's prefix from its position in its parent's child list.
7. **Apply fold state**: for each `folded[rootUID]`, mark every child
   row of that thread `hidden = true`.

`SetMessages` resets fold state to empty before re-running the
pipeline. Any header reload is a fresh start.

`ToggleFold()`, `FoldAll()`, `UnfoldAll()` mutate the fold map and
re-run only the flatten + apply-fold-state stages — bucketing and
sorting are cached.

## Prefix computation

Walk each thread depth-first, tracking at each level whether the
current node is the last sibling. Build the prefix from the trail of
ancestor "is-last" flags:

- For each ancestor (excluding the root itself): if that ancestor was
  the last sibling, append `"   "`; otherwise append `"│  "`.
- For the current node's own connector: if last sibling, `"└─ "`; else
  `"├─ "`.

Examples for the mock conversation (depths 0–2, branching):

```
Frank Lee         Server migration plan          ""
├─ Grace Kim      Re: Server migration plan      "├─ "
│  └─ Frank Lee   Re: Server migration plan      "│  └─ "
└─ Henry Park     Re: Server migration plan      "└─ "
```

When a thread root is folded, the root row's prefix becomes
`"[N] "` where N = `threadSize`. The `[N] ` lives in the same slot as
the box-drawing prefix — they never coexist on the same row.

## Rendering

Changes scoped to `MessageList.renderRow`:

- The prefix string is rendered first inside the subject column,
  styled with a new `Styles.MsgListThreadPrefix` slot resolving to
  `fg_dim`. It is rendered with `applyBg(prefix, bgStyle)` like every
  other column so the selection background extends through it.
- Subject text is then truncated to `subjectWidth - lipgloss.Width(prefix)`.
  Prefix is never truncated; the subject loses cells before the prefix
  does.
- The `[N] ` collapsed badge uses the same prefix slot. It also styles
  as `fg_dim`. Width is variable (`[3] `, `[12] `) but bounded to
  ~5 cells in practice.
- Read-state styling on sender/subject is unchanged. The prefix is
  always `fg_dim`, never inheriting the unread bright treatment, so
  the eye lands on senders and subjects rather than tree characters.
- Cursor `▐` and selection background still cover the full row width
  including the prefix cells. Selection is row-level.

`docs/poplar/styling.md` is updated to add the
`MsgListThreadPrefix → fg_dim` row to the slot table **before** the
renderer change is committed (per the styling invariant).

## Cursor and key handling

`AccountTab.handleKey` gains three new cases:

```go
case " ":
    m.msglist.ToggleFold()
case "F":
    m.msglist.FoldAll()
case "U":
    m.msglist.UnfoldAll()
```

`ToggleFold()` operates on the thread root of whichever row the cursor
is on. If the cursor is on a child, the toggle still folds from the
root. After a toggle, if `selected` lands on a now-hidden row (cursor
was inside the thread that just folded), `selected` snaps to that
thread's root index.

`MoveDown`/`MoveUp` skip hidden rows: the existing `moveBy` walks
`selected` in the requested direction, advancing past any
`displayRow.hidden == true` rows. `MoveToTop` and `MoveToBottom` snap
to the first/last visible row. Half-page and page jumps count visible
rows, not raw displayRow indices.

`clampOffset` continues to use `selected` directly — the displayRow
indices remain the unit of viewport math. Hidden rows still occupy
indices; they just aren't rendered. The render loop in `View()` skips
hidden rows when emitting lines and pulls the next visible row, so the
visible-row count drives how many lines fill the panel.

## Sort

`MessageList` gains a `sort SortOrder` field. `SortOrder` is a small
typed enum:

```go
type SortOrder int

const (
    SortDateDesc SortOrder = iota
    SortDateAsc
)
```

`AccountTab` resolves the order at folder-load time:

```go
order := SortDateDesc
if fc, ok := m.uiCfg.Folders[folderName]; ok && fc.Sort == "date-asc" {
    order = SortDateAsc
}
m.msglist.SetSort(order)
m.msglist.SetMessages(msgs)
```

The build pipeline's step 5 (sort threads) uses `order`. Step 3 (sort
children) is always ascending regardless of `order` — children always
read top-to-bottom oldest-to-newest because that's how a conversation
is naturally read.

`config.UIConfig.Folders[name].Sort` becomes load-bearing in this
pass. The `Sort` field has lived as parsed-but-unused config since
Pass 2.5b-3.5; this pass wires it through.

## Footer and keybindings doc

`internal/ui/footer.go` gains a new "Threads" hint group with three
hints. Drop ranks place them between Search (rank 3) and Go-To (rank
6) in priority:

- `␣ fold` — rank 4
- `F fold all` — rank 5
- `U unfold all` — rank 5

The `␣` symbol is U+2423 OPEN BOX, matching the convention already in
the help popover wireframe.

`docs/poplar/keybindings.md` promotes `Space`, `F`, `U` from
"reserved" to "live" with descriptions matching the footer hints.

## Mock backend conversation

`internal/mail/mock.go` grows a 4-message threaded conversation in the
Inbox. Reusing the wireframe's "Server migration plan" subject with a
branching shape (root + linear chain + sibling) so the `├─ │ └─`
prefixes are all exercised:

```
UID  ThreadID  InReplyTo  From          Date      Flags
20   T1        ""         Frank Lee     Apr 05    FlagSeen|FlagAnswered
21   T1        20         Grace Kim     Apr 05    0           (unread)
22   T1        21         Frank Lee     Apr 05    FlagSeen
23   T1        20         Henry Park    Apr 05    FlagSeen
```

Tree shape derived by the UI:

```
Frank Lee (20, root)
├─ Grace Kim (21)
│  └─ Frank Lee (22)
└─ Henry Park (23)
```

The first child (`UID 21`) is unread so the thread carries an unread
status into both its expanded and collapsed states — verifying that a
collapsed thread containing unread messages renders correctly.

The 10 existing flat messages stay unchanged. They each become
single-message threads with `ThreadID == UID`, `InReplyTo == ""`,
`Depth == 0`. Total: 14 source messages → 14 displayRows expanded,
11 displayRows when the threaded conversation is folded.

## Testing

### `internal/ui/msglist_test.go`

New cases:

1. **Thread grouping**: 14-message source produces 14 expanded
   displayRows in the right order (10 flat sorted by date-desc, then
   the threaded conversation positioned according to its
   latest-activity date).
2. **Prefix computation**: each row in the threaded conversation has
   the expected prefix string.
3. **Fold toggle**: collapsing the threaded conversation produces 11
   displayRows; expanding restores 14.
4. **Fold all / unfold all**: every multi-message thread collapses;
   every single-message thread is unaffected (already a leaf).
5. **Sort direction**: switching `SortOrder` from `SortDateDesc` to
   `SortDateAsc` reverses the thread order in the displayRow list.
6. **Synthetic root fallback**: a thread where no message has empty
   `InReplyTo` (broken parent chain) picks the earliest by date as
   the root and renders the others as its children.
7. **Cursor skip**: with the threaded conversation folded, `MoveDown`
   from the row above skips past the hidden children to the next
   visible row.
8. **Cursor snap on fold**: cursor on a child row, then `ToggleFold`,
   results in cursor on that thread's root row.
9. **SetMessages resets fold state**: collapse the thread, call
   `SetMessages` again with the same data, observe all rows visible.

Existing non-threading test cases continue to pass unchanged. The 10
flat messages each become single-message threads with empty prefix and
`threadSize == 1`, so existing rendering assertions hold.

### `internal/mail/mock_test.go`

Update count assertions for the new total (14). Add one case verifying
the threaded conversation's `ThreadID`/`InReplyTo`/`Depth` shape
matches the design above.

### Live render verification

Per the build invariant ("install and verify real renders before
claiming a rendering task is done"), the pass ends with a tmux capture
of the index view in three states: expanded, fully folded, and
partially folded with cursor on a hidden child. Verified against
wireframes §3 and §7 #14.

## ADR queue

ADRs to write at pass-end. Numbering picks up from the next free
slot at the time of commit (after the in-flight mailrender training
pass lands its ADRs):

1. **Thread data lives on `MessageInfo`, tree built transiently in UI** —
   why fields-on-MessageInfo + flat displayRow, why not a tree struct.
   Cites JMAP wire format and Camp 2 / Thunderbird precedent.
2. **Latest-activity is the thread sort key** — why threads sort by
   their most recent message rather than root date; matches Gmail /
   Apple Mail / Fastmail web.
3. **Threads start expanded; fold state per-session, reset on reload** —
   Apple Mail / Fastmail-style, no persistence in v1, YAGNI.
4. **Folder `Sort` config wired through** — promotes the parsed-but-
   unused config field from Pass 2.5b-3.5 to load-bearing.

## Out of scope

- IMAP-specific incremental thread updates (Camp 1 migration). Pass 8
  problem if it materializes.
- Persisted fold state across runs.
- Manual thread join/split, mute, or "ignore subthread" actions.
- The viewer's behavior on a thread (ADR-3 "thread navigation in
  viewer" is Pass 2.5b-4).
- Visual-select mode's `Space` row-toggle (ADR 0052 reserves it; Pass
  6 implements it).
