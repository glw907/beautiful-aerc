package reader

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// LinkPicker is the modal overlay launched by Tab while the viewer is
// open and ready. Single-column list of harvested URLs with cursor +
// Enter, 1-9 quick launch, Esc/Tab to close. App owns open state and
// overlay composition (ADR-0082).
type LinkPicker struct {
	shell  uicore.ModalShell
	links  []string
	cursor int
	offset int
	styles Styles
	keys   linkPickerKeys
}

type linkPickerKeys struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Close key.Binding
	// Digits[i] binds the digit key for harvested-link slot i+1.
	Digits [9]key.Binding
}

func NewLinkPicker(styles Styles) LinkPicker {
	keys := linkPickerKeys{
		Up:    key.NewBinding(key.WithKeys("k", "up")),
		Down:  key.NewBinding(key.WithKeys("j", "down")),
		Enter: key.NewBinding(key.WithKeys("enter")),
		Close: key.NewBinding(key.WithKeys("esc", "tab")),
	}
	for i := range keys.Digits {
		d := string(rune('1' + i))
		keys.Digits[i] = key.NewBinding(key.WithKeys(d))
	}
	return LinkPicker{
		styles: styles,
		keys:   keys,
	}
}

func (p LinkPicker) IsOpen() bool { return p.shell.IsOpen() }
func (p LinkPicker) Cursor() int  { return p.cursor }

// Open transitions the picker into the open state with the given URL
// list, resetting cursor and offset.
func (p LinkPicker) Open(links []string) LinkPicker {
	p.shell = p.shell.WithOpen(true)
	p.links = links
	p.cursor = 0
	p.offset = 0
	return p
}

func (p LinkPicker) Close() LinkPicker {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p LinkPicker) SetSize(width, height int) LinkPicker {
	p.shell = p.shell.SetSize(width, height)
	return p
}

// Update dispatches a tea.Msg while the picker is open and emits the
// launch/close Cmds for Enter, 1-9, Esc, and Tab.
func (p LinkPicker) Update(msg tea.Msg) (LinkPicker, tea.Cmd) {
	if !p.shell.IsOpen() {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Down):
		if p.cursor < len(p.links)-1 {
			p.cursor++
		}
		return p.clampOffset(), nil
	case key.Matches(keyMsg, p.keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
		return p.clampOffset(), nil
	case key.Matches(keyMsg, p.keys.Enter):
		if p.cursor < 0 || p.cursor >= len(p.links) {
			return p, nil
		}
		return p, tea.Batch(
			func() tea.Msg { return LaunchURLMsg{URL: p.links[p.cursor]} },
			func() tea.Msg { return LinkPickerClosedMsg{} },
		)
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return LinkPickerClosedMsg{} }
	}
	for i, b := range p.keys.Digits {
		if key.Matches(keyMsg, b) {
			if i < len(p.links) {
				return p, tea.Batch(
					func() tea.Msg { return LaunchURLMsg{URL: p.links[i]} },
					func() tea.Msg { return LinkPickerClosedMsg{} },
				)
			}
			return p, nil
		}
	}
	return p, nil
}

const linkPickerMaxWidth = 70

// visibleLinkRows is the number of list rows the picker shows at the
// given box height. The 7-row reservation is top + bottom border + rule
// + 2 preview lines + 1 title slack.
func visibleLinkRows(total, height int) int {
	maxRows := height - 7
	if maxRows < 1 {
		maxRows = 1
	}
	if total < maxRows {
		return total
	}
	return maxRows
}

// clampOffset adjusts p.offset so p.cursor stays in the visible window.
func (p LinkPicker) clampOffset() LinkPicker {
	p.offset = uicore.ClampScrollOffset(p.cursor, visibleLinkRows(len(p.links), p.shell.Height()), p.offset)
	return p
}

// linkPickerInlineCap caps the per-row inline URL display so the picker
// stays visually tight on wide terminals.
const linkPickerInlineCap = 50

// View renders the picker as a standalone string. Production composition
// goes through Box + Position + PlaceOverlay; this is the fallback for
// tests and degenerate sizes.
func (p LinkPicker) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

// Box renders the modal at the size derived from (w, h).
func (p LinkPicker) Box(w, h int) string {
	boxW := linkPickerMaxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < 20 {
		boxW = 20
	}
	contentW := boxW - 2 // left/right border
	maxIndexDigits := len(strconv.Itoa(len(p.links)))
	indexW := 2 + maxIndexDigits
	urlW := contentW - indexW - 1 // 1 space between index and URL
	if urlW > linkPickerInlineCap {
		urlW = linkPickerInlineCap
	}

	visibleRows := visibleLinkRows(len(p.links), h)

	bodyRows := make([]string, visibleRows)
	for i := 0; i < visibleRows; i++ {
		row := p.offset + i
		if row >= len(p.links) {
			bodyRows[i] = uicore.PadOrTruncate("", contentW)
			continue
		}
		bodyRows[i] = p.formatRow(row, maxIndexDigits, urlW, contentW)
	}

	previewLines := p.previewLines(contentW)
	footerRows := make([]string, 2)
	for i := 0; i < 2; i++ {
		line := ""
		if i < len(previewLines) {
			line = previewLines[i]
		}
		footerRows[i] = uicore.PadOrTruncate(line, contentW)
	}

	return p.shell.Box("Links", bodyRows, footerRows, contentW)
}

// formatRow renders one list row as "  [N] URL", painted with the cursor
// background when row == p.cursor.
func (p LinkPicker) formatRow(row, maxIndexDigits, urlW, contentW int) string {
	idxStr := strconv.Itoa(row + 1)
	pad := strings.Repeat(" ", maxIndexDigits-len(idxStr))
	url := p.links[row]
	if uicore.DisplayCells(url) > urlW {
		url = uicore.DisplayTruncate(url, urlW)
	}
	body := uicore.PadOrTruncate(fmt.Sprintf("%s[%d] %s", pad, row+1, url), contentW)
	if row == p.cursor {
		return p.styles.Cursor.Render(body)
	}
	return body
}

// previewLines returns up to 2 wrapped lines of the cursor row's full
// URL. The second line is truncated with "…" when the URL exceeds two
// rows of cells.
func (p LinkPicker) previewLines(width int) []string {
	if p.cursor < 0 || p.cursor >= len(p.links) {
		return nil
	}
	full := p.links[p.cursor]
	wrapped := strings.Split(linkPickerWrap(full, width), "\n")
	if len(wrapped) <= 2 {
		return wrapped
	}
	row2 := wrapped[1]
	if uicore.DisplayCells(row2) >= width {
		row2 = uicore.DisplayTruncate(row2, width-1) + "…"
	} else {
		row2 += "…"
	}
	return []string{wrapped[0], row2}
}

// linkPickerWrap wraps s to width. URLs are unbreakable tokens that
// Wordwrap can't split, so a Hardwrap pass forces the residue.
func linkPickerWrap(s string, width int) string {
	if width < 1 {
		width = 1
	}
	return ansi.Hardwrap(ansi.Wordwrap(s, width, ""), width, false)
}

// Position returns the centered top-left for the rendered box; App
// feeds it to PlaceOverlay.
func (p LinkPicker) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}
