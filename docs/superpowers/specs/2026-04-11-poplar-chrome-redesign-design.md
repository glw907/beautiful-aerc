# Poplar Chrome Redesign: Frame, Status, Layout

**Date:** 2026-04-11
**Status:** Draft
**Parent spec:** `2026-04-09-poplar-design.md`
**Supersedes:** Tab bar, status bar, footer, and message viewer
sections of `2026-04-10-poplar-ui-wireframing-design.md` and
wireframe sections 1 (Composite Layout), 2 (Tab Bar), 5 (Message
Viewer), 7 (Status Bar), 8 (Command Footer).

## Summary

Remove the tab bar. Replace it with a single-line top frame edge.
The sidebar stays visible in all views (including message viewer)
and shows the account name. The status bar becomes the bottom
frame edge with a rounded bottom-right corner. A combined status
indicator on the right shows counts and connection state. Toast
notifications overlay the top-right. The command footer sits
outside the frame. Responsive layout collapses the sidebar below
106 columns.

## Problems with Current Implementation

1. **Tab bar wastes 3 rows** on information the sidebar already
   shows (active folder). With the sidebar always visible, tabs
   provide no new information.
2. **Nerd Font icon width mismatch.** `lipgloss.Width("󰇰")`
   returns 1 but the terminal renders it as 2 cells.
3. **Status bar floats.** The grey bar has no frame integration
   — it terminates abruptly above the footer.
4. **Status bar and footer crowded.** Two dense rows jammed
   together with no visual separation.
5. **Redundant information.** Folder name appears in both the
   active tab and the status bar.
6. **Message viewer hides sidebar.** The current wireframe shows
   the viewer as full-width, but the message body only needs 72
   columns.

## Design

### 1. Drop Tabs, Add Top Frame Line

The 3-row tab bar is removed entirely. In its place, a single
`─` line with `╮` at the right and `┬` at the panel divider:

```
───────────────────────────┬──────────────────────────────────────────────────╮
│                          │                                                  │
```

- No left border, no `╭`. The left edge is open, matching the
  bottom where the status bar also starts at column 0.
- The `┬` marks where the panel divider meets the top line.
- The `╮` is the top-right frame corner (rounded).
- The right `│` border runs from `╮` down to `╯` on the bottom.

This reclaims 2 rows of vertical space (3-row tab bar replaced
by 1-row frame line).

**Navigation without tabs:** Opening a message renders it in the
right panel (same position as the message list). `q` returns to
the message list. No tab lifecycle, no tab switching keys.
One view at a time — "Better Pine" simplicity.

**Folder jumps:** Single uppercase keys jump to canonical folders
from any context: `I` Inbox, `D` Drafts, `S` Sent, `A` Archive,
`X` Spam, `T` Trash. Works whether the sidebar is visible or
collapsed. See `docs/poplar/keybindings.md` for the full map.

### 2. Account Label in Sidebar

The sidebar shows the account name at the top, followed by a
blank line, then folder groups:

```
│ geoff@907.life           │
│                          │
│ ┃ 󰇰  Inbox           3  │
│   󰏫  Drafts              │
│   󰑚  Sent                │
```

- Account name in `fg_dim` (low-profile, glance-at info).
- One blank line separates account from folders.
- When multi-account lands (Pass 11), a key cycles between
  accounts. The entire sidebar refreshes: account name, folders,
  unread counts. One account visible at a time — not stacked.

### 3. Bottom Frame: Status Bar as Bottom Edge

The grey status bar becomes the bottom edge of the content frame.
The right `│` border curves into the status bar via `─╯`,
connecting horizontally rather than terminating vertically.

**Layout:**

```
│ content                                                                  │
│                                                                          │
 ──────────────────────────┴──────────────── 10 messages · 3 unread · ● connected ─╯
  d:del  a:archive  s:star  ┊  r:reply  R:all  f:fwd  c:compose  ┊  /:search  ?:help  ::cmd
```

- The `╯` is the bottom-right rounded corner of the content
  frame.
- A short `─` segment connects `╯` to the status indicator text.
- The grey background (`bg_border`) extends from the right edge
  all the way left, forming the bottom of the frame.
- The `┴` marks where the panel divider meets the bottom line.
- The left edge of the grey bar has no corner or border — it
  starts at column 0.
- The command footer sits below, outside the frame, with no
  borders or background. One space of left padding before the
  first hint. Bindings are grouped logically with `┊` (light
  quadruple dash vertical) separators in `fg_dim` between
  groups. See `docs/poplar/keybindings.md` for the full map,
  groupings, and curated footer display per context.

### 4. Combined Status Indicator (Bottom Right)

The right portion of the bottom status bar is the combined status
indicator. This is persistent — always visible. Default content:

```
10 messages · 3 unread · ● connected ─╯
```

**Connection states (colorblind-accessible):**

| State | Icon | Color | Text |
|-------|------|-------|------|
| Connected | `●` (filled) | `color_success` (green) | `connected` |
| Offline | `○` (hollow) | `color_error` (red) | `offline` |
| Reconnecting | `◐` (half) | `color_warning` (orange) | `reconnecting` |

Shape + color + text = triple redundancy. Survives colorblind
conditions, monochrome terminals, and screen readers.

**Active operation states** replace the connection indicator
temporarily:

| State | Display | Behavior |
|-------|---------|----------|
| Searching | `spinner searching...` | Animated bubbletea spinner, persists until done |
| Sending | `spinner sending...` | Animated bubbletea spinner, persists until done |
| Fetching | `spinner loading...` | Animated bubbletea spinner, persists until done |

Use `bubbles/spinner` with the `Dot` style for the animated
indicator. The spinner replaces only the connection portion
(`● connected`), not the message counts.

### 5. Toast Notifications (Top Right)

Transient feedback overlays the top-right of the top frame line.
The persistent status indicator on the bottom stays visible at
all times.

```
───────────────────────────┬──────────────────────────── ✓ 3 archived ─╮
│ geoff@907.life           │                                            │
```

Toast dismisses, top line returns to plain frame:

```
───────────────────────────┬────────────────────────────────────────────╮
│ geoff@907.life           │                                            │
```

**Toast behavior:**

- Overlays from the right end of the top frame line, just
  inside the `╮` corner.
- Auto-dismisses after 3 seconds (via `tea.Tick`).
- Multiple rapid toasts: latest wins (replaces previous).

**Toast types:**

| Type | Icon | Color | Examples |
|------|------|-------|----------|
| Success | `✓` | `color_success` | "3 archived", "Message sent", "Draft saved" |
| Info | `›` | `accent_tertiary` | "Moved to Trash", "Marked as read" |
| Error | `✗` | `color_error` | "Send failed", "Connection lost" |

**Error toasts persist** until dismissed by keypress or condition
clearing. They do not auto-dismiss.

### 6. Sidebar Always Visible

The sidebar remains visible when viewing a message. The message
viewer renders in the right panel (same position as the message
list) rather than taking over the full content area.

**Width budget:**

| Element | Width |
|---------|-------|
| Sidebar | 30 |
| Divider | 1 |
| Message body | 72 |
| Left margin | 2 |
| Right border | 1 |
| **Total minimum** | **106** |

The message body is hard-wrapped at 72 characters (matching the
compose editor's `textwidth`). Headers may extend wider but are
truncated or wrapped at the panel boundary.

**Benefits:**

- Folder context always visible — can see unread counts change
  in real time.
- Balanced visual weight — sidebar + 72-char body fills the
  screen without the body floating in excessive whitespace.
- Consistent layout — no jarring layout shift when opening or
  closing a message.
- Sidebar navigation still works — can switch folders without
  closing the viewer.

### 7. Responsive Layout

The sidebar collapses when the terminal is too narrow to fit
both panels. Bubbletea sends `tea.WindowSizeMsg` on every resize.

**Breakpoints:**

| Width | Layout |
|-------|--------|
| >= 106 | Sidebar + right panel (message list or viewer) |
| < 106 | Sidebar hidden, right panel gets full width |

**Behavior:**

- Below the breakpoint, the sidebar panel is not rendered and
  the divider is removed. The right panel (message list or
  viewer) uses the full content width.
- The top frame line shows `account · folder` on the left so
  the user retains context when the sidebar is hidden:
  `geoff@907.life · Inbox ──── ✓ 3 archived ─╮`
- The sidebar can still be accessed via `Tab` focus cycling —
  when the sidebar gains focus below the breakpoint, it renders
  as a full-width panel replacing the message list/viewer.
- Above the breakpoint, both panels are always visible. `Tab`
  cycles focus between them. The top line is a plain frame edge
  (account and folder are in the sidebar).
- The breakpoint is evaluated on every `tea.WindowSizeMsg`.
  Resizing the terminal live transitions smoothly.
- The `┬` and `┴` junction characters on the top and bottom
  frame lines are omitted when the sidebar is hidden (no
  divider = no junction).

### 8. Frame Summary

Three-sided frame with rounded right corners, open left edge:

| Edge | Characters | Notes |
|------|-----------|-------|
| Top | `──┬──╮` | Toast overlays from right |
| Right | `│` | Runs from `╮` to `╯` |
| Bottom | `──┴── status ─╯` | Grey `bg_border` background, status indicator right-aligned |
| Left | (open) | No border, no corner |

The `┬` (top) and `┴` (bottom) mark the panel divider position.
The frame encloses: sidebar, divider, right panel (message list
or viewer).

### 9. Updated Wireframes

#### Composite Layout (message list)

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
│                          │                                                                              │
│                          │                                                                              │
│                          │                                                                              │
│                          │                                                                              │
│                          │                                                                              │
│                          │                                                                              │
│                          │                                                                              │
 ──────────────────────────┴──────────────────────────────────── 10 messages · 3 unread · ● connected ─╯
  d:del  a:archive  s:star  ┊  r:reply  R:all  f:fwd  c:compose  ┊  /:search  ?:help  ::cmd
```

#### Message Viewer with Sidebar

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
│                          │  Just wanted to follow up on the Q2 launch timeline. I've                    │
│   󰂚  Notifications       │  attached the updated project plan with the revised milestones.              │
│   󰑴  Remind              │                                                                              │
│   󰡡  Lists/golang        │  ## Key changes                                                              │
│                          │                                                                              │
│                          │  - Beta release moved to April 15                                            │
│                          │  - QA window extended by one week                                            │
│                          │  - Launch date is now May 1                                                  │
│                          │                                                                              │
│                          │  Let me know if you have any concerns about the new timeline.                │
│                          │                                                                              │
│                          │  > On Apr 9, 2026, Geoff Wright wrote:                                      │
│                          │  > Can you send me the updated project plan?                                 │
 ──────────────────────────┴──────────────────────────────────────────── 100% · ● connected ─╯
  d:del  a:archive  s:star  ┊  r:reply  R:all  f:fwd  ┊  Tab:links  q:close  ?:help
```

#### Toast Notification (transient, overlays top-right)

```
───────────────────────────┬──────────────────────────────────────────── ✓ 3 archived ─╮
│ geoff@907.life           │                                                            │
│                          │  message list...                                           │
```

#### Responsive: Narrow Terminal (< 106 cols)

```
 geoff@907.life · Inbox ──────────────────────────────────────────────────╮
│                                                                         │
│ 󰇮  Alice Johnson          Re: Project update for Q2 launch   10:32 AM  │
│ 󰇮  Bob Smith               Weekly standup notes                9:15 AM  │
│ 󰑚  Carol White             Re: Budget review                  Yesterday  │
│                                                                         │
 ─────────────────────────────────── 10 messages · 3 unread · ● connected ─╯
  d:del  a:archive  s:star  r:reply  R:all  f:fwd  c:compose  /:search
```

## Implementation Impact

### Removed Components

- **TabBar** (`tab_bar.go`): Deleted entirely. No tab bar, no
  tab lifecycle, no tab switching keys (`1-9`).
- **Tab interface** (`tab.go`): Simplified. AccountTab no longer
  needs to satisfy a Tab interface for a tab manager. The App
  model holds an AccountTab directly.

### New Components

- **TopLine:** Renders the `──┬──╮` top frame line. Handles
  toast overlay on the right.
- **StatusIndicator:** Renders the right-side combined indicator
  (counts + connection + spinner). Owns the `bubbles/spinner`
  instance for active operations.
- **Toast:** Renders transient messages overlaid on the top-right.
  Auto-dismiss via `tea.Tick`.

### Modified Components

- **StatusBar:** Becomes the bottom frame edge. Renders the grey
  `bg_border` bar with `┴` divider junction and `─╯` corner.
  Integrates `StatusIndicator` on the right. No longer shows
  folder name or icon.
- **Footer:** Removed from frame. Renders below the status bar
  with no borders or background. 1-space left padding.
- **AccountTab:** Sidebar shows account name at top. Sidebar
  visible in all views (message list and viewer). Responsive
  collapse below 106 columns.
- **App:** Simplified — holds one AccountTab directly instead of
  a tab list. `contentHeight` updated (1 row top line + 1 row
  status bar + 1 row footer = 3 rows chrome, down from 5).

### Minimum Terminal Size

106 columns for full layout (sidebar + message body). Below that,
sidebar collapses. The application should remain usable down to
~60 columns (message list only, no sidebar, narrower columns).
