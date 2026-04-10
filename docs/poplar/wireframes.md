# Poplar Text Wireframes

Reference wireframes for every UI element in the poplar interface
inventory. Each wireframe defines layout, proportions, and
information density for the bubbletea prototype (Pass 2.5b).

See the UI design spec for the complete interface inventory:
`docs/superpowers/specs/2026-04-10-poplar-ui-wireframing-design.md`

## Conventions

- Box-drawing characters for borders: `╭╮╰╯│─┃`
- `┃` thick left bar for selected row indicator
- Nerd Font glyphs rendered directly
- Color annotations use theme slot names (`accent_primary`, `fg_dim`)
- Default terminal: 120 columns x 40 rows
- `←N→` for column widths
- `[key]` for interactive elements

---

## 1. Composite Layout

Full application with all persistent chrome and both panels visible.
This is the default view on launch — Inbox selected, message list
focused.

```
╭─ 󰇰 Inbox ─────────────────────────────────────────────────────────────────────────────────────────────────────╮
│←────────── 30 ──────────→│←───────────────────────────── remaining ──────────────────────────────────────────→│
│                           │                                                                                   │
│ ┃ 󰇰 Inbox              3 │  󰇮  Alice Johnson            Re: Project update for Q2 launch         10:32 AM   │
│   󰏫 Drafts               │  󰇮  Bob Smith                 Weekly standup notes                      9:15 AM   │
│   󰑚 Sent                 │  󰑚  Carol White               Re: Budget review                       Yesterday   │
│   󰀼 Archive              │      Dave Chen                 Meeting minutes from Monday                Apr 07   │
│                           │  󰈻  Eve Martinez              Quarterly report draft                     Apr 06   │
│   󰍷 Spam             12  │      Frank Lee                 Re: Server migration plan                  Apr 05   │
│   󰩺 Trash                │      ├─ Grace Kim              └─ Re: Server migration plan               Apr 05   │
│                           │      │  └─ Frank Lee              Re: Server migration plan               Apr 05   │
│   󰂚 Notifications        │      Hannah Park               New office supplies order                  Apr 04   │
│   󰑴 Remind               │      Ivan Petrov                Conference travel request                 Apr 03   │
│   󰡡 Lists/golang         │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
├───────────────────────────┴───────────────────────────────────────────────────────────────────────────────────┤
│ 󰇰 Inbox · 10 messages · 2 unread                                                                  ● connected│
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ d:del  a:archive  s:star  r:reply  R:all  f:fwd  c:compose  /:search  ?:help  ::cmd                          │
╰───────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

**Annotations:**

- **Tab bar** (row 1): Active tab shows folder icon + name.
  `accent_secondary` text on `bg_base`. Inactive tabs use `fg_dim`
  on `bg_elevated`.
- **Sidebar** (left, 30 cols): Three folder groups separated by
  blank lines. Selected row has `┃` thick left border in
  `accent_primary` + `bg_selection` full-width fill. Unread counts
  right-aligned in `accent_tertiary`, shown only when > 0.
- **Message list** (right, remaining width): Columns — flags (2),
  sender (22), subject (fill), date (12). Double-space separator.
  Unread rows in `accent_tertiary` with bold sender. Read rows in
  `fg_dim`. Thread prefixes use box-drawing `├─ └─ │`.
- **Vertical divider**: `│` between panels in `bg_border`.
- **Status bar**: `fg_bright` on `bg_border`. Folder icon + name,
  message count, unread count. Connection indicator right-aligned.
- **Command footer**: Key in `fg_bright` bold, `:` separator and
  hint text in `fg_dim`. Context = message list.
- **Focus**: Message list focused (sidebar selection shown but
  without the `┃` active border). `Tab` cycles focus between panels.

---

## 2. Tab Bar (#18)

### Single account tab (default on launch)

```
╭─ 󰇰 Inbox ─────────────────────────────────────────────────────────────────────────────────────────────────────╮
```

### Multiple tabs (viewer open)

```
╭─ 󰇰 Inbox ─┬─ Re: Project update for Q2 launch ─┬─────────────────────────────────────────────────────────────╮
```

### Three tabs (two viewers open)

```
╭─ 󰇰 Inbox ─┬─ Re: Project update ─┬─ Budget review ─┬──────────────────────────────────────────────────────────╮
```

**Annotations:**

- **Active tab:** `accent_secondary` text on `bg_base`. Bottom
  border merges with content area (no visible line between tab
  and content).
- **Inactive tabs:** `fg_dim` text on `bg_elevated`. Visible
  bottom border separating tab from content.
- **Tab separator:** `┬` where tab borders meet the top edge.
  `─` fills remaining space to the right edge.
- **Account folder tabs:** Show folder icon + folder name.
  Not closeable — always present. Title updates when folder
  changes (e.g., switch from Inbox to Sent).
- **Viewer tabs:** Show message subject, truncated to fit.
  Closeable with `q`.
- **Numeric switching:** `1-9` keys switch to tab by position.
- **Overflow:** If tabs exceed terminal width, rightmost tabs
  are truncated with `…`. Active tab is always fully visible.

---

## 3. Sidebar (#1 — left panel)

### Focused, Inbox selected

```
 ┃ 󰇰 Inbox              3
   󰏫 Drafts
   󰑚 Sent
   󰀼 Archive

   󰍷 Spam             12
   󰩺 Trash

   󰂚 Notifications
   󰑴 Remind
   󰡡 Lists/golang
   󰡡 Lists/rust
```

### Focused, selection in Disposal group

```
   󰇰 Inbox              3
   󰏫 Drafts
   󰑚 Sent
   󰀼 Archive

   󰍷 Spam             12
 ┃ 󰩺 Trash

   󰂚 Notifications
   󰑴 Remind
   󰡡 Lists/golang
```

### Unfocused (message list has focus)

```
   󰇰 Inbox              3
   󰏫 Drafts
   󰑚 Sent
   󰀼 Archive

   󰍷 Spam             12
   󰩺 Trash

   󰂚 Notifications
   󰑴 Remind
```

The selected folder (Inbox) still has `bg_selection` background
but no `┃` border — the border only appears in the focused state.

**Annotations:**

- **Width:** 30 columns fixed.
- **Selected row (focused):** `┃` thick left border in
  `accent_primary` + full-width `bg_selection` background.
  Folder name in `fg_bright`.
- **Selected row (unfocused):** `bg_selection` background only,
  no `┃` border. Folder name in `fg_base`.
- **Unread counts:** Right-aligned in `accent_tertiary`. Only
  shown when > 0.
- **Folder icons:** Nerd Font in `fg_base`. When folder has
  unread messages, icon switches to `accent_tertiary`.
- **Group spacing:** One blank line between Primary, Disposal,
  and Custom groups. No group headers rendered.
- **Scrolling:** If folders exceed panel height, viewport clips
  with j/k scrolling. No scrollbar.
- **Footer (when focused):**
  `Enter:open  c:compose  ::cmd`

---

## 4. Message List (#1 — right panel)

### Default with cursor and threading

```
 󰇮  Alice Johnson            Re: Project update for Q2 launch          10:32 AM
▐󰇮  Bob Smith                 Weekly standup notes                       9:15 AM
 󰑚  Carol White               Re: Budget review                        Yesterday
     Dave Chen                 Meeting minutes from Monday                 Apr 07
 󰈻  Eve Martinez              Quarterly report draft                      Apr 06
     Frank Lee                 Re: Server migration plan                   Apr 05
     ├─ Grace Kim              └─ Re: Server migration plan                Apr 05
     │  └─ Frank Lee              Re: Server migration plan                Apr 05
     Hannah Park               New office supplies order                   Apr 04
     Ivan Petrov                Conference travel request                  Apr 03
```

### Column layout

```
←2→  ←──────── 22 ────────→  ←──────── fill ─────────────────────→  ←── 12 ──→
 FL  SENDER                   SUBJECT                                 DATE
```

No column header row is rendered in the actual UI — the header
above is for wireframe reference only.

**Annotations:**

- **Cursor:** `▐` right-half block in `accent_primary` at left
  edge of current row + full-width `bg_selection` background.
- **Columns:** flags (2), sender (22), subject (fill), date (12).
  Double-space column separator.
- **Unread rows:** `󰇮` envelope icon in flags column. Sender in
  `accent_tertiary` bold. Subject in `accent_tertiary`.
- **Read rows:** No flag icon (blank). Sender and subject in
  `fg_dim`.
- **Replied:** `󰑚` reply icon in `color_special`.
- **Flagged:** `󰈻` flag icon in `color_warning`.
- **Thread prefixes:** Rendered in subject column. `├─`
  has-siblings, `└─` last-sibling, `│` stem. Thread chars
  in `fg_dim`.
- **Date format:** Today = time (`10:32 AM`), this week =
  `Yesterday`/day name, older = `Mon DD`, previous year =
  `Mon DD, YYYY`. Right-aligned.
- **Sender truncation:** Long names truncated with `…` at
  column boundary.
- **Sort:** Newest first by default. Inbox/Notifications
  override to oldest first (chronological).

---

## 5. Message Viewer (#2)

Full-width viewer in its own tab. No sidebar — the viewer uses
the entire content area.

```
╭─ 󰇰 Inbox ─┬─ Re: Project update for Q2 launch ─┬─────────────────────────────────────────────────────────────╮
│                                                                                                               │
│  From:     Alice Johnson <alice@example.com>                                                                  │
│  To:       Geoff Wright <geoff@907.life>                                                                      │
│  Date:     Thu, 10 Apr 2026 10:32:07 -0600                                                                    │
│  Subject:  Re: Project update for Q2 launch                                                                   │
│  ─────────────────────────────────────────────────────────────────────────────────────────────────────         │
│                                                                                                               │
│  Hey Geoff,                                                                                                   │
│                                                                                                               │
│  Just wanted to follow up on the Q2 launch timeline. I've attached the                                        │
│  updated project plan with the revised milestones.                                                            │
│                                                                                                               │
│  ## Key changes                                                                                               │
│                                                                                                               │
│  - Beta release moved to April 15                                                                             │
│  - QA window extended by one week                                                                             │
│  - Launch date is now May 1                                                                                   │
│                                                                                                               │
│  Let me know if you have any concerns about the new timeline.                                                 │
│                                                                                                               │
│  > On Apr 9, 2026, Geoff Wright wrote:                                                                        │
│  > Can you send me the updated project plan? I want to review the                                              │
│  > milestones before our meeting on Friday.                                                                    │
│                                                                                                               │
│  Best,                                                                                                        │
│  Alice                                                                                                        │
│                                                                                                               │
│                                                                                                               │
│                                                                                                               │
│                                                                                                               │
│                                                                                                               │
│                                                                                                               │
│                                                                                                               │
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Re: Project update for Q2 launch · 100%                                                            ● connected│
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ d:del  a:archive  s:star  r:reply  R:all  f:fwd  Tab:links  q:close                                          │
╰───────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

**Annotations:**

- **Full width:** Viewer uses entire content area (no sidebar
  split). Opens in a new tab — the account folder tab remains
  available via `1` or tab switching.
- **Header block:** Rendered by the existing header filter
  (shared with mailrender). Keys in `accent_primary` bold,
  values in `fg_base`, `<email>` in angle brackets in `fg_dim`.
  Separator `─` line in `fg_dim` below headers.
- **Body:** Rendered by the content pipeline (`ParseBlocks` →
  `RenderBody`). Lipgloss styles from compiled theme. Headings
  in `color_success` bold. Blockquotes prefixed with `>` in
  `accent_tertiary` (level 1) or `fg_dim` (level 2+). Links
  in `accent_primary` underline.
- **Viewport:** `bubbles/viewport`. Scroll percentage in status
  bar. `j/k` lines, `C-d/C-u` half page, `C-f/C-b` full page,
  `gg/G` top/bottom.
- **Left margin:** 2-char indent for body content readability.
- **Status bar:** Message subject + scroll percentage.
- **Footer:** Viewer-specific bindings. `Tab:links` replaces
  `/:search`. `q:close` added. No `?:help` — `?` still works
  but isn't shown (footer space is precious).

---

## 6. Keybinding Help Popover (#7)

Modal overlay triggered by `?` in any context. Centered on screen
with dimmed content behind. Content changes per context.

### Message list context

```
                  ╭─ Message List ──────────────────────────────────────────╮
                  │                                                         │
                  │  Navigate           Triage          Reply               │
                  │  j/k  up/down       d  delete       r  reply            │
                  │  gg   top           D  delete…      R  all              │
                  │  G    bottom        a  archive      f  forward          │
                  │  C-d  half pg dn    A  archive…     c  compose          │
                  │  C-u  half pg up    s  star                             │
                  │  C-f  page dn       .  read/unrd                        │
                  │  C-b  page up                                           │
                  │                                                         │
                  │  Search             Select          Threads             │
                  │  /    search        v  select       zo  unfold          │
                  │  n    next          ␣  toggle       zc  fold            │
                  │  N    prev                          za  toggle          │
                  │                                                         │
                  │  Go To                                                  │
                  │  gi  inbox    gd  drafts    gs  sent                    │
                  │  ga  archive  gx  spam      gt  trash                   │
                  │                                                         │
                  │  Enter  open        :  command       ?  close           │
                  │                                                         │
                  ╰─────────────────────────────────────────────────────────╯
```

### Viewer context

```
                  ╭─ Message Viewer ────────────────────────────────────────╮
                  │                                                         │
                  │  Navigate           Triage          Reply               │
                  │  j/k  scroll        d  delete       r  reply            │
                  │  gg   top           a  archive      R  all              │
                  │  G    bottom        s  star         f  forward          │
                  │  C-d  half pg dn                    c  compose          │
                  │  C-u  half pg up                                        │
                  │  C-f  page dn                                           │
                  │  C-b  page up                                           │
                  │                                                         │
                  │  Tab  link picker   q  close        ?  close            │
                  │                                                         │
                  ╰─────────────────────────────────────────────────────────╯
```

### Sidebar context

```
                  ╭─ Sidebar ───────────────────────────────────────────────╮
                  │                                                         │
                  │  Navigate           Go To                               │
                  │  j/k  up/down       gi  inbox      gd  drafts          │
                  │  gg   top           gs  sent       ga  archive         │
                  │  G    bottom        gx  spam       gt  trash           │
                  │                                                         │
                  │  Enter  open        c  compose      ?  close           │
                  │                                                         │
                  ╰─────────────────────────────────────────────────────────╯
```

**Annotations:**

- **Modal overlay:** Centered horizontally and vertically. Content
  behind is dimmed (lipgloss reduced-opacity background).
- **Border:** Rounded corners in `bg_border`.
- **Title:** Context name in `accent_primary` bold, embedded in
  top border.
- **Group headings:** `fg_bright` bold (Navigate, Triage, etc.).
- **Key column:** `fg_bright` bold. Fixed width within each column.
- **Description column:** `fg_dim`. Left-aligned within group.
- **Layout:** Three groups per row where content fits. Groups
  separated by whitespace, no divider lines.
- **Close:** `?` or `Escape`. Both close the popover.
- **Input routing:** All keypresses route to popover when open.
  Only `?` and `Escape` are handled; everything else is ignored.
- **Size constraint:** Must fit on screen without scrolling. If
  too many bindings, prune — this constraint forces curation.

---

## 7. Transient UI (#8, #9, #10, #11, #12)

All transient elements render in the status bar area (between
content and command footer). Only one transient element at a time.
Priority: error banner > undo bar > toast > normal status.

### Status toast (#8)

Auto-dismissing feedback after an action. Replaces normal status
bar content for 3 seconds.

```
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ✓ Archived 1 message                                                                               ● connected│
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

Toast variants by action type:

```
✓ Archived 1 message                color_success
✓ Message sent                      color_success
✓ Draft saved                       color_success
󰈻 Flagged                           color_warning
󰇮 Marked unread                     accent_tertiary
```

### Undo bar (#9)

Replaces status bar for reversible destructive actions. Action is
deferred — not executed until the 5-second window expires.

```
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Deleted 1 message · press u to undo                                                                     [5s] │
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

```
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Deleted 3 messages · press u to undo                                                                    [3s] │
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

### Error banner (#10)

Persistent — does not auto-dismiss. Cleared by keypress or
condition resolving (e.g., reconnection succeeds).

```
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ✗ Connection lost — reconnecting…                                                                             │
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

```
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ✗ Send failed: SMTP authentication error                                                                      │
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

### Loading spinner (#11)

Centered in content area while fetching data. Uses `bubbles/spinner`
with braille dot pattern.

#### Message list (fetching headers)

```
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                        ⣾ Loading messages…                                        │
│                           │                                                                                   │
│                           │                                                                                   │
```

#### Viewer (fetching body)

```
│                                                                                                               │
│                                                                                                               │
│                                    ⣾ Loading message…                                                         │
│                                                                                                               │
│                                                                                                               │
```

### Connection status (#12)

Persistent indicator at the right edge of the status bar.

```
│ 󰇰 Inbox · 10 messages · 2 unread                                                                  ● connected│
│ 󰇰 Inbox · 10 messages · 2 unread                                                             ◌ reconnecting… │
│ 󰇰 Inbox · 10 messages · 2 unread                                                                  ○ offline  │
```

**Annotations:**

- **Toast (#8):** `tea.Tick` auto-dismiss after 3s. Icon + message
  text. Color varies by action type (see variants above).
- **Undo bar (#9):** `u` key undoes the action and shows a toast
  confirming. Countdown `[5s]` right-aligned in `fg_dim`, counts
  down each second. Text in `fg_base` on `bg_elevated`.
- **Error banner (#10):** `color_error` text. `✗` prefix. Persists
  until dismissed by any keypress or the underlying condition clears.
- **Spinner (#11):** `bubbles/spinner` with braille dot style
  (`⣾⣽⣻⢿⡿⣟⣯⣷`). Centered in content area. Spinner char + label
  in `fg_dim`.
- **Connection (#12):** Right-aligned in status bar. Always visible.
  `●` connected = `color_success`. `◌` reconnecting = `color_warning`
  (with inline spinner). `○` offline = `fg_dim`.

---

## 8. Screen States (#13, #14, #15, #16, #17)

### Empty folder (#13)

Centered placeholder when a folder has no messages.

```
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                          No messages                                               │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
│                           │                                                                                   │
```

### Threaded view — expanded (#14)

Default state. All thread children visible with box-drawing
prefixes.

```
     Eve Martinez              Re: Server migration plan                   Apr 05
     ├─ Grace Kim              └─ Re: Server migration plan                Apr 05
     │  └─ Frank Lee              Re: Server migration plan                Apr 05
```

### Threaded view — collapsed (#14)

Thread folded with `zc`. Shows message count badge.

```
     Eve Martinez           [3] Re: Server migration plan                  Apr 05
```

### Threaded view — partially collapsed (#14)

A mid-thread node folded, root still expanded.

```
     Eve Martinez              Re: Server migration plan                   Apr 05
     ├─ Grace Kim           [2] └─ Re: Server migration plan               Apr 05
```

### Search results (#15)

Search query and result count shown in status bar. Message list
filters to matching messages only.

```
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ 󰍉 search: "project update" · 3 results                                                            ● connected│
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

`n/N` jump between results. `:clear` restores the full list.

### Multi-select (#16)

`v` enters visual select mode. `Space` toggles individual rows.
Selected messages show a check icon in the flags column.

```
 󰇮   Alice Johnson            Re: Project update for Q2 launch         10:32 AM
 󰇮  󰄬 Bob Smith                Weekly standup notes                      9:15 AM
 󰑚  󰄬 Carol White              Re: Budget review                       Yesterday
      Dave Chen                 Meeting minutes from Monday                Apr 07
 󰈻  󰄬 Eve Martinez             Quarterly report draft                    Apr 06
```

Status bar and footer swap to bulk mode:

```
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ 3 selected                                                                                                    │
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Space:toggle  d:del all  a:archive all  v:cancel  Esc:cancel                                                  │
╰───────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

### Focused panel cycling (#17)

`Tab` toggles focus between sidebar and message list. The focused
panel receives j/k navigation. Visual difference is the `┃`
selection border.

```
Sidebar focused:                    Message list focused:
┌──────────┬──────────────────┐     ┌──────────┬──────────────────┐
│ ┃ Inbox  │  Alice Johnson   │     │   Inbox  │▐ Alice Johnson   │
│   Sent   │  Bob Smith       │     │   Sent   │  Bob Smith       │
│   Trash  │  Carol White     │     │   Trash  │  Carol White     │
└──────────┴──────────────────┘     └──────────┴──────────────────┘
  ↑ j/k navigate here                           ↑ j/k navigate here
```

**Annotations:**

- **Empty folder (#13):** "No messages" text in `fg_dim`. Centered
  horizontally and vertically in the message list panel.
- **Thread collapse (#14):** `zo` unfold, `zc` fold, `za` toggle.
  Collapsed thread shows `[N]` count in `fg_dim` before subject.
  Thread root always visible. Count includes root.
- **Search (#15):** `󰍉` search icon in `color_info`. Query text
  in `fg_bright`. Result count in `fg_dim`. Status bar retains
  connection indicator. `:clear` restores normal view.
- **Multi-select (#16):** `󰄬` check icon in `color_success` on
  selected rows. Selected rows get `bg_selection` background.
  Status bar shows count. Footer swaps to bulk actions. `Esc`
  or `v` exits multi-select mode, deselecting all.
- **Focus cycling (#17):** `Tab` key. Focused panel shows `┃`
  (sidebar) or `▐` (message list) on the selected row. Unfocused
  panel shows `bg_selection` background only, no border indicator.
  j/k only operates in the focused panel.

---

## 9. Overlays (#4, #5, #6)

### Compose review (#4)

Inline prompt in the status bar after the editor exits with code 0.
Blocks all other input until answered.

```
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Send message?  y:send  n:abort  e:edit  p:postpone                                                            │
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

### Folder picker (#5)

Modal overlay for `:move` and `:copy`. Fuzzy-filtered folder list.

#### Empty query (all folders shown)

```
                       ╭─ Move to folder ────────────────────────╮
                       │                                          │
                       │  >                                       │
                       │                                          │
                       │  ┃ 󰇰 Inbox                               │
                       │    󰏫 Drafts                               │
                       │    󰑚 Sent                                 │
                       │    󰀼 Archive                              │
                       │    󰍷 Spam                                 │
                       │    󰩺 Trash                                │
                       │    󰂚 Notifications                        │
                       │    󰑴 Remind                               │
                       │    󰡡 Lists/golang                         │
                       │    󰡡 Lists/rust                           │
                       │                                          │
                       ╰──────────────────────────────────────────╯
```

#### Filtered results

```
                       ╭─ Move to folder ────────────────────────╮
                       │                                          │
                       │  > arch                                  │
                       │                                          │
                       │  ┃ 󰀼 Archive                             │
                       │    󰡡 Lists/arch-linux                    │
                       │                                          │
                       ╰──────────────────────────────────────────╯
```

### Confirm delete (#6)

Inline prompt in status bar for bulk delete (3+ messages).
Single-message delete skips this and uses the undo bar instead.

```
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Delete 5 messages?  y:confirm  n:cancel                                                                       │
├───────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

**Annotations:**

- **Compose review (#4):** Status bar prompt, not a modal. Keys in
  `fg_bright` bold, hints in `fg_dim`. Blocks all input. Pass 9.
- **Folder picker (#5):** Modal overlay, centered. Dimmed background.
  `>` prefix on `bubbles/textinput` filter line. Results update
  as you type (fuzzy match on folder name). `j/k` or arrows move
  selection. `Enter` confirms, `Escape` cancels. Selected row has
  `┃` left border + `bg_selection`. Rounded border in `bg_border`.
  Title shows action ("Move to folder" / "Copy to folder") in
  `accent_primary`. Picker shrinks to fit results (no fixed height).
  Pass 7.
- **Confirm delete (#6):** Status bar prompt. Count in
  `color_warning`. Only for 3+ messages. Single-message delete is
  instant with undo bar (#9). Pass 6.
