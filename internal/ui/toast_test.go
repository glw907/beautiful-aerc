// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

func TestRenderToast(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	cases := []struct {
		name     string
		p        pendingAction
		width    int
		wantSubs []string
		empty    bool
	}{
		{name: "zero pending → empty", p: pendingAction{}, width: 80, empty: true},
		{name: "delete one", p: pendingAction{op: "delete", n: 1}, width: 80, wantSubs: []string{"Deleted 1 message", "u undo"}},
		{name: "delete many", p: pendingAction{op: "delete", n: 3}, width: 80, wantSubs: []string{"Deleted 3 messages", "u undo"}},
		{name: "archive one", p: pendingAction{op: "archive", n: 1}, width: 80, wantSubs: []string{"Archived 1 message"}},
		{name: "star", p: pendingAction{op: "star", n: 1}, width: 80, wantSubs: []string{"Starred"}},
		{name: "unstar", p: pendingAction{op: "unstar", n: 2}, width: 80, wantSubs: []string{"Unstarred 2"}},
		{name: "read", p: pendingAction{op: "read", n: 1}, width: 80, wantSubs: []string{"Marked read"}},
		{name: "unread", p: pendingAction{op: "unread", n: 1}, width: 80, wantSubs: []string{"Marked unread"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderToast(tc.p, tc.width, styles)
			if tc.empty {
				if got != "" {
					t.Fatalf("want empty, got %q", got)
				}
				return
			}
			for _, sub := range tc.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("output %q missing %q", got, sub)
				}
			}
			if w := lipgloss.Width(got); w > tc.width {
				t.Errorf("width %d exceeds %d", w, tc.width)
			}
		})
	}
}

func TestRenderToast_Truncation(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	got := renderToast(pendingAction{op: "delete", n: 999}, 12, styles)
	if w := lipgloss.Width(got); w > 12 {
		t.Errorf("truncated width %d > 12", w)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis in %q", got)
	}
}
