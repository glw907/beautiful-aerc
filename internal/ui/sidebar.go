package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/mail"
)

// folderGroup classifies folders for visual grouping.
type folderGroup int

const (
	primaryGroup  folderGroup = iota // inbox, drafts, sent, archive
	disposalGroup                    // spam, trash
	customGroup                      // everything else
)

// folderEntry holds a folder and its display metadata.
type folderEntry struct {
	folder mail.Folder
	icon   string
	group  folderGroup
}

// Sidebar renders the folder list with groups, selection, and unread badges.
type Sidebar struct {
	entries  []folderEntry
	selected int
	styles   Styles
	width    int
	height   int
}

// NewSidebar creates a Sidebar from a folder list.
func NewSidebar(styles Styles, folders []mail.Folder, width, height int) Sidebar {
	entries := make([]folderEntry, len(folders))
	for i, f := range folders {
		entries[i] = folderEntry{
			folder: f,
			icon:   sidebarIcon(f),
			group:  classifyGroup(f),
		}
	}
	return Sidebar{
		entries:  entries,
		selected: 0,
		styles:   styles,
		width:    width,
		height:   height,
	}
}

// Selected returns the index of the currently selected folder.
func (s Sidebar) Selected() int { return s.selected }

// SelectedFolder returns the name of the currently selected folder.
func (s Sidebar) SelectedFolder() string {
	if s.selected < len(s.entries) {
		return s.entries[s.selected].folder.Name
	}
	return ""
}

// SelectedFolderInfo returns the folder at the current selection.
func (s Sidebar) SelectedFolderInfo() (mail.Folder, bool) {
	if s.selected < len(s.entries) {
		return s.entries[s.selected].folder, true
	}
	return mail.Folder{}, false
}

// SelectedIcon returns the icon of the currently selected folder.
func (s Sidebar) SelectedIcon() string {
	if s.selected < len(s.entries) {
		return s.entries[s.selected].icon
	}
	return ""
}

// SetSize updates the sidebar dimensions.
func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// MoveUp moves the selection up by one.
func (s *Sidebar) MoveUp() {
	if s.selected > 0 {
		s.selected--
	}
}

// MoveDown moves the selection down by one.
func (s *Sidebar) MoveDown() {
	if s.selected < len(s.entries)-1 {
		s.selected++
	}
}

// MoveToTop moves the selection to the first folder.
func (s *Sidebar) MoveToTop() { s.selected = 0 }

// MoveToBottom moves the selection to the last folder.
func (s *Sidebar) MoveToBottom() {
	if len(s.entries) > 0 {
		s.selected = len(s.entries) - 1
	}
}

// View renders the sidebar as a vertical list of folder rows.
func (s Sidebar) View() string {
	if len(s.entries) == 0 || s.width == 0 || s.height == 0 {
		return ""
	}

	plainBg := s.styles.SidebarBg
	selectedBg := s.styles.SidebarSelected

	var lines []string
	prevGroup := s.entries[0].group

	for i, entry := range s.entries {
		if i > 0 && entry.group != prevGroup {
			lines = append(lines, s.renderBlankLine())
			prevGroup = entry.group
		}
		bg := plainBg
		if i == s.selected {
			bg = selectedBg
		}
		lines = append(lines, s.renderRow(i, entry, bg))
		prevGroup = entry.group
	}

	for len(lines) < s.height {
		lines = append(lines, s.renderBlankLine())
	}

	if len(lines) > s.height {
		lines = lines[:s.height]
	}

	return strings.Join(lines, "\n")
}

// renderRow renders a single folder row with proper background layering.
// bgStyle carries the row's background color (selection or plain).
func (s Sidebar) renderRow(idx int, entry folderEntry, bgStyle lipgloss.Style) string {
	isSelected := idx == s.selected
	hasUnread := entry.folder.Unseen > 0

	// Indicator: ┃ when selected, space otherwise
	var indicator string
	if isSelected {
		indicator = applyBg(s.styles.SidebarIndicator, bgStyle).Render("┃")
	} else {
		indicator = bgStyle.Render(" ")
	}

	textStyle := s.styles.SidebarFolder
	if hasUnread {
		textStyle = s.styles.SidebarUnread
	}
	icon := applyBg(textStyle, bgStyle).Render(entry.icon)
	name := applyBg(textStyle, bgStyle).Render(entry.folder.Name)

	var countStr string
	var countWidth int
	if hasUnread {
		countStr = applyBg(textStyle, bgStyle).Render(fmt.Sprintf("%d", entry.folder.Unseen))
		countWidth = lipgloss.Width(countStr)
	}

	// Layout: indicator(1) + sp(1) + icon(~2) + sp(2) + name + gap + count + margin(1)
	leftContent := indicator + bgStyle.Render(" ") + icon + bgStyle.Render("  ") + name
	leftWidth := lipgloss.Width(leftContent)

	rightMargin := 1
	gap := max(1, s.width-leftWidth-countWidth-rightMargin)

	row := leftContent +
		bgStyle.Render(strings.Repeat(" ", gap)) +
		countStr +
		bgStyle.Render(strings.Repeat(" ", rightMargin))

	return fillRowToWidth(row, s.width, bgStyle)
}

// renderBlankLine renders an empty line at the sidebar width with the sidebar background.
func (s Sidebar) renderBlankLine() string {
	return s.styles.SidebarBg.Width(s.width).Render("")
}

// sidebarIcon returns the Nerd Font icon for a folder based on role and name.
func sidebarIcon(f mail.Folder) string {
	switch strings.ToLower(f.Role) {
	case "inbox":
		return "󰇰"
	case "drafts":
		return "󰏫"
	case "sent":
		return "󰑚"
	case "archive":
		return "󰀼"
	case "junk":
		return "󰍷"
	case "trash":
		return "󰩺"
	}
	lower := strings.ToLower(f.Name)
	switch {
	case strings.Contains(lower, "notification"):
		return "󰂚"
	case strings.Contains(lower, "remind"):
		return "󰑴"
	default:
		return "󰡡"
	}
}

// classifyGroup assigns a folder to its visual group.
func classifyGroup(f mail.Folder) folderGroup {
	switch strings.ToLower(f.Role) {
	case "inbox", "drafts", "sent", "archive":
		return primaryGroup
	case "junk", "trash":
		return disposalGroup
	default:
		return customGroup
	}
}
