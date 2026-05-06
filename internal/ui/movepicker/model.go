// SPDX-License-Identifier: MIT

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
)

// Styles holds the subset of UI styles the move picker needs.
// Populated from ui.Styles at construction time.
type Styles struct {
	Dim           lipgloss.Style
	MsgListCursor lipgloss.Style
}

// FolderEntry is one item in the picker list. Mirrors ui.FolderEntry;
// the sidebar package owns the canonical definition.
type FolderEntry struct {
	Display  string
	Provider string
	Group    mail.Group
}

// OpenMsg asks App to open the move-to-folder picker.
type OpenMsg struct {
	UIDs    []mail.UID
	Src     string
	Folders []FolderEntry
}

// PickedMsg is emitted when the user selects a destination folder.
type PickedMsg struct {
	UIDs []mail.UID
	Src  string
	Dest string
}

// ClosedMsg is emitted when the picker is dismissed without a pick.
type ClosedMsg struct{}

// modelCache holds the memoised list-row slice and the inputs that
// determine when it must be rebuilt.
//
// Pre-beta escape hatch: Model is an immutable value type, so a cache
// that lives in the value would be lost on every With*/Open/Update return.
// Rather than switching to pointer receivers (which would break the Elm
// immutable-model contract), we heap-allocate one small cache struct at
// construction time and access it through a pointer. The pointer itself is
// copied with the value, so every generation of the picker shares the same
// cache. A deliberate choice: they all render the same logical state, and
// the dirty flag ensures stale renders are never served.
type modelCache struct {
	dirty       bool
	rows        []string
	contentW    int
	visibleRows int
}

// shell holds the shared lifecycle state for the overlay: open flag and
// terminal dimensions.
type shell struct {
	open          bool
	width, height int
}

func (s shell) isOpen() bool             { return s.open }
func (s shell) withOpen(open bool) shell { s.open = open; return s }
func (s shell) setSize(w, h int) shell   { s.width, s.height = w, h; return s }

// box renders the ┌─ title ─┐ / content rows / ├─┤ footer rows / └─┘ frame.
func (s shell) box(title string, bodyRows []string, footerRows []string, contentW int) string {
	boxW := contentW + 2
	titleSeg := " " + title + " "
	maxTitleW := boxW - 3
	if maxTitleW < 0 {
		maxTitleW = 0
	}
	if lipgloss.Width(titleSeg) > maxTitleW {
		tgtW := maxTitleW - 2
		if tgtW < 1 {
			titleSeg = ""
		} else {
			titleSeg = " " + truncateToWidth(title, tgtW) + " "
		}
	}
	rest := boxW - 3 - lipgloss.Width(titleSeg)
	if rest < 0 {
		rest = 0
	}

	var b strings.Builder
	b.WriteString("┌─" + titleSeg + strings.Repeat("─", rest) + "┐\n")
	for _, row := range bodyRows {
		b.WriteString("│" + row + "│\n")
	}
	if len(footerRows) > 0 {
		b.WriteString("├" + strings.Repeat("─", contentW) + "┤\n")
		for _, row := range footerRows {
			b.WriteString("│" + row + "│\n")
		}
	}
	b.WriteString("└" + strings.Repeat("─", contentW) + "┘")
	return b.String()
}

// Model is the modal overlay launched by `m` from the account view.
// App owns open state and overlay composition (mirrors LinkPicker, ADR-0087).
type Model struct {
	sh      shell
	uids    []mail.UID
	src     string
	all     []FolderEntry
	filter  string
	matches []int
	cursor  int
	offset  int
	styles  Styles
	keys    modelKeys
	cache   *modelCache // heap-allocated, shared across value copies
}

type modelKeys struct {
	Up        key.Binding
	Down      key.Binding
	Pick      key.Binding
	Close     key.Binding
	Backspace key.Binding
	// Swallow consumes 'q' so the picker doesn't quit the app while
	// open, consistent with the other overlay surfaces.
	Swallow key.Binding
}

// New constructs a closed Model with the given styles.
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

// IsOpen reports whether the overlay is visible.
func (p Model) IsOpen() bool { return p.sh.isOpen() }

// Open snapshots the targets and folder list. Source folder is
// excluded so the picker never offers a no-op move-to-self.
func (p Model) Open(uids []mail.UID, src string, folders []FolderEntry) Model {
	p.sh = p.sh.withOpen(true)
	p.uids = uids
	p.src = src
	p.all = make([]FolderEntry, 0, len(folders))
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

// Close marks the overlay closed.
func (p Model) Close() Model {
	p.sh = p.sh.withOpen(false)
	return p
}

// SetSize updates the terminal dimensions.
func (p Model) SetSize(width, height int) Model {
	p.sh = p.sh.setSize(width, height)
	return p
}

// visibleRows is the list-row capacity at the given total box height.
// Reserves rows for top + bottom border + filter line + preview lines + slack.
func visibleRows(height int) int {
	rows := height - 7
	if rows < 1 {
		rows = 1
	}
	return rows
}

// clampOffset adjusts p.offset so p.cursor lies within the visible window.
// Called after every cursor move.
func (p Model) clampOffset() Model {
	p.offset = clampScrollOffset(p.cursor, visibleRows(p.sh.height), p.offset)
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

// Update handles key input for the overlay.
func (p Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !p.sh.isOpen() {
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

// View renders the overlay or returns "" when closed.
func (p Model) View() string {
	if !p.sh.isOpen() {
		return ""
	}
	return p.Box(p.sh.width, p.sh.height)
}

// Box renders the picker at the given dimensions regardless of open state.
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

	// Serve from cache when the rendered inputs are unchanged.
	// Dimension change (contentW, maxListRows) counts as dirty even if the
	// flag is clear. SetSize doesn't need to touch the flag.
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
			built[i] = padOrTruncate(line, contentW)
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
		p.styles.Dim.Render(padOrTruncate(hint, contentW)),
		p.styles.Dim.Render(padOrTruncate("↑↓ select · enter pick · esc cancel", contentW)),
	}

	title := "Move to (" + strconv.Itoa(len(p.matches)) + ")"
	return p.sh.box(title, bodyRows, footerRows, contentW)
}

func (p Model) buildListRows(contentW int) []string {
	if len(p.matches) == 0 && p.filter != "" {
		return []string{"  no folders match \"" + truncateToWidth(p.filter, contentW-22) + "\""}
	}
	rows := make([]string, 0, len(p.matches)+2)
	prevGroup := mail.Group(-1)
	for i, idx := range p.matches {
		entry := p.all[idx]
		if p.filter == "" && i > 0 && entry.Group != prevGroup {
			rows = append(rows, "")
		}
		prevGroup = entry.Group
		marker := "  "
		if i == p.cursor {
			marker = "> "
		}
		row := marker + entry.Display
		if i == p.cursor {
			row = p.styles.MsgListCursor.Render(padOrTruncate(row, contentW))
		}
		rows = append(rows, row)
	}
	return rows
}

// padOrTruncate pads s with spaces or truncates it to exactly width display cells.
func padOrTruncate(s string, width int) string {
	w := lipgloss.Width(s)
	if w == width {
		return s
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return truncateToWidth(s, width)
}

// truncateToWidth shortens s to at most width display cells, adding "…" if truncated.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	// Step back rune-by-rune until we fit with the ellipsis.
	ellipsis := "…"
	ew := lipgloss.Width(ellipsis)
	out := []rune(s)
	for lipgloss.Width(string(out))+ew > width && len(out) > 0 {
		out = out[:len(out)-1]
	}
	return string(out) + ellipsis
}

// clampScrollOffset returns offset adjusted so cursor lies within the visible window.
func clampScrollOffset(cursor, visible, offset int) int {
	if cursor < offset {
		return cursor
	}
	if cursor >= offset+visible {
		return cursor - visible + 1
	}
	return offset
}

// centerOverlay returns the top-left (x, y) cell coordinates to center box
// within a terminal of totalW × totalH cells.
func centerOverlay(box string, totalW, totalH int) (int, int) {
	lines := strings.Split(box, "\n")
	h := len(lines)
	w := 0
	for _, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			w = lw
		}
	}
	x := (totalW - w) / 2
	y := (totalH - h) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

// Position returns the top-left coordinates to center this overlay.
func (p Model) Position(box string, totalW, totalH int) (int, int) {
	return centerOverlay(box, totalW, totalH)
}
