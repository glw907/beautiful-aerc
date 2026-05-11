package reader

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/ui/uicore"
)

type LinkPicker struct {
	shell  uicore.ModalShell
	list   list.Model
	links  []string
	styles Styles
	keys   linkPickerKeys
}

type linkPickerKeys struct {
	Enter  key.Binding
	Close  key.Binding
	Digits [9]key.Binding
}

type linkItem struct {
	url string
}

func (i linkItem) FilterValue() string { return i.url }

func NewLinkPicker(styles Styles) LinkPicker {
	keys := linkPickerKeys{
		Enter: key.NewBinding(key.WithKeys("enter")),
		Close: key.NewBinding(key.WithKeys("esc", "tab")),
	}
	for i := range keys.Digits {
		d := string(rune('1' + i))
		keys.Digits[i] = key.NewBinding(key.WithKeys(d))
	}

	l := list.New(nil, linkItemDelegate{styles: styles}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.Styles = styles.List
	l.DisableQuitKeybindings()

	return LinkPicker{styles: styles, keys: keys, list: l}
}

func (p LinkPicker) IsOpen() bool { return p.shell.IsOpen() }
func (p LinkPicker) Cursor() int  { return p.list.Index() }

func (p LinkPicker) Open(links []string) LinkPicker {
	p.shell = p.shell.WithOpen(true)
	p.links = links
	items := make([]list.Item, len(links))
	for i, u := range links {
		items[i] = linkItem{url: u}
	}
	p.list.SetItems(items)
	p.list.ResetSelected()
	return p
}

func (p LinkPicker) Close() LinkPicker {
	p.shell = p.shell.WithOpen(false)
	return p
}

func (p LinkPicker) SetSize(width, height int) LinkPicker {
	p.shell = p.shell.SetSize(width, height)
	contentW, listH := uicore.PickerListSize(width, height, linkPickerMaxWidth, 20, 7)
	p.list.SetSize(contentW, listH)
	return p
}

func (p LinkPicker) Update(msg tea.Msg) (LinkPicker, tea.Cmd) {
	if !p.shell.IsOpen() {
		return p, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(keyMsg, p.keys.Enter):
		idx := p.list.Index()
		if idx < 0 || idx >= len(p.links) {
			return p, nil
		}
		url := p.links[idx]
		return p, tea.Batch(
			func() tea.Msg { return LaunchURLMsg{URL: url} },
			func() tea.Msg { return LinkPickerClosedMsg{} },
		)
	case key.Matches(keyMsg, p.keys.Close):
		return p, func() tea.Msg { return LinkPickerClosedMsg{} }
	}
	for i, b := range p.keys.Digits {
		if key.Matches(keyMsg, b) {
			if i < len(p.links) {
				url := p.links[i]
				return p, tea.Batch(
					func() tea.Msg { return LaunchURLMsg{URL: url} },
					func() tea.Msg { return LinkPickerClosedMsg{} },
				)
			}
			return p, nil
		}
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

const linkPickerMaxWidth = 70

func (p LinkPicker) View() string {
	if !p.shell.IsOpen() {
		return ""
	}
	return p.Box(p.shell.Width(), p.shell.Height())
}

func (p LinkPicker) Box(w, h int) string {
	contentW, _ := uicore.PickerListSize(w, h, linkPickerMaxWidth, 20, 7)
	bodyRows := uicore.SplitAndPad(p.list.View(), contentW)

	previewLines := p.previewLines(contentW)
	footerRows := make([]string, 2)
	for i := range 2 {
		line := ""
		if i < len(previewLines) {
			line = previewLines[i]
		}
		footerRows[i] = uicore.PadOrTruncate(line, contentW)
	}

	return p.shell.Box("Links", bodyRows, footerRows, contentW)
}

const linkPickerInlineCap = 50

type linkItemDelegate struct {
	styles Styles
}

func (d linkItemDelegate) Height() int                             { return 1 }
func (d linkItemDelegate) Spacing() int                            { return 0 }
func (d linkItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d linkItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	li, ok := item.(linkItem)
	if !ok {
		return
	}
	url := li.url
	// Picker is capped at ≤9 items by its digit-key surface, so index is always 1 digit.
	const indexW = 4 // "[N] "
	contentW := m.Width()
	urlW := contentW - indexW
	if urlW > linkPickerInlineCap {
		urlW = linkPickerInlineCap
	}
	if ansix.Width(url) > urlW {
		url = ansix.Truncate(url, urlW)
	}
	body := uicore.PadOrTruncate(fmt.Sprintf("[%d] %s", index+1, url), contentW)
	if index == m.Index() {
		body = d.styles.Cursor.Render(body)
	}
	fmt.Fprint(w, body)
}

func (p LinkPicker) previewLines(width int) []string {
	idx := p.list.Index()
	if idx < 0 || idx >= len(p.links) {
		return nil
	}
	full := p.links[idx]
	wrapped := strings.Split(linkPickerWrap(full, width), "\n")
	if len(wrapped) <= 2 {
		return wrapped
	}
	row2 := wrapped[1]
	if ansix.Width(row2) >= width {
		row2 = ansix.Truncate(row2, width-1) + "…"
	} else {
		row2 += "…"
	}
	return []string{wrapped[0], row2}
}

func linkPickerWrap(s string, width int) string {
	if width < 1 {
		width = 1
	}
	return ansi.Hardwrap(ansi.Wordwrap(s, width, ""), width, false)
}

func (p LinkPicker) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}
