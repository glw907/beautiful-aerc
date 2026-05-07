package ui

import "github.com/charmbracelet/bubbles/key"

// GlobalKeys are handled by the root App model. Quit and ForceQuit are
// split because q is context-sensitive (closes the viewer, clears an
// active search, then quits) while Ctrl+C always quits.
type GlobalKeys struct {
	Help            key.Binding
	Quit            key.Binding
	ForceQuit       key.Binding
	CloseHelp       key.Binding
	Undo            key.Binding
	OutboxOverlay   key.Binding
	ConflictOverlay key.Binding
	Compose         key.Binding
	Reply           key.Binding
	ReplyAll        key.Binding
	Forward         key.Binding
	SenderPopover   key.Binding
	ContactsMode    key.Binding
	MailMode        key.Binding
}

func NewGlobalKeys() GlobalKeys {
	return GlobalKeys{
		Help:            key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:            key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit:       key.NewBinding(key.WithKeys("ctrl+c")),
		CloseHelp:       key.NewBinding(key.WithKeys("?", "esc"), key.WithHelp("?/esc", "close help")),
		Undo:            key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
		OutboxOverlay:   key.NewBinding(key.WithKeys("Q"), key.WithHelp("Q", "outbox")),
		ConflictOverlay: key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "conflicts")),
		Compose:         key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compose")),
		Reply:           key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reply")),
		ReplyAll:        key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "reply all")),
		Forward:         key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "forward")),
		SenderPopover:   key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "sender info")),
		ContactsMode:    key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "contacts")),
		MailMode:        key.NewBinding(key.WithKeys("M"), key.WithHelp("M", "mail")),
	}
}

// ComposeKeys are the bindings active while compose.Model has focus.
// Ctrl chords are deliberate; text-entry surfaces are exempt from the
// modifier-free rule (ADR-0076).
type ComposeKeys struct {
	Send       key.Binding
	Cancel     key.Binding
	NextField  key.Binding
	PrevField  key.Binding
	EscapeBody key.Binding
}

func NewComposeKeys() ComposeKeys {
	return ComposeKeys{
		Send:       key.NewBinding(key.WithKeys("ctrl+x"), key.WithHelp("^X", "send")),
		Cancel:     key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "cancel")),
		NextField:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("⇥", "next")),
		PrevField:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧⇥", "prev")),
		EscapeBody: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "focus")),
	}
}
