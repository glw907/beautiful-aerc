// SPDX-License-Identifier: MIT

package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ConfirmRequest holds the content and callback for one modal invocation.
type ConfirmRequest struct {
	Title string
	Body  string
	OnYes func() tea.Msg
}

// ConfirmModal is a yes/no confirmation overlay. App owns it and composes it
// via Box + Position + PlaceOverlay, mirroring MovePicker and LinkPicker.
type ConfirmModal struct {
	open   bool
	req    ConfirmRequest
	width  int
	height int
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

// IsOpen reports whether the modal is visible.
func (m ConfirmModal) IsOpen() bool { return m.open }

// Open transitions the modal into the open state with req.
func (m ConfirmModal) Open(req ConfirmRequest) ConfirmModal {
	m.open = true
	m.req = req
	return m
}

// Close transitions the modal out of view.
func (m ConfirmModal) Close() ConfirmModal {
	m.open = false
	return m
}

// SetSize updates dimensions. App threads WindowSizeMsg here.
func (m ConfirmModal) SetSize(width, height int) ConfirmModal {
	m.width = width
	m.height = height
	return m
}

// Update dispatches a key while the modal is open.
func (m ConfirmModal) Update(msg tea.Msg) (ConfirmModal, tea.Cmd) {
	if !m.open {
		return m, nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, m.keys.Yes):
		onYes := m.req.OnYes
		return m, tea.Batch(
			func() tea.Msg { return onYes() },
			func() tea.Msg { return ConfirmModalClosedMsg{} },
		)
	case key.Matches(keyMsg, m.keys.Dismiss):
		return m, func() tea.Msg { return ConfirmModalClosedMsg{} }
	}
	// q is swallowed — consistent with help/link/move picker overlays.
	return m, nil
}

const (
	confirmModalMaxWidth = 50
	confirmModalMinWidth = 24
)

// View renders the modal or "" when closed.
func (m ConfirmModal) View() string {
	if !m.open {
		return ""
	}
	return m.Box(m.width, m.height)
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

	body := confirmWrap(m.req.Body, contentW)
	bodyLines := strings.Split(body, "\n")

	var b strings.Builder

	// Top border with inset title.
	title := " " + m.req.Title + " "
	rest := boxW - 2 - lipgloss.Width(title)
	if rest < 0 {
		rest = 0
	}
	b.WriteString("┌─" + title + strings.Repeat("─", rest) + "┐\n")

	// Body rows.
	for _, line := range bodyLines {
		b.WriteString("│" + padOrTruncate(line, contentW) + "│\n")
	}

	// Separator + help row.
	b.WriteString("├" + strings.Repeat("─", contentW) + "┤\n")

	help := "[y] yes   [n] no   [esc] cancel"
	b.WriteString("│" + m.styles.Dim.Render(padOrTruncate(help, contentW)) + "│\n")

	b.WriteString("└" + strings.Repeat("─", contentW) + "┘")

	return b.String()
}

// Position returns the centered top-left for PlaceOverlay.
func (m ConfirmModal) Position(box string, totalW, totalH int) (int, int) {
	return centerOverlay(box, totalW, totalH)
}

// confirmWrap wordwraps body text to width, hardwrapping long tokens.
func confirmWrap(s string, width int) string {
	if width < 1 {
		width = 1
	}
	return ansi.Hardwrap(ansi.Wordwrap(s, width, ""), width, false)
}
