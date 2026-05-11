package reader

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// humanBytes formats n as a 1-decimal 1024-based size string.
func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	const k = 1024.0
	v := float64(n) / k
	if v < 1024 {
		return fmt.Sprintf("%.1f KB", v)
	}
	v /= k
	if v < 1024 {
		return fmt.Sprintf("%.1f MB", v)
	}
	v /= k
	if v < 1024 {
		return fmt.Sprintf("%.1f GB", v)
	}
	v /= k
	return fmt.Sprintf("%.1f TB", v)
}

type AttachPicker struct {
	shell    uicore.ModalShell
	list     list.Model
	uid      mail.UID
	items    []mail.Attachment
	styles   Styles
	icons    uicore.IconSet
	measurer ansix.Measurer
	keys     AttachPickerKeyMap
}

type AttachPickerKeyMap struct {
	Enter  key.Binding
	Open   key.Binding
	Save   key.Binding
	Close  key.Binding
	Digits [9]key.Binding
}

type attachItem struct {
	att mail.Attachment
}

func (i attachItem) FilterValue() string { return i.att.Filename }

func NewAttachPicker(styles Styles, icons uicore.IconSet, m ansix.Measurer) AttachPicker {
	keys := AttachPickerKeyMap{
		Enter: key.NewBinding(key.WithKeys("enter")),
		Open:  key.NewBinding(key.WithKeys("o")),
		Save:  key.NewBinding(key.WithKeys("s")),
		Close: key.NewBinding(key.WithKeys("esc", "q", "@")),
	}
	for i := range keys.Digits {
		d := string(rune('1' + i))
		keys.Digits[i] = key.NewBinding(key.WithKeys(d))
	}

	l := list.New(nil, attachItemDelegate{styles: styles, icons: icons, measurer: m}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.Styles = styles.List
	l.DisableQuitKeybindings()

	return AttachPicker{styles: styles, icons: icons, measurer: m, keys: keys, list: l}
}

func (p AttachPicker) IsOpen() bool { return p.shell.IsOpen() }
func (p AttachPicker) Cursor() int  { return p.list.Index() }

func (p AttachPicker) Open(uid mail.UID, items []mail.Attachment) AttachPicker {
	p.shell = p.shell.WithOpen(true)
	p.uid = uid
	p.items = items
	listItems := make([]list.Item, len(items))
	for i, a := range items {
		listItems[i] = attachItem{att: a}
	}
	p.list.SetItems(listItems)
	p.list.ResetSelected()
	return p
}

func (p AttachPicker) Close() AttachPicker {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p AttachPicker) SetSize(width, height int) AttachPicker {
	p.shell = p.shell.SetSize(width, height)
	contentW, listH := uicore.PickerListSize(width, height, attachPickerMaxWidth, 24, 5)
	p.list.SetSize(contentW, listH)
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
	case key.Matches(keyMsg, p.keys.Enter), key.Matches(keyMsg, p.keys.Open):
		return p, p.openIndex(p.list.Index())
	case key.Matches(keyMsg, p.keys.Save):
		return p, p.saveIndex(p.list.Index())
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
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p AttachPicker) openIndex(i int) tea.Cmd {
	if i < 0 || i >= len(p.items) {
		return nil
	}
	uid, att := p.uid, p.items[i]
	return tea.Batch(
		func() tea.Msg { return OpenAttachmentMsg{UID: uid, Att: att} },
		func() tea.Msg { return AttachPickerClosedMsg{} },
	)
}

func (p AttachPicker) saveIndex(i int) tea.Cmd {
	if i < 0 || i >= len(p.items) {
		return nil
	}
	uid, att := p.uid, p.items[i]
	return tea.Batch(
		func() tea.Msg { return SaveAttachmentMsg{UID: uid, Att: att} },
		func() tea.Msg { return AttachPickerClosedMsg{} },
	)
}

const attachPickerMaxWidth = 70

func (p AttachPicker) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

func (p AttachPicker) Box(w, h int) string {
	contentW, _ := uicore.PickerListSize(w, h, attachPickerMaxWidth, 24, 5)
	bodyRows := uicore.SplitAndPad(p.list.View(), contentW)
	footer := uicore.PadOrTruncate("Enter/o open  s save  Esc close", contentW)
	return p.shell.Box("Attachments", bodyRows, []string{footer}, contentW)
}

type attachItemDelegate struct {
	styles   Styles
	icons    uicore.IconSet
	measurer ansix.Measurer
}

func (d attachItemDelegate) Height() int                             { return 1 }
func (d attachItemDelegate) Spacing() int                            { return 0 }
func (d attachItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d attachItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ai, ok := item.(attachItem)
	if !ok {
		return
	}
	att := ai.att
	contentW := m.Width()
	name := att.Filename
	if name == "" {
		name = "attachment"
	}
	size := humanBytes(int64(att.Size))
	body := d.measurer.PadOrTruncate(
		fmt.Sprintf("%s[%d] %s (%s)", d.icons.Attachment, index+1, name, size),
		contentW)
	if index == m.Index() {
		body = d.styles.Cursor.Render(body)
	}
	fmt.Fprint(w, body)
}

func (p AttachPicker) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}
