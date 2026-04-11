package ui

import "github.com/charmbracelet/bubbles/key"

// keyGroup is a slice of bindings that belong together visually.
type keyGroup []key.Binding

// GlobalKeys are handled by the root App model.
type GlobalKeys struct {
	Help key.Binding
	Cmd  key.Binding
	Quit key.Binding
}

// NewGlobalKeys returns the default global key bindings.
func NewGlobalKeys() GlobalKeys {
	return GlobalKeys{
		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Cmd:  key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "cmd")),
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// FolderJumpKeys jump to canonical folders.
type FolderJumpKeys struct {
	Inbox   key.Binding
	Drafts  key.Binding
	Sent    key.Binding
	Archive key.Binding
	Spam    key.Binding
	Trash   key.Binding
}

// NewFolderJumpKeys returns the default folder jump key bindings.
func NewFolderJumpKeys() FolderJumpKeys {
	return FolderJumpKeys{
		Inbox:   key.NewBinding(key.WithKeys("I"), key.WithHelp("I", "inbox")),
		Drafts:  key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "drafts")),
		Sent:    key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sent")),
		Archive: key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "archive")),
		Spam:    key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "spam")),
		Trash:   key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "trash")),
	}
}

// MsgListKeys groups for the message list footer.
type MsgListKeys struct {
	nav    keyGroup
	triage keyGroup
	reply  keyGroup
	app    keyGroup
}

// NewMsgListKeys returns the default message list key bindings.
func NewMsgListKeys() MsgListKeys {
	return MsgListKeys{
		nav: keyGroup{
			key.NewBinding(key.WithKeys("j"), key.WithHelp("j/k", "messages")),
			key.NewBinding(key.WithKeys("J"), key.WithHelp("J/K", "folders")),
		},
		triage: keyGroup{
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "del")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "star")),
		},
		reply: keyGroup{
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reply")),
			key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "all")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "fwd")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compose")),
		},
		app: keyGroup{
			key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "cmd")),
		},
	}
}

// Groups returns the keybinding groups for footer rendering.
func (k MsgListKeys) Groups() []keyGroup {
	return []keyGroup{k.nav, k.triage, k.reply, k.app}
}

// SidebarKeys groups for the sidebar footer.
type SidebarKeys struct {
	nav    keyGroup
	action keyGroup
	folder keyGroup
	app    keyGroup
}

// NewSidebarKeys returns the default sidebar key bindings.
func NewSidebarKeys() SidebarKeys {
	fj := NewFolderJumpKeys()
	return SidebarKeys{
		nav: keyGroup{
			key.NewBinding(key.WithKeys("j"), key.WithHelp("j/J/k/K", "nav")),
		},
		action: keyGroup{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "open")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compose")),
		},
		folder: keyGroup{fj.Inbox, fj.Drafts, fj.Sent, fj.Archive},
		app: keyGroup{
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "cmd")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		},
	}
}

// Groups returns the keybinding groups for footer rendering.
func (k SidebarKeys) Groups() []keyGroup {
	return []keyGroup{k.nav, k.action, k.folder, k.app}
}

// ViewerKeys groups for the viewer footer.
type ViewerKeys struct {
	triage keyGroup
	reply  keyGroup
	viewer keyGroup
}

// NewViewerKeys returns the default viewer key bindings.
func NewViewerKeys() ViewerKeys {
	return ViewerKeys{
		triage: keyGroup{
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "del")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "star")),
		},
		reply: keyGroup{
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reply")),
			key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "all")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "fwd")),
		},
		viewer: keyGroup{
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "links")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "close")),
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		},
	}
}

// Groups returns the keybinding groups for footer rendering.
func (k ViewerKeys) Groups() []keyGroup {
	return []keyGroup{k.triage, k.reply, k.viewer}
}
