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
| `BgSelection` | Selected row highlight |
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
| `MsgListSelected` | — | `BgSelection` | Selected row background override |
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

The viewer shares `BgBase` with the message list — the right panel is
a single surface, so the viewer fills its full pane (header rows,
body rows, and the blank padding rows at the top and bottom) with
`BgBase`. Header keys (From/To/Cc/Bcc/Date/Subject) are right-padded
to a common 8-cell column so values align.

| Field | fg | bg | Role |
|-------|----|----|------|
| `ViewerBg` | — | `BgBase` | Base pane background (all padding rows + leading column + right-edge fill) |
| `HeaderKey` (theme) | `AccentPrimary` bold | — | `From:`/`To:`/`Cc:`/`Bcc:`/`Date:`/`Subject:` label |
| `HeaderValue` (theme) | `FgBase` | — | Address name and scalar value |
| `HeaderDim` (theme) | `FgDim` | — | `<email>` brackets and the `─` separator under the headers |

Background composition: `clipPaneBg` and `padLeftLinesBg` use
bg-styled spaces so the right-edge fill, left column, and top/bottom
blank padding rows all carry `BgBase`. Each rendered content line is
then run through `bgFillLine` (in `styles.go`), which prepends the
bg ANSI prefix and re-emits it after every embedded `\x1b[0m` reset
so cells under styled content don't fall back to the terminal default.

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
