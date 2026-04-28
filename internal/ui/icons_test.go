// SPDX-License-Identifier: MIT

package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// surfaceFields returns one rune (or short string) per IconSet field
// — added to in lockstep with IconSet itself. Used by both
// width/range tests below.
func surfaceFields(s IconSet) map[string]string {
	return map[string]string{
		"Inbox":        s.Inbox,
		"Drafts":       s.Drafts,
		"Sent":         s.Sent,
		"Archive":      s.Archive,
		"Spam":         s.Spam,
		"Trash":        s.Trash,
		"Notification": s.Notification,
		"Reminder":     s.Reminder,
		"CustomFolder": s.CustomFolder,
		"Search":       s.Search,
		"FlagFlagged":  s.FlagFlagged,
		"FlagAnswered": s.FlagAnswered,
		"FlagUnread":   s.FlagUnread,
	}
}

func TestSimpleIcons_AllNarrow(t *testing.T) {
	for name, s := range surfaceFields(SimpleIcons) {
		if s == "" {
			t.Errorf("SimpleIcons.%s is empty", name)
			continue
		}
		if w := lipgloss.Width(s); w != 1 {
			t.Errorf("SimpleIcons.%s = %q has lipgloss.Width=%d, want 1 (must be Narrow class)", name, s, w)
		}
		// Reject any rune in SPUA-A.
		for _, r := range s {
			if r >= 0xF0000 && r <= 0xFFFFD {
				t.Errorf("SimpleIcons.%s = %q contains SPUA-A rune U+%X", name, s, r)
			}
		}
	}
}

func TestFancyIcons_AllSPUA(t *testing.T) {
	for name, s := range surfaceFields(FancyIcons) {
		if s == "" {
			t.Errorf("FancyIcons.%s is empty", name)
			continue
		}
		runes := []rune(s)
		if len(runes) != 1 {
			t.Errorf("FancyIcons.%s = %q is %d runes, want 1", name, s, len(runes))
			continue
		}
		r := runes[0]
		if r < 0xF0000 || r > 0xFFFFD {
			t.Errorf("FancyIcons.%s = U+%X is outside SPUA-A [F0000..FFFFD]", name, r)
		}
	}
}
