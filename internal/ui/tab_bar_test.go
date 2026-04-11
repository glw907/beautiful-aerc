package ui

import (
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestRenderTabBar(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("single tab", func(t *testing.T) {
		tabs := []tabInfo{{title: "Inbox", icon: "󰇰"}}
		result := renderTabBar(tabs, 0, 80, 30, styles)
		lines := strings.Split(result, "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d", len(lines))
		}
		if !strings.Contains(lines[0], "╭") {
			t.Error("row 1 missing top-left corner")
		}
		if !strings.Contains(lines[2], "╮") {
			t.Error("row 3 missing right frame corner")
		}
		if !strings.Contains(lines[1], "Inbox") {
			t.Error("row 2 missing tab title")
		}
		if !strings.Contains(lines[2], "╯") {
			t.Error("row 3 missing bottom corner")
		}
		if !strings.Contains(lines[2], "┬") {
			t.Error("row 3 missing divider junction")
		}
	})

	t.Run("two tabs active first", func(t *testing.T) {
		tabs := []tabInfo{
			{title: "Inbox", icon: "󰇰"},
			{title: "Re: Project update", icon: "󰇰"},
		}
		result := renderTabBar(tabs, 0, 80, 30, styles)
		if !strings.Contains(result, "Inbox") {
			t.Error("missing active tab title")
		}
		if !strings.Contains(result, "Re: Project update") {
			t.Error("missing inactive tab title")
		}
	})

	t.Run("two tabs active second", func(t *testing.T) {
		tabs := []tabInfo{
			{title: "Inbox", icon: "󰇰"},
			{title: "Re: Project update", icon: "󰇰"},
		}
		result := renderTabBar(tabs, 1, 80, 30, styles)
		lines := strings.Split(result, "\n")
		if !strings.Contains(lines[1], "Inbox") {
			t.Error("row 2 missing inactive tab")
		}
	})

	t.Run("title truncation", func(t *testing.T) {
		tabs := []tabInfo{{
			title: "This is a very long subject line that exceeds thirty characters",
			icon:  "󰇰",
		}}
		result := renderTabBar(tabs, 0, 80, 30, styles)
		if !strings.Contains(result, "…") {
			t.Error("long title not truncated with ellipsis")
		}
	})
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"short", "Inbox", 30, "Inbox"},
		{"exact", "123456789012345678901234567890", 30, "123456789012345678901234567890"},
		{"long", "1234567890123456789012345678901", 30, "12345678901234567890123456789…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateTitle(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncateTitle(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}
