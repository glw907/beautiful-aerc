package movepicker

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// Styles is the move picker's projection of internal/ui.Styles.
type Styles struct {
	Dim    lipgloss.Style
	Cursor lipgloss.Style
	// Match underlines runes that match the active filter substring.
	// Underline-only so it composes with the row's base foreground.
	Match lipgloss.Style
}

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

// modelCache memoises the list-row slice. Heap-allocated so the pointer
// survives the value-type model's copy-on-mutation contract; ADR-0130
// covers the escape hatch.
type modelCache struct {
	dirty       bool
	rows        []string
	contentW    int
	visibleRows int
}

// Model is the modal overlay launched by `m` from the account view.
// App owns open state and overlay composition (ADR-0087).
type Model struct {
	shell   uicore.ModalShell
	uids    []mail.UID
	src     string
	all     []mail.FolderEntry
	filter  string
	matches []int
	cursor  int
	offset  int
	styles  Styles
	keys    modelKeys
	cache   *modelCache
}

type modelKeys struct {
	Up        key.Binding
	Down      key.Binding
	Pick      key.Binding
	Close     key.Binding
	Backspace key.Binding
	// Swallow consumes 'q' so the picker doesn't quit while open.
	Swallow key.Binding
}

func New(styles Styles) Model {
	return Model{
		styles: styles,
		cache:  &modelCache{dirty: true},
		keys: modelKeys{
			Up:        key.NewBinding(key.WithKeys("up")),
			Down:      key.NewBinding(key.WithKeys("down")),
			Pick:      key.NewBinding(key.WithKeys("enter")),
			Close:     key.NewBinding(key.WithKeys("esc")),
			Backspace: key.NewBinding(key.WithKeys("backspace")),
			Swallow:   key.NewBinding(key.WithKeys("q")),
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
	p.filter = ""
	p.cursor = 0
	p.offset = 0
	p = p.recompute()
	return p
}

func (p Model) Close() Model {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p Model) SetSize(width, height int) Model {
	p.shell = p.shell.SetSize(width, height)
	return p
}

// visibleRows is the list-row capacity at total height. The 7-row reserve
// covers top + bottom border, the filter line, the preview rows, and slack.
func visibleRows(height int) int {
	rows := height - 7
	if rows < 1 {
		rows = 1
	}
	return rows
}

// clampOffset adjusts p.offset so p.cursor lies inside the visible window.
func (p Model) clampOffset() Model {
	p.offset = uicore.ClampScrollOffset(p.cursor, visibleRows(p.shell.Height()), p.offset)
	return p
}

func (p Model) recompute() Model {
	p.matches = p.matches[:0]
	if cap(p.matches) < len(p.all) {
		p.matches = make([]int, 0, len(p.all))
	}
	needle := strings.ToLower(p.filter)
	for i, f := range p.all {
		if needle == "" || strings.Contains(strings.ToLower(f.Display), needle) {
			p.matches = append(p.matches, i)
		}
	}
	p.cursor = 0
	p.offset = 0
	p.cache.dirty = true
	return p
}

func (p Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !p.shell.IsOpen() {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Down):
		if p.cursor < len(p.matches)-1 {
			p.cursor++
			p.cache.dirty = true
		}
		return p.clampOffset(), nil
	case key.Matches(keyMsg, p.keys.Up):
		if p.cursor > 0 {
			p.cursor--
			p.cache.dirty = true
		}
		return p.clampOffset(), nil
	case key.Matches(keyMsg, p.keys.Pick):
		if p.cursor < 0 || p.cursor >= len(p.matches) {
			return p, nil
		}
		dest := p.all[p.matches[p.cursor]].Provider
		picked := PickedMsg{UIDs: p.uids, Src: p.src, Dest: dest}
		return p, tea.Batch(
			func() tea.Msg { return picked },
			func() tea.Msg { return ClosedMsg{} },
		)
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return ClosedMsg{} }
	case key.Matches(keyMsg, p.keys.Backspace):
		if p.filter == "" {
			return p, nil
		}
		_, size := utf8.DecodeLastRuneInString(p.filter)
		p.filter = p.filter[:len(p.filter)-size]
		return p.recompute(), nil
	}
	if key.Matches(keyMsg, p.keys.Swallow) {
		return p, nil
	}
	if r, ok := singlePrintableRune(keyMsg); ok {
		p.filter += string(r)
		return p.recompute(), nil
	}
	return p, nil
}

func singlePrintableRune(k tea.KeyMsg) (rune, bool) {
	if len(k.Runes) != 1 {
		return 0, false
	}
	r := k.Runes[0]
	if !unicode.IsPrint(r) {
		return 0, false
	}
	return r, true
}

const (
	maxWidth = 50
	minWidth = 24
)

func (p Model) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

// Box renders the picker at the given dims regardless of open state.
func (p Model) Box(w, h int) string {
	boxW := maxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < minWidth {
		boxW = minWidth
	}
	contentW := boxW - 2

	maxListRows := visibleRows(h)

	// A dimension change counts as dirty even if the flag is clear, so
	// SetSize doesn't need to touch it.
	c := p.cache
	if c.dirty || c.contentW != contentW || c.visibleRows != maxListRows {
		allRows := p.buildListRows(contentW)
		start, end := p.offset, p.offset+maxListRows
		if end > len(allRows) {
			end = len(allRows)
		}
		if start > end {
			start = end
		}
		visible := allRows[start:end]

		built := make([]string, maxListRows)
		for i := 0; i < maxListRows; i++ {
			line := ""
			if i < len(visible) {
				line = visible[i]
			}
			built[i] = uicore.PadOrTruncate(line, contentW)
		}
		c.rows = built
		c.contentW = contentW
		c.visibleRows = maxListRows
		c.dirty = false
	}
	bodyRows := c.rows

	hint := ""
	if p.filter != "" {
		hint = "filter: " + p.filter
	}
	footerRows := []string{
		p.styles.Dim.Render(uicore.PadOrTruncate(hint, contentW)),
		p.styles.Dim.Render(uicore.PadOrTruncate("↑↓ select · enter pick · esc cancel", contentW)),
	}

	title := "Move to (" + strconv.Itoa(len(p.matches)) + ")"
	return p.shell.Box(title, bodyRows, footerRows, contentW)
}

func (p Model) buildListRows(contentW int) []string {
	if len(p.matches) == 0 && p.filter != "" {
		return []string{"  no folders match \"" + uicore.TruncateToWidth(p.filter, contentW-22) + "\""}
	}
	const markerW = 2
	displayW := contentW - markerW
	if displayW < 1 {
		displayW = 1
	}
	needleLower := strings.ToLower(p.filter)
	rows := make([]string, 0, len(p.matches)+2)
	prevGroup := mail.Group(-1)
	for i, idx := range p.matches {
		entry := p.all[idx]
		if p.filter == "" && i > 0 && entry.Group != prevGroup {
			rows = append(rows, "")
		}
		prevGroup = entry.Group
		isCursor := i == p.cursor
		marker := "  "
		if isCursor {
			marker = "> "
		}
		plain := uicore.TruncateToWidth(entry.Display, displayW)
		pad := ""
		if w := lipgloss.Width(plain); w < displayW {
			pad = strings.Repeat(" ", displayW-w)
		}
		if !isCursor && needleLower != "" {
			if runes := matchRunes(plain, needleLower); len(runes) > 0 {
				plain = lipgloss.StyleRunes(plain, runes, p.styles.Match, lipgloss.NewStyle())
			}
		}
		row := marker + plain + pad
		if isCursor {
			row = p.styles.Cursor.Render(row)
		}
		rows = append(rows, row)
	}
	return rows
}

// matchRunes returns the rune indices of the first substring match.
// needleLower must already be lowercased by the caller.
func matchRunes(haystack, needleLower string) []int {
	if needleLower == "" {
		return nil
	}
	lo := strings.Index(strings.ToLower(haystack), needleLower)
	if lo < 0 {
		return nil
	}
	hi := lo + len(needleLower)
	var out []int
	runeIdx := 0
	for byteOff := range haystack {
		if byteOff >= hi {
			break
		}
		if byteOff >= lo {
			out = append(out, runeIdx)
		}
		runeIdx++
	}
	return out
}

func (p Model) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}
