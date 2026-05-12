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

// Mode tags the two input states. Filter is the default: keystrokes
// feed the filter, only arrows navigate. Nav is the typing reprieve
// where `j`/`k` advance the cursor and filter text stays put.
type Mode int

const (
	ModeFilter Mode = iota
	ModeNav
)

// Model is the modal overlay launched by `m` from the account view.
type Model struct {
	shell    uicore.ModalShell
	list     list.Model
	uids     []mail.UID
	src      string
	all      []mail.FolderEntry
	styles   Styles
	measurer ansix.Measurer
	keys     KeyMap
	mode     Mode
}

type KeyMap struct {
	ArrowUp    key.Binding
	ArrowDown  key.Binding
	NavUp      key.Binding
	NavDown    key.Binding
	ToggleMode key.Binding
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

func New(styles Styles, m ansix.Measurer) Model {
	l := list.New(nil, folderItemDelegate{styles: styles, measurer: m}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.Styles = styles.List
	l.DisableQuitKeybindings()
	// Strip `j`/`k` from the list's own nav keymap so filter mode can
	// accept them as filter text.
	l.KeyMap.CursorUp = key.NewBinding(key.WithKeys("up"))
	l.KeyMap.CursorDown = key.NewBinding(key.WithKeys("down"))

	return Model{
		styles:   styles,
		measurer: m,
		list:     l,
		keys: KeyMap{
			ArrowUp:    key.NewBinding(key.WithKeys("up")),
			ArrowDown:  key.NewBinding(key.WithKeys("down")),
			NavUp:      key.NewBinding(key.WithKeys("k")),
			NavDown:    key.NewBinding(key.WithKeys("j")),
			ToggleMode: key.NewBinding(key.WithKeys("tab")),
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
	p.mode = ModeFilter
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
	// SetFilterText("") forces filteredItems to populate before we enter Filtering.
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
	contentW, listH := uicore.PickerListSize(width, height, movepickerMaxWidth, movepickerMinWidth, 7)
	p.list.SetSize(contentW, listH)
	return p
}

func (p Model) Len() int        { return len(p.all) }
func (p Model) Filter() string  { return p.list.FilterValue() }
func (p Model) MatchCount() int { return len(p.list.VisibleItems()) }
func (p Model) Mode() Mode      { return p.mode }

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
	case key.Matches(keyMsg, p.keys.ToggleMode):
		p.mode ^= 1
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
	case key.Matches(keyMsg, p.keys.ArrowUp):
		p.list.CursorUp()
		return p, nil
	case key.Matches(keyMsg, p.keys.ArrowDown):
		p.list.CursorDown()
		return p, nil
	case p.mode == ModeNav && key.Matches(keyMsg, p.keys.NavUp):
		p.list.CursorUp()
		return p, nil
	case p.mode == ModeNav && key.Matches(keyMsg, p.keys.NavDown):
		p.list.CursorDown()
		return p, nil
	}
	if p.mode == ModeNav {
		// Nav mode parks the filter. Swallow non-nav key input so the
		// list's filter text stays the user's last typed query.
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

func (p Model) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

// Box renders the picker at the given dims regardless of open state.
func (p Model) Box(w, h int) string {
	contentW, _ := uicore.PickerListSize(w, h, movepickerMaxWidth, movepickerMinWidth, 7)
	bodyRows := uicore.SplitAndPad(p.list.View(), contentW)
	var hint string
	switch p.mode {
	case ModeNav:
		hint = "jk select · tab filter · enter pick · esc cancel"
	default:
		hint = "↑↓ select · tab nav · enter pick · esc cancel"
	}
	footerRows := []string{
		p.styles.Dim.Render(uicore.PadOrTruncate(hint, contentW)),
	}
	title := fmt.Sprintf("Move to (%d)", p.MatchCount())
	return p.shell.Box(title, bodyRows, footerRows, contentW)
}

func (p Model) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}

type folderItemDelegate struct {
	styles   Styles
	measurer ansix.Measurer
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
	body = d.measurer.PadOrTruncate(body, contentW)
	if index == m.Index() {
		body = d.styles.Cursor.Render(body)
	}
	fmt.Fprint(w, body)
}

func renderWithMatches(s string, matches []int, matchStyle lipgloss.Style) string {
	if len(matches) == 0 {
		return s
	}
	mi := 0
	var b strings.Builder
	for i, r := range s {
		if mi < len(matches) && matches[mi] == i {
			b.WriteString(matchStyle.Render(string(r)))
			mi++
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
