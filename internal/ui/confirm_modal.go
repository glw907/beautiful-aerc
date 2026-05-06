// SPDX-License-Identifier: MIT

package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// ConfirmRequest holds the content for one modal invocation. The
// followup action on confirmation is dispatched by App based on
// state it sets when opening the modal. ConfirmModal itself only
// emits ConfirmModalYesMsg / ConfirmModalClosedMsg.
type ConfirmRequest struct {
	Title string
	Body  string
}

// ConfirmModalYesMsg fires when the user presses 'y' on an open
// ConfirmModal. App dispatches the appropriate followup based on
// its own pending-confirm state.
type ConfirmModalYesMsg struct{}

// ConfirmModal is a yes/no confirmation overlay. App owns it and composes it
// via Box + Position + PlaceOverlay, mirroring MovePicker and LinkPicker.
type ConfirmModal struct {
	shell  uicore.ModalShell
	req    ConfirmRequest
	styles Styles
	keys   confirmKeys
}

type confirmKeys struct {
	Yes     key.Binding
	Dismiss key.Binding
}

// NewConfirmModal returns a closed modal.
func NewConfirmModal(styles Styles) ConfirmModal {
	return ConfirmModal{
		styles: styles,
		keys: confirmKeys{
			Yes:     key.NewBinding(key.WithKeys("y")),
			Dismiss: key.NewBinding(key.WithKeys("n", "esc")),
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
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, m.keys.Yes):
		return m, tea.Batch(
			func() tea.Msg { return ConfirmModalYesMsg{} },
			func() tea.Msg { return ConfirmModalClosedMsg{} },
		)
	case key.Matches(keyMsg, m.keys.Dismiss):
		return m, func() tea.Msg { return ConfirmModalClosedMsg{} }
	}
	// q is swallowed, consistent with help/link/move picker overlays.
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

// Box returns the rendered modal at the size derived from (w, h).
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

	// Pad body lines to exactly contentW cells.
	bodyRows := make([]string, len(bodyLines))
	for i, line := range bodyLines {
		bodyRows[i] = uicore.PadOrTruncate(line, contentW)
	}

	// Footer row: help text.
	help := "[y] yes   [n] no   [esc] cancel"
	footerRows := []string{m.styles.Dim.Render(uicore.PadOrTruncate(help, contentW))}

	return m.shell.Box(m.req.Title, bodyRows, footerRows, contentW)
}

// Position returns the centered top-left for PlaceOverlay.
func (m ConfirmModal) Position(box string, totalW, totalH int) (int, int) {
	return centerOverlay(box, totalW, totalH)
}
