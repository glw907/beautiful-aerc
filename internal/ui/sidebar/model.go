package sidebar

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

type folderEntry struct {
	cf   mail.ClassifiedFolder
	icon string
}

// Model renders the folder list with groups, selection, and unread badges.
type Model struct {
	entries     []folderEntry
	selected    int
	outboxCount int
	styles      Styles
	icons       uicore.IconSet
	layout      uicore.LayoutMode
	width       int
	height      int
}

// New builds a Model from a pre-classified folder list. UIConfig drives
// ordering, hiding, and labelling; hidden folders are dropped before indexing.
func New(styles Styles, classified []mail.ClassifiedFolder, uiCfg config.UIConfig, width, height int, icons uicore.IconSet) Model {
	return Model{
		entries:  buildEntries(classified, uiCfg, icons),
		selected: 0,
		styles:   styles,
		icons:    icons,
		width:    width,
		height:   height,
	}
}

// SetFolders replaces the folder set under a given UIConfig. Selection
// is preserved by provider name where possible; otherwise it resets to 0.
func (s *Model) SetFolders(classified []mail.ClassifiedFolder, uiCfg config.UIConfig) {
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

// SetOutboxCount sets the depth of the synthetic Outbox entry. When n > 0,
// the entry renders at the top of the Disposal group; when 0, it is omitted.
func (s *Model) SetOutboxCount(n int) {
	if n < 0 {
		n = 0
	}
	s.outboxCount = n
}

// effectiveEntries returns s.entries with the synthetic Outbox row injected
// at the top of the Disposal group when outboxCount > 0. Pure, no mutation.
func (s Model) effectiveEntries() []folderEntry {
	if s.outboxCount == 0 {
		return s.entries
	}
	out := make([]folderEntry, 0, len(s.entries)+1)
	inserted := false
	for _, e := range s.entries {
		if !inserted && e.cf.Group == mail.GroupDisposal {
			out = append(out, syntheticOutboxEntry(s.outboxCount, s.icons))
			inserted = true
		}
		out = append(out, e)
	}
	if !inserted {
		out = append(out, syntheticOutboxEntry(s.outboxCount, s.icons))
	}
	return out
}

func syntheticOutboxEntry(count int, icons uicore.IconSet) folderEntry {
	return folderEntry{
		cf: mail.ClassifiedFolder{
			Folder:      mail.Folder{Name: mail.CanonicalOutbox, Unseen: count},
			Canonical:   mail.CanonicalOutbox,
			DisplayName: mail.CanonicalOutbox,
			Group:       mail.GroupDisposal,
		},
		icon: icons.Sent, // closest visual fit; a dedicated icon is post-1.0
	}
}

func (s Model) Selected() int { return s.selected }

// SelectedFolder returns the provider name of the currently selected folder.
// Backends look up folders by provider name, not display name.
func (s Model) SelectedFolder() string {
	entries := s.effectiveEntries()
	if s.selected < len(entries) {
		return entries[s.selected].cf.Folder.Name
	}
	return ""
}

// SelectedCanonical returns the canonical name (e.g. "Inbox", "Sent") of the
// currently selected folder. Returns "" for custom folders.
func (s Model) SelectedCanonical() string {
	entries := s.effectiveEntries()
	if s.selected < len(entries) {
		return entries[s.selected].cf.Canonical
	}
	return ""
}

func (s Model) SelectedFolderInfo() (mail.Folder, bool) {
	entries := s.effectiveEntries()
	if s.selected < len(entries) {
		return entries[s.selected].cf.Folder, true
	}
	return mail.Folder{}, false
}

// ConfigKey returns the UIConfig.Folders lookup key for the folder with the
// given provider name (canonical name for canonicals, provider name for
// custom). Returns "" if no matching folder is in the sidebar.
func (s Model) ConfigKey(providerName string) string {
	for _, e := range s.effectiveEntries() {
		if e.cf.Folder.Name == providerName {
			return e.cf.ConfigKey()
		}
	}
	return ""
}

// FolderNameByCanonical returns the provider folder name whose canonical
// matches target. Returns ("", false) when no folder matches.
func (s Model) FolderNameByCanonical(target string) (string, bool) {
	for _, e := range s.effectiveEntries() {
		if e.cf.Canonical == target {
			return e.cf.Folder.Name, true
		}
	}
	return "", false
}

func (s Model) FolderByProviderName(name string) (mail.Folder, bool) {
	for _, e := range s.effectiveEntries() {
		if e.cf.Folder.Name == name {
			return e.cf.Folder, true
		}
	}
	return mail.Folder{}, false
}

func (s Model) OrderedFolders() []mail.FolderEntry {
	entries := s.effectiveEntries()
	out := make([]mail.FolderEntry, 0, len(entries))
	for _, e := range entries {
		display := e.cf.DisplayName
		if display == "" {
			display = e.cf.Canonical
		}
		if display == "" {
			display = e.cf.Folder.Name
		}
		out = append(out, mail.FolderEntry{
			Display:  display,
			Provider: e.cf.Folder.Name,
			Group:    e.cf.Group,
		})
	}
	return out
}

// SelectByCanonical moves the selection to the folder whose canonical name
// matches target (e.g. "Inbox", "Drafts"). Returns true if found.
func (s *Model) SelectByCanonical(target string) bool {
	for i, e := range s.effectiveEntries() {
		if e.cf.Canonical == target {
			s.selected = i
			return true
		}
	}
	return false
}

func (s Model) SelectedIcon() string {
	entries := s.effectiveEntries()
	if s.selected < len(entries) {
		return entries[s.selected].icon
	}
	return ""
}

func (s *Model) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s Model) Layout() uicore.LayoutMode { return s.layout }

// SetLayout updates the icon toggle. Width is owned by SetSize.
func (s *Model) SetLayout(l uicore.LayoutMode) {
	s.layout = l
}

func (s *Model) MoveUp() {
	if s.selected > 0 {
		s.selected--
	}
}

func (s *Model) MoveDown() {
	if s.selected < len(s.effectiveEntries())-1 {
		s.selected++
	}
}

func (s *Model) MoveToTop() { s.selected = 0 }

func (s *Model) MoveToBottom() {
	if n := len(s.effectiveEntries()); n > 0 {
		s.selected = n - 1
	}
}

func (s Model) View() string {
	entries := s.effectiveEntries()
	if len(entries) == 0 || s.width == 0 || s.height == 0 {
		return ""
	}

	plainBg := s.styles.SidebarBg
	selectedBg := s.styles.SidebarSelected

	var lines []string
	prevGroup := entries[0].cf.Group

	for i, entry := range entries {
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

// renderRow draws one folder row. The selection indicator ┃ sits in
// column 0; the icon block is included only when s.layout.Icons is true.
func (s Model) renderRow(idx int, entry folderEntry, bgStyle lipgloss.Style) string {
	isSelected := idx == s.selected
	hasUnread := entry.cf.Folder.Unseen > 0

	var indicator string
	if isSelected {
		indicator = uicore.ApplyBg(s.styles.SidebarIndicator, bgStyle).Render("┃")
	} else {
		indicator = bgStyle.Render(" ")
	}

	textStyle := s.styles.SidebarFolder
	if hasUnread {
		textStyle = s.styles.SidebarUnread
	}

	var icon string
	var leadCells int
	if s.layout.Icons {
		icon = uicore.ApplyBg(textStyle, bgStyle).Render(entry.icon)
		leadCells = ansix.Width(indicator) + 1 + ansix.Width(icon) + 2
	} else {
		leadCells = ansix.Width(indicator) + 1
	}

	var countStr string
	var countWidth int
	if hasUnread {
		countStr = uicore.ApplyBg(textStyle, bgStyle).Render(strconv.Itoa(entry.cf.Folder.Unseen))
		countWidth = lipgloss.Width(countStr)
	}

	const rightMargin = 1
	countGap := 0
	if hasUnread {
		countGap = 1
	}
	labelBudget := s.width - leadCells - countWidth - countGap - rightMargin
	if labelBudget < 1 {
		labelBudget = 1
	}
	displayName := ansix.TruncateEllipsis(entry.cf.DisplayName, labelBudget)
	name := uicore.ApplyBg(textStyle, bgStyle).Render(displayName)

	var leftContent string
	if s.layout.Icons {
		leftContent = indicator + bgStyle.Render(" ") + icon + bgStyle.Render("  ") + name
	} else {
		leftContent = indicator + bgStyle.Render(" ") + name
	}
	leftWidth := ansix.Width(leftContent)

	gap := max(1, s.width-leftWidth-countWidth-rightMargin)

	row := leftContent +
		bgStyle.Render(strings.Repeat(" ", gap)) +
		countStr +
		bgStyle.Render(strings.Repeat(" ", rightMargin))

	return uicore.FillRowToWidth(row, s.width, bgStyle)
}

func (s Model) renderBlankLine() string {
	return s.styles.SidebarBg.Width(s.width).Render("")
}

// buildEntries drops hidden folders, resolves display labels, and sorts
// each group by rank then display name. Output order is Primary, Disposal,
// Custom.
func buildEntries(classified []mail.ClassifiedFolder, uiCfg config.UIConfig, icons uicore.IconSet) []folderEntry {
	var primary, disposal, custom []folderEntry
	for _, cf := range classified {
		fc := uiCfg.Folders[cf.ConfigKey()]
		if fc.Hide {
			continue
		}
		entry := folderEntry{
			cf:   cf,
			icon: iconFrom(icons, cf),
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

// nonCanonicalDefaultRank is the rank assigned to custom folders
// (Canonical == ""). It sorts after every canonical default.
const nonCanonicalDefaultRank = 1000

func sortEntries(entries []folderEntry, uiCfg config.UIConfig) {
	slices.SortStableFunc(entries, func(a, b folderEntry) int {
		return cmp.Or(
			cmp.Compare(rankOf(a.cf, uiCfg), rankOf(b.cf, uiCfg)),
			cmp.Compare(a.cf.DisplayName, b.cf.DisplayName),
		)
	})
}

func rankOf(cf mail.ClassifiedFolder, uiCfg config.UIConfig) int {
	if fc := uiCfg.Folders[cf.ConfigKey()]; fc.RankSet {
		return fc.Rank
	}
	switch cf.Canonical {
	case "Inbox", "Spam":
		return 100
	case "Drafts", "Trash":
		return 200
	case "Sent":
		return 300
	case "Archive":
		return 400
	}
	return nonCanonicalDefaultRank
}

func iconFrom(icons uicore.IconSet, cf mail.ClassifiedFolder) string {
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
