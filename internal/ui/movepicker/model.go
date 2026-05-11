package movepicker

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// OpenMsg asks App to open the move-to-folder picker.
type OpenMsg struct {
	UIDs    []mail.UID
	Src     string
	Folders []mail.FolderEntry
}

// PickedMsg fires when the user selects a destination folder.
type PickedMsg struct {
	UIDs []mail.UID
	Src  string
	Dest string
}

// ClosedMsg fires when the picker is dismissed without a pick.
type ClosedMsg struct{}

// Model is the modal overlay launched by `m` from the account view.
type Model struct {
	shell  uicore.ModalShell
	list   list.Model
	uids   []mail.UID
	src    string
	all    []mail.FolderEntry
	styles Styles
	keys   modelKeys
}

type modelKeys struct {
	CursorUp   key.Binding
	CursorDown key.Binding
	Pick       key.Binding
	Close      key.Binding
	Swallow    key.Binding
}

type folderItem struct {
	entry mail.FolderEntry
}

func (i folderItem) FilterValue() string {
	if i.entry.Display != "" {
		return i.entry.Display
	}
	return i.entry.Provider
}

func New(styles Styles) Model {
	l := list.New(nil, folderItemDelegate{styles: styles}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.Styles = styles.List
	l.DisableQuitKeybindings()
	l.KeyMap.CursorUp = key.NewBinding(key.WithKeys("up", "k"))
	l.KeyMap.CursorDown = key.NewBinding(key.WithKeys("down", "j"))

	return Model{
		styles: styles,
		list:   l,
		keys: modelKeys{
			CursorUp:   key.NewBinding(key.WithKeys("up", "k")),
			CursorDown: key.NewBinding(key.WithKeys("down", "j")),
			Pick:       key.NewBinding(key.WithKeys("enter")),
			Close:      key.NewBinding(key.WithKeys("esc")),
			Swallow:    key.NewBinding(key.WithKeys("q")),
		},
	}
}

func (p Model) IsOpen() bool { return p.shell.IsOpen() }

// Open snapshots the targets and folder list. The source folder is
// excluded so the picker never offers a no-op move-to-self.
func (p Model) Open(uids []mail.UID, src string, folders []mail.FolderEntry) Model {
	p.shell = p.shell.WithOpen(true)
	p.uids = uids
	p.src = src
	p.all = make([]mail.FolderEntry, 0, len(folders))
	for _, f := range folders {
		if f.Provider == src {
			continue
		}
		p.all = append(p.all, f)
	}
	items := make([]list.Item, len(p.all))
	for i, f := range p.all {
		items[i] = folderItem{entry: f}
	}
	p.list.SetItems(items)
	// SetFilterText("") synchronously populates filteredItems (all items, no
	// matches), then sets filterState=FilterApplied. We follow with
	// SetFilterState(Filtering) to enter the always-on input mode.
	p.list.SetFilterText("")
	p.list.SetFilterState(list.Filtering)
	return p
}

func (p Model) Close() Model {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p Model) SetSize(width, height int) Model {
	p.shell = p.shell.SetSize(width, height)
	contentW, listH := movepickerListSize(width, height)
	p.list.SetSize(contentW, listH)
	return p
}

func (p Model) Len() int        { return len(p.all) }
func (p Model) Filter() string  { return p.list.FilterValue() }
func (p Model) MatchCount() int { return len(p.list.VisibleItems()) }

func (p Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !p.shell.IsOpen() {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Swallow):
		return p, nil
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return ClosedMsg{} }
	case key.Matches(keyMsg, p.keys.Pick):
		item, ok := p.list.SelectedItem().(folderItem)
		if !ok {
			return p, nil
		}
		dest := item.entry.Provider
		if dest == "" {
			dest = item.entry.Display
		}
		uids, src := p.uids, p.src
		return p, tea.Batch(
			func() tea.Msg { return PickedMsg{UIDs: uids, Src: src, Dest: dest} },
			func() tea.Msg { return ClosedMsg{} },
		)
	case key.Matches(keyMsg, p.keys.CursorUp):
		p.list.CursorUp()
		return p, nil
	case key.Matches(keyMsg, p.keys.CursorDown):
		p.list.CursorDown()
		return p, nil
	}
	prevFilter := p.list.FilterValue()
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	if p.list.FilterValue() != prevFilter {
		// The list emits filterItems as an async cmd, but we run always-on
		// filter mode and need VisibleItems() to reflect the new text
		// synchronously (no tea loop to deliver FilterMatchesMsg). Drive the
		// apply inline, then restore Filtering state so text input stays open.
		p.list.SetFilterText(p.list.FilterValue())
		p.list.SetFilterState(list.Filtering)
		p.list.GoToStart()
	}
	return p, cmd
}

const (
	movepickerMaxWidth = 50
	movepickerMinWidth = 24
)

func movepickerListSize(boxW, boxH int) (contentW, listH int) {
	bw := movepickerMaxWidth
	if boxW-4 < bw {
		bw = boxW - 4
	}
	if bw < movepickerMinWidth {
		bw = movepickerMinWidth
	}
	contentW = bw - 2
	listH = boxH - 7
	if listH < 1 {
		listH = 1
	}
	return contentW, listH
}

func (p Model) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

// Box renders the picker at the given dims regardless of open state.
func (p Model) Box(w, h int) string {
	contentW, _ := movepickerListSize(w, h)
	listView := p.list.View()
	bodyRows := strings.Split(listView, "\n")
	for i, row := range bodyRows {
		bodyRows[i] = uicore.PadOrTruncate(row, contentW)
	}
	footerRows := []string{
		p.styles.Dim.Render(uicore.PadOrTruncate("↑↓ select · enter pick · esc cancel", contentW)),
	}
	title := fmt.Sprintf("Move to (%d)", p.MatchCount())
	return p.shell.Box(title, bodyRows, footerRows, contentW)
}

func (p Model) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}

type folderItemDelegate struct {
	styles Styles
}

func (d folderItemDelegate) Height() int                             { return 1 }
func (d folderItemDelegate) Spacing() int                            { return 0 }
func (d folderItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d folderItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(folderItem)
	if !ok {
		return
	}
	display := fi.entry.Display
	if display == "" {
		display = fi.entry.Provider
	}
	contentW := m.Width()
	matches := m.MatchesForItem(index)
	body := renderWithMatches(display, matches, d.styles.Match)
	body = ansix.PadOrTruncate(body, contentW)
	if index == m.Index() {
		body = d.styles.Cursor.Render(body)
	}
	fmt.Fprint(w, body)
}

func renderWithMatches(s string, matches []int, matchStyle lipgloss.Style) string {
	if len(matches) == 0 {
		return s
	}
	mset := make(map[int]bool, len(matches))
	for _, i := range matches {
		mset[i] = true
	}
	var b strings.Builder
	for i, r := range s {
		if mset[i] {
			b.WriteString(matchStyle.Render(string(r)))
		} else {
			b.WriteString(string(r))
		}
	}
	return b.String()
}
