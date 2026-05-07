package account

import "github.com/charmbracelet/bubbles/key"

// Keys spans message-list motion, sidebar motion, folder jumps, the
// search shelf, fold control, and the n/N advance keys consumed while
// the viewer is open.
type Keys struct {
	OpenSearch    key.Binding
	ClearSearch   key.Binding
	SearchCommit  key.Binding
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
	Delete        key.Binding
	Archive       key.Binding
	Star          key.Binding
	ReadToggle    key.Binding
	EnterVisual   key.Binding
	Move          key.Binding
	Empty         key.Binding
}

func NewKeys() Keys {
	return Keys{
		OpenSearch:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		ClearSearch:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear search")),
		SearchCommit:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "commit search")),
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
		Delete:        key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Archive:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
		Star:          key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "star")),
		ReadToggle:    key.NewBinding(key.WithKeys("."), key.WithHelp(".", "read")),
		EnterVisual:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "select")),
		Move:          key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move")),
		Empty:         key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "empty")),
	}
}
