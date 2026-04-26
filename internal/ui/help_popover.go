package ui

// HelpContext selects which binding layout the popover renders.
type HelpContext int

const (
	HelpAccount HelpContext = iota
	HelpViewer
)

// HelpPopover is the modal help overlay. App owns key routing;
// this model only renders.
type HelpPopover struct {
	styles  Styles
	context HelpContext
}

// NewHelpPopover constructs a popover for the given context.
func NewHelpPopover(styles Styles, context HelpContext) HelpPopover {
	return HelpPopover{styles: styles, context: context}
}

// bindingRow is a single key/description entry in the popover.
// wired is false for keys whose action is not yet implemented;
// such rows render dim per the future-binding policy.
type bindingRow struct {
	key   string
	desc  string
	wired bool
}

// bindingGroup is a labeled cluster of bindingRow entries
// (e.g., "Navigate", "Triage").
type bindingGroup struct {
	title string
	rows  []bindingRow
}

// accountGroups is the binding map shown when the popover opens
// from the account view. Order is the visual layout order.
var accountGroups = []bindingGroup{
	{
		title: "Navigate",
		rows: []bindingRow{
			{"j/k", "up/down", true},
			{"g/G", "top/bot", true},
		},
	},
	{
		title: "Triage",
		rows: []bindingRow{
			{"d", "delete", false},
			{"a", "archive", false},
			{"s", "star", false},
			{".", "read/unrd", false},
		},
	},
	{
		title: "Reply",
		rows: []bindingRow{
			{"r", "reply", false},
			{"R", "all", false},
			{"f", "forward", false},
			{"c", "compose", false},
		},
	},
	{
		title: "Search",
		rows: []bindingRow{
			{"/", "search", true},
			{"n", "next", false},
			{"N", "prev", false},
		},
	},
	{
		title: "Select",
		rows: []bindingRow{
			{"v", "select", false},
			{"␣", "toggle", false},
		},
	},
	{
		title: "Threads",
		rows: []bindingRow{
			{"␣", "fold", true},
			{"F", "fold all", true},
		},
	},
	{
		title: "Go To",
		rows: []bindingRow{
			{"I", "inbox", true},
			{"D", "drafts", true},
			{"S", "sent", true},
			{"A", "archive", true},
			{"X", "spam", true},
			{"T", "trash", true},
		},
	},
}

// accountBottomHints is the trailing line under the groups in the
// account context: "Enter open    ?  close".
var accountBottomHints = []bindingRow{
	{"Enter", "open", true},
	{"?", "close", true},
}

// viewerGroups is the binding map shown when the popover opens
// from the message viewer.
var viewerGroups = []bindingGroup{
	{
		title: "Navigate",
		rows: []bindingRow{
			{"j/k", "scroll", true},
			{"g/G", "top/bot", true},
			{"␣/b", "page d/u", true},
			{"1-9", "open link", true},
		},
	},
	{
		title: "Triage",
		rows: []bindingRow{
			{"d", "delete", false},
			{"a", "archive", false},
			{"s", "star", false},
		},
	},
	{
		title: "Reply",
		rows: []bindingRow{
			{"r", "reply", false},
			{"R", "all", false},
			{"f", "forward", false},
			{"c", "compose", false},
		},
	},
}

// viewerBottomHints is the trailing line in the viewer context:
// "Tab link picker    q  close    ?  close".
var viewerBottomHints = []bindingRow{
	{"Tab", "link picker", false},
	{"q", "close", true},
	{"?", "close", true},
}
