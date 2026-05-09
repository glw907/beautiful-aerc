# Pass 10b — Schedule send + outbox sidebar

Completes BACKLOG #35. Builds on the schema-v10 `scheduled_for` /
`draft_id` foundation and the cancel-row primitive landed in 10a.

## Goal

A user can:

1. Schedule a compose send for a future time, picked from three
   presets or typed as free text.
2. See pending sends in a sidebar "Outbox" entry that appears
   above Trash when the queue is non-empty.
3. Open the Outbox view to cancel, reschedule, or edit-as-draft
   any pending row.

## Settled decisions

These came out of the Pass 10b brainstorm and are not re-opened
in implementation.

| Topic | Decision | Rationale |
|---|---|---|
| Compose key | `Ctrl+L` (Later) | ADR-0076 exempts compose; only sensible free chord given `^X`/`^C`/`^O`/`^T` in use. `^S` is XOFF; `^J/G/N/P` collide with readline. |
| Preset list | Gmail-shape, 3 presets + Custom | Matrix consensus (Gmail/Apple/Outlook/Fastmail all use day+time-of-day English labels). Gmail labels are most-copied. 4-row modal stays compact. |
| Custom entry | Hand-rolled text parser | Every reference is GUI; terminal precedent is empty. Vim-first users prefer typing to navigating a calendar grid. |
| Sidebar slot | Synthetic entry at top of Disposal group | Preserves the fixed-3-groups invariant. "Outbox" name rather than "Scheduled" because the slot covers both undo-send holds and scheduled holds. |

## Cache layer

`internal/cache/` gains two methods, both additive — no schema
change (schema v10 is already in place from Pass 10a).

### `RescheduleOp`

```go
func (a *Account) RescheduleOp(ctx context.Context, opID int64, newScheduledFor int64) error
```

`UPDATE outbox SET scheduled_for = ? WHERE id = ? AND status IN
('pending', 'failed') AND scheduled_for > ?` (the third bind is
`time.Now().UnixNano()` — guards against rescheduling a row that
the drainer is about to pick up).

Returns `ErrNotPending` (the same sentinel `CancelOps` returns)
when the row has advanced. Idempotent — rescheduling to the same
value is a no-op zero-rows-affected.

### `OutboxScheduled`

```go
type OutboxRow struct {
    ID            int64
    Kind          OpKind     // OpSend or OpAppend
    Folder        string     // sent folder name (Send) or APPEND target (Append)
    To            []string   // decoded from args.Envelope.Rcpts (Send) or empty (Append)
    Subject       string     // decoded from MIME headers in payload
    ScheduledFor  time.Time  // converted from unix nanos; zero if NULL
    Status        string     // "pending" | "failed"
    Attempts      int
    LastError     string
    Draft         *Draft     // joined from drafts table; nil if FK is NULL
}

func (a *Account) OutboxScheduled(ctx context.Context) ([]OutboxRow, error)
```

Reads `outbox` rows where `status IN ('pending', 'failed')`,
ordered by `scheduled_for ASC` (NULL last so undo-window holds
sort stably below scheduled rows). LEFT JOIN against `drafts` on
`outbox.draft_id = drafts.id` — when present, hydrates `Draft`
for the edit-as-draft path. Subject decoding parses the MIME
header from `outbox.payload` (one regex pass on the first 4 KB,
no full MIME parse). Pre-existing `outbox.OutboxCount` covers
the status-bar `⇅N` segment unchanged.

## Compose schedule picker

New file `internal/ui/compose/schedulepicker.go`. A bubbles-shaped
sub-model on `compose.Model` rendered as an overlay via
`uicore.PlaceOverlay` (same pattern as `compose.AttachPicker`,
ADR-0179) — rides inside compose's input window, outside the
global modal cascade.

### Surface

```
┌─ Schedule send ──────────────────────────┐
│  Tomorrow morning      8:00 AM           │
│  Tomorrow afternoon    1:00 PM           │
│  Monday morning        8:00 AM           │
│  Custom…                                 │
└──────────────────────────────────────────┘
  j/k nav  ⏎ pick  ^L close
```

When Custom is selected and `Enter` pressed, the modal grows one
row to expose a `bubbles/textinput`:

```
┌─ Schedule send ──────────────────────────┐
│  Tomorrow morning      8:00 AM           │
│  Tomorrow afternoon    1:00 PM           │
│  Monday morning        8:00 AM           │
│  ▶ Custom…                               │
│  ┌─────────────────────────────────────┐ │
│  │ tomorrow 3pm_                       │ │
│  └─────────────────────────────────────┘ │
└──────────────────────────────────────────┘
  ⏎ schedule  Esc cancel
```

Parse-error renders inline below the input as one dim row:

```
│  not recognized — try "tomorrow 3pm"     │
```

### Bindings

| Key | Action |
|---|---|
| `j` / `k` / `↑` / `↓` | Cursor |
| `Enter` on preset | Commit at preset time |
| `Enter` on Custom | Expand input row |
| `Enter` in input | Parse; commit on success |
| `Esc` | Cancel (returns to compose) |
| `Ctrl+L` | Toggle (close if open) |

### Messages

```go
type ScheduleAcceptedMsg struct{ When time.Time }
type ScheduleCancelledMsg struct{}
```

Compose's existing send path threads `When.UnixNano()` into
`cache.Account.QueueOutbound`'s `scheduledFor` parameter
(currently always `0`). Zero remains "send now"; positive arms
the drainer skip.

### Monday-morning preset

Computed from `time.Now()`: if today is Monday, advances to *next*
Monday (matching Gmail's behavior). Time set to 08:00 in the
local zone.

## Custom-entry parser

New file `internal/compose/scheduleparse.go`. Pure function:

```go
func ParseSchedule(s string, now time.Time) (time.Time, error)
```

Strategy:

1. **Trim and lowercase a working copy** for keyword detection.
2. **Keyword shortcuts** matched first (regex anchors):
   - `tomorrow` → `now + 24h` truncated to date, default time 09:00
   - `tonight` → today 21:00 (rolls to tomorrow 21:00 if already past)
   - `next <weekday>` → following occurrence (skips this week)
   - `<weekday>` (alone) → next occurrence including today if future
   - `+Nh` / `+Nm` / `+Nd` → relative offset
   - Optional trailing `HH:MM` / `H[:MM] AM/PM` / `Ham`/`Hpm` overrides the default time
3. **Layout sweep** — try `time.Parse` with each in order:
   - `2006-01-02 15:04`, `2006-01-02 3:04 PM`, `2006-01-02 3 PM`, `2006-01-02`
   - `01/02/2006 15:04`, `01/02/2006 3:04 PM`, `01/02/2006`
   - `01/02 15:04`, `01/02 3:04 PM`, `01/02`
   - `Jan 2 2006 15:04`, `Jan 2 2006`, `Jan 2 15:04`, `Jan 2 3:04 PM`, `Jan 2`
   - `2 Jan 2006 15:04`, `2 Jan 2006`, `2 Jan 15:04`, `2 Jan`
   - `15:04`, `3:04 PM`, `3 PM`, `3am`, `3pm` (date defaults to today)
4. **Year defaulting** — when the matched layout has no year, set
   `result.Year() = now.Year()`.
5. **Past-rolling** — if `result.Before(now)` after year-default,
   advance one unit appropriate to the layout: a date-only match
   rolls to next year; a time-only match rolls to tomorrow; a
   month/day match rolls to next year.
6. **Failure** — return `error` with a fixed hint string the
   picker renders verbatim.

Times parse in `now.Location()` (no UTC handling at the input
layer). All return values are absolute `time.Time`.

Tested table-driven against ~30 input shapes. No third-party
dependency.

## Sidebar virtual folder

`sidebar.Model` gains:

```go
func (m *Model) SetOutboxCount(n int)
```

State: one `int` field on the model, default 0. Render path
(`renderGroup` for Disposal) checks the count and prepends a
synthetic `mail.FolderEntry{Name: "Outbox", Canonical: "Outbox"}`
when `n > 0`. Badge `(N)` rendered in `FgDim` to the right of
the name, inside the existing flag-column budget.

`J/K` navigation includes the synthetic entry like any other
folder. The selection model already keys off folder display
name — no change there.

App-side wiring (`internal/ui/app.go`):

- On every `cache.UpdateMsg`, App calls `cache.Account.OutboxCount()`
  and threads through to `sidebar.SetOutboxCount`.
- When the user selects "Outbox" (compares the sidebar's selected
  display name), the App's right-pane controller renders
  `outbox.Model` instead of `messagelist.Model`. The previous
  folder selection is remembered in
  `app.preOutboxFolder` so `Esc`/`q` from the outbox view
  restores it.
- `cache.Account.QueryFolder("Outbox", …)` is **not** called —
  the outbox view reads via `OutboxScheduled` directly.

## Outbox view

New subpackage `internal/ui/outbox/`. Files:

```
internal/ui/outbox/
  model.go     — Model + Update + View
  msgs.go      — RefreshMsg, OpenScheduleMsg, OpenComposeAsDraftMsg
  styles.go    — Styles built from theme.CompiledTheme
```

### Model

A `bubbles/list`-shaped value type. Holds `[]OutboxRow` from
`OutboxScheduled`, a cursor index, and a snapshot `time.Time` for
"in 2h 14m" relative rendering.

```go
type Model struct {
    rows    []cache.OutboxRow
    cursor  int
    now     time.Time
    width   int
    height  int
    styles  Styles
}

func New(theme *theme.CompiledTheme) Model
func (m Model) Init() tea.Cmd
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)
func (m Model) View() string
```

### Render

```
Outbox (3)

  Tomorrow 8:00 AM   alice@example.com    Re: deploy plan      pending
  Tomorrow 1:00 PM   bob@example.com      monthly status       pending
> Mon May 12 8:00 AM team@example.com     onboarding doc       pending

  c cancel  s reschedule  e edit-as-draft  q close
```

Columns: scheduled time (relative when ≤24h, absolute beyond),
first recipient, subject, status. Truncate via `ansix.Truncate`.
Empty queue renders a centered placeholder ("Outbox is empty").

### Bindings

| Key | Action |
|---|---|
| `j` / `k` | Cursor |
| `c` | Cancel — calls `cache.CancelOps([row.ID])` |
| `s` | Reschedule — emits `OpenScheduleMsg{OpID, Initial}` where `Initial` is the row's current `ScheduledFor` formatted as `2006-01-02 15:04`; App opens the schedule picker with the Custom row expanded and the input pre-filled |
| `e` | Edit-as-draft — emits `OpenComposeAsDraftMsg{OpID, Draft}`; App calls `cache.CancelOps`, then opens compose seeded from the linked `Draft` |
| `Esc` / `q` | Close, restore previous folder |

### Refresh

Re-reads `OutboxScheduled` on every `cache.UpdateMsg` and on
every successful action message (`CancelOps`, `RescheduleOp`).
Cursor preserved on row ID — re-found by ID after refresh; if
the row is gone, cursor clamps to nearest.

`s` on a row whose Draft is nil (legacy queued op pre-dating
draft persistence) opens the picker but the resulting accept
calls `RescheduleOp` only — no compose round-trip.

`e` on a row whose Draft is nil falls through to a one-line
error banner ("no draft on record — cancel and recompose"). The
join is the only edit path; no MIME parser involved.

## Bindings summary (delta from current keybindings.md)

```
Compose:
  Ctrl+L  Schedule send (text-entry exempt per ADR-0076)

Outbox view (new context O):
  j / k   Cursor
  c       Cancel scheduled send
  s       Reschedule (opens picker)
  e       Edit as draft (cancels + opens compose)
  Esc, q  Close
```

`docs/poplar/keybindings.md` adds a "## Outbox" section between
"Cache & Outbox" and "Reply & Compose"; "## Compose" gains the
`Ctrl+L` row.

## ADR scope

One ADR for the pass: **ADR-0184 — Schedule send + outbox view**.

Documents:
- `Ctrl+L` choice and modal shape
- Preset calibration vs Gmail
- Custom-entry text-input deviation from the GUI matrix
- "Outbox" naming over "Scheduled"
- Synthetic Disposal entry approach (preserves three-group invariant)
- `RescheduleOp` / `OutboxScheduled` cache surface
- Edit-as-draft = join query, no MIME parser

## Out of scope

- Time-zone display in the outbox list (always renders in local
  zone). Timezone-aware send is post-1.0.
- Recurring scheduled sends.
- Per-account schedule defaults (a future config block, not for
  v1).

## Test plan

- `cache_test.go` covers `RescheduleOp` success, `ErrNotPending`,
  guard against `scheduled_for <= now`.
- `outbox_reads_test.go` covers `OutboxScheduled` ordering, draft
  join, subject decoding from MIME bytes.
- `scheduleparse_test.go` table-drives ~30 input strings with a
  fixed `now`, asserting parsed times.
- `outbox/model_test.go` covers cursor preservation across
  refresh, action emission, empty-state render.
- `compose/schedulepicker_test.go` covers preset commit, custom
  parse-error display, Ctrl+L toggle.
- Live tmux verification at 80×24 and 120×40.

## Pass split watch

If task count exceeds 12 during planning, split as:

- **10b.1** — cache (`RescheduleOp`, `OutboxScheduled`) + sidebar
  seam + outbox view (read-only, just `c` cancel).
- **10b.2** — schedule picker, custom parser, reschedule + edit-
  as-draft wiring, ADR.

## Pass-end checklist

Standard `poplar-pass` ritual: `/simplify`, idiomatic-bubbletea
review against §10 of `bubbletea-conventions.md`, ADR-0184,
`invariants.md` update (Send + Append section gains `RescheduleOp`
and `OutboxScheduled` lines; Sidebar section gains the synthetic-
entry rule; Compose section gains `Ctrl+L`), `keybindings.md`
update, plan + spec archived, `make check`, commit, push,
`make install`.
