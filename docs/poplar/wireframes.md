# Poplar Text Wireframes

Reference wireframes for poplar's UI. One canonical wireframe per
screen state. Layout, proportions, information density. Behavior
lives in `.claude/rules/ui-invariants.md`; key tables in
`docs/poplar/keybindings.md`. Cross-reference rather than duplicate.

## Conventions

- Box-drawing characters for borders: `╭╮╰╯│─┃`
- `┃` thick left bar for selected row indicator
- Nerd Font glyphs rendered directly (2-cell wide in terminal)
- Color annotations use theme slot names (`accent_primary`, `fg_dim`)
- Default terminal: 120 columns × 40 rows
- `←N→` for column widths
- Three-sided frame: top `──┬──╮`, right `│`, bottom `──┴──╯`. No
  left border.

---

## 1. Composite layout

Full application — sidebar + message list. No tab bar. Inbox
selected.

```
───────────────────────────┬──────────────────────────────────────────────────────────────────────────────╮
│ geoff@907.life           │                                                                              │
│                          │                                                                              │
│ ┃ 󰇰  Inbox           3  │  󰇮  Alice Johnson          Re: Project update for Q2 launch       10:32 AM  │
│   󰏫  Drafts              │  󰇮  Bob Smith               Weekly standup notes                    9:15 AM  │
│   󰑚  Sent                │  󰑚  Carol White             Re: Budget review                     Yesterday  │
│   󰀼  Archive             │      Dave Chen               Meeting minutes from Monday              Apr 07  │
│                          │  󰈻  Eve Martinez            Quarterly report draft                   Apr 06  │
│   󰍷  Spam           12   │      Frank Lee               Re: Server migration plan                Apr 05  │
│   󰩺  Trash               │      ├─ Grace Kim            └─ Re: Server migration plan             Apr 05  │
│                          │      │  └─ Frank Lee            Re: Server migration plan              Apr 05  │
│   󰂚  Notifications       │      Hannah Park             New office supplies order                Apr 04  │
│   󰑴  Remind              │      Ivan Petrov             Conference travel request                Apr 03  │
│   󰡡  Lists/golang        │                                                                              │
│                          │                                                                              │
 ──────────────────────────┴────────────────────────────── 10 messages · 3 unread · ⇅3 · ● connected ─╯
  d:del  a:archive  s:star  ┊  r:reply  R:all  f:fwd  c:compose  ┊  /:search  ?:help  q:quit
```

- Sidebar: 30 cols. Account header, three folder groups separated
  by blank lines, search shelf pinned to bottom (§2.1).
- Message list: remaining width. Columns: flags (2), sender (22),
  subject (fill), date (12). Double-space separator.
- Three-sided frame (top `──┬──╮`, right `│`, bottom `──┴──╯`). No
  left border.
- Status bar: bottom frame edge. Message count, unread count,
  outbox depth, connection indicator right-aligned.
- Status bar segments (right-to-left): connection indicator,
  outbox depth (`⇅N` in flight, `⚠N` when any conflict), unread
  count, message count. The outbox segment is hidden when the
  queue is empty.
- Footer: below status bar. Hint groups separated by `┊`.

---

## 2. Sidebar

Inbox selected. Disposal group (Spam, Trash) separated from Primary
by a blank line; Custom group (Notifications, Remind, Lists/…)
separated likewise.

```
 ┃ 󰇰  Inbox             3
   󰏫  Drafts
   󰑚  Sent
   󰀼  Archive

   󰍷  Spam            12
   󰩺  Trash

   󰂚  Notifications
   󰑴  Remind
   󰡡  Lists/golang
   󰡡  Lists/rust
```

- Width: 30 columns fixed.
- Selected row: `┃` thick left border in `accent_secondary`,
  full-width `bg_selection` background, name in `fg_bright`.
- Unread counts: right-aligned in `accent_tertiary`, only when > 0.
- Folder icons: `fg_base`; switch to `accent_tertiary` when the
  folder has unread messages.
- Nested folder names render flat — `/` in the display name is the
  only affordance (no tree).

---

## 2.1 Sidebar search shelf

Three rows pinned to the bottom of the sidebar column. Row 1 is a
blank separator; rows 2–3 host the prompt and mode/count.

### Idle / typing / committed

```
│   󰡡  Lists/rust          │
│                          │
│                          │   ← shelf row 1 (separator)
│  󰍉 / to search           │   ← idle hint
│                          │   ← reserved for mode/count
```

```
│  󰍉 /proj▏                │   ← typing
│  [name]       3 results  │
```

```
│  󰍉 /asdf▏                │   ← no results
│  [name]      no results  │
```

When the filter is committed and matches nothing, the message list
shows a centered "No matches" placeholder distinct from the
empty-folder state (§7).

- Activation: `/` from idle, or re-focus from active (preserves
  query). `Tab` cycles `[name]` ↔ `[all]`. `Enter` commits.
  `Esc` clears.
- Colors: icon `󰍉` in `fg_dim`/`accent_tertiary`, query in
  `fg_base`/`fg_bright`, mode badge in `fg_dim`, result count in
  `accent_tertiary`, "no results" in `color_warning`.

---

## 3. Message list

```
 󰇮  Alice Johnson            Re: Project update for Q2 launch          10:32 AM
▐󰇮  Bob Smith                 Weekly standup notes                       9:15 AM
 󰑚  Carol White               Re: Budget review                        Yesterday
     Dave Chen                 Meeting minutes from Monday                 Apr 07
 󰈻  Eve Martinez              Quarterly report draft                      Apr 06
     Frank Lee                 Re: Server migration plan                   Apr 05
     ├─ Grace Kim              └─ Re: Server migration plan                Apr 05
     │  └─ Frank Lee              Re: Server migration plan                Apr 05
```

```
←2→  ←──────── 22 ────────→  ←──────── fill ─────────────────────→  ←── 12 ──→
 FL  SENDER                   SUBJECT                                 DATE
```

- Cursor: `▐` right-half block in `accent_primary` at row left,
  full-width `bg_selection` background.
- Flags column: `󰇮` envelope = unread, `󰑚` reply icon
  (`color_special`), `󰈻` flag (`color_warning`).
- Read state by brightness: unread sender bold `fg_bright`, unread
  subject `fg_bright`, read rows `fg_dim`. Hue is reserved for
  cursor + unread+flagged.
- Thread prefixes in subject column: `├─` has-siblings, `└─`
  last-sibling, `│` stem, all in `fg_dim`.
- Date format and sort behavior: see invariants (date column,
  threads sort).

---

## 4. Message viewer

Viewer opens in the right panel; sidebar still visible. `q` returns
to the message list — no tab switching.

```
───────────────────────────┬──────────────────────────────────────────────────────────────────────────────╮
│ geoff@907.life           │                                                                              │
│                          │  From:     Alice Johnson <alice@example.com>                                 │
│   󰇰  Inbox           3  │  To:       Geoff Wright <geoff@907.life>                                     │
│   󰏫  Drafts              │  Date:     Thu, 10 Apr 2026 10:32:07 -0600                                  │
│   󰑚  Sent                │  Subject:  Re: Project update for Q2 launch                                 │
│   󰀼  Archive             │  ────────────────────────────────────────────────────────────                │
│                          │                                                                              │
│   󰍷  Spam           12   │  Hey Geoff,                                                                  │
│   󰩺  Trash               │                                                                              │
│                          │  Just wanted to follow up on the Q2 launch timeline.                         │
│   󰂚  Notifications       │                                                                              │
│   󰑴  Remind              │  ## Key changes                                                              │
│   󰡡  Lists/golang        │                                                                              │
│                          │  - Beta release moved to April 15                                            │
│                          │  - Launch date is now May 1                                                  │
│                          │                                                                              │
│                          │  > On Apr 9, 2026, Geoff Wright wrote:                                      │
│                          │  > Can you send me the updated project plan?                                 │
 ──────────────────────────┴──────────────────────────────────────────── 100% · ● connected ─╯
  d:del  a:archive  s:star  ┊  r:reply  R:all  f:fwd  ┊  Tab:links  q:close  ?:help
```

- Body: 72-cell cap. Headers wrap at panel width. Headings
  `color_success` bold; blockquotes `accent_tertiary` (L1) /
  `fg_dim` (L2+); links `accent_primary` underline.
- Header keys `accent_primary` bold; values `fg_base`; angle-bracketed
  email in `fg_dim`.
- Viewport scroll: `j/k`, `Space`/`b`, `g`/`G`. Modifier-free.
- Footnotes: outbound links rendered `[N]: <url>` below a rule;
  inline link text gets ` [^N]` glued via U+00A0. See invariants
  (Viewer) for the full rule.

---

## 5. Help popover

Modal overlay, `?` opens it. Two contexts: account view and viewer.
Help advertises the full planned vocabulary; unwired rows render
dim throughout.

### Account view context

```
                  ╭─ Account View ──────────────────────────────────────────╮
                  │                                                         │
                  │  Navigate           Triage          Reply               │
                  │  j/k  up/down       d  delete       r  reply            │
                  │  g/G  top/bottom    a  archive      R  all              │
                  │  J/K  folders       s  star         f  forward          │
                  │                     .  read/unrd    c  compose          │
                  │                                                         │
                  │  Search             Select          Threads             │
                  │  /    search        v  select       ␣  fold             │
                  │  n    next          ␣  toggle       F  fold all         │
                  │  N    prev                                              │
                  │                                                         │
                  │  Go To                                                  │
                  │  I  inbox    D  drafts    S  sent                       │
                  │  A  archive  X  spam      T  trash    E  empty (T/X)    │
                  │                                                         │
                  │  Enter  open        ?/Esc  close     m  move            │
                  │                                                         │
                  ╰─────────────────────────────────────────────────────────╯
```

### Viewer context

```
                  ╭─ Message Viewer ────────────────────────────────────────╮
                  │                                                         │
                  │  Navigate           Triage          Reply               │
                  │  j/k    scroll      d  delete       r  reply            │
                  │  g/G    top/bot     a  archive      R  all              │
                  │  ␣/b    page d/u    s  star         f  forward          │
                  │  n/N    next/prev   .  read/unrd    c  compose          │
                  │  1-9    open link                                       │
                  │                                                         │
                  │  Tab  link picker   q  close        ?/Esc  close help   │
                  │                                                         │
                  ╰─────────────────────────────────────────────────────────╯
```

- Modal: centered, content behind dimmed via `DimANSI`.
- Title bar embeds the context name in `accent_primary` bold.
- Group headings `fg_bright` bold. Wired keys `fg_bright` bold +
  desc `fg_dim`. Unwired rows dim throughout.
- Input routing: only `?` and `Esc` are handled while open; `q` is
  swallowed (help is a view, not a state to escape).

---

## 6. Chrome row (toast / undo / error)

The chrome row above the status bar is shared. Priority: error
banner > toast > collapsed. Triage uses an inline toast with an
`[u undo]` hint; permanent-delete operations (empty) use the same
toast row but suppress the hint (no inverse).

```
───┴───── ✓ Archived 3 messages         [u undo · 5s] ────╯    triage toast
───┴───── ✓ Emptied Trash (5)                            ────╯    permanent toast (no undo hint)
───┴───── ⚠ mark read: connection refused                ────╯    error banner
```

Toast variants by op:

```
✓ Archived 1 message                color_success
✓ Deleted 3 messages                color_success
✓ Emptied Trash (5)                 color_success     (no undo hint)
󰈻 Flagged                           color_warning
󰇮 Marked unread                     accent_tertiary
```

- Toast: `tea.Tick` schedules undo expiry per `[ui] undo_seconds`
  (default 6, clamp `[2, 30]`). Empty/destroy-style ops omit the
  hint because the primitive is irreversible.
- Error banner: `⚠` prefix, single foreground row, truncated with
  `…`. Persists until overwritten or cleared. No dismiss key, no
  severity tiers. See invariants (Triage, undo, error banner).
- Loading spinner: `bubbles/spinner` braille (`⣾⣽⣻⢿⡿⣟⣯⣷`),
  centered in content area in `fg_dim` ("Loading messages…",
  "Loading message…", etc.).
- Connection indicator (right edge of status bar): `●` connected
  (`color_success`), `◐` reconnecting (`color_warning`), `○`
  offline (`fg_dim`). Triple redundancy for colorblind.

---

## 7. Screen states

### Empty folder

```
│                         │                                                                               │
│                         │                                                                               │
│                         │                       No messages                                              │
│                         │                                                                               │
│                         │                                                                               │
```

"No messages" centered, `fg_dim`.

### Threading — expanded, collapsed, mid-thread fold

Default expanded. `Space` toggles fold under cursor; collapsed root
shows `[N]` count badge replacing the box-drawing prefix.

```
     Eve Martinez              Re: Server migration plan                   Apr 05    expanded root
     ├─ Grace Kim              └─ Re: Server migration plan                Apr 05    child
     │  └─ Frank Lee              Re: Server migration plan                Apr 05    grandchild

     Eve Martinez           [3] Re: Server migration plan                  Apr 05    fully collapsed

     Eve Martinez              Re: Server migration plan                   Apr 05    partially —
     ├─ Grace Kim           [2] └─ Re: Server migration plan               Apr 05    mid-thread fold
```

`F` is the bulk counterpart (folds all if any unfolded, else
unfolds all). See invariants (Reading & navigation).

### Search filter applied

```
│ geoff@907.life           │                                                                               │
│                          │  󰇮  Alice Johnson            Re: Project update for Q2 launch         10:32 AM │
│   󰇰  Inbox           3  │  󰑚  Carol White              Re: Project budget review              Yesterday │
│   󰏫  Drafts              │                                                                               │
│   󰑚  Sent                │                                                                               │
│   󰀼  Archive             │                                                                               │
│                          │                                                                               │
│   󰍷  Spam           12   │                                                                               │
│  󰍉 /proj                 │                                                                               │
│  [name]       2 results  │                                                                               │
 ──────────────────────────┴──────────────────────────────── 10 messages · 3 unread · ● connected ─╯
```

Filter-and-hide: non-matching threads disappear; matching threads
render fully expanded. Status bar retains its normal contents (no
search indicator there). `Esc` clears + restores cursor.

### Multi-select (visual mode)

`v` enters visual mode. `Space` toggles individual rows.

```
 󰇮   Alice Johnson            Re: Project update for Q2 launch         10:32 AM
 󰇮  󰄬 Bob Smith                Weekly standup notes                      9:15 AM
 󰑚  󰄬 Carol White              Re: Budget review                       Yesterday
      Dave Chen                 Meeting minutes from Monday                Apr 07
 󰈻  󰄬 Eve Martinez             Quarterly report draft                    Apr 06
```

```
 ──────────────────────────┴──────────────────────────────────────────── 3 selected ─╯
  Space:toggle  d:del all  a:archive all  v:cancel  Esc:cancel
```

- Check icon `󰄬` in `color_success` in flags column on selected
  rows. Selected rows get `bg_selection`.
- `ActionTargets` returns marks in source order on dispatch;
  visual mode auto-exits after dispatch.

---

## 8. Overlays

### Move picker

Modal overlay invoked by `m` from the account view. Fuzzy filter
on folder name. `Enter` confirms, `Esc` cancels.

```
                       ╭─ Move to folder ────────────────────────╮
                       │                                          │
                       │  > arch                                  │
                       │                                          │
                       │  ┃ 󰀼  Archive                            │
                       │    󰡡  Lists/arch-linux                   │
                       │                                          │
                       ╰──────────────────────────────────────────╯
```

- Centered, dimmed underlay. `>` prefix on `bubbles/textinput`.
  Selected row: `┃` + `bg_selection`. Rounded border `bg_border`,
  title `accent_primary`.
- Picker height shrinks to fit results.

### Confirm modal

Generic destructive-action prompt (`ConfirmModal`). Currently used
by manual empty (`E` on Disposal folders). Topmost overlay.

```
                       ╭─ Empty Trash? ──────────────────────────╮
                       │                                          │
                       │  Permanently delete 12 messages.         │
                       │  This cannot be undone.                  │
                       │                                          │
                       │  y  empty           n  cancel            │
                       │                                          │
                       ╰──────────────────────────────────────────╯
```

- `y` confirms → `EmptyFolderConfirmedMsg` → empty pipeline.
  `n`/`Esc` dismisses. No undo hint on the resulting toast (the
  primitive is irreversible).

### Link picker

Viewer-context-only. `Tab` opens it when ≥1 URL is harvested.

```
                       ╭─ Links ─────────────────────────────────╮
                       │                                          │
                       │  ┃ 1  example.com/docs/foo                │
                       │    2  github.com/glw907/poplar            │
                       │    3  fastmail.com                        │
                       │                                          │
                       ╰──────────────────────────────────────────╯
```

- `j/k` cursor, `Enter` / `1`–`9` launch + close, `Esc`/`Tab`
  close, `q` swallowed.

### Attachment picker

Viewer-context-only. `@` opens it when the message has ≥1
attachment. The chip row in the viewer body sits between the
header panel and the body; it lists `§ N. filename (size)` chips
greedy-wrapped to width.

```
┌─ from / to / date / subject panel ───────────────────────┐
│ … header lines …                                          │
└──────────────────────────────────────────────────────────┘
 § 1. report.pdf (2.3 KB)  § 2. agenda.txt (412 B)
 § 3. photo.jpg (118 KB)
─ body ────────────────────────────────────────────────────
```

```
                       ╭─ Attachments ───────────────────────────╮
                       │                                         │
                       │  ┃§[1] report.pdf (2.3 KB)               │
                       │   §[2] agenda.txt (412 B)                │
                       │   §[3] photo.jpg (118 KB)                │
                       │                                         │
                       │ Enter/o open  s save  Esc close          │
                       ╰─────────────────────────────────────────╯
```

- `j/k` cursor, `Enter` / `o` / digit open via `xdg-open` on a
  tempfile, `s` saves to `[ui] download_dir` (default
  `$XDG_DOWNLOAD_DIR` or `~/Downloads`); `Esc` / `q` / `@` close.
- A successful save surfaces `Saved to <path>` in the toast row;
  the row collapses on the undo timer (no undo affordance — the
  file is on disk).

---

## 10. Outbox overlay (Q)

Modal opened by `Q`. Read-only summary of pending / executing /
failed / conflicted ops, grouped by `(kind, folder, status)`. No
cursor — telemetry surface, not interactive.

```
┌─ Outbox ──────────────────────────────────────────────┐
│ Move → Archive · 23 pending                           │
│ Flag · 2 executing                                    │
│ Delete · 1 failed, retrying in 12s                    │
│ Move → Inbox · 1 conflict                             │
├───────────────────────────────────────────────────────┤
│ ! conflicts  ·  q close                               │
└───────────────────────────────────────────────────────┘
```

Empty outbox renders a single body row "Outbox is empty." Conflict
rows here are summary-only; the per-row detail and resolution UI
lives in §11.

---

## 11. Conflict overlay (!)

Modal opened by `!`. Per-row retry / discard for ops the drainer
gave up on (auth-failure, max-attempts-exceeded, args-decode,
crashed-mid-execute).

```
┌─ Conflicts ─────────────────────────────────────────────────────────────────┐
│ ┃ Flag abc12345 in Inbox                                                    │
│ │   auth-failure: invalid credentials  (3 attempts, 12m ago)                │
│ │ Move xyz98765 in Inbox                                                    │
│ │   max-attempts-exceeded: timeout reading from server  (10 attempts, 2m ago)│
├─────────────────────────────────────────────────────────────────────────────┤
│ r retry  ·  d discard  ·  q close                                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

- `┃` thick bar marks the cursor row's header. `│` thin bar on
  detail and on non-cursor rows.
- `r` retries (resets attempts, signals drainer); `d` discards
  (reverts the optimistic flip and deletes the outbox row).
- Empty state: "No conflicts." Footer hides retry/discard hints.
- Overflow: when `2 * len(rows) > available body rows`, the list
  hard-caps and the last row reads `+N more (resolve to see)`.

---

## 12. Contact edit form

Two render contexts:

- **Right-pane mode** — `n` (new) or `e` (edit on cursor) from
  Contacts mode replaces the detail card with the form. Sidebar
  + middle list stay drawn but inert.
- **Modal mode** — `n` from the `i`-popover's "no contact"
  affordance opens the form centered, with mail chrome dimmed
  underneath.

Person variant:

```
┌─ New contact ──────────────────────────────────────────┐
│                                                        │
│  Kind:    ● Person   ○ Business                        │
│                                                        │
│  First:   [Alice___________________________]           │
│  Last:    [Chen____________________________]           │
│  Org:     [ACME____________________________]           │
│  Title:   [Senior Engineer_________________]           │
│                                                        │
│  Emails:  [alice@example.com_______________] ◀Work▶ ★− │
│           [a.chen@personal.io______________] ◀Home▶  − │
│           [+ add email]                                │
│                                                        │
│  Phones:  [+1 555-0100_____________________] ◀Mob.▶ ★− │
│           [+1 555-0199_____________________] ◀Work▶  − │
│           [+ add phone]                                │
│                                                        │
│  Note:    [Met at GopherCon 2024.________________]     │
│           [Cares about error messages.___________]     │
│                                                        │
│  Save to: ● Local file   ○ geoff@907.life              │
├────────────────────────────────────────────────────────┤
│ Tab/Shift+Tab navigate · Ctrl+S save · Esc cancel      │
└────────────────────────────────────────────────────────┘
```

Business variant: kind toggle hides First/Last/Org/Title and
replaces with a single `Name:` row. Email/phone/note/save layout
matches Person.

Field rules:
- Kind toggle: `Tab` reaches it. `Space`, `←`, or `→` flips. The
  flip leaves text-field state untouched but drops the now-hidden
  fields from tab order.
- `★` filled = primary (row 0). `☆` hollow = non-primary. Pressing
  Enter on `☆` promotes that row to row 0 (slice rotate).
- `−` deletes a row. Disabled on the only email row.
- Validation runs on `Ctrl+S`. Failure sets a one-line warn under
  the Save row and blocks save. Success emits ContactSaveMsg
  carrying the assembled Contact and chosen Save-to destination.
- `Esc` cancels. Pristine forms close immediately. Dirty forms
  open the "Discard changes?" confirm.

Right-pane mode renders without the `┌─┐` chrome. The surrounding
contacts frame already supplies borders.
