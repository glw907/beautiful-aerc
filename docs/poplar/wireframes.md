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
 ──────────────────────────┴──────────────────────────────────────── 10 messages · 3 unread · ● connected ─╯
  d:del  a:archive  s:star  ┊  r:reply  R:all  f:fwd  c:compose  ┊  /:search  ?:help  q:quit
```

- Sidebar: 30 cols. Account header, three folder groups separated
  by blank lines, search shelf pinned to bottom (§2.1).
- Message list: remaining width. Columns: flags (2), sender (22),
  subject (fill), date (12). Double-space separator.
- Three-sided frame (top `──┬──╮`, right `│`, bottom `──┴──╯`). No
  left border.
- Status bar: bottom frame edge. Message count, unread count,
  connection indicator right-aligned.
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
