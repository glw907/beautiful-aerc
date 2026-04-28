// SPDX-License-Identifier: MIT

package ui

// IconSet is the per-mode iconography vocabulary for poplar's UI
// surfaces. SimpleIcons uses Unicode Narrow-class codepoints — every
// field has lipgloss.Width == 1. FancyIcons uses Nerd Font SPUA-A
// glyphs (U+F0000–U+FFFFD); their rendered cell width is determined
// at startup by term.MeasureSPUACells and applied via spuaCellWidth.
//
// Add a field here whenever a new render surface needs an icon; both
// tables must be updated together. Tests in icons_test.go enforce the
// class invariants.
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
}

// SimpleIcons is the Unicode-Narrow iconography used when no Nerd Font
// is detected (or icons = "simple"). Every rune must be East Asian
// Width Na or N. Verified by TestSimpleIcons_AllNarrow.
var SimpleIcons = IconSet{
	Inbox:        "▣", // U+25A3
	Drafts:       "✎", // U+270E
	Sent:         "→", // U+2192
	Archive:      "▢", // U+25A2
	Spam:         "!", // ASCII; U+26A0 ⚠ is Ambiguous-class on some terminals
	Trash:        "✗", // U+2717
	Notification: "•", // U+2022
	Reminder:     "◷", // U+25F7
	CustomFolder: "▪", // U+25AA
	Search:       "/", // ASCII; canonical search affordance
	FlagFlagged:  "⚑", // U+2691
	FlagAnswered: "↩", // U+21A9
	FlagUnread:   "●", // U+25CF
}

// FancyIcons is the Nerd Font SPUA-A iconography used when a Nerd Font
// is detected (or icons = "fancy"). Verified by TestFancyIcons_AllSPUA.
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
	FlagAnswered: "\U000F045A", // nf-md-send (placeholder for answered; matches legacy mlIconAnswered)
	FlagUnread:   "\U000F01EE", // nf-md-mailbox (matches legacy mlIconUnread)
}
