# Poplar Styling Reference

Authoritative map from theme palette slots to UI surfaces. Every style
lives in `internal/ui/styles.go` (`Styles` struct, `NewStyles`
constructor). Nothing hardcodes hex values — every color flows from
`theme.CompiledTheme`.

**Rule:** before changing a color, update this document first. If the
change doesn't fit the existing semantic assignments, add a new row
rather than overloading an existing slot.

## Palette Slots

Each theme exports 16 slots via `theme.CompiledTheme`. Slot names are
semantic, not literal colors — "AccentPrimary" means the theme's primary
accent, whatever hue that is.

| Slot | Meaning |
|------|---------|
| `BgBase` | Primary background (main content area) |
| `BgElevated` | Raised surface (sidebar, popups, completion) |
| `BgSelection` | Selected-row highlight on `BgElevated` (sidebar) and `BgBase` (message list) — both surfaces use it for consistency |
| `BgSubtle` | Barely-elevated panel surface — viewer header sits on this, a small tonal step off `BgBase` to mark the header as a sub-region of the message context (not the sidebar's chrome track) |
| `BgBorder` | Borders, status bar, inactive chrome |
| `FgBase` | Primary text |
| `FgBright` | Emphasized text (tab labels, unread) |
| `FgBrightest` | Maximum contrast (rarely used) |
| `FgDim` | De-emphasized text (read messages, hints) |
| `AccentPrimary` | Title bar, header keys, spinner |
| `AccentSecondary` | Account name, sidebar indicator, active tab |
| `AccentTertiary` | Quotes, URLs, links |
| `ColorError` | Errors, deleted, diff del |
| `ColorWarning` | Warnings, flagged, reconnecting |
| `ColorSuccess` | Success, connected, toast confirm |
| `ColorInfo` | Search results, highlights |
| `ColorSpecial` | Answered/forwarded, link text |

## Surface Map

Every field in `Styles` with its semantic role and palette assignment.

### Chrome frame

| Field | fg | bg | Role |
|-------|----|----|------|
| `FrameBorder` | `BgBorder` | — | Top line, right edge `│`, status bar corners |
| `PanelDivider` | `BgBorder` | — | Vertical `│` between sidebar and main panel |
| `TopLine` | `BgBorder` | — | Characters of the top frame line |

### Status bar

| Field | fg | bg | Role |
|-------|----|----|------|
| `StatusBar` | `FgBright` | `BgBorder` | Bottom status bar base text |
| `StatusConnected` | `ColorSuccess` | `BgBorder` | `●` connected indicator |
| `StatusReconnect` | `ColorWarning` | `BgBorder` | `◐` reconnecting indicator |
| `StatusOffline` | `ColorError` | `BgBorder` | `○` offline indicator |
| `ErrorBanner` | `ColorError` | — | One-line banner above the status bar (`⚠ <op>: <err>`) |

The `ErrorBanner` is the only chrome surface that conditionally takes
a row: when `App.lastErr.Err != nil` the banner renders between the
content area and the status bar, and the account region shrinks by
one cell so total view height is unchanged. Foreground only — no
background fill — so the banner sits visually on the same plane as
the content rather than chrome.

### Footer (command hints)

| Field | fg | bg | Role |
|-------|----|----|------|
| `FooterKey` | `FgBright` bold | — | Key names (`j`, `Tab`, `Enter`) |
| `FooterHint` | `FgDim` | — | Action descriptions after `:` |
| `FooterSep` | `FgDim` | — | `┊` group separators |

### Sidebar

The sidebar is a raised surface — every row has `BgElevated` as its
background. Selected rows override with `BgSelection`. The account name
is the only row with a foreground accent; folder rows use `FgBase`
until unread, then escalate to `FgBright` + bold.

| Field | fg | bg | Role |
|-------|----|----|------|
| `SidebarBg` | — | `BgElevated` | Base sidebar panel background (applied to every row) |
| `SidebarAccount` | `AccentSecondary` bold | `BgElevated` | Account name at top |
| `SidebarSelected` | — | `BgSelection` | Selected row override |
| `SidebarFolder` | `FgBase` | inherit | Folder icon + name (no unread) |
| `SidebarUnread` | `FgBright` bold | inherit | Folder icon + name + count (has unread) |
| `SidebarIndicator` | `AccentSecondary` | inherit | `┃` selection indicator (selected row) |

Background composition: sidebar rows carry their bg via the `bgStyle`
parameter into `renderRow`. Each text segment runs through `withBg()`
which layers the background onto a base foreground style. This is the
only way lipgloss lets us compose fg + bg without clobbering
already-rendered ANSI.

### Message list

The message list is the primary content surface — its background is
`BgBase`, the same as the rest of the right panel. Selected rows
overlay `BgSelection`. **The list has two visual layers: brightness
(read vs unread) and glyph (flag vs answered vs unread vs none).
Color hue is used only for the cursor and for the single
unread+flagged case** — every other distinction is carried by text
brightness or by which glyph appears, not by hue. This keeps the row
to at most two accent hues simultaneously and matches the
"brightness, not hue" convention used by Apple Mail, Fastmail,
Gmail, and Mutt.

| Field | fg | bg | Role |
|-------|----|----|------|
| `MsgListBg` | — | `BgBase` | Base panel background (every row) |
| `MsgListSelected` | — | `BgSelection` | Selected row background override (matches the sidebar's selection treatment for consistency) |
| `MsgListCursor` | `AccentPrimary` | inherit | `▐` left-edge cursor on selected row |
| `MsgListUnreadSender` | `FgBright` bold | inherit | Sender column when message is unread |
| `MsgListUnreadSubject` | `FgBright` | inherit | Subject column when message is unread |
| `MsgListReadSender` | `FgDim` | inherit | Sender column when message has been read |
| `MsgListReadSubject` | `FgDim` | inherit | Subject column when message has been read |
| `MsgListDate` | `FgDim` | inherit | Date column (always de-emphasized) |
| `MsgListIconUnread` | `FgBright` | inherit | Any glyph (`󰇮 󰑚`) on an unread row — matches the row's text brightness |
| `MsgListIconRead` | `FgDim` | inherit | Any glyph (`󰑚 󰈻`) on a read row — inherits the row's dimness |
| `MsgListFlagFlagged` | `ColorWarning` | inherit | `󰈻` flag icon **only when row is also unread** — the single permitted accent for the highest-priority row state |
| `MsgListThreadPrefix` | `FgDim` | inherit | Box-drawing thread prefix (`├─`, `└─`, `│`) and `[N]` collapsed-thread badge |

Background composition uses the shared `applyBg(base, bgStyle)`
helper from `styles.go`: each text segment layers its base foreground
style over the row background without clobbering already-rendered
ANSI. The cursor `▐` is the only character that escalates to
`AccentPrimary`; the rest of the selected row keeps its unread/read
foreground colors so the visual rhythm of unread vs read survives
selection.

**Read state always wins over flag state for icon color.** A read
flagged message gets the dim flag glyph (`󰈻` in `FgDim`), not the
orange one. Once you've read it you've acknowledged the importance —
the row dims completely. Only the *unread* flagged row gets the
orange accent, so it pops as the single most attention-worthy item
in the list. This follows Tufte's data-ink principle: spend hue on
the row that demands action, withhold it everywhere else.

**Date is always dim, even on unread rows.** The date is metadata,
not content — escalating it would compete with the sender/subject for
the eye's first stop. The wireframe shows this consistently across all
states.

**Why brightness and not hue for unread.** An earlier iteration used
`AccentTertiary` (teal) for unread sender/subject and three different
accent hues (teal/orange/purple) for the three flag types. Four
competing hues per row on a muted Nord background fragments
attention — no single element wins the eye, and "garish" is the
right word for it. Brightness alone (`FgBright` vs `FgDim`) is enough
signal for read state, and it leaves the hue budget free for the
cursor and for flagged-unread, the two things that genuinely need to
pop.

### Message viewer

The viewer is composed of two surfaces. The header is a panel on
`BgElevated` running from row 1 down through its bottom border;
the body sits below it on `BgBase`, sharing that surface with the
message list. The 1-cell leading column of the pane stays `BgBase`
in every row — the panel inhabits column 1 through column
`v.width-1`, leaving the pane's leading gutter intact for vertical
alignment with the sidebar.

Inside the panel the Subject sits on the first row at column 2
(after the BgBase gutter and the panel's 1-cell `PaddingLeft`).
A blank row separates the Subject from the metadata block (From/To/
Cc/Bcc/Date), which is indented two cells further inward and
rendered with uppercase labels in `FgDim` and no colons. The panel
closes with a single-line bottom border (`─`) in `FgDim` on
`BgElevated`. A blank gutter row on `BgBase` separates the panel
from the body, and a final blank row on `BgBase` closes the pane.

| Field | fg | bg | Role |
|-------|----|----|------|
| `ViewerBg` | — | `BgBase` | Body surface + leading 1-cell column + gutter and bottom blank rows |
| `ViewerHeader` | — | `BgSubtle` | Header panel surface — barely-elevated off the body's `BgBase`, signaling the header belongs to the message context rather than the sidebar's chrome track. `lipgloss.Style` with `Padding(1, 0, 1, 1)`, a bottom border in `FgDim` on `BgBase`, and `Width(v.width)` set per render |
| `SubjectTitle` (theme) | `FgBright` bold | — | Subject rendered on the first row of the panel |
| `HeaderValue` (theme) | `FgBase` | — | Address name and scalar value in the metadata block |
| `HeaderDim` (theme) | `FgDim` | — | Uppercase `FROM`/`TO`/`CC`/`BCC`/`DATE` labels and `<email>` brackets in the metadata block; also the panel's bottom border |

Background composition: the header panel is rendered by lipgloss
with `Width()` set, so each panel row arrives pre-filled to the
panel width with `BgElevated` — no manual right-edge fill is needed
inside the panel. `padLeftLinesBg` then prepends a `BgBase` cell at
column 0. Body rows are still padded by `padLeftLinesBg` and fully
re-filled by `clipPaneBg` (via `bgFillLine` + `fillRowToWidth`) so
the body's `BgBase` extends to the pane edge across every line.

### Help popover (modal overlay, `?`)

| Field | fg | bg | Role |
|-------|----|----|------|
| `HelpTitle` | `AccentPrimary` (bold) | — | Popover title embedded in top border ("Message List" / "Message Viewer") |
| `HelpGroupHeader` | `FgBright` (bold) | — | Group headings ("Navigate", "Triage", etc.) |
| `HelpKey` | `FgBright` (bold) | — | Key column for *wired* rows |
| `Dim` (reuse) | `FgDim` | — | Description column (all rows) and entire key+desc for *unwired* (future) rows |
| `FrameBorder` (reuse) | `BgBorder` | — | Rounded box border |

**Wired vs. unwired:** rows whose binding is not yet implemented
render the entire row in `Dim` (no bold). Group headings stay
`HelpGroupHeader` (bright) regardless. The contrast between the
bright-bold key column on wired rows and the flat-dim key column
on unwired rows is the only visual signal — no glyph, no legend.
See ADR for help popover future-binding policy.

### Tab bar (unused in current chrome, reserved)

| Field | fg | bg | Role |
|-------|----|----|------|
| `TabActiveBorder` | `BgBorder` | — | Active tab border |
| `TabActiveText` | `AccentSecondary` | `BgBase` | Active tab label |
| `TabInactiveText` | `FgDim` | — | Inactive tab label |
| `TabConnectLine` | `BgBorder` | — | Tab-to-frame connector |

### Miscellaneous

| Field | fg | bg | Role |
|-------|----|----|------|
| `Selection` | — | `BgSelection` | Generic selection highlight (message list, etc.) |
| `Dim` | `FgDim` | — | Placeholder text ("Message List" etc.) |
| `ToastText` | `ColorSuccess` | — | Toast notifications |

## Guidelines

1. **Never hardcode a hex value in Go code.** Always go through a
   `CompiledTheme` slot.
2. **Never call `lipgloss.NewStyle()` directly in a component.** Pull
   from `Styles`. If the surface doesn't have a style yet, add one to
   `Styles` + update this doc.
3. **Surface = one `Styles` field.** Don't reuse `Selection` when you
   mean `SidebarSelected`. Semantic names let themes evolve without
   scavenging for callsites.
4. **Document why, not what.** The table entries tell you the role.
   Any deviation (e.g., escalation from `FgBase` to `FgBright` on
   unread) needs a line in this doc explaining the rule.
5. **Background composition:** when a style needs to layer on top of
   another row background, use the `withBg()` closure pattern from
   `sidebar.go:renderRow`. Don't set backgrounds on individual text
   segments directly.
