# Poplar Text Wireframes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Draw monospace text wireframes for all 20 UI elements in the
poplar interface inventory, producing the reference drawings that
define layout, proportions, and information density for the bubbletea
prototype (Pass 2.5b).

**Architecture:** One wireframe document
(`docs/poplar/wireframes.md`) containing every screen, overlay,
transient element, and screen state from the interface inventory in
the UI design spec. Each wireframe is a fenced code block using
box-drawing characters, with annotations below explaining styling,
dimensions, and behavior.

**Deliverable:** `docs/poplar/wireframes.md` — reviewed and approved.

**Reference docs:**
- UI design spec: `docs/superpowers/specs/2026-04-10-poplar-ui-wireframing-design.md`
- Design spec: `docs/superpowers/specs/2026-04-09-poplar-design.md`
- Architecture: `docs/poplar/architecture.md`
- Styling: `docs/styling.md`
- aerc setup: `~/.claude/docs/aerc-setup.md`

---

## Conventions

All wireframes use these drawing conventions:

- **Box-drawing characters** for borders: `╭╮╰╯│─`
- **`█`** for thick selection borders
- **`·`** for placeholder/fill areas
- **Nerd Font icons** rendered as their actual glyphs where possible
- **Color annotations** reference theme slot names (e.g., `accent_primary`,
  `fg_dim`) not hex values
- **Terminal size:** 120×40 assumed default, noted where dimensions matter
- **`←N→`** notation for column widths
- **`[key]`** notation for interactive elements

Each wireframe has:
1. A fenced monospace code block
2. Annotations section explaining colors, dimensions, behavior
3. Cross-references to the UI spec element number

---

## Task 1: Document scaffold + composite wireframe

Create the wireframes document with the full-screen composite view
that shows how all persistent chrome elements fit together. This
establishes the spatial framework that all subsequent wireframes
reference.

**Files:**
- Create: `docs/poplar/wireframes.md`

- [ ] **Step 1: Create wireframes.md with header and composite wireframe**

Write the document header, conventions section, and the first
wireframe showing the full application layout with all chrome
elements labeled:

```markdown
# Poplar Text Wireframes

Reference wireframes for every UI element in the poplar interface
inventory. Each wireframe defines layout, proportions, and
information density for the bubbletea prototype (Pass 2.5b).

See `docs/superpowers/specs/2026-04-10-poplar-ui-wireframing-design.md`
for the complete interface inventory and design rationale.

## Conventions

- Box-drawing characters for borders: `╭╮╰╯│─`
- `█` for thick selection indicator
- Nerd Font glyphs rendered directly
- Color annotations use theme slot names (`accent_primary`, `fg_dim`)
- Default terminal: 120 columns × 40 rows
- `←N→` for column widths
- `[key]` for interactive elements

## 1. Composite Layout

Full application with all persistent chrome and both panels visible.
This is the default view on launch.

` ` `
╭─ 󰇰 Inbox ──────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│                                                                                                                    │
│ ←────── 30 ──────→│←──────────────────────────────── remaining ──────────────────────────────────→                  │
│                    │                                                                                                │
│  █ 󰇰 Inbox     3  │  ☐  flags ← sender ──────────→  subject ──────────────────────────→  date ──→                  │
│    󰏫 Drafts       │  ─────────────────────────────────────────────────────────────────────────────                  │
│    󰑚 Sent         │  󰇮  Alice Johnson               Re: Project update for Q2 launch    10:32 AM                  │
│    󰀼 Archive      │  󰇮  Bob Smith                    Weekly standup notes                 9:15 AM                  │
│                    │  󰑚  Carol White                  Re: Budget review                  Yesterday                  │
│    󰍷 Spam     12  │     Dave Chen                    Meeting minutes from Monday         Apr 07                    │
│    󰩺 Trash        │  󰈻  Eve Martinez                 Quarterly report draft              Apr 06                    │
│                    │     Frank Lee                    Re: Server migration plan           Apr 05                    │
│    󰂚 Notifications│  ├─ Grace Kim                    └─ Re: Server migration plan        Apr 05                    │
│    󰑴 Remind       │  │  └─ Frank Lee                    Re: Server migration plan        Apr 05                    │
│    󰡡 Lists/golang │     Hannah Park                  New office supplies order           Apr 04                    │
│                    │     Ivan Petrov                   Conference travel request           Apr 03                    │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
├────────────────────┴────────────────────────────────────────────────────────────────────────────────────────────────┤
│ 󰇰 Inbox · 10 messages · 2 unread                                                                                  │
├─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ d:del  a:archive  s:star  r:reply  R:all  f:fwd  c:compose  /:search  ?:help  ::cmd                               │
╰─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
` ` `

**Annotations:**

- **Tab bar** (row 1): Active tab shows folder icon + name.
  `accent_secondary` text on `bg_base`. Inactive tabs in `fg_dim`
  on `bg_elevated`.
- **Sidebar** (left panel, 30 cols): Three folder groups separated
  by blank lines. Selected row has `█` left border in
  `accent_primary` + `bg_selection` fill. Unread counts
  right-aligned in `accent_tertiary`.
- **Message list** (right panel, remaining width): Columns —
  flags (2), sender (22), subject (fill), date (12). Column
  separator: double space. Unread rows in `accent_tertiary`,
  read rows in `fg_dim`. Thread prefixes use box-drawing `├─└─│`.
- **Status bar**: `fg_bright` on `bg_border`. Shows folder icon,
  name, message count, unread count.
- **Command footer**: Key in `fg_bright` bold, separator `:` and
  hint in `fg_dim`. Context = message list.
- **Vertical divider**: Single `│` between sidebar and message
  list, in `bg_border` color.
- **Focus**: Message list panel focused (implied by thicker
  sidebar selection border being inactive style). Active panel
  determined by `focusedPanel` enum.
```

- [ ] **Step 2: Review against UI spec elements #1, #18, #19, #20**

Verify the composite wireframe covers:
- Element #1 (Folder + Message List) — sidebar and message list
- Element #18 (Tab Bar) — top row
- Element #19 (Command Footer) — bottom row
- Element #20 (Status Bar) — between content and footer

- [ ] **Step 3: Commit**

```bash
git add docs/poplar/wireframes.md
git commit -m "Add wireframes doc with composite layout (Pass 2.5a)"
```

---

## Task 2: Tab bar wireframes

Draw the tab bar in its various states: single account tab, multiple
tabs with a viewer open, and the tab overflow case.

**Files:**
- Modify: `docs/poplar/wireframes.md`

- [ ] **Step 1: Add tab bar wireframes**

Append to `docs/poplar/wireframes.md`:

```markdown
## 2. Tab Bar (#18)

### Single account tab (default)

` ` `
╭─ 󰇰 Inbox ──────────────────────────────────────────────────────────────────────────────────────────────────────────╮
` ` `

### Multiple tabs (viewer open)

` ` `
┃ 󰇰 Inbox ┃ Re: Project update for Q2 launch ┃                                                                      │
` ` `

### Tab with close indicator

` ` `
┃ 󰇰 Inbox ┃ Re: Project update ✕ ┃ Budget review ✕ ┃                                                                │
` ` `

**Annotations:**

- Active tab: `accent_secondary` text, `bg_base` background,
  visible bottom border connecting to content area.
- Inactive tabs: `fg_dim` text, `bg_elevated` background.
- Account folder tabs: No close indicator (not closeable).
- Viewer tabs: `✕` in `fg_dim`, brightens on hover/focus.
- Tab title: Folder icon + name for account tabs, message
  subject (truncated) for viewer tabs.
- Numeric switching: `1-9` keys switch to tab by position.
- Separator: `┃` between tabs in `bg_border`.
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/wireframes.md
git commit -m "Add tab bar wireframes"
```

---

## Task 3: Sidebar wireframes

Draw the sidebar showing folder groups, selection states, and the
focused vs unfocused appearance.

**Files:**
- Modify: `docs/poplar/wireframes.md`

- [ ] **Step 1: Add sidebar wireframes**

Append to `docs/poplar/wireframes.md`:

```markdown
## 3. Sidebar (#1 — left panel)

### Default state (focused)

` ` `
 █ 󰇰 Inbox                    3
   󰏫 Drafts
   󰑚 Sent
   󰀼 Archive

   󰍷 Spam                   12
   󰩺 Trash

   󰂚 Notifications
   󰑴 Remind
   󰡡 Lists/golang
   󰡡 Lists/rust
` ` `

### Selection on different group

` ` `
   󰇰 Inbox                    3
   󰏫 Drafts
   󰑚 Sent
   󰀼 Archive

   󰍷 Spam                   12
 █ 󰩺 Trash

   󰂚 Notifications
   󰑴 Remind
   󰡡 Lists/golang
` ` `

### Unfocused state (message list has focus)

` ` `
   󰇰 Inbox                    3
   󰏫 Drafts
   󰑚 Sent
   󰀼 Archive

   󰍷 Spam                   12
   󰩺 Trash

   󰂚 Notifications
   󰑴 Remind
` ` `

**Annotations:**

- **Width:** 30 columns fixed.
- **Selected row (focused):** `█` thick left border in
  `accent_primary` + full-width `bg_selection` background.
  Folder name in `fg_bright`.
- **Selected row (unfocused):** `bg_selection` background only,
  no `█` border. Folder name in `fg_base`.
- **Unread counts:** Right-aligned in `accent_tertiary`. Only
  shown when > 0.
- **Folder icons:** Nerd Font in `fg_base`. When folder has
  unread messages, icon in `accent_tertiary`.
- **Group spacing:** One blank line between Primary, Disposal,
  and Custom groups. No group headers rendered.
- **Scrolling:** If folders exceed panel height, viewport clips
  with j/k scrolling. No scrollbar.
- **Footer context:** When sidebar is focused, footer shows:
  `Enter:open  c:compose  ::cmd`
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/wireframes.md
git commit -m "Add sidebar wireframes"
```

---

## Task 4: Message list wireframes

Draw the message list showing column layout, threading, cursor
position, and read/unread styling.

**Files:**
- Modify: `docs/poplar/wireframes.md`

- [ ] **Step 1: Add message list wireframes**

Append to `docs/poplar/wireframes.md`:

```markdown
## 4. Message List (#1 — right panel)

### Default with cursor and threading

` ` `
 FL  SENDER                  SUBJECT                                                       DATE
 ──  ──────────────────────  ────────────────────────────────────────────────────────────  ──────────
 󰇮   Alice Johnson           Re: Project update for Q2 launch                              10:32 AM
▌󰇮   Bob Smith                Weekly standup notes                                           9:15 AM
 󰑚   Carol White              Re: Budget review                                            Yesterday
      Dave Chen                Meeting minutes from Monday                                    Apr 07
 󰈻   Eve Martinez             Quarterly report draft                                         Apr 06
      Frank Lee                Re: Server migration plan                                      Apr 05
      ├─ Grace Kim             └─ Re: Server migration plan                                   Apr 05
      │  └─ Frank Lee             Re: Server migration plan                                   Apr 05
      Hannah Park              New office supplies order                                      Apr 04
` ` `

### Column widths

` ` `
←1→←2→←22─────────────────→  ←── fill ──────────────────────────────────────────────────→  ←───12──→
 FL  SENDER                   SUBJECT                                                       DATE
` ` `

**Annotations:**

- **Cursor:** `▌` left bar in `accent_primary` on current row,
  with `bg_selection` full-width background.
- **Columns:** padding (1), flags (2), sender (22), subject
  (fill), date (12). Double-space separator between columns.
- **Unread rows:** `󰇮` icon in flags. Sender and subject in
  `accent_tertiary`. Bold sender.
- **Read rows:** No flag icon (empty). Sender and subject in
  `fg_dim`.
- **Replied:** `󰑚` icon in `color_special`.
- **Flagged:** `󰈻` icon in `color_warning`.
- **Thread prefixes:** In subject column. `├─` has-siblings,
  `└─` last-sibling, `│` stem. Thread chars in `fg_dim`.
- **Date:** Right-aligned. Today shows time, this week shows
  day, older shows `Mon DD` or `Mon DD, YYYY`.
- **No column headers rendered** — the header row above is for
  wireframe reference only. The actual UI has no header row.
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/wireframes.md
git commit -m "Add message list wireframes"
```

---

## Task 5: Message viewer wireframe

Draw the viewer tab showing the header block and rendered message
body in a scrollable viewport.

**Files:**
- Modify: `docs/poplar/wireframes.md`

- [ ] **Step 1: Add message viewer wireframe**

Append to `docs/poplar/wireframes.md`:

```markdown
## 5. Message Viewer (#2)

### Full viewer in its own tab

` ` `
┃ 󰇰 Inbox ┃ Re: Project update for Q2 launch ┃                                                                      │
│                                                                                                                      │
│  From:     Alice Johnson <alice@example.com>                                                                         │
│  To:       Geoff Wright <geoff@907.life>                                                                             │
│  Date:     Thu, 10 Apr 2026 10:32:07 -0600                                                                          │
│  Subject:  Re: Project update for Q2 launch                                                                          │
│  ────────────────────────────────────────────────────────────────────────────────────────────────────────             │
│                                                                                                                      │
│  Hey Geoff,                                                                                                          │
│                                                                                                                      │
│  Just wanted to follow up on the Q2 launch timeline. I've attached the                                               │
│  updated project plan with the revised milestones.                                                                   │
│                                                                                                                      │
│  ## Key changes                                                                                                      │
│                                                                                                                      │
│  - Beta release moved to April 15                                                                                    │
│  - QA window extended by one week                                                                                    │
│  - Launch date is now May 1                                                                                          │
│                                                                                                                      │
│  Let me know if you have any concerns about the new timeline.                                                        │
│                                                                                                                      │
│  > On Apr 9, 2026, Geoff Wright wrote:                                                                               │
│  > Can you send me the updated project plan? I want to review the                                                    │
│  > milestones before our meeting on Friday.                                                                          │
│                                                                                                                      │
│  Best,                                                                                                               │
│  Alice                                                                                                               │
│                                                                                                                      │
│                                                                                                                      │
│                                                                                                                      │
│                                                                                                                      │
│                                                                                                                      │
│                                                                                                                      │
│                                                                                                                      │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Re: Project update for Q2 launch · 100%                                                                              │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ d:del  a:archive  s:star  r:reply  R:all  f:fwd  Tab:links  q:close                                                 │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
` ` `

**Annotations:**

- **Full width:** Viewer uses entire content area (no sidebar).
- **Header block:** Rendered by the existing header filter
  (shared with mailrender). Keys in `accent_primary` bold,
  values in `fg_base`, angle-bracket emails in `fg_dim`.
  Separator `─` line in `fg_dim` below headers.
- **Body:** Rendered by the existing content pipeline
  (`ParseBlocks` → `RenderBody`). Lipgloss styles from
  compiled theme. Headings in `color_success` bold. Quotes
  prefixed with `>` in `accent_tertiary` (level 1) or
  `fg_dim` (level 2+). Links in `accent_primary` underline.
- **Viewport:** `bubbles/viewport` with scroll percentage in
  status bar. `j/k` scroll lines, `C-d/C-u` half page,
  `C-f/C-b` full page, `gg/G` top/bottom.
- **Status bar:** Subject + scroll percentage. Same styling as
  folder view status bar.
- **Footer context:** Viewer-specific bindings. `Tab:links`
  replaces `/:search`. `q:close` added.
- **Padding:** 2-char left margin for body content readability.
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/wireframes.md
git commit -m "Add message viewer wireframe"
```

---

## Task 6: Help popover wireframe

Draw the `?` keybinding help overlay for each context (message list,
viewer, sidebar).

**Files:**
- Modify: `docs/poplar/wireframes.md`

- [ ] **Step 1: Add help popover wireframes**

Append to `docs/poplar/wireframes.md`:

```markdown
## 6. Keybinding Help Popover (#7)

### Message list context

` ` `
                    ╭─ Message List ─────────────────────────────────────╮
                    │                                                    │
                    │  Navigate          Triage         Reply            │
                    │  j/k  up/down      d  delete      r  reply        │
                    │  gg   top          D  delete…     R  all          │
                    │  G    bottom       a  archive     f  forward      │
                    │  C-d  half pg dn   A  archive…    c  compose      │
                    │  C-u  half pg up   s  star                        │
                    │  C-f  page dn      .  read/unrd                   │
                    │  C-b  page up                                     │
                    │                                                    │
                    │  Search            Select         Threads         │
                    │  /    search       v  select      zo  open        │
                    │  n    next         ␣  toggle      zc  close       │
                    │  N    prev                        za  toggle      │
                    │                                                    │
                    │  Go To                                            │
                    │  gi  inbox   gd  drafts   gs  sent                │
                    │  ga  archive gx  spam     gt  trash               │
                    │                                                    │
                    │  Enter  open      :  command      ?  close        │
                    │                                                    │
                    ╰────────────────────────────────────────────────────╯
` ` `

### Viewer context

` ` `
                    ╭─ Message Viewer ───────────────────────────────────╮
                    │                                                    │
                    │  Navigate          Triage         Reply            │
                    │  j/k  scroll       d  delete      r  reply        │
                    │  gg   top          a  archive     R  all          │
                    │  G    bottom       s  star        f  forward      │
                    │  C-d  half pg dn                  c  compose      │
                    │  C-u  half pg up                                  │
                    │  C-f  page dn                                     │
                    │  C-b  page up                                     │
                    │                                                    │
                    │  Tab  link picker  q  close       ?  close        │
                    │                                                    │
                    ╰────────────────────────────────────────────────────╯
` ` `

### Sidebar context

` ` `
                    ╭─ Sidebar ──────────────────────────────────────────╮
                    │                                                    │
                    │  Navigate          Go To                          │
                    │  j/k  up/down      gi  inbox     gd  drafts      │
                    │  gg   top          gs  sent      ga  archive     │
                    │  G    bottom       gx  spam      gt  trash       │
                    │                                                    │
                    │  Enter  open       c  compose     ?  close        │
                    │                                                    │
                    ╰────────────────────────────────────────────────────╯
` ` `

**Annotations:**

- **Modal overlay:** Centered on screen. Content behind is
  dimmed (lipgloss overlay or reduced-opacity background).
- **Border:** Rounded corners (`╭╮╰╯`) in `bg_border`.
- **Title:** Context name in `accent_primary` bold, integrated
  into top border.
- **Group headings:** `fg_bright` bold (Navigate, Triage, etc.)
- **Key column:** `fg_bright` bold. Fixed width per column.
- **Description column:** `fg_dim`. Aligned within each group.
- **Columns:** Three groups per row where content fits. Groups
  are visually separated by whitespace (no divider lines).
- **Close:** `?` or `Escape`. Both shown in footer row.
- **Must fit on screen:** No scrolling. If too many bindings,
  prune the map — this constraint forces curation.
- **All keypresses route to popover** — only `?` and `Escape`
  handled, everything else ignored.
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/wireframes.md
git commit -m "Add help popover wireframes"
```

---

## Task 7: Transient UI wireframes

Draw the status toast, undo bar, error banner, and loading spinner.

**Files:**
- Modify: `docs/poplar/wireframes.md`

- [ ] **Step 1: Add transient UI wireframes**

Append to `docs/poplar/wireframes.md`:

```markdown
## 7. Transient UI (#8, #9, #10, #11, #12)

### Status toast (#8)

Appears in the status bar area, replacing the folder/message info
temporarily.

` ` `
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ✓ Archived 1 message                                                                                                 │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ d:del  a:archive  s:star  r:reply  R:all  f:fwd  c:compose  /:search  ?:help  ::cmd                                 │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
` ` `

Toast variants:

` ` `
│ ✓ Archived 1 message                     │  color_success
│ ✓ Message sent                           │  color_success
│ ✓ Draft saved                            │  color_success
│ 󰈻 Flagged                                │  color_warning
│ 󰇮 Marked unread                          │  accent_tertiary
` ` `

### Undo bar (#9)

Replaces toast for reversible destructive actions. Shows countdown
and undo hint.

` ` `
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Deleted 1 message · press u to undo                                                                            [5s] │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
` ` `

` ` `
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Deleted 3 messages · press u to undo                                                                           [5s] │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
` ` `

### Error banner (#10)

Persistent error state. Does not auto-dismiss.

` ` `
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ✗ Connection lost — reconnecting…                                                                                    │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
` ` `

` ` `
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ✗ Send failed: SMTP authentication error                                                                             │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
` ` `

### Loading spinner (#11)

#### Message list spinner (fetching headers)

` ` `
│                    │                                                                                                │
│                    │                          ⣾ Loading messages…                                                    │
│                    │                                                                                                │
` ` `

#### Viewer spinner (fetching body)

` ` `
│                                                                                                                      │
│                                    ⣾ Loading message…                                                                │
│                                                                                                                      │
` ` `

### Connection status (#12)

Subtle indicator at the right edge of the status bar.

` ` `
│ 󰇰 Inbox · 10 messages · 2 unread                                                                        ● connected │
│ 󰇰 Inbox · 10 messages · 2 unread                                                                   ◌ reconnecting…  │
│ 󰇰 Inbox · 10 messages · 2 unread                                                                        ○ offline   │
` ` `

**Annotations:**

- **Toast (#8):** Renders in status bar area. Auto-dismisses
  after 3s via `tea.Tick`. Icon + message. Color from theme
  based on action type.
- **Undo bar (#9):** Replaces status bar content. `u` undoes
  the action. Countdown `[5s]` right-aligned in `fg_dim`.
  Action is deferred — not executed until the 5s window
  expires. `fg_base` text on `bg_elevated` background.
- **Error banner (#10):** Persistent in status bar. `color_error`
  text. `✗` prefix. Dismissed by keypress or condition clearing
  (e.g., reconnection succeeds).
- **Spinner (#11):** `bubbles/spinner` component. Centered in
  the content area. Spinner char + message in `fg_dim`.
  Dot spinner style (braille pattern: `⣾⣽⣻⢿⡿⣟⣯⣷`).
- **Connection status (#12):** Right-aligned in status bar.
  `●` connected in `color_success`, `◌` reconnecting in
  `color_warning` (with spinner), `○` offline in `fg_dim`.
- **Priority:** Error banner > Undo bar > Toast > Normal status.
  Only one transient element in the status bar at a time.
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/wireframes.md
git commit -m "Add transient UI wireframes"
```

---

## Task 8: Screen state wireframes

Draw the screen states: empty folder, threaded view (fold/unfold),
search results, multi-select, and focused panel cycling.

**Files:**
- Modify: `docs/poplar/wireframes.md`

- [ ] **Step 1: Add screen state wireframes**

Append to `docs/poplar/wireframes.md`:

```markdown
## 8. Screen States (#13, #14, #15, #16, #17)

### Empty folder (#13)

` ` `
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                            No messages                                                          │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
│                    │                                                                                                │
` ` `

### Threaded view — collapsed (#14)

` ` `
      Eve Martinez          [3] Re: Server migration plan                                     Apr 05
` ` `

### Threaded view — expanded (#14)

` ` `
      Eve Martinez               Re: Server migration plan                                    Apr 05
      ├─ Grace Kim               └─ Re: Server migration plan                                 Apr 05
      │  └─ Frank Lee               Re: Server migration plan                                 Apr 05
` ` `

### Threaded view — partially collapsed (#14)

` ` `
      Eve Martinez               Re: Server migration plan                                    Apr 05
      ├─ Grace Kim            [2] └─ Re: Server migration plan                                Apr 05
` ` `

### Search results (#15)

` ` `
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ 󰍉 search: "project update" · 3 results                                                                              │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
` ` `

The message list shows only matching messages. `n/N` jump between
results. `:clear` restores the full message list.

### Multi-select (#16)

` ` `
 󰇮   Alice Johnson           Re: Project update for Q2 launch                              10:32 AM
 󰇮  󰄬Bob Smith                Weekly standup notes                                           9:15 AM
 󰑚  󰄬Carol White              Re: Budget review                                            Yesterday
      Dave Chen                Meeting minutes from Monday                                    Apr 07
 󰈻  󰄬Eve Martinez             Quarterly report draft                                         Apr 06
` ` `

Footer swaps to bulk actions:

` ` `
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ 3 selected                                                                                                           │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Space:toggle  d:del all  a:archive all  v:cancel  Esc:cancel                                                         │
╰──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
` ` `

### Focused panel cycling (#17)

` ` `
Tab press 1: Sidebar focused          Tab press 2: Message list focused
┌──────────┬───────────────────┐      ┌──────────┬───────────────────┐
│ █ Inbox  │  Alice Johnson    │      │   Inbox  │▌ Alice Johnson    │
│   Sent   │  Bob Smith        │      │   Sent   │  Bob Smith        │
│   Trash  │  Carol White      │      │   Trash  │  Carol White      │
└──────────┴───────────────────┘      └──────────┴───────────────────┘
  ↑ j/k here                                        ↑ j/k here
` ` `

**Annotations:**

- **Empty folder (#13):** Centered text "No messages" in `fg_dim`.
  Vertically and horizontally centered in the message list area.
- **Thread collapse (#14):** `zo` unfold, `zc` fold, `za` toggle.
  Collapsed thread shows `[N]` count in `fg_dim` before subject.
  Thread root always visible. Partially collapsed = some children
  folded under a mid-thread node.
- **Search (#15):** Search query shown in status bar with `󰍉`
  icon. Result count. `color_info` for search indicator.
  `:clear` command restores normal view.
- **Multi-select (#16):** `v` enters visual select mode. `Space`
  toggles individual messages. `󰄬` check icon in `color_success`
  on selected rows. Selected row background in `bg_selection`.
  Status bar shows count. Footer swaps to bulk action bindings.
  `Esc` or `v` exits multi-select.
- **Focus cycling (#17):** `Tab` toggles between sidebar and
  message list. Focused panel receives j/k navigation. Visual
  indicator: selected item styling (bold border, background)
  only active in focused panel. Unfocused panel's selection is
  dimmed.
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/wireframes.md
git commit -m "Add screen state wireframes"
```

---

## Task 9: Overlay wireframes

Draw the remaining overlays: compose review, folder picker, and
confirm delete.

**Files:**
- Modify: `docs/poplar/wireframes.md`

- [ ] **Step 1: Add overlay wireframes**

Append to `docs/poplar/wireframes.md`:

```markdown
## 9. Overlays (#4, #5, #6)

### Compose review (#4)

Inline prompt rendered in the status bar area after the editor
exits with code 0.

` ` `
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Send message? y:send  n:abort  e:edit  p:postpone                                                                    │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
` ` `

### Folder picker (#5)

Modal overlay for `:move` and `:copy` commands.

` ` `
                         ╭─ Move to folder ──────────────────────╮
                         │                                       │
                         │  > arch                               │
                         │                                       │
                         │    󰀼 Archive                          │
                         │    󰡡 Lists/arch-linux                 │
                         │                                       │
                         ╰───────────────────────────────────────╯
` ` `

` ` `
                         ╭─ Move to folder ──────────────────────╮
                         │                                       │
                         │  >                                    │
                         │                                       │
                         │  █ 󰇰 Inbox                            │
                         │    󰏫 Drafts                           │
                         │    󰑚 Sent                             │
                         │    󰀼 Archive                          │
                         │    󰍷 Spam                             │
                         │    󰩺 Trash                            │
                         │    󰂚 Notifications                    │
                         │    󰑴 Remind                           │
                         │    󰡡 Lists/golang                     │
                         │    󰡡 Lists/rust                       │
                         │                                       │
                         ╰───────────────────────────────────────╯
` ` `

### Confirm delete (#6)

Single-line prompt for bulk delete (3+ messages).

` ` `
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Delete 5 messages? y:confirm  n:cancel                                                                               │
├──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
` ` `

**Annotations:**

- **Compose review (#4):** Renders in status bar, not a modal.
  Keys in `fg_bright` bold, hints in `fg_dim`. Blocks all other
  input until answered. Only appears after editor exit code 0.
  Pass 9 implementation.
- **Folder picker (#5):** Modal overlay, centered. Dimmed
  background. `bubbles/textinput` for filter query (prefixed
  with `>`). Results update as you type — fuzzy matching on
  folder name. `j/k` or arrow keys to move selection. `Enter`
  to confirm, `Escape` to cancel. Selected row has `█` left
  border + `bg_selection`. Rounded border in `bg_border`.
  Title shows action ("Move to folder" or "Copy to folder")
  in `accent_primary`. Pass 7 implementation.
- **Confirm delete (#6):** Renders in status bar, same pattern
  as compose review. Only triggers for 3+ messages. Single
  message delete is instant with undo bar. `color_warning`
  for the count. Pass 6 implementation.
```

- [ ] **Step 2: Commit**

```bash
git add docs/poplar/wireframes.md
git commit -m "Add overlay wireframes"
```

---

## Task 10: Compose handoff wireframe + final review

Draw the compose/external editor handoff sequence and perform the
final review against the full interface inventory.

**Files:**
- Modify: `docs/poplar/wireframes.md`

- [ ] **Step 1: Add compose handoff wireframe**

Append to `docs/poplar/wireframes.md`:

```markdown
## 10. Compose — External Editor (#3)

Not a poplar screen. Bubbletea suspends via `tea.ExecProcess`,
terminal is handed to the editor. Poplar disappears entirely and
reappears when the editor exits.

### Sequence

` ` `
┌─────────────────────────────────────────────────┐
│                                                 │
│  Poplar running (bubbletea)                     │
│                                                 │
│  User presses 'c' (compose) or 'r' (reply)     │
│                                                 │
└────────────────────┬────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│                                                 │
│  Poplar writes temp file:                       │
│  - Headers (To, From, Subject, etc.)            │
│  - Quoted body (for reply/forward)              │
│  - Signature (if configured)                    │
│                                                 │
│  tea.ExecProcess("nvim-mail", tempfile)          │
│  Bubbletea suspends — terminal belongs to nvim  │
│                                                 │
└────────────────────┬────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│                                                 │
│  nvim-mail running (full screen)                │
│  User composes message                          │
│                                                 │
│  Exit code 0: :wq (send)                       │
│  Exit code 1: :cq (abort)                      │
│                                                 │
└────────────────────┬────────────────────────────┘
                     │
              ┌──────┴──────┐
              ▼             ▼
     Exit code 0      Exit code ≠ 0
     ┌──────────┐     ┌──────────┐
     │ Compose  │     │ Abort    │
     │ review   │     │ (toast)  │
     │ prompt   │     └──────────┘
     └──────────┘
` ` `

**Annotations:**

- **`tea.ExecProcess`:** Bubbletea's built-in mechanism for
  handing terminal control to a child process. Bubbletea
  suspends its event loop, restores the terminal, and resumes
  when the child exits.
- **Editor:** Default `$EDITOR` (poplar default: `micro`).
  For this user: `nvim-mail`.
- **Temp file:** Created by poplar using `internal/compose/`
  for header formatting and quoted text reflow.
- **Exit code 0:** Triggers compose review prompt in status
  bar (see Overlay #4).
- **Exit code ≠ 0:** Compose aborted. Toast: "Compose aborted"
  in `fg_dim`. No review prompt.
- **Pass 9 implementation.**
```

- [ ] **Step 2: Add interface inventory coverage checklist**

Append to `docs/poplar/wireframes.md`:

```markdown
## Coverage

All 20 UI elements from the interface inventory:

| # | Element | Section |
|---|---------|---------|
| 1 | Folder + Message List | §1 Composite, §3 Sidebar, §4 Message List |
| 2 | Message Viewer | §5 Viewer |
| 3 | Compose (external) | §10 Compose |
| 4 | Compose Review | §9 Overlays |
| 5 | Folder Picker | §9 Overlays |
| 6 | Confirm Delete | §9 Overlays |
| 7 | Keybinding Help | §6 Help Popover |
| 8 | Status Toast | §7 Transient UI |
| 9 | Undo Bar | §7 Transient UI |
| 10 | Error Banner | §7 Transient UI |
| 11 | Loading Spinner | §7 Transient UI |
| 12 | Connection Status | §7 Transient UI |
| 13 | Empty Folder | §8 Screen States |
| 14 | Threaded View | §8 Screen States |
| 15 | Search Results | §8 Screen States |
| 16 | Multi-Select | §8 Screen States |
| 17 | Focused Panel | §8 Screen States |
| 18 | Tab Bar | §1 Composite, §2 Tab Bar |
| 19 | Command Footer | §1 Composite (shown in all wireframes) |
| 20 | Status Bar | §1 Composite (shown in all wireframes) |
```

- [ ] **Step 3: Self-review against UI spec**

Verify:
1. Every element in the inventory table (§1 of UI spec) has a
   wireframe
2. Keybinding footer content matches §7 of UI spec
3. Sidebar rendering matches §9 of UI spec
4. Help popover layout matches §8 of UI spec
5. Thread prefix characters match architecture.md conventions
6. Column widths match aerc setup doc conventions
7. Nerd Font icons match aerc setup doc icon tables

- [ ] **Step 4: Commit**

```bash
git add docs/poplar/wireframes.md
git commit -m "Add compose wireframe and coverage checklist (Pass 2.5a)"
```
