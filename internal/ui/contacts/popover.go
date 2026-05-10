package contacts

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ui/uicore"
)

const (
	popoverContentW = 50
	popoverMinW     = 20
)

// Popover is the sender-info overlay opened by 'i' from the account view.
// App owns open state and constructs a fresh Popover per open.
type Popover struct {
	shell       uicore.ModalShell
	styles      Styles
	displayName string
	email       string
	match       Contact
	hasMatch    bool
	width       int
	height      int
}

func NewPopover(s Styles) Popover {
	return Popover{styles: s}
}

// SetMatch stores the sender identity and the resolved contact, if any.
func (p *Popover) SetMatch(displayName, email string, match Contact, hasMatch bool) {
	p.displayName = displayName
	p.email = email
	p.match = match
	p.hasMatch = hasMatch
}

func (p *Popover) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.shell = p.shell.SetSize(w, h)
}

func (p Popover) Update(msg tea.Msg) (Popover, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch {
	case key.Matches(k, keys.Esc), key.Matches(k, keys.I):
		return p, func() tea.Msg { return ClosePopoverMsg{} }
	case key.Matches(k, keys.N) && !p.hasMatch:
		initial := Contact{
			Kind:   KindPerson,
			Name:   p.displayName,
			Emails: []Email{{Address: p.email}},
		}
		return p, func() tea.Msg {
			return OpenFormMsg{Initial: initial, FromPopover: true}
		}
	}
	return p, nil
}

func (p Popover) View() string {
	if p.width == 0 || p.height == 0 {
		return ""
	}
	return p.Box(p.width, p.height)
}

// Box renders the popover at the given terminal dims. App passes the
// result to uicore.PlaceOverlay.
func (p Popover) Box(termW, _ int) string {
	cw := popoverContentW
	if termW-4 < cw {
		cw = termW - 4
	}
	if cw < popoverMinW {
		cw = popoverMinW
	}

	var bodyRows []string
	if p.hasMatch {
		card := RenderDetailCard(p.match, cw, p.styles)
		for _, l := range strings.Split(card, "\n") {
			bodyRows = append(bodyRows, uicore.PadOrTruncate(l, cw))
		}
	} else {
		bodyRows = append(bodyRows,
			uicore.PadOrTruncate(p.styles.Name.Render(p.displayName), cw),
			uicore.PadOrTruncate(p.styles.Body.Render(p.email), cw),
			uicore.PadOrTruncate("", cw),
			uicore.PadOrTruncate(p.styles.Dim.Render("No contact in address book."), cw),
		)
	}

	var footerRows []string
	if p.hasMatch {
		footerRows = []string{
			uicore.PadOrTruncate(p.styles.Dim.Render("i close · n new contact · Esc dismiss"), cw),
		}
	} else {
		footerRows = []string{
			uicore.PadOrTruncate(p.styles.Dim.Render("n add contact · Esc dismiss"), cw),
		}
	}

	return p.shell.Box("Sender", bodyRows, footerRows, cw)
}

func (p Popover) Position(box string, totalW, totalH int) (int, int) {
	return uicore.CenterOverlay(box, totalW, totalH)
}
