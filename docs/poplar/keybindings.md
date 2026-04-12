# Poplar Keybinding Map

Authoritative reference for all keybindings. Organized by logical
group. Each binding lists the key, action, and which contexts it
applies to.

Contexts: **A** = account view (sidebar + message list, one pane),
**V** = message viewer.

The account view is one pane. No focus cycling — every key is
always live. `j/k` navigates messages, `J/K` navigates folders.
This matches aerc's convention.

## Navigation

| Key | Action | Context |
|-----|--------|---------|
| `j` | Message down / scroll down | A, V |
| `k` | Message up / scroll up | A, V |
| `J` | Folder down | A |
| `K` | Folder up | A |
| `G` | Jump to bottom | A |
| `C-d` | Half page down | A, V |
| `C-u` | Half page up | A, V |
| `C-f` | Full page down | A, V |
| `C-b` | Full page up | A, V |
| `Enter` | Open message | A |

## Triage

Act on the current message (or selection in multi-select mode).

| Key | Action | Context |
|-----|--------|---------|
| `d` | Delete | A, V |
| `a` | Archive | A, V |
| `s` | Star / unstar | A, V |
| `.` | Toggle read / unread | A, V |

## Reply & Compose

| Key | Action | Context |
|-----|--------|---------|
| `r` | Reply | A, V |
| `R` | Reply all | A, V |
| `f` | Forward | A, V |
| `c` | Compose new | A, V |

## Folder Jump

Single uppercase key jumps to a canonical folder from any context.
Moves the sidebar selection and switches the message list.

| Key | Folder | Context |
|-----|--------|---------|
| `I` | Inbox | A |
| `D` | Drafts | A |
| `S` | Sent | A |
| `A` | Archive | A |
| `X` | Spam | A |
| `T` | Trash | A |

## Search

| Key | Action | Context |
|-----|--------|---------|
| `/` | Start search | A |
| `n` | Next result | A |
| `N` | Previous result | A |

## Select

Multi-select mode for bulk operations.

| Key | Action | Context |
|-----|--------|---------|
| `v` | Enter/exit visual select | A |
| `Space` | Toggle selection on current message | A |

## Viewer

| Key | Action | Context |
|-----|--------|---------|
| `Tab` | Link picker | V |
| `q` | Close viewer, return to list | V |

## App

| Key | Action | Context |
|-----|--------|---------|
| `?` | Help popover | A, V |
| `:` | Command mode | A, V |
| `q` | Quit (from account view) | A |
| `C-c` | Force quit | A, V |

## Footer Display

The command footer shows a curated subset of keybindings relevant
to the current context. Bindings are grouped logically with
`┊` separators between groups.

### Account footer

The unified one-pane footer. `j/k` and `J/K` both live, triage
and reply always available. Primary folder jumps (`I/D/S/A`) sit
in the nav group so they read as navigation aids.

```
 j/k:messages  J/K:folders  I:inbox  D:drafts  S:sent  A:archive  ┊  d:del  a:archive  s:star  ┊  r:reply  R:all  f:fwd  c:compose  ┊  /:search  ?:help  ::cmd
 ◂── navigation ─────────────────────────────────────────────────▸   ◂── triage ──────────────▸   ◂── reply/compose ──────────────▸   ◂── app ──────────▸
```

`X` (Spam) and `T` (Trash) are still live keys but omitted from
the footer — disposal folders are jumped to rarely enough that
the footer real estate is better spent on hints for the primary
group.

### Viewer footer

```
 d:del  a:archive  s:star  ┊  r:reply  R:all  f:fwd  ┊  Tab:links  q:close  ?:help
 ◂── triage ──────────────▸   ◂── reply ─────────▸      ◂── viewer / app ──────────▸
```

### Group separator

`┊` (U+250A, light quadruple dash vertical) rendered in `fg_dim`.
Padded with one space on each side. Reads as a soft divider
without the visual weight of `│`.

## Design Decisions

**Single keys only.** Bubbletea sends one `tea.KeyMsg` per
keypress. No multi-key sequences (no `g i`, `g g`). Folder
jumps use uppercase single keys instead.

**Uppercase for folder jumps.** First letter of canonical folder
name. Avoids conflict with lowercase triage keys (`d` delete vs
`D` Drafts, `a` archive vs `A` Archive, `s` star vs `S` Sent).

**One pane, no focus cycling.** The account view is a single pane
from a keyboard nav standpoint (like pine). Every key is always
live — `j/k` for messages, `J/K` for folders, triage and reply
always available. No Tab focus cycling, no sidebar/msglist
contexts. The footer changes only when the viewer opens over the
list.

**Group separation via `┊`.** Light quadruple dash vertical in
`fg_dim`, padded with one space on each side. Subtle enough to
recede behind the key hints, clear enough to read the groups.
