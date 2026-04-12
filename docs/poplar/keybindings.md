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

## Select (deferred — Pass 6)

Multi-select is not yet implemented. The bindings below are
reserved in the design so later passes don't collide with them.
`v` is the entry point; `Space` toggles selection on the current
message once multi-select is active. Until Pass 6 lands, neither
key does anything.

| Key | Action | Context |
|-----|--------|---------|
| `v` | Enter/exit visual select *(reserved, Pass 6)* | A |
| `Space` | Toggle selection on current message *(reserved, Pass 6)* | A |

## Viewer

| Key | Action | Context |
|-----|--------|---------|
| `Tab` | Link picker | V |
| `q` | Close viewer, return to list | V |

## App

| Key | Action | Context |
|-----|--------|---------|
| `?` | Help popover | A, V |
| `q` | Quit (from account view) | A |
| `C-c` | Force quit | A, V |

Poplar has no `:` command mode. Every action is a key or a
modal picker launched by a key. See "No command mode" in the
Design Decisions section below.

## Footer Display

The command footer shows a curated subset of keybindings relevant
to the current context. Bindings are grouped logically with
`┊` separators between groups.

### Account footer

The unified one-pane footer. Multi-key bindings are compressed
(`j/k/J/K`, `I/D/S/A`) into single hint entries so the footer
has room for every account-view action, including hints for
features that aren't yet wired up.

```
 j/k/J/K nav  I/D/S/A folders ┊ d del  a archive  s star  . read ┊ r/R reply  f fwd  c compose ┊ / find  n/N results  v select  ? help  q quit
 ◂── navigation ─────────────▸  ◂── triage ─────────────────────▸  ◂── reply/compose ──────────▸  ◂── tools & app ──────────────────────────────▸
```

**Compressed nav hints.** `j/k/J/K nav` covers both `j/k`
(messages) and `J/K` (folders) as one navigation affordance.
`I/D/S/A folders` covers inbox/drafts/sent/archive jumps.
Expanding these into six entries took too much room — the
compressed form tells a vim-literate user everything they need.

**Future hints shown.** `. read`, `v select`, and `n/N results`
are in the footer even though the actions aren't implemented
yet. This surfaces the full planned vocabulary so users discover
features as they come online. The footer's goal is to show what
it will look like when poplar is done — aspirational content is
deliberate.

`X` (Spam) and `T` (Trash) are still live keys but omitted from
the footer — disposal folders are jumped to rarely enough that
the footer real estate is better spent elsewhere.

### Responsive footer

Each hint has a `dropRank` (0-10). When the terminal is too
narrow to fit every hint, the footer drops hints in descending
rank order until the content fits. Rank 0 hints (`? help`,
`q quit`) are always kept as an escape hatch — even on a
40-column terminal you can reach help and quit.

Drop tiers (highest rank → first to go):

| Rank | Hints | Why drop first |
|------|-------|----------------|
| 10–9 | `j/k/J/K nav`, `I/D/S/A folders` | Vim/arrow users don't need the hint |
| 8 | `v select` | Niche mode, discoverable in `?` help |
| 7 | `n/N results` | Only useful after `/`, infer from convention |
| 5 | `. read` | Secondary triage |
| 4 | `s star` | Secondary triage |
| 3 | `f fwd`, `/ find` | Tertiary actions |
| 2 | `r/R reply`, `c compose` | Primary compose actions |
| 1 | `d del`, `a archive` | Primary triage |
| 0 | `? help`, `q quit` | Always kept |

Approximate breakpoints (account context):

- **155+ cols**: full footer (all 14 hints)
- **140**: nav drops
- **100–120**: tools degrade (`v`, `n/N`)
- **80**: triage trims to del/archive, only `/ find` from tools
- **60**: minimum email loop (`d`, `a`, `c`, app)
- **40**: survival (one triage hint + app)

A group with no remaining visible hints collapses with its
preceding `┊` separator, so the footer never shows hanging
separators or empty groups.

### Viewer footer

```
 d del  a archive  s star  . read ┊ r/R reply  f fwd  c compose ┊ Tab links  q close  ? help
 ◂── triage ─────────────────────▸  ◂── reply/compose ──────────▸  ◂── viewer / app ─────────▸
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

**No command mode.** Poplar does not have a `:` command line.
Every action is bound to a key, or is invoked by a key that
opens a modal picker (folder move/copy, search, etc.). The
curated footer is the authoritative discoverability surface —
a hidden command layer would double it. Pine doesn't have one
either. See architecture.md "Drop `:` command mode" for the
full rationale.

**Non-modal for pane focus; visual-select is the only mode.**
The account view is a single pane — `j/k` always navigates the
message list, `J/K` always navigates folders, every other key
is always live. The eventual multi-select feature (Pass 6) is
the one narrow exception: `v` enters a vim-style visual mode
where `Space` toggles row selection and `Esc` exits. That
feature is deferred and currently unbound.
