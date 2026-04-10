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
