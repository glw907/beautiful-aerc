# Poplar Text Wireframes

Reference wireframes for every UI element in the poplar interface
inventory. Each wireframe defines layout, proportions, and
information density for the bubbletea prototype (Pass 2.5b).

See the UI design spec for the complete interface inventory:
`docs/superpowers/specs/2026-04-10-poplar-ui-wireframing-design.md`

## Conventions

- Box-drawing characters for borders: `╭╮╰╯│─┃`
- `┃` thick left bar for selected row indicator
- Nerd Font glyphs rendered directly (2-cell wide in terminal)
- Color annotations use theme slot names (`accent_primary`, `fg_dim`)
- Default terminal: 120 columns x 40 rows
- `←N→` for column widths
- `[key]` for interactive elements
- Three-sided frame: top `──┬──╮`, right `│`, bottom `──┴──╯`. No left border.

---

## 1. Composite Layout

Full application with all persistent chrome and both panels visible.
No tab bar — sidebar provides folder context. Inbox selected.

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

**Annotations:**

- **No tab bar**: Removed entirely. The sidebar (always visible)
  shows folder context. Folder name and message counts are in the
  status bar. No `1-9` switching or tab lifecycle.
- **Three-sided frame**: Top edge `───┬───╮` (left of divider is
  open, right closes with `╮`). Right border `│`. Bottom status
  bar `──┴──╯`. No left border — left edge is open.
- **Sidebar** (left, 30 cols): Account name (`geoff@907.life`) at
  top in `fg_dim`. Three folder groups separated by blank lines.
  Selected row has `┃` thick left border in `accent_primary` +
  `bg_selection` full-width fill. Unread counts right-aligned in
  `accent_tertiary`, shown only when > 0.
- **Message list** (right, remaining width): Columns — flags (2),
  sender (22), subject (fill), date (12). Double-space separator.
  Unread rows in `accent_tertiary` with bold sender. Read rows in
  `fg_dim`. Thread prefixes use box-drawing `├─ └─ │`.
- **Vertical divider**: `│` between panels in `bg_border`.
- **Status bar**: Bottom frame edge. `fg_bright` on `bg_border`.
  Message count, unread count, connection indicator right-aligned.
  Closes frame with `╯`.
- **Command footer**: Below the status bar frame edge. Key in
  `fg_bright` bold, `:` separator and hint text in `fg_dim`.
  Groups separated by `┊` in `fg_dim`. Single account context —
  j/k messages, J/K folders, triage and reply always live.
- **One pane (like pine)**: No Tab focus cycling. The `┃` selection
  indicator always renders on the selected folder. Every key is
  dispatched by identity, not by "which panel is active".

---

## 2. Sidebar (#1 — left panel)

### Inbox selected

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

### Selection in Disposal group

```
   󰇰  Inbox             3
   󰏫  Drafts
   󰑚  Sent
   󰀼  Archive

   󰍷  Spam            12
 ┃ 󰩺  Trash

   󰂚  Notifications
   󰑴  Remind
   󰡡  Lists/golang
```

**Annotations:**

- **Width:** 30 columns fixed.
- **Selected row:** `┃` thick left border in `accent_secondary`
  + full-width `bg_selection` background. Folder name in
  `fg_bright`. The `┃` is always shown — no focus state because
  the screen is one pane (like pine).
- **Unread counts:** Right-aligned in `accent_tertiary`. Only
  shown when > 0.
- **Folder icons:** Nerd Font in `fg_base`. When folder has
  unread messages, icon switches to `accent_tertiary`.
- **Group spacing:** One blank line between Primary, Disposal,
  and Custom groups. No group headers rendered.
- **Scrolling:** If folders exceed panel height, viewport clips
  with J/K scrolling. No scrollbar.

---

## 3. Message List (#1 — right panel)

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

## 4. Message Viewer (#2)

Viewer opens in the right panel with sidebar still visible. `q`
returns to the message list — no tab switching needed.

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

**Annotations:**

- **No tab bar:** Viewer opens in the right panel — no new tab
  created. Sidebar remains visible. `q` returns to message list.
  No `1-9` switching or tab lifecycle.
- **Sidebar:** Still visible and showing current folder selection
  with the usual `┃` + `bg_selection` row.
- **Message body:** 72-char fixed width (same as mailrender render
  width). Content pipeline (`ParseBlocks` → `RenderBody`). Headings
  in `color_success` bold. Blockquotes in `accent_tertiary` (level
  1) or `fg_dim` (level 2+). Links in `accent_primary` underline.
- **Header block:** Keys in `accent_primary` bold, values in
  `fg_base`, `<email>` in angle brackets in `fg_dim`. Separator
  `─` line in `fg_dim` below headers.
- **Viewport:** `bubbles/viewport`. Scroll percentage in status
  bar. `j/k` lines, `C-d/C-u` half page, `C-f/C-b` full page,
  `G` bottom.
- **Status bar:** Bottom frame edge. Scroll percentage + connection
  indicator. Closes frame with `╯`.
- **Footer:** Viewer-specific bindings. `Tab:links` opens link
  picker. `q:close` returns to message list. Groups separated by
  `┊`.

---

## 5. Keybinding Help Popover (#7)

Modal overlay triggered by `?` in any context. Centered on screen
with dimmed content behind. Content changes per context.

### Message list context

```
                  ╭─ Message List ──────────────────────────────────────────╮
                  │                                                         │
                  │  Navigate           Triage          Reply               │
                  │  j/k  up/down       d  delete       r  reply            │
                  │  G    bottom        a  archive      R  all              │
                  │  C-d  half pg dn    s  star         f  forward          │
                  │  C-u  half pg up    .  read/unrd    c  compose          │
                  │  C-f  page dn                                           │
                  │  C-b  page up                                           │
                  │                                                         │
                  │  Search             Select          Threads             │
                  │  /    search        v  select       …  fold (TBD)      │
                  │  n    next          ␣  toggle                           │
                  │  N    prev                                              │
                  │                                                         │
                  │  Go To                                                  │
                  │  I  inbox    D  drafts    S  sent                       │
                  │  A  archive  X  spam      T  trash                      │
                  │                                                         │
                  │  Enter  open        ?  close                            │
                  │                                                         │
                  ╰─────────────────────────────────────────────────────────╯
```

### Viewer context

```
                  ╭─ Message Viewer ────────────────────────────────────────╮
                  │                                                         │
                  │  Navigate           Triage          Reply               │
                  │  j/k  scroll        d  delete       r  reply            │
                  │  G    bottom        a  archive      R  all              │
                  │  C-d  half pg dn    s  star         f  forward          │
                  │  C-u  half pg up                    c  compose          │
                  │  C-f  page dn                                           │
                  │  C-b  page up                                           │
                  │                                                         │
                  │  Tab  link picker   q  close        ?  close            │
                  │                                                         │
                  ╰─────────────────────────────────────────────────────────╯
```

### Sidebar context *(out of date — merged into account context)*

Under the one-pane decision (architecture.md, Pass 2.5b-2
refinement), there is no separate sidebar focus. Every key is
always live: `j/k` navigates messages, `J/K` navigates folders.
The help popover has only two contexts — account and viewer.
This mockup is preserved as a reference until Pass 2.5b-5
(help popover prototype) rebuilds the popover layout, at which
point the sidebar section should be removed entirely.

```
                  ╭─ Sidebar ───────────────────────────────────────────────╮
                  │                                                         │
                  │  Navigate           Go To                               │
                  │  J/K  up/down       I  inbox     D  drafts              │
                  │  G    bottom        S  sent      A  archive             │
                  │                     X  spam      T  trash               │
                  │                                                         │
                  │  Enter  open        c  compose      ?  close            │
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

## 6. Transient UI (#8, #9, #10, #11, #12)

All transient elements render in the status bar area (between
content and command footer). Only one transient element at a time.
Priority: error banner > undo bar > toast > normal status.

### Status toast (#8)

Auto-dismissing feedback after an action. Toast appears inline in the
top frame line at the right side for 3 seconds.

```
───────────────────────────┬──────────────────────────────────────── ✓ 3 archived ─╮
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

Replaces status bar content for reversible destructive actions. Action
is deferred — not executed until the 5-second window expires.

```
 ──────────────────────────┴────────────── Deleted 1 message · press u to undo · [5s] ─╯
```

```
 ──────────────────────────┴─────────────── Deleted 3 messages · press u to undo · [3s] ─╯
```

### Error banner (#10)

Persistent — does not auto-dismiss. Cleared by keypress or
condition resolving (e.g., reconnection succeeds).

```
 ──────────────────────────┴──────────────────── ✗ Connection lost — reconnecting… ─╯
```

```
 ──────────────────────────┴─────────────────── ✗ Send failed: SMTP authentication error ─╯
```

### Loading spinner (#11)

Centered in content area while fetching data. Uses `bubbles/spinner`
with braille dot pattern.

#### Message list (fetching headers)

```
│                         │                                                                               │
│                         │                                                                               │
│                         │                     ⣾ Loading messages…                                       │
│                         │                                                                               │
│                         │                                                                               │
```

#### Viewer (fetching body)

```
│                                                                                                          │
│                                                                                                          │
│                                  ⣾ Loading message…                                                      │
│                                                                                                          │
│                                                                                                          │
```

### Connection status (#12)

Persistent indicator at the right edge of the status bar frame edge.
Uses shape + color + text for colorblind accessibility.

```
 ──────────────────────────┴──────────────────── 10 messages · 2 unread · ● connected ─╯
 ──────────────────────────┴──────────────── 10 messages · 2 unread · ◐ reconnecting… ─╯
 ──────────────────────────┴──────────────────── 10 messages · 2 unread · ○ offline   ─╯
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
- **Connection (#12):** Right-aligned in status bar frame edge. Always
  visible. Triple redundancy (shape + color + text) for colorblind
  accessibility. `●` filled = connected (`color_success`). `◐` half =
  reconnecting (`color_warning`). `○` hollow = offline (`fg_dim`).

---

## 7. Screen States (#13, #14, #15, #16, #17)

### Empty folder (#13)

Centered placeholder when a folder has no messages.

```
│                         │                                                                               │
│                         │                                                                               │
│                         │                                                                               │
│                         │                                                                               │
│                         │                                                                               │
│                         │                       No messages                                              │
│                         │                                                                               │
│                         │                                                                               │
│                         │                                                                               │
│                         │                                                                               │
│                         │                                                                               │
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

Thread folded via the fold key (TBD — see Pass 2.5b-3.5
brainstorm). Shows message count badge.

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
 ──────────────────────────┴───────────────── 󰍉  search: "project update" · 3 results · ● connected ─╯
```

`n/N` jump between results. `Esc` restores the full list.

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
 ──────────────────────────┴──────────────────────────────────────────────── 3 selected ─╯
  Space:toggle  d:del all  a:archive all  v:cancel  Esc:cancel
```

**Annotations:**

- **Empty folder (#13):** "No messages" text in `fg_dim`. Centered
  horizontally and vertically in the message list panel.
- **Thread collapse (#14):** Fold key TBD — the original
  `zo`/`zc`/`za` proposal violates the no-multikey rule
  (architecture.md). Candidates under discussion are `Tab`
  and `Space`; final choice is pending the Pass 2.5b-3.5
  brainstorm. Fold-all / unfold-all may ship in this pass or
  be deferred. Collapsed thread shows `[N]` count in `fg_dim`
  before subject. Thread root always visible. Count includes
  root.
- **Search (#15):** `󰍉` search icon in `color_info`. Query text
  in `fg_bright`. Result count in `fg_dim`. Status bar retains
  connection indicator. Search is cleared with `Esc`
  (`:` command mode was dropped — no `:clear` command).
- **Multi-select (#16):** `󰄬` check icon in `color_success` on
  selected rows. Selected rows get `bg_selection` background.
  Status bar shows count. Footer swaps to bulk actions. `Esc`
  or `v` exits multi-select mode, deselecting all.
---

## 8. Overlays (#4, #5, #6)

### Compose review (#4)

Inline prompt in the status bar after the editor exits with code 0.
Blocks all other input until answered.

```
 ──────────────────────────┴─────────────────── Send message?  y:send  n:abort  e:edit  p:postpone ─╯
```

### Folder picker (#5)

Modal overlay for move/copy actions. Invoked by a key (key
assignment TBD — originally documented as `:move`/`:copy`
commands before `:` command mode was dropped). Fuzzy-filtered
folder list.

#### Empty query (all folders shown)

```
                       ╭─ Move to folder ────────────────────────╮
                       │                                          │
                       │  >                                       │
                       │                                          │
                       │  ┃ 󰇰  Inbox                              │
                       │    󰏫  Drafts                              │
                       │    󰑚  Sent                                │
                       │    󰀼  Archive                             │
                       │    󰍷  Spam                                │
                       │    󰩺  Trash                               │
                       │    󰂚  Notifications                       │
                       │    󰑴  Remind                              │
                       │    󰡡  Lists/golang                        │
                       │    󰡡  Lists/rust                          │
                       │                                          │
                       ╰──────────────────────────────────────────╯
```

#### Filtered results

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

### Confirm delete (#6)

Inline prompt in status bar for bulk delete (3+ messages).
Single-message delete skips this and uses the undo bar instead.

```
 ──────────────────────────┴────────────────────────────── Delete 5 messages?  y:confirm  n:cancel ─╯
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

---

## 9. Compose — External Editor (#3)

Not a poplar screen. Bubbletea suspends via `tea.ExecProcess`,
handing the terminal to the editor. Poplar disappears entirely
and reappears when the editor exits.

```
┌─────────────────────────────────────────────────────┐
│  Poplar running (bubbletea)                         │
│  User presses c (compose) or r (reply)              │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│  Poplar writes temp file:                           │
│  - Headers (To, From, Subject)                      │
│  - Quoted body (reply/forward)                      │
│  - Signature (if configured)                        │
│                                                     │
│  tea.ExecProcess($EDITOR, tempfile)                 │
│  Bubbletea suspends — terminal belongs to editor    │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│  Editor running (full screen, e.g., nvim-mail)      │
│  User composes message                              │
│                                                     │
│  :wq → exit code 0                                  │
│  :cq → exit code 1                                  │
└──────────────────────┬──────────────────────────────┘
                       │
                ┌──────┴──────┐
                ▼             ▼
          Exit code 0    Exit code ≠ 0
          ┌──────────┐   ┌───────────┐
          │ Compose  │   │ Toast:    │
          │ review   │   │ "Compose  │
          │ prompt   │   │  aborted" │
          │ (§9 #4)  │   └───────────┘
          └──────────┘
```

**Annotations:**

- **`tea.ExecProcess`:** Bubbletea's mechanism for handing terminal
  control to a child process. Event loop suspends, terminal restores,
  resumes on child exit.
- **Default editor:** `$EDITOR` (poplar default: `micro`). For this
  user: `nvim-mail`.
- **Temp file:** Created by poplar using `internal/compose/` for
  header formatting and quoted text reflow.
- **Exit code 0:** Triggers compose review prompt (§9, element #4).
- **Exit code ≠ 0:** Compose aborted. Toast "Compose aborted" in
  `fg_dim`. No review prompt.
- **Pass 9 implementation.**

---

## Coverage

All 19 UI elements from the interface inventory (Tab Bar removed):

| # | Element | Wireframe |
|---|---------|-----------|
| 1 | Folder + Message List | §1 Composite, §2 Sidebar, §3 Message List |
| 2 | Message Viewer | §4 Viewer |
| 3 | Compose (external) | §9 Compose |
| 4 | Compose Review | §8 Overlays |
| 5 | Folder Picker | §8 Overlays |
| 6 | Confirm Delete | §8 Overlays |
| 7 | Keybinding Help | §5 Help Popover |
| 8 | Status Toast | §6 Transient UI |
| 9 | Undo Bar | §6 Transient UI |
| 10 | Error Banner | §6 Transient UI |
| 11 | Loading Spinner | §6 Transient UI |
| 12 | Connection Status | §6 Transient UI |
| 13 | Empty Folder | §7 Screen States |
| 14 | Threaded View | §7 Screen States |
| 15 | Search Results | §7 Screen States |
| 16 | Multi-Select | §7 Screen States |
| 19 | Command Footer | §1 Composite (all wireframes). Grouped by function with `┊` separators. |
| 20 | Status Bar | §1 Composite (all wireframes). Bottom frame edge `──┴──╯`. |
