// SPDX-License-Identifier: MIT

package ui

import "github.com/charmbracelet/bubbles/key"

// GlobalKeys are handled by the root App model. Quit is split from
// ForceQuit because q is context-sensitive (closes the viewer, clears
// an active search, then quits) while Ctrl+C always quits.
type GlobalKeys struct {
	Help      key.Binding
	Quit      key.Binding
	ForceQuit key.Binding
	CloseHelp key.Binding
}

// NewGlobalKeys returns the default global key bindings.
func NewGlobalKeys() GlobalKeys {
	return GlobalKeys{
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c")),
		CloseHelp: key.NewBinding(key.WithKeys("?", "esc"), key.WithHelp("?/esc", "close help")),
	}
}

// AccountKeys are handled by AccountTab. The set spans message-list
// motion, sidebar motion, folder jumps, search shelf, fold control,
// and the n/N message advance keys consumed by AccountTab when the
// viewer is open.
type AccountKeys struct {
	OpenSearch    key.Binding
	ClearSearch   key.Binding
	OpenMessage   key.Binding
	SidebarDown   key.Binding
	SidebarUp     key.Binding
	JumpInbox     key.Binding
	JumpDrafts    key.Binding
	JumpSent      key.Binding
	JumpArchive   key.Binding
	JumpSpam      key.Binding
	JumpTrash     key.Binding
	MsgListTop    key.Binding
	MsgListBottom key.Binding
	MsgListDown   key.Binding
	MsgListUp     key.Binding
	ToggleFold    key.Binding
	ToggleFoldAll key.Binding
	NextMessage   key.Binding
	PrevMessage   key.Binding
}

// NewAccountKeys returns the default account-tab key bindings.
func NewAccountKeys() AccountKeys {
	return AccountKeys{
		OpenSearch:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		ClearSearch:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear search")),
		OpenMessage:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		SidebarDown:   key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "next folder")),
		SidebarUp:     key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "prev folder")),
		JumpInbox:     key.NewBinding(key.WithKeys("I"), key.WithHelp("I", "inbox")),
		JumpDrafts:    key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "drafts")),
		JumpSent:      key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sent")),
		JumpArchive:   key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "archive")),
		JumpSpam:      key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "spam")),
		JumpTrash:     key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "trash")),
		MsgListTop:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top of list")),
		MsgListBottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom of list")),
		MsgListDown:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
		MsgListUp:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
		ToggleFold:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "fold")),
		ToggleFoldAll: key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "fold all")),
		NextMessage:   key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next message")),
		PrevMessage:   key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev message")),
	}
}

// ViewerKeys are handled by Viewer.handleKey. Body scrolling
// (j/k/space/b) is delegated to the embedded viewport's own KeyMap;
// only the keys Viewer consumes directly appear here.
type ViewerKeys struct {
	Close      key.Binding
	OpenPicker key.Binding
	BodyTop    key.Binding
	BodyBottom key.Binding
	Link1      key.Binding
	Link2      key.Binding
	Link3      key.Binding
	Link4      key.Binding
	Link5      key.Binding
	Link6      key.Binding
	Link7      key.Binding
	Link8      key.Binding
	Link9      key.Binding
}

// NewViewerKeys returns the default viewer key bindings.
func NewViewerKeys() ViewerKeys {
	return ViewerKeys{
		Close:      key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "close")),
		OpenPicker: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "links")),
		BodyTop:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top of body")),
		BodyBottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom of body")),
		Link1:      key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "link 1")),
		Link2:      key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "link 2")),
		Link3:      key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "link 3")),
		Link4:      key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "link 4")),
		Link5:      key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "link 5")),
		Link6:      key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "link 6")),
		Link7:      key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "link 7")),
		Link8:      key.NewBinding(key.WithKeys("8"), key.WithHelp("8", "link 8")),
		Link9:      key.NewBinding(key.WithKeys("9"), key.WithHelp("9", "link 9")),
	}
}

// linkBindingByIndex returns the 1-based link binding from vk, or
// zero binding when out of range. Used by Viewer.handleKey to fold
// the nine digit keys into a single dispatch path.
func linkBindingByIndex(vk ViewerKeys, n int) key.Binding {
	switch n {
	case 1:
		return vk.Link1
	case 2:
		return vk.Link2
	case 3:
		return vk.Link3
	case 4:
		return vk.Link4
	case 5:
		return vk.Link5
	case 6:
		return vk.Link6
	case 7:
		return vk.Link7
	case 8:
		return vk.Link8
	case 9:
		return vk.Link9
	}
	return key.Binding{}
}
