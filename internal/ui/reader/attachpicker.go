package reader

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/humanize"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// AttachPicker is the modal overlay launched by `@` in the viewer.
// Single-column list of attachment metadata. Cursor + Enter (open),
// `o` (open), `s` (save), 1-9 (open Nth), Esc/q/@ close.
type AttachPicker struct {
	shell  uicore.ModalShell
	uid    mail.UID
	items  []mail.Attachment
	cursor int
	offset int
	styles Styles
	icons  uicore.IconSet
	keys   attachPickerKeys
}

type attachPickerKeys struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Open   key.Binding
	Save   key.Binding
	Close  key.Binding
	Digits [9]key.Binding
}

func NewAttachPicker(styles Styles, icons uicore.IconSet) AttachPicker {
	keys := attachPickerKeys{
		Up:    key.NewBinding(key.WithKeys("k", "up")),
		Down:  key.NewBinding(key.WithKeys("j", "down")),
		Enter: key.NewBinding(key.WithKeys("enter")),
		Open:  key.NewBinding(key.WithKeys("o")),
		Save:  key.NewBinding(key.WithKeys("s")),
		Close: key.NewBinding(key.WithKeys("esc", "q", "@")),
	}
	for i := range keys.Digits {
		d := string(rune('1' + i))
		keys.Digits[i] = key.NewBinding(key.WithKeys(d))
	}
	return AttachPicker{styles: styles, icons: icons, keys: keys}
}

func (p AttachPicker) IsOpen() bool { return p.shell.IsOpen() }
func (p AttachPicker) Cursor() int  { return p.cursor }

func (p AttachPicker) Open(uid mail.UID, items []mail.Attachment) AttachPicker {
	p.shell = p.shell.WithOpen(true)
	p.uid = uid
	p.items = items
	p.cursor = 0
	p.offset = 0
	return p
}

func (p AttachPicker) Close() AttachPicker {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p AttachPicker) SetSize(width, height int) AttachPicker {
	p.shell = p.shell.SetSize(width, height)
	return p
}

func (p AttachPicker) Update(msg tea.Msg) (AttachPicker, tea.Cmd) {
	if !p.shell.IsOpen() {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Down):
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
		return p.clampOffset(), nil
	case key.Matches(keyMsg, p.keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
		return p.clampOffset(), nil
	case key.Matches(keyMsg, p.keys.Enter), key.Matches(keyMsg, p.keys.Open):
		return p, p.openCursor()
	case key.Matches(keyMsg, p.keys.Save):
		return p, p.saveCursor()
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return AttachPickerClosedMsg{} }
	}
	for i, b := range p.keys.Digits {
		if key.Matches(keyMsg, b) {
			if i < len(p.items) {
				return p, p.openIndex(i)
			}
			return p, nil
		}
	}
	return p, nil
}

func (p AttachPicker) openCursor() tea.Cmd {
	if p.cursor >= len(p.items) {
		return nil
	}
	return p.openIndex(p.cursor)
}

func (p AttachPicker) openIndex(i int) tea.Cmd {
	uid, att := p.uid, p.items[i]
	return tea.Batch(
		func() tea.Msg { return OpenAttachmentMsg{UID: uid, Att: att} },
		func() tea.Msg { return AttachPickerClosedMsg{} },
	)
}

func (p AttachPicker) saveCursor() tea.Cmd {
	if p.cursor >= len(p.items) {
		return nil
	}
	uid, att := p.uid, p.items[p.cursor]
	return tea.Batch(
		func() tea.Msg { return SaveAttachmentMsg{UID: uid, Att: att} },
		func() tea.Msg { return AttachPickerClosedMsg{} },
	)
}

const attachPickerMaxWidth = 70

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

func (p AttachPicker) clampOffset() AttachPicker {
	p.offset = uicore.ClampScrollOffset(p.cursor, visibleLinkRows(len(p.items), p.shell.Height()), p.offset)
	return p
}

func (p AttachPicker) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

func (p AttachPicker) Box(w, h int) string {
	boxW := attachPickerMaxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < 24 {
		boxW = 24
	}
	contentW := boxW - 2
	maxIndexDigits := len(strconv.Itoa(max(1, len(p.items))))
	visibleRows := visibleLinkRows(len(p.items), h)

	bodyRows := make([]string, visibleRows)
	for i := 0; i < visibleRows; i++ {
		row := p.offset + i
		if row >= len(p.items) {
			bodyRows[i] = uicore.PadOrTruncate("", contentW)
			continue
		}
		bodyRows[i] = p.formatRow(row, maxIndexDigits, contentW)
	}

	footer := uicore.PadOrTruncate("Enter/o open  s save  Esc close", contentW)
	footerRows := []string{footer}

	return p.shell.Box("Attachments", bodyRows, footerRows, contentW)
}

func (p AttachPicker) formatRow(row, maxIndexDigits, contentW int) string {
	att := p.items[row]
	idxStr := strconv.Itoa(row + 1)
	idxPad := strings.Repeat(" ", maxIndexDigits-len(idxStr))
	name := att.Filename
	if name == "" {
		name = "attachment"
	}
	size := humanize.Bytes(int64(att.Size))
	body := ansix.PadOrTruncate(fmt.Sprintf("%s%s[%d] %s (%s)",
		idxPad, p.icons.Attachment, row+1, name, size), contentW)
	if row == p.cursor {
		return p.styles.Cursor.Render(body)
	}
	return body
}

func (p AttachPicker) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}
