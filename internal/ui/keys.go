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

// AccountKeys groups for the unified one-pane account footer:
// folder nav, message nav, triage, reply, and app keys are all live.
type AccountKeys struct {
	nav    keyGroup
	triage keyGroup
	reply  keyGroup
	app    keyGroup
}

// NewAccountKeys returns the default account view key bindings.
//
// The nav group compresses multi-key bindings into single entries
// (j/k/J/K, I/D/S/A) so the footer has room for the full set of
// planned account-view hints, including actions that are not yet
// operative (`. read`, `v select`, `n/N results`).
func NewAccountKeys() AccountKeys {
	return AccountKeys{
		nav: keyGroup{
			key.NewBinding(key.WithKeys("j"), key.WithHelp("j/k/J/K", "nav")),
			key.NewBinding(key.WithKeys("I"), key.WithHelp("I/D/S/A", "folders")),
		},
		triage: keyGroup{
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "del")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "star")),
			key.NewBinding(key.WithKeys("."), key.WithHelp(".", "read")),
		},
		reply: keyGroup{
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r/R", "reply")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "fwd")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compose")),
		},
		app: keyGroup{
			key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "find")),
			key.NewBinding(key.WithKeys("n"), key.WithHelp("n/N", "results")),
			key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "select")),
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "cmd")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		},
	}
}

// Groups returns the keybinding groups for footer rendering.
func (k AccountKeys) Groups() []keyGroup {
	return []keyGroup{k.nav, k.triage, k.reply, k.app}
}

// ViewerKeys groups for the viewer footer.
type ViewerKeys struct {
	triage keyGroup
	reply  keyGroup
	viewer keyGroup
}

// NewViewerKeys returns the default viewer key bindings.
//
// Includes planned hints that aren't yet wired up (`. read`,
// `c compose`, `: cmd`) so the footer shows the full viewer
// vocabulary as the implementation catches up.
func NewViewerKeys() ViewerKeys {
	return ViewerKeys{
		triage: keyGroup{
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "del")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "star")),
			key.NewBinding(key.WithKeys("."), key.WithHelp(".", "read")),
		},
		reply: keyGroup{
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r/R", "reply")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "fwd")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compose")),
		},
		viewer: keyGroup{
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "links")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "close")),
			key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "cmd")),
		},
	}
}

// Groups returns the keybinding groups for footer rendering.
func (k ViewerKeys) Groups() []keyGroup {
	return []keyGroup{k.triage, k.reply, k.viewer}
}
