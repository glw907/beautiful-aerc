# Pass 8.4c — Cache III design

**Goal.** Make the outbox visible to the user. A status-bar depth
indicator, a `Q` overlay listing grouped pending/executing/failed
ops, and a `!` overlay for conflicts that need user intervention.
Plus offline-mode framing when the network is down with queued
ops on disk.

**Scope.** `internal/cache/` (new conflict-resolution primitives,
read queries for overlays) and `internal/ui/` (badge augmentation,
two new overlays, key dispatch, banner hint). No backend changes.
No schema changes — the schema already carries everything needed.

**Settled inputs.** Outbox state machine, drainer events, terminal
classification — locked in ADR-0110 / 0114 / 0115 / 0117.
Connection-state visual language (`●` / `◐` / `○`) locked by
ui-invariants. Modal overlays use `ModalShell` (ADR-0129). The
view-stable render-cache escape hatch is the `*<T>Cache` pattern
(ADR-0130).

**Pre-settled questions.**

- *Badge placement.* Augment the existing status bar
  (`status_bar.go`). New chrome rows are expensive at the 80×24
  polish bar; the badge fits inline next to the connection icon.
- *Offline-mode failure threshold.* None. The drainer's existing
  exponential backoff (60s cap) is sufficient. "Offline" is a UI
  hint keyed off `mail.ConnState` already plumbed through
  `pumpUpdatesCmd`. The drainer keeps trying with backoff
  regardless; ConnState is a belief about the network, not a
  drainer policy.

---

## §1. Cache layer additions

`internal/cache/ops.go` — new public methods on `*Account`:

```go
func (a *Account) RetryOp(ctx context.Context, opID int64) error
func (a *Account) DiscardOp(ctx context.Context, opID int64) error
```

Plus a private mirror of `applyOptimisticTx`:

```go
func revertOptimisticTx(tx *sql.Tx, msgID int64, args OpArgs) error
```

`revertOptimisticTx` semantics, by `OpArgs` variant:

| Variant | Forward (`applyOptimisticTx`) | Revert (`revertOptimisticTx`) |
|---|---|---|
| `MoveArgs` | `ui_hide = 1` on source | `ui_hide = 0` on source |
| `DestroyArgs` | `ui_hide = 1` | `ui_hide = 0` |
| `FlagArgs{Flag, Set}` | `ui_flags |= bit` (Set) or `&^= bit` (!Set) | flip the bit the other way |
| `SendArgs` / `AppendArgs` | (Pass 9) | return `errors.New("revert not supported for send/append")` |

`RetryOp` (single tx):

1. `SELECT status FROM outbox WHERE id = ?`. Require `'conflict'`;
   else return a sentinel `ErrNotConflict`.
2. `UPDATE outbox SET status='pending', attempts=0,
   next_eligible_at=NULL, error='' WHERE id=? AND status='conflict'`.
3. After commit, call `signalDrainer()`.

Rationale for `attempts=0`: user-initiated retry grants a fresh
budget. Without it, an `auth-failure` at `attempts >= max` would
re-enter conflict on the very next failure and the retry button
would feel broken.

`DiscardOp` (single tx):

1. `SELECT status, message, kind, args FROM outbox WHERE id=?`.
   Require `'conflict'`; else return `ErrNotConflict`.
2. Decode args.
3. If `message` is non-null, call `revertOptimisticTx(tx, msgID, args)`.
4. `DELETE FROM outbox WHERE id=?`.

No drainer signal — nothing to drain.

### Read queries for overlays + badge

Three new read methods in `internal/cache/reads.go`:

```go
type OutboxGroup struct {
    Kind    OpKind
    Folder  string         // canonical folder name; empty when message-less
    Status  OpStatus
    Count   int
    NextAt  sql.NullInt64  // earliest next_eligible_at within group (failed only)
}

type ConflictRow struct {
    ID           int64
    Kind         OpKind
    Folder       string
    ProtocolID   string
    ErrorKind    string
    ErrorMessage string
    Attempts     int
    EnqueuedAt   time.Time
}

type OutboxDepth struct {
    Pending   int
    Executing int
    Failed    int
    Conflict  int
}

func (a *Account) OutboxSummary(ctx context.Context) ([]OutboxGroup, error)
func (a *Account) OutboxConflicts(ctx context.Context) ([]ConflictRow, error)
func (a *Account) OutboxDepth(ctx context.Context) (OutboxDepth, error)
```

- `OutboxSummary`: `SELECT kind, f.name, status, COUNT(*),
  MIN(next_eligible_at) FROM outbox o LEFT JOIN folders f ON
  f.id = o.folder GROUP BY kind, folder, status ORDER BY
  CASE status WHEN 'executing' THEN 0 WHEN 'pending' THEN 1
  WHEN 'failed' THEN 2 WHEN 'conflict' THEN 3 ELSE 4 END,
  kind, name`. Status order is intentional: in-flight first
  (executing → pending), then troubled (failed → conflict).
- `OutboxConflicts`: `SELECT o.id, o.kind, COALESCE(f.name,''),
  COALESCE(m.protocol_id,''), o.error, o.attempts, o.enqueued_at
  FROM outbox o LEFT JOIN folders f ON f.id=o.folder LEFT JOIN
  messages m ON m.id=o.message WHERE status='conflict' ORDER BY
  o.enqueued_at ASC`. Decodes `error` JSON inline (`error.kind`,
  `error.message`).
- `OutboxDepth`: a single `SELECT status, COUNT(*) FROM outbox
  GROUP BY status` mapped into the `OutboxDepth` struct.

`OpStatus` and `OpKind` are already typed (ADR-0117); these
queries return them without string conversion at the boundary.

### Sentinel errors

```go
var ErrNotConflict = errors.New("cache: op is not in conflict state")
```

Returned by `RetryOp` / `DiscardOp` when the row has already been
resolved by another path. UI surfaces this as a benign "already
resolved" — refresh the conflict list and continue.

---

## §2. Status-bar badge

`internal/ui/status_bar.go` gains two fields and a setter:

```go
type StatusBar struct {
    // existing fields …
    outboxInflight int  // pending + executing + failed
    outboxConflict int  // conflict only
}

func (sb StatusBar) SetOutboxDepth(inflight, conflict int) StatusBar
```

Render rule, evaluated in `View` before the connection segment:

- `inflight == 0 && conflict == 0` → segment omitted.
- `conflict > 0` → render `⚠N` in `ColorWarning`, where `N` =
  `inflight + conflict`. Conflict signal dominates the glyph;
  the count is the full queue depth so the user sees scale.
- else → render `⇅N` in `FgDim` where `N = inflight`.

Glyph rationale: `⇅` (U+21C5) reads as "in transit / sync"; `⚠`
(U+26A0) reads as warning. Both are East Asian Width N (single
cell). They don't go through `displayCells` — `lipgloss.Width`
suffices. `IconSet` doesn't gate them; they aren't Nerd Font runes.

Spartan tier (W=80–89): segment renders. Worst case `⚠99` is 3
cells; the segment with surrounding ` · ` separators is 7 cells.
The status bar already drops to compact form at this width and
has room.

The `right` composition becomes (when both segments active):

```
" 12 messages · 3 unread · ⇅3 · ◐ reconnecting "
```

Empty outbox renders unchanged. App pumps depth via
`SetOutboxDepth(d.Pending+d.Executing+d.Failed, d.Conflict)` on
every `cacheEventMsg`.

---

## §3. `Q` overlay (outbox status)

`internal/ui/outbox_overlay.go` — new file. Uses `ModalShell`
(ADR-0129).

```go
type OutboxOverlay struct {
    shell  ModalShell
    styles Styles
    groups []cache.OutboxGroup
    open   bool
    width  int
    height int
}

func NewOutboxOverlay(styles Styles) OutboxOverlay
func (o OutboxOverlay) IsOpen() bool
func (o OutboxOverlay) Open(groups []cache.OutboxGroup) OutboxOverlay
func (o OutboxOverlay) Close() OutboxOverlay
func (o OutboxOverlay) SetGroups(groups []cache.OutboxGroup) OutboxOverlay
func (o OutboxOverlay) SetSize(w, h int) OutboxOverlay
func (o OutboxOverlay) Update(msg tea.Msg) (OutboxOverlay, tea.Cmd)
func (o OutboxOverlay) View() string
```

### Body rendering

One row per `OutboxGroup`. Format:

```
<Kind verb>[ → <Folder>] · <Count> <Status>[, retrying in <Ns>]
```

- Kind verb: `Move`, `Star`/`Unstar` (FlagArgs with FlagFlagged),
  `Mark read`/`Mark unread` (FlagArgs with FlagSeen), `Delete`
  (DestroyArgs). The verb is derived from `(Kind, args)` — but
  groups don't carry args. Resolution: `OutboxSummary` returns
  `Kind` only; flag groups display as `Flag` with no
  set/unset distinction. (FlagArgs grouping by exact bit is
  out of scope; `Flag · 2 pending` is acceptable telemetry.)
  Move groups display the destination via a join on
  `args->>'$.Dest'`. Implementation: compute the verb in
  `OutboxOverlay.View`, not in cache.
- Folder arrow appears for `Move` (destination) and is omitted
  for `Flag` and `Delete` (Delete groups always target Trash by
  invariant; redundant to show).
- Status text: lowercase (`pending`, `executing`, `failed`,
  `conflict`). For `failed` groups with `NextAt` valid and in
  the future, append `, retrying in <Ns>` where `N` is
  rounded-up seconds.

Empty state: single body row `Outbox is empty.` in `FgDim`.

Order: groups arrive pre-sorted from cache (executing → pending
→ failed → conflict, then kind, then folder).

### Footer

```
! conflicts  ·  q close
```

The `!` hint is dimmed when `conflict == 0`.

### Geometry

- `width = min(60, termWidth-4)`.
- `height = max(3, len(groups)) + 4` clamped to `termHeight-4`.
  The +4 covers ModalShell chrome (top/bottom border, title,
  footer separator).
- Body never scrolls. Grouping by `(kind, folder, status)` keeps
  totals bounded — a queue of 1000 single-message moves to one
  destination is one row.

### Render cache

Outbox content changes per cache event (high churn while a queue
drains). Not a `*<T>Cache` candidate — render fresh each frame.

---

## §4. `!` overlay (conflicts)

`internal/ui/conflict_overlay.go` — new file. `ModalShell`-based.
Render-cached via the `*<T>Cache` pattern (ADR-0130 extension).

```go
type ConflictOverlay struct {
    shell  ModalShell
    styles Styles
    rows   []cache.ConflictRow
    cursor int
    open   bool
    width  int
    height int
    cache  *conflictCache  // pointer + dirty flag per ADR-0130
}
```

### Body rendering

Two physical lines per row:

```
┃ <Kind verb> <protocol-id-short>[ → <Folder>]
│   <error.kind>: <error.message>  (<attempts> attempts, <age> ago)
```

- `<protocol-id-short>` = first 8 chars of `protocol_id` (or empty
  when missing — folder-scoped op).
- Cursor: `┃` thick bar in `AccentSecondary` on the header line of
  the highlighted row; `│` thin bar on detail and on non-cursor
  rows. Matches sidebar/msglist convention.
- `<age>` = humanized duration since `enqueued_at` ("3m", "2h",
  "1d"). Rounded down.
- `error.message` truncates with `…` to width.

Empty state: single body row `No conflicts.` Footer hides
`r retry · d discard`.

### Keys

| Key | Action |
|---|---|
| `j` / `k` | Move cursor (clamped, no wrap) |
| `r` | Retry highlighted op → `cache.RetryOp(opID)` |
| `d` | Discard highlighted op → `cache.DiscardOp(opID)` |
| `q` / `Esc` / `!` | Close overlay |

`d` shadows the account-context delete binding while the overlay
is open (App's modal cascade short-circuits keys before
delegating to `AccountTab`). No conflict with the `Q→!`
transition: when `Q` is open, `!` closes outbox and opens
conflicts.

### Cmds

```go
func retryConflictCmd(acct *cache.Account, opID int64) tea.Cmd
func discardConflictCmd(acct *cache.Account, opID int64) tea.Cmd
```

Both return a `conflictResolvedMsg{OpID int64, Err error}`. On
success: refresh `rows` from a fresh `cache.OutboxConflicts` call;
if list empties, close overlay. On error (other than
`ErrNotConflict`): emit `ErrorMsg{Op:"resolve conflict", Err:err}`
into the App banner. `ErrNotConflict` is treated as benign —
refresh and continue.

### Geometry

- `width = min(80, termWidth-4)`.
- Body height = `2 * len(rows)`. Total height = `2*len(rows)+5`
  (ModalShell chrome) clamped to `termHeight-4`.
- If body would overflow visible space, hard-cap to first
  `floor((termHeight-9)/2)` rows; append a trailing line `+N
  more (resolve to see)` in `FgDim`. (Viewport-based scroll
  deferred — conflict lists are realistically very small.)

### Render cache

`*conflictCache` per ADR-0130. Dirty on:

- `cacheEventMsg` arriving while open
- cursor move
- size change (forwarded from App via `SetSize`)
- post-action refresh

---

## §5. App wiring

`internal/ui/app.go` deltas:

### New fields

```go
outboxOpen   bool
outbox       OutboxOverlay
conflictOpen bool
conflict     ConflictOverlay
offlineHinted bool   // suppresses repeat banner emissions
```

### Modal cascade

Insert below `confirm`, above `helpOpen`/`linkPicker`/`movePicker`:

```go
if m.conflictOpen { /* route key into conflict overlay */ }
if m.outboxOpen   { /* route key into outbox overlay */ }
```

### Key dispatch

In the account-view / viewer key path:

```go
case key.Matches(msg, m.keys.OutboxOverlay):  // bound to "Q"
    return m.openOutbox()
case key.Matches(msg, m.keys.ConflictOverlay): // bound to "!"
    return m.openConflicts()
```

`openOutbox` / `openConflicts` issue `loadOutboxSummaryCmd` /
`loadOutboxConflictsCmd`, set `*Open = true`, return Cmd. The
overlays render empty until the first load completes (no flicker
risk — overlays open over a dimmed underlay).

`Q→!` transition: while `outboxOpen`, the `!` key closes outbox
and opens conflicts via the same dispatch path — handled inside
the outbox overlay's own Update (returns a `OpenConflictsMsg`
that App recognizes).

### `cacheEventMsg` handling

App.Update grows a sibling refresh path before delegating to
AccountTab:

```go
case cacheEventMsg:
    cmds := []tea.Cmd{pumpCacheCmd(m.acct.Cache())}
    cmds = append(cmds, refreshOutboxDepthCmd(m.acct.Cache()))
    if m.outboxOpen {
        cmds = append(cmds, loadOutboxSummaryCmd(m.acct.Cache()))
    }
    if m.conflictOpen {
        cmds = append(cmds, loadOutboxConflictsCmd(m.acct.Cache()))
    }
    // existing AccountTab delegation continues
```

`refreshOutboxDepthCmd` returns a new `outboxDepthMsg{Depth
cache.OutboxDepth}`; its handler updates the status bar via
`SetOutboxDepth`.

### Offline UI hint

In `backendUpdateMsg` handling, after `SetConnectionState`:

```go
if cs == Offline && !m.offlineHinted {
    if m.lastOutboxDepth.Pending+m.lastOutboxDepth.Failed > 0 {
        m.lastErr = lastError{Op: "connection",
            Err: fmt.Errorf("offline — queued ops will sync on reconnect")}
        m.offlineHinted = true
    }
}
if cs == Connected {
    m.offlineHinted = false
}
```

The banner clears on the next `ErrorMsg` last-write-wins or on
explicit dismissal. Threshold for emission: outbox non-empty.
Empty outbox on offline transition: silent (no point flagging
"offline" if there's nothing pending).

`m.lastOutboxDepth` is set by `outboxDepthMsg` handler so the
hint logic has fresh data.

---

## §6. Doc updates

### `docs/poplar/keybindings.md`

New section "Cache & Outbox" after Triage:

| Key | Action | Context |
|---|---|---|
| `Q` | Open outbox overlay | A, V |
| `!` | Open conflict overlay (or transition from `Q`) | A, V |
| `r` | Retry highlighted conflict | (in `!` overlay) |
| `d` | Discard highlighted conflict | (in `!` overlay) |

### `docs/poplar/wireframes.md`

- Update §1 composite layout's status-bar line:
  `… 12 messages · 3 unread · ⇅3 · ● connected ─╯`
- Add §10 "Outbox overlay (Q)" with rendered example.
- Add §11 "Conflict overlay (!)" with rendered example.

### `.claude/rules/ui-invariants.md`

Update the Overlays section:

- Six modal overlays now: help popover, link picker, move picker,
  confirm modal, **outbox overlay (Q)**, **conflict overlay (!)**.
  Confirm stays topmost.
- Outbox + conflict are `ModalShell` consumers. Conflict adds a
  `*conflictCache` instance to the ADR-0130 escape-hatch list;
  outbox does not (high-churn content, fresh render each frame).

### `docs/poplar/invariants.md`

Under Cache section, add binding facts:

- `(*Account).RetryOp(ctx, opID)` and `(*Account).DiscardOp(ctx,
  opID)` are the user-initiated conflict-resolution primitives.
  Both reject non-conflict rows with `ErrNotConflict`. Retry resets
  `attempts=0` and signals the drainer; discard reverts the
  optimistic flip via `revertOptimisticTx` and deletes the outbox
  row in one transaction.
- `(*Account).OutboxSummary` / `OutboxConflicts` / `OutboxDepth`
  are the read queries that feed the `Q` / `!` overlays and the
  status-bar depth segment.
- Status bar carries an outbox-depth segment between counts and
  connection icon: `⇅N` (`FgDim`) when only pending; `⚠N`
  (`ColorWarning`) when conflict count > 0; segment hidden when
  outbox is empty.
- Offline + non-empty outbox emits a one-shot ErrorMsg banner;
  empty outbox stays silent. The drainer's behavior is unchanged
  by ConnState.

Decision index gets new rows for ADR-0132/0133/0134.

---

## §7. ADRs to write at pass end

- **ADR-0132 — Outbox + conflict overlays.** Grouped-summary
  density for `Q` (rationale: per-op detail isn't user-relevant,
  groups collapse to single rows when count = 1, bounded total
  rows). Two-line rows for `!` (header + detail). `ModalShell`
  consumers. Render-cache opt-in for conflict only.
- **ADR-0133 — Conflict resolution primitives in cache.**
  `RetryOp` / `DiscardOp` / `revertOptimisticTx`. Rationale:
  local-state revert via mirror of `applyOptimisticTx`, not a
  forward inverse op via `QueueOp`. The conflicted op never
  reached the server — there is nothing to reverse remotely.
- **ADR-0134 — Status-bar outbox depth + offline hint.** Inline
  badge segment, conflict-dominates-pending. Offline framing as
  a one-shot UI banner gated on non-empty outbox; drainer policy
  unchanged.

---

## §8. Test coverage

`internal/cache/`:

- `ops_test.go` (extend):
  - `RetryOp` resets `attempts` to 0, clears `next_eligible_at`,
    transitions to `pending`, and signals the drainer.
  - `RetryOp` on a non-conflict row returns `ErrNotConflict`
    (idempotent for callers).
  - `DiscardOp` reverts each `OpArgs` variant correctly: Move
    clears `ui_hide`; Destroy clears `ui_hide`; Flag flips the
    bit back; Send/Append return error.
  - `DiscardOp` deletes the outbox row.
  - `DiscardOp` on a non-conflict row returns `ErrNotConflict`.

- `reads_test.go` (new or extend):
  - `OutboxSummary` returns one group per `(kind, folder,
    status)`, ordered correctly, with `MIN(next_eligible_at)`
    populated for `failed` groups only.
  - `OutboxConflicts` returns `conflict`-status rows, decoded
    `error.kind` + `error.message`, ordered by `enqueued_at` ASC.
  - `OutboxDepth` returns counts per status; empty outbox returns
    zeros.

`internal/ui/`:

- `outbox_overlay_test.go`: empty state; mixed-status rendering;
  `failed`-group "retrying in Ns" formatting; ModalShell
  width/height clamping; close on `q` / `Esc` / `Q`.
- `conflict_overlay_test.go`: cursor wrap (clamped, no wrap);
  `r` dispatches `retryConflictCmd` with the correct opID; `d`
  dispatches `discardConflictCmd`; auto-close when the last
  conflict resolves; `+N more` overflow line.
- `status_bar_test.go` (extend): `⇅N` rendering; `⚠N` precedence
  over `⇅N`; segment hidden at zero pending+conflict.
- `app_test.go` (extend): `Q` opens outbox; `!` opens conflict;
  `Q→!` transition closes outbox; `cacheEventMsg` refreshes
  badge + open overlay; offline + non-empty depth emits banner;
  offline + empty depth stays silent; `Connected` clears
  `offlineHinted`.

Goldens (one per overlay × two widths):

- `outbox_overlay_120x40.golden`
- `outbox_overlay_80x24.golden`
- `conflict_overlay_120x40.golden`
- `conflict_overlay_80x24.golden`

Live tmux verification at 120×40 and 80×24 per pass-end checklist
§1b. Capture both overlays open; capture status-bar segment in
each of the three states (empty, pending, conflict).

---

## §9. Out of scope

- Bulk retry-all / discard-all in `!`. Conflicts are rare; per-row
  is sufficient. Add only if a real workload demonstrates pain.
- Drainer-side offline detection. Backend `ConnState` is the
  single source of truth; the drainer does not consult it.
- Body-text scroll inside `!`. Hard-capped row list with `+N more`
  is enough for v1.
- A "force re-anchor" UI for cache divergence after manual
  discards. The next sync naturally reconciles via the syncer's
  upsert path; no user surface needed.
- `send` / `append` op kinds. Pass 9 will add them and extend
  `revertOptimisticTx`.
