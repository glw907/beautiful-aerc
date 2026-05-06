// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gomail "github.com/emersion/go-message/mail"
	"github.com/glw907/poplar/internal/compose"
)

// ComposeTab is the inline compose surface. Send and discard surface
// as tea.Msg values that App translates into cache ops.
type ComposeTab struct {
	styles Styles
	icons  IconSet

	from string

	to      textinput.Model
	cc      textinput.Model
	bcc     textinput.Model
	subject textinput.Model
	editor  compose.Editor

	focus int
	// err is rendered as a one-line banner above the divider when set.
	err string

	width   int
	height  int
	divider string
}

const (
	composeFocusTo = iota
	composeFocusCc
	composeFocusBcc
	composeFocusSubject
	composeFocusBody
)

// composeLabelWidth is the column reserved for header labels. "Subject:"
// is 8 cells, plus one space of separation.
const composeLabelWidth = 9

// composeChromeRows is the fixed row count above the body: 5 headers
// plus the divider rule. The error banner adds one more when set.
const composeChromeRows = 6

// NewComposeTab returns a fresh, empty ComposeTab with focus on To.
func NewComposeTab(styles Styles, self string, icons IconSet) *ComposeTab {
	mk := func() textinput.Model {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = ""
		return ti
	}
	c := &ComposeTab{
		styles:  styles,
		icons:   icons,
		from:    self,
		to:      mk(),
		cc:      mk(),
		bcc:     mk(),
		subject: mk(),
		editor:  compose.NewCatkinEditor(),
	}
	c.to.Focus()
	c.focus = composeFocusTo
	return c
}

func (c *ComposeTab) Init() tea.Cmd {
	return c.editor.Init()
}

func (c *ComposeTab) SetSize(w, h int) {
	c.width = w
	c.height = h
	c.divider = strings.Repeat("─", w)

	inputW := w - composeLabelWidth - 1
	if inputW < 1 {
		inputW = 1
	}
	c.to.Width = inputW
	c.cc.Width = inputW
	c.bcc.Width = inputW
	c.subject.Width = inputW

	bodyHeight := h - composeChromeRows
	if c.err != "" {
		bodyHeight--
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	c.editor.SetSize(w, bodyHeight)
}

// View enforces the size contract: every returned line is exactly
// c.width display cells and the result is exactly c.height rows. The
// parent joins panes without defensive clipping.
func (c *ComposeTab) View() string {
	if c.width == 0 || c.height == 0 {
		return ""
	}
	rows := make([]string, 0, c.height)
	rows = append(rows, c.headerRow("From:", c.from))
	rows = append(rows, c.headerRow("To:", c.to.View()))
	rows = append(rows, c.headerRow("Cc:", c.cc.View()))
	rows = append(rows, c.headerRow("Bcc:", c.bcc.View()))
	rows = append(rows, c.headerRow("Subject:", c.subject.View()))
	if c.err != "" {
		rows = append(rows, c.padRow(c.styles.ErrorBanner.Render(c.err)))
	}
	rows = append(rows, c.padRow(c.divider))
	for _, line := range strings.Split(c.editor.View(), "\n") {
		rows = append(rows, c.padRow(line))
	}
	for len(rows) < c.height {
		rows = append(rows, c.padRow(""))
	}
	if len(rows) > c.height {
		rows = rows[:c.height]
	}
	return strings.Join(rows, "\n")
}

func (c *ComposeTab) headerRow(label, value string) string {
	pad := composeLabelWidth - lipgloss.Width(label)
	if pad < 1 {
		pad = 1
	}
	return c.padRow(label + strings.Repeat(" ", pad) + value)
}

func (c *ComposeTab) padRow(s string) string {
	w := lipgloss.Width(s)
	if w >= c.width {
		return displayTruncate(s, c.width)
	}
	return s + strings.Repeat(" ", c.width-w)
}

// ComposeSendMsg is emitted when the user presses Ctrl+X with a valid
// draft. App assembles MIME and queues the outbox op.
type ComposeSendMsg struct {
	Draft compose.Draft
}

// ComposeCancelMsg is emitted when the user presses Ctrl+C. App opens
// a discard ConfirmModal when Dirty. Clean drafts close immediately.
type ComposeCancelMsg struct {
	Dirty bool
}

func (c *ComposeTab) Update(msg tea.Msg) (*ComposeTab, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlX:
			d, err := c.Draft()
			if err != nil {
				return c, nil
			}
			return c, func() tea.Msg { return ComposeSendMsg{Draft: d} }
		case tea.KeyCtrlC:
			dirty := c.IsDirty()
			return c, func() tea.Msg { return ComposeCancelMsg{Dirty: dirty} }
		case tea.KeyTab:
			c.advanceFocus(+1)
			return c, nil
		case tea.KeyShiftTab:
			c.advanceFocus(-1)
			return c, nil
		case tea.KeyEsc:
			if c.focus == composeFocusBody {
				c.setFocus(composeFocusSubject)
			} else {
				c.setFocus(composeFocusBody)
			}
			return c, nil
		}
	}

	var cmd tea.Cmd
	switch c.focus {
	case composeFocusTo:
		c.to, cmd = c.to.Update(msg)
	case composeFocusCc:
		c.cc, cmd = c.cc.Update(msg)
	case composeFocusBcc:
		c.bcc, cmd = c.bcc.Update(msg)
	case composeFocusSubject:
		c.subject, cmd = c.subject.Update(msg)
	case composeFocusBody:
		c.editor, cmd = c.editor.Update(msg)
	}
	return c, cmd
}

func (c *ComposeTab) advanceFocus(delta int) {
	const fields = 5
	c.setFocus(((c.focus + delta) + fields) % fields)
}

func (c *ComposeTab) setFocus(target int) {
	c.to.Blur()
	c.cc.Blur()
	c.bcc.Blur()
	c.subject.Blur()
	c.editor.Blur()
	switch target {
	case composeFocusTo:
		_ = c.to.Focus()
	case composeFocusCc:
		_ = c.cc.Focus()
	case composeFocusBcc:
		_ = c.bcc.Focus()
	case composeFocusSubject:
		_ = c.subject.Focus()
	case composeFocusBody:
		_ = c.editor.Focus()
	}
	c.focus = target
}

// Draft rebuilds a compose.Draft from the current inputs. Address fields
// are parsed via net/mail; a parse error is stored in the inline err row
// and returned to the caller.
func (c *ComposeTab) Draft() (compose.Draft, error) {
	to, err := parseAddrField(c.to.Value(), "To")
	if err != nil {
		c.err = err.Error()
		return compose.Draft{}, err
	}
	cc, err := parseAddrField(c.cc.Value(), "Cc")
	if err != nil {
		c.err = err.Error()
		return compose.Draft{}, err
	}
	bcc, err := parseAddrField(c.bcc.Value(), "Bcc")
	if err != nil {
		c.err = err.Error()
		return compose.Draft{}, err
	}
	c.err = ""
	return compose.Draft{
		From:    gomail.Address{Address: c.from},
		To:      to,
		Cc:      cc,
		Bcc:     bcc,
		Subject: c.subject.Value(),
		Body:    c.editor.Value(),
	}, nil
}

// IsDirty reports whether any input field contains user-entered content.
func (c *ComposeTab) IsDirty() bool {
	return c.to.Value() != "" || c.cc.Value() != "" || c.bcc.Value() != "" ||
		c.subject.Value() != "" || c.editor.Value() != ""
}

// Seed populates the inputs from d. Called for reply/forward pre-fill.
func (c *ComposeTab) Seed(d compose.Draft) {
	c.to.SetValue(joinAddresses(d.To))
	c.cc.SetValue(joinAddresses(d.Cc))
	c.bcc.SetValue(joinAddresses(d.Bcc))
	c.subject.SetValue(d.Subject)
	c.editor.SetValue(d.Body)
}

func parseAddrField(raw, label string) ([]gomail.Address, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	addrs, err := mail.ParseAddressList(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	out := make([]gomail.Address, len(addrs))
	for i, a := range addrs {
		out[i] = gomail.Address{Name: a.Name, Address: a.Address}
	}
	return out, nil
}

func joinAddresses(addrs []gomail.Address) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a.Name != "" {
			parts = append(parts, fmt.Sprintf("%q <%s>", a.Name, a.Address))
		} else {
			parts = append(parts, a.Address)
		}
	}
	return strings.Join(parts, ", ")
}
