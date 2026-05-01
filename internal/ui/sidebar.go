// SPDX-License-Identifier: MIT

package ui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

type FolderGroup int

const (
	GroupPrimary FolderGroup = iota
	GroupDisposal
	GroupCustom
)

type FolderEntry struct {
	Display  string
	Provider string
	Group    FolderGroup
}

// folderEntry holds a classified folder plus its rendered metadata.
type folderEntry struct {
	cf   mail.ClassifiedFolder
	icon string
}

// Sidebar renders the folder list with groups, selection, and unread badges.
type Sidebar struct {
	entries  []folderEntry
	selected int
	styles   Styles
	icons    IconSet
	width    int
	height   int
}

// NewSidebar creates a Sidebar from a pre-classified folder list and
// a UIConfig. Ordering, hiding, labelling, and indent calculation
// happen here. Hidden folders are dropped before indexing.
func NewSidebar(styles Styles, classified []mail.ClassifiedFolder, uiCfg config.UIConfig, width, height int, icons IconSet) Sidebar {
	return Sidebar{
		entries:  buildEntries(classified, uiCfg, icons),
		selected: 0,
		styles:   styles,
		icons:    icons,
		width:    width,
		height:   height,
	}
}

// SetFolders replaces the sidebar's folder set with a newly classified
// list under a given UIConfig. Selection is preserved by provider name
// where possible; otherwise it resets to 0.
func (s *Sidebar) SetFolders(classified []mail.ClassifiedFolder, uiCfg config.UIConfig) {
	var prevName string
	if s.selected < len(s.entries) {
		prevName = s.entries[s.selected].cf.Folder.Name
	}
	s.entries = buildEntries(classified, uiCfg, s.icons)
	s.selected = 0
	if prevName != "" {
		for i, e := range s.entries {
			if e.cf.Folder.Name == prevName {
				s.selected = i
				break
			}
		}
	}
}

// Selected returns the index of the currently selected folder.
func (s Sidebar) Selected() int { return s.selected }

// SelectedFolder returns the provider name of the currently selected folder.
// Backends look up folders by provider name, not display name.
func (s Sidebar) SelectedFolder() string {
	if s.selected < len(s.entries) {
		return s.entries[s.selected].cf.Folder.Name
	}
	return ""
}

// SelectedFolderInfo returns the raw backend Folder at the current selection.
func (s Sidebar) SelectedFolderInfo() (mail.Folder, bool) {
	if s.selected < len(s.entries) {
		return s.entries[s.selected].cf.Folder, true
	}
	return mail.Folder{}, false
}

// ConfigKey returns the UIConfig.Folders lookup key for the folder
// with the given provider name (canonical name for canonicals,
// provider name for custom). Returns "" if no matching folder is in
// the sidebar — the zero-value FolderConfig falls through to
// group/global defaults.
func (s Sidebar) ConfigKey(providerName string) string {
	for _, e := range s.entries {
		if e.cf.Folder.Name == providerName {
			return e.cf.ConfigKey()
		}
	}
	return ""
}

// FolderNameByCanonical returns the provider folder name (e.g.
// "Archive", "[Gmail]/Trash") whose canonical name matches target.
// Returns ("", false) when no folder matches — used by triage actions
// to look up the Archive/Trash destinations.
func (s Sidebar) FolderNameByCanonical(target string) (string, bool) {
	for _, e := range s.entries {
		if e.cf.Canonical == target {
			return e.cf.Folder.Name, true
		}
	}
	return "", false
}

// FolderByProviderName returns the mail.Folder whose backend name matches.
// Returns (Folder{}, false) when no entry matches.
func (s Sidebar) FolderByProviderName(name string) (mail.Folder, bool) {
	for _, e := range s.entries {
		if e.cf.Folder.Name == name {
			return e.cf.Folder, true
		}
	}
	return mail.Folder{}, false
}

func (s Sidebar) OrderedFolders() []FolderEntry {
	out := make([]FolderEntry, 0, len(s.entries))
	for _, e := range s.entries {
		display := e.cf.Canonical
		if display == "" {
			display = e.cf.Folder.Name
		}
		out = append(out, FolderEntry{
			Display:  display,
			Provider: e.cf.Folder.Name,
			Group:    translateGroup(e.cf.Group),
		})
	}
	return out
}

func translateGroup(g mail.Group) FolderGroup {
	switch g {
	case mail.GroupPrimary:
		return GroupPrimary
	case mail.GroupDisposal:
		return GroupDisposal
	default:
		return GroupCustom
	}
}

// SelectByCanonical moves the selection to the folder whose
// canonical name matches target (e.g. "Inbox", "Drafts"). Returns
// true if a matching folder was found and selected. No-op when no
// folder matches.
func (s *Sidebar) SelectByCanonical(target string) bool {
	for i, e := range s.entries {
		if e.cf.Canonical == target {
			s.selected = i
			return true
		}
	}
	return false
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
	prevGroup := s.entries[0].cf.Group

	for i, entry := range s.entries {
		if i > 0 && entry.cf.Group != prevGroup {
			lines = append(lines, s.renderBlankLine())
		}
		prevGroup = entry.cf.Group
		bg := plainBg
		if i == s.selected {
			bg = selectedBg
		}
		lines = append(lines, s.renderRow(i, entry, bg))
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
// The selection indicator ┃ always sits in column 0. All folders render
// at the same indent regardless of nesting.
func (s Sidebar) renderRow(idx int, entry folderEntry, bgStyle lipgloss.Style) string {
	isSelected := idx == s.selected
	hasUnread := entry.cf.Folder.Unseen > 0

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
	name := applyBg(textStyle, bgStyle).Render(entry.cf.DisplayName)

	var countStr string
	var countWidth int
	if hasUnread {
		countStr = applyBg(textStyle, bgStyle).Render(strconv.Itoa(entry.cf.Folder.Unseen))
		countWidth = lipgloss.Width(countStr)
	}

	// Layout: indicator(1) + sp(1) + icon(2) + sp×2 + name + gap + count + margin(1)
	leftContent := indicator + bgStyle.Render(" ") + icon + bgStyle.Render("  ") + name
	leftWidth := displayCells(leftContent)

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

// buildEntries applies UIConfig to the classified folders: drops hidden
// folders, resolves display labels, sorts each group by rank then display
// name, and concatenates Primary + Disposal + Custom in that order.
func buildEntries(classified []mail.ClassifiedFolder, uiCfg config.UIConfig, icons IconSet) []folderEntry {
	var primary, disposal, custom []folderEntry
	for _, cf := range classified {
		fc := uiCfg.Folders[cf.ConfigKey()]
		if fc.Hide {
			continue
		}
		entry := folderEntry{
			cf:   cf,
			icon: sidebarIconFrom(icons, cf),
		}
		if fc.Label != "" {
			entry.cf.DisplayName = fc.Label
		}
		switch cf.Group {
		case mail.GroupPrimary:
			primary = append(primary, entry)
		case mail.GroupDisposal:
			disposal = append(disposal, entry)
		default:
			custom = append(custom, entry)
		}
	}
	sortEntries(primary, uiCfg)
	sortEntries(disposal, uiCfg)
	sortEntries(custom, uiCfg)

	out := make([]folderEntry, 0, len(primary)+len(disposal)+len(custom))
	out = append(out, primary...)
	out = append(out, disposal...)
	out = append(out, custom...)
	return out
}

// canonicalDefaultRank is the implicit in-group sort key for each
// canonical folder. Custom folders (Canonical == "") fall through to
// the non-canonical default and are ordered alphabetically.
var canonicalDefaultRank = map[string]int{
	"Inbox":   100,
	"Drafts":  200,
	"Sent":    300,
	"Archive": 400,
	"Spam":    100,
	"Trash":   200,
}

const nonCanonicalDefaultRank = 1000

// sortEntries orders a group by (rank, display name). Rank comes
// from user config if set, otherwise from canonicalDefaultRank for
// canonicals or nonCanonicalDefaultRank for custom folders.
func sortEntries(entries []folderEntry, uiCfg config.UIConfig) {
	sort.SliceStable(entries, func(i, j int) bool {
		ri := rankOf(entries[i].cf, uiCfg)
		rj := rankOf(entries[j].cf, uiCfg)
		if ri != rj {
			return ri < rj
		}
		return entries[i].cf.DisplayName < entries[j].cf.DisplayName
	})
}

func rankOf(cf mail.ClassifiedFolder, uiCfg config.UIConfig) int {
	if fc := uiCfg.Folders[cf.ConfigKey()]; fc.RankSet {
		return fc.Rank
	}
	if r, ok := canonicalDefaultRank[cf.Canonical]; ok {
		return r
	}
	return nonCanonicalDefaultRank
}

// sidebarIconFrom returns the icon for a classified folder from the given
// IconSet. Canonicals use their canonical icon; all other folders use
// CustomFolder regardless of name.
func sidebarIconFrom(icons IconSet, cf mail.ClassifiedFolder) string {
	switch cf.Canonical {
	case "Inbox":
		return icons.Inbox
	case "Drafts":
		return icons.Drafts
	case "Sent":
		return icons.Sent
	case "Archive":
		return icons.Archive
	case "Spam":
		return icons.Spam
	case "Trash":
		return icons.Trash
	}
	return icons.CustomFolder
}
