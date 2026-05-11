package sidebar

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
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

// KeyMap binds sidebar actions to keys. account.Model is responsible for
// rebinding Down/Up to capital J/K in the account context.
type KeyMap struct {
	Down     key.Binding
	Up       key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Expand   key.Binding
	Collapse key.Binding
}

// DefaultKeyMap returns the sidebar-local key bindings. account.Model
// rebinds Down/Up to J/K when installing the sidebar in its key context.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Down:     key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "folder down")),
		Up:       key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "folder up")),
		Top:      key.NewBinding(key.WithKeys("g")),
		Bottom:   key.NewBinding(key.WithKeys("G")),
		Expand:   key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "expand")),
		Collapse: key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "collapse")),
	}
}

// Model renders the folder list with groups, selection, and unread badges.
type Model struct {
	entries     []folderEntry
	selected    int
	outboxCount int
	expanded    map[string]bool
	keys        KeyMap
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
		expanded: map[string]bool{},
		keys:     DefaultKeyMap(),
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
	s.pruneExpanded(customPaths(s.entries))
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

// IsExpanded reports whether the parent at path is expanded.
func (s Model) IsExpanded(path string) bool {
	return s.expanded[path]
}

// ToggleExpanded flips expansion at path.
func (s *Model) ToggleExpanded(path string) {
	if s.expanded == nil {
		s.expanded = map[string]bool{}
	}
	if s.expanded[path] {
		delete(s.expanded, path)
		return
	}
	s.expanded[path] = true
}

// pruneExpanded drops paths no longer present in known.
func (s *Model) pruneExpanded(known map[string]struct{}) {
	for k := range s.expanded {
		if _, ok := known[k]; !ok {
			delete(s.expanded, k)
		}
	}
}

// customPaths returns every full path that could appear as an expand-state
// key — real Custom folder names plus every synthesized intermediate.
func customPaths(entries []folderEntry) map[string]struct{} {
	out := map[string]struct{}{}
	for _, e := range entries {
		if e.cf.Group != mail.GroupCustom {
			continue
		}
		segs := strings.Split(e.cf.Folder.Name, "/")
		for i := range segs {
			out[strings.Join(segs[:i+1], "/")] = struct{}{}
		}
	}
	return out
}

// allEntries returns s.entries with the synthetic Outbox row injected at the
// top of the Disposal group when outboxCount > 0. Used by lookup methods that
// need all entries regardless of tree expand state.
func (s Model) allEntries() []folderEntry {
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

// visibleRows builds the ordered row list for View and selection: Primary,
// Disposal (with synthetic Outbox if non-zero), then Custom tree-walked.
func (s Model) visibleRows() []rowMeta {
	var primary, disposal, custom []folderEntry
	for _, e := range s.entries {
		switch e.cf.Group {
		case mail.GroupPrimary:
			primary = append(primary, e)
		case mail.GroupDisposal:
			disposal = append(disposal, e)
		default:
			custom = append(custom, e)
		}
	}
	if s.outboxCount > 0 {
		disposal = append([]folderEntry{syntheticOutboxEntry(s.outboxCount, s.icons)}, disposal...)
	}
	// Task 6 wires s.layout.Spartan to set maxDepth = 1 for the Spartan tier.
	maxDepth := 0
	customRows := walkCustom(custom, s.expanded, maxDepth)

	out := make([]rowMeta, 0, len(primary)+len(disposal)+len(customRows))
	for _, e := range primary {
		out = append(out, rowMeta{entry: e})
	}
	for _, e := range disposal {
		out = append(out, rowMeta{entry: e})
	}
	out = append(out, customRows...)
	return out
}

// synthFolder builds a ClassifiedFolder for an intermediate tree node that has
// no on-server folder of its own (e.g. "Lists" when only "Lists/golang" exists).
func synthFolder(path, displayName string) mail.ClassifiedFolder {
	return mail.ClassifiedFolder{
		Folder:      mail.Folder{Name: path},
		DisplayName: displayName,
		Group:       mail.GroupCustom,
	}
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
	rows := s.visibleRows()
	if s.selected < len(rows) {
		return rows[s.selected].entry.cf.Folder.Name
	}
	return ""
}

// SelectedCanonical returns the canonical name (e.g. "Inbox", "Sent") of the
// currently selected folder. Returns "" for custom folders.
func (s Model) SelectedCanonical() string {
	rows := s.visibleRows()
	if s.selected < len(rows) {
		return rows[s.selected].entry.cf.Canonical
	}
	return ""
}

func (s Model) SelectedFolderInfo() (mail.Folder, bool) {
	rows := s.visibleRows()
	if s.selected < len(rows) {
		return rows[s.selected].entry.cf.Folder, true
	}
	return mail.Folder{}, false
}

// ConfigKey returns the UIConfig.Folders lookup key for the folder with the
// given provider name (canonical name for canonicals, provider name for
// custom). Returns "" if no matching folder is in the sidebar.
func (s Model) ConfigKey(providerName string) string {
	for _, e := range s.allEntries() {
		if e.cf.Folder.Name == providerName {
			return e.cf.ConfigKey()
		}
	}
	return ""
}

// FolderNameByCanonical returns the provider folder name whose canonical
// matches target. Returns ("", false) when no folder matches.
func (s Model) FolderNameByCanonical(target string) (string, bool) {
	for _, e := range s.allEntries() {
		if e.cf.Canonical == target {
			return e.cf.Folder.Name, true
		}
	}
	return "", false
}

func (s Model) FolderByProviderName(name string) (mail.Folder, bool) {
	for _, e := range s.allEntries() {
		if e.cf.Folder.Name == name {
			return e.cf.Folder, true
		}
	}
	return mail.Folder{}, false
}

func (s Model) OrderedFolders() []mail.FolderEntry {
	entries := s.allEntries()
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
	for i, r := range s.visibleRows() {
		if r.entry.cf.Canonical == target {
			s.selected = i
			return true
		}
	}
	return false
}

func (s Model) SelectedIcon() string {
	rows := s.visibleRows()
	if s.selected < len(rows) {
		return rows[s.selected].entry.icon
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

func (s *Model) SetKeyMap(km KeyMap) { s.keys = km }
func (s Model) KeyMap() KeyMap       { return s.keys }

// Update dispatches sidebar key events. Inert on non-key messages.
func (s Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch {
	case key.Matches(km, s.keys.Down):
		if s.selected < len(s.visibleRows())-1 {
			s.selected++
		}
	case key.Matches(km, s.keys.Up):
		if s.selected > 0 {
			s.selected--
		}
	case key.Matches(km, s.keys.Top):
		s.selected = 0
	case key.Matches(km, s.keys.Bottom):
		if n := len(s.visibleRows()); n > 0 {
			s.selected = n - 1
		}
	case key.Matches(km, s.keys.Expand):
		rows := s.visibleRows()
		if s.selected < len(rows) && rows[s.selected].hasChildren {
			if s.expanded == nil {
				s.expanded = map[string]bool{}
			}
			s.expanded[rows[s.selected].expandKey()] = true
		}
	case key.Matches(km, s.keys.Collapse):
		rows := s.visibleRows()
		if s.selected < len(rows) {
			r := rows[s.selected]
			if r.hasChildren && s.expanded[r.expandKey()] {
				delete(s.expanded, r.expandKey())
			} else if r.depth > 0 {
				s.collapseToAncestor(rows)
			}
		}
	}
	return s, nil
}

// collapseToAncestor finds the immediate ancestor of the selected row and
// jumps the cursor there, then collapses it.
func (s *Model) collapseToAncestor(rows []rowMeta) {
	cur := rows[s.selected]
	for i := s.selected - 1; i >= 0; i-- {
		if rows[i].depth < cur.depth && rows[i].hasChildren {
			s.selected = i
			delete(s.expanded, rows[i].expandKey())
			return
		}
	}
}

func (s Model) View() string {
	rows := s.visibleRows()
	if len(rows) == 0 || s.width == 0 || s.height == 0 {
		return ""
	}

	plainBg := s.styles.SidebarBg
	selectedBg := s.styles.SidebarSelected

	var lines []string
	prevGroup := rows[0].entry.cf.Group

	for i, r := range rows {
		if i > 0 && r.entry.cf.Group != prevGroup {
			lines = append(lines, s.renderBlankLine())
		}
		prevGroup = r.entry.cf.Group
		bg := plainBg
		if i == s.selected {
			bg = selectedBg
		}
		lines = append(lines, s.renderRow(i, r, bg))
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
// unseen combines entry.cf.Folder.Unseen with r.aggUnread (descendants of
// collapsed parents).
func (s Model) renderRow(idx int, r rowMeta, bgStyle lipgloss.Style) string {
	entry := r.entry
	unseen := entry.cf.Folder.Unseen + r.aggUnread
	isSelected := idx == s.selected
	hasUnread := unseen > 0

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

	treePrefix := r.prefix()
	var treePrefixRendered string
	var treePrefixCells int
	if treePrefix != "" {
		treePrefixRendered = uicore.ApplyBg(s.styles.SidebarTreeRule, bgStyle).Render(treePrefix)
		treePrefixCells = ansix.Width(treePrefixRendered)
	}

	var icon string
	var leadCells int
	if s.layout.Icons {
		icon = uicore.ApplyBg(textStyle, bgStyle).Render(entry.icon)
		leadCells = ansix.Width(indicator) + 1 + treePrefixCells + ansix.Width(icon) + 2
	} else {
		leadCells = ansix.Width(indicator) + 1 + treePrefixCells
	}

	var countStr string
	var countWidth int
	if hasUnread {
		countStr = uicore.ApplyBg(textStyle, bgStyle).Render(strconv.Itoa(unseen))
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
		leftContent = indicator + bgStyle.Render(" ") + treePrefixRendered + icon + bgStyle.Render("  ") + name
	} else {
		leftContent = indicator + bgStyle.Render(" ") + treePrefixRendered + name
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
