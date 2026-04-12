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
| `StatusOffline` | `FgDim` | `BgBorder` | `○` offline indicator |

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
