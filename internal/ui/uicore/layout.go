// SPDX-License-Identifier: MIT

package uicore

// LayoutMode is the resolved set of layout decisions for a given
// terminal width. Computed once per WindowSizeMsg by ComputeLayout
// and threaded into Sidebar and MessageList. Pure data, no I/O,
// no side effects, no methods that change observable behavior
// based on flags outside the struct.
type LayoutMode struct {
	// Sidebar is the sidebar width in cells (folder list). Range
	// [14, 30]. Floor 14 fits "Archive" with icons off. Ceiling
	// 30 gives breathing room for nested custom folder names.
	Sidebar int

	// Sender is the message-list sender column width in cells.
	// Range [22, 32]. Floor 22 covers ~86% of real sender names
	// untruncated. Ceiling 32 covers ~95%.
	Sender int

	// Date is the message-list date column width in cells.
	//   0: column hidden
	//   3: compact relative format ("now", "5m", "Apr", "'24")
	//   5: short absolute format ("04-30" / "3:04p")
	Date int

	// FlagColumn is true when the message-list flag/status icon
	// column is rendered. When false, the column is omitted
	// entirely (saves 4 cells: glyph + inter-column gap).
	FlagColumn bool

	// Icons is true when the sidebar renders folder icons. When
	// false, the icon block is omitted from each sidebar row,
	// shrinking the lead from 8 (fancy) to 2 cells.
	Icons bool
}

// IconSet is the per-mode iconography vocabulary for poplar's UI
// surfaces. SimpleIcons uses Unicode Narrow-class codepoints. Every
// field has lipgloss.Width == 1. FancyIcons uses Nerd Font SPUA-A
// glyphs (U+F0000–U+FFFFD). Their rendered cell width is determined
// at startup by term.MeasureSPUACells and applied via SPUACellWidth.
//
// Add a field here whenever a new render surface needs an icon. Both
// tables must be updated together. Tests enforce the class invariants.
type IconSet struct {
	Inbox        string
	Drafts       string
	Sent         string
	Archive      string
	Spam         string
	Trash        string
	Notification string
	Reminder     string
	CustomFolder string
	Search       string
	FlagFlagged  string
	FlagAnswered string
	FlagUnread   string
	Attachment   string
}

// SimpleIcons is the Unicode-Narrow iconography used when no Nerd Font
// is detected. Every rune must be East Asian Width Na or N.
var SimpleIcons = IconSet{
	Inbox:        "▣", // U+25A3
	Drafts:       "✎", // U+270E
	Sent:         "→", // U+2192
	Archive:      "▢", // U+25A2
	Spam:         "!", // ASCII
	Trash:        "✗", // U+2717
	Notification: "•", // U+2022
	Reminder:     "◷", // U+25F7
	CustomFolder: "▪", // U+25AA
	Search:       "/", // ASCII
	FlagFlagged:  "⚑", // U+2691
	FlagAnswered: "↩", // U+21A9
	FlagUnread:   "●", // U+25CF
	Attachment:   "§", // U+00A7
}

// FancyIcons is the Nerd Font SPUA-A iconography used when a Nerd Font
// is detected.
var FancyIcons = IconSet{
	Inbox:        "\U000F01F0", // nf-md-inbox
	Drafts:       "\U000F03EB", // nf-md-file_document_edit
	Sent:         "\U000F045A", // nf-md-send
	Archive:      "\U000F003C", // nf-md-archive
	Spam:         "\U000F0377", // nf-md-shield-alert
	Trash:        "\U000F0A7A", // nf-md-trash-can-outline
	Notification: "\U000F009A", // nf-md-bell
	Reminder:     "\U000F0474", // nf-md-clock-alert
	CustomFolder: "\U000F0861", // nf-md-folder-outline
	Search:       "\U000F0349", // nf-md-magnify
	FlagFlagged:  "\U000F023B", // nf-md-flag
	FlagAnswered: "\U000F045A", // nf-md-send
	FlagUnread:   "\U000F01EE", // nf-md-mailbox
	Attachment:   "\U000F0184", // nf-md-paperclip
}
