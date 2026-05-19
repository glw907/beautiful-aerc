package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// ConfirmRequest holds the content for one modal invocation. App
// dispatches the followup action based on its own pending-confirm state;
// the modal only emits Yes/No/Closed msgs.
type ConfirmRequest struct {
	Title string
	Body  string
}

// ConfirmModalYesMsg fires on 'y'. App dispatches the followup.
type ConfirmModalYesMsg struct{}

// ConfirmModalNoMsg fires on 'n', distinguishing active rejection from
// the neutral Esc-dismiss.
type ConfirmModalNoMsg struct{}

// ConfirmModalClosedMsg signals the modal was dismissed without confirmation.
type ConfirmModalClosedMsg struct{}

// ConfirmModal is a yes/no confirmation overlay composed via Box +
// Position + PlaceOverlay, mirroring MovePicker and LinkPicker.
type ConfirmModal struct {
	shell  uicore.ModalShell
	req    ConfirmRequest
	styles Styles
	keys   confirmKeys
}

type confirmKeys struct {
	Yes key.Binding
	No  key.Binding
	Esc key.Binding
}

func NewConfirmModal(styles Styles) ConfirmModal {
	return ConfirmModal{
		styles: styles,
		keys: confirmKeys{
			Yes: key.NewBinding(key.WithKeys("y")),
			No:  key.NewBinding(key.WithKeys("n")),
			Esc: key.NewBinding(key.WithKeys("esc")),
		},
	}
}

func (m ConfirmModal) IsOpen() bool { return m.shell.IsOpen() }

func (m ConfirmModal) Open(req ConfirmRequest) ConfirmModal {
	m.shell = m.shell.WithOpen(true)
	m.req = req
	return m
}

func (m ConfirmModal) Close() ConfirmModal {
	m.shell = m.shell.WithOpen(false)
	return m
}

func (m ConfirmModal) SetSize(width, height int) ConfirmModal {
	m.shell = m.shell.SetSize(width, height)
	return m
}

func (m ConfirmModal) Update(msg tea.Msg) (ConfirmModal, tea.Cmd) {
	if !m.shell.IsOpen() {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, m.keys.Yes):
		return m, func() tea.Msg { return ConfirmModalYesMsg{} }
	case key.Matches(keyMsg, m.keys.No):
		return m, func() tea.Msg { return ConfirmModalNoMsg{} }
	case key.Matches(keyMsg, m.keys.Esc):
		return m, func() tea.Msg { return ConfirmModalClosedMsg{} }
	}
	// q is swallowed, matching help/link/move picker behavior.
	return m, nil
}

const (
	confirmModalMaxWidth = 50
	confirmModalMinWidth = 24
)

func (m ConfirmModal) View() string {
	if !m.shell.IsOpen() {
		return ""
	}
	return m.Box(m.shell.Width(), m.shell.Height())
}

// Box renders the modal at the size derived from (w, h).
func (m ConfirmModal) Box(w, h int) string {
	boxW := confirmModalMaxWidth
	if w-4 < boxW {
		boxW = w - 4
	}
	if boxW < confirmModalMinWidth {
		boxW = confirmModalMinWidth
	}
	contentW := boxW - 2

	wrapW := contentW
	if wrapW < 1 {
		wrapW = 1
	}
	body := ansi.Hardwrap(ansi.Wordwrap(m.req.Body, wrapW, ""), wrapW, false)
	bodyLines := strings.Split(body, "\n")

	bodyRows := make([]string, len(bodyLines))
	for i, line := range bodyLines {
		bodyRows[i] = uicore.PadOrTruncate(line, contentW)
	}

	help := "[y] yes   [n] no   [esc] cancel"
	footerRows := []string{m.styles.Dim.Render(uicore.PadOrTruncate(help, contentW))}

	return m.shell.Box(m.req.Title, bodyRows, footerRows, contentW)
}

func (m ConfirmModal) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}
