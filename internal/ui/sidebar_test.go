package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestSidebar(t *testing.T) {
	styles := NewStyles(theme.Nord)
	folders := mockFolders()

	t.Run("renders all folders", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		view := sb.View()
		plain := stripANSI(view)
		for _, f := range folders {
			if !strings.Contains(plain, f.Name) {
				t.Errorf("missing folder %q in view", f.Name)
			}
		}
	})

	t.Run("groups separated by blank lines", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		view := sb.View()
		plain := stripANSI(view)
		lines := strings.Split(plain, "\n")

		var blankIdxs []int
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				blankIdxs = append(blankIdxs, i)
			}
		}
		if len(blankIdxs) < 2 {
			t.Errorf("expected at least 2 blank separator lines, got %d", len(blankIdxs))
		}
	})

	t.Run("initial selection is first folder", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		if sb.Selected() != 0 {
			t.Errorf("initial selection = %d, want 0", sb.Selected())
		}
		if sb.SelectedFolder() != "Inbox" {
			t.Errorf("selected folder = %q, want Inbox", sb.SelectedFolder())
		}
	})

	t.Run("unread count shown only when positive", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		view := sb.View()
		plain := stripANSI(view)
		lines := strings.Split(plain, "\n")

		inboxLine := findLineContaining(lines, "Inbox")
		if inboxLine == "" {
			t.Fatal("no line containing Inbox")
		}
		if !strings.Contains(inboxLine, "3") {
			t.Error("Inbox line missing unread count 3")
		}

		sentLine := findLineContaining(lines, "Sent")
		if sentLine == "" {
			t.Fatal("no line containing Sent")
		}
		re := regexp.MustCompile(`Sent\s+\d`)
		if re.MatchString(sentLine) {
			t.Errorf("Sent line shows count when unseen=0: %q", sentLine)
		}
	})

	t.Run("selected row has focus indicator when focused", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		sb.focused = true
		view := sb.View()
		plain := stripANSI(view)
		lines := strings.Split(plain, "\n")
		if len(lines) == 0 {
			t.Fatal("empty view")
		}
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				if !strings.Contains(line, "┃") {
					t.Errorf("focused selected row missing ┃: %q", line)
				}
				break
			}
		}
	})

	t.Run("no focus indicator when unfocused", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		sb.focused = false
		view := sb.View()
		plain := stripANSI(view)
		if strings.Contains(plain, "┃") {
			t.Error("unfocused sidebar should not have ┃ indicator")
		}
	})

	t.Run("all lines same display width", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		view := sb.View()
		lines := strings.Split(view, "\n")
		for i, line := range lines {
			w := lipgloss.Width(line)
			if w != 30 {
				t.Errorf("line %d width = %d, want 30: %q",
					i, w, stripANSI(line))
			}
		}
	})

	t.Run("j moves down", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		sb.MoveDown()
		if sb.Selected() != 1 {
			t.Errorf("after MoveDown, selected = %d, want 1", sb.Selected())
		}
		if sb.SelectedFolder() != "Drafts" {
			t.Errorf("selected folder = %q, want Drafts", sb.SelectedFolder())
		}
	})

	t.Run("k moves up", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		sb.MoveDown()
		sb.MoveDown()
		sb.MoveUp()
		if sb.Selected() != 1 {
			t.Errorf("after Down+Down+Up, selected = %d, want 1", sb.Selected())
		}
	})

	t.Run("k at top stays at 0", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		sb.MoveUp()
		if sb.Selected() != 0 {
			t.Errorf("MoveUp at top: selected = %d, want 0", sb.Selected())
		}
	})

	t.Run("j at bottom stays at last", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		for i := 0; i < 20; i++ {
			sb.MoveDown()
		}
		last := len(folders) - 1
		if sb.Selected() != last {
			t.Errorf("MoveDown past end: selected = %d, want %d", sb.Selected(), last)
		}
	})

	t.Run("G moves to bottom", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		sb.MoveToBottom()
		last := len(folders) - 1
		if sb.Selected() != last {
			t.Errorf("MoveToBottom: selected = %d, want %d", sb.Selected(), last)
		}
	})

	t.Run("gg moves to top", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		sb.MoveDown()
		sb.MoveDown()
		sb.MoveDown()
		sb.MoveToTop()
		if sb.Selected() != 0 {
			t.Errorf("MoveToTop: selected = %d, want 0", sb.Selected())
		}
	})

	t.Run("height exactly matches", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 15)
		view := sb.View()
		lines := strings.Split(view, "\n")
		if len(lines) != 15 {
			t.Errorf("line count = %d, want 15", len(lines))
		}
	})

	t.Run("spam shows unread count 12", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		view := sb.View()
		plain := stripANSI(view)
		lines := strings.Split(plain, "\n")
		spamLine := findLineContaining(lines, "Spam")
		if spamLine == "" {
			t.Fatal("no line containing Spam")
		}
		if !strings.Contains(spamLine, "12") {
			t.Errorf("Spam line missing unread count 12: %q", spamLine)
		}
	})

	t.Run("selected icon tracks selection", func(t *testing.T) {
		sb := NewSidebar(styles, folders, 30, 20)
		if sb.SelectedIcon() != "󰇰" {
			t.Errorf("SelectedIcon() = %q, want inbox icon", sb.SelectedIcon())
		}
		sb.MoveDown()
		if sb.SelectedIcon() != "󰏫" {
			t.Errorf("SelectedIcon() after MoveDown = %q, want drafts icon", sb.SelectedIcon())
		}
	})

	t.Run("empty folders returns empty view", func(t *testing.T) {
		sb := NewSidebar(styles, nil, 30, 20)
		if sb.View() != "" {
			t.Error("expected empty view for nil folders")
		}
	})
}

func mockFolders() []mail.Folder {
	return []mail.Folder{
		{Name: "Inbox", Exists: 10, Unseen: 3, Role: "inbox"},
		{Name: "Drafts", Exists: 2, Unseen: 0, Role: "drafts"},
		{Name: "Sent", Exists: 145, Unseen: 0, Role: "sent"},
		{Name: "Archive", Exists: 1893, Unseen: 0, Role: "archive"},
		{Name: "Spam", Exists: 12, Unseen: 12, Role: "junk"},
		{Name: "Trash", Exists: 5, Unseen: 0, Role: "trash"},
		{Name: "Notifications", Exists: 47, Unseen: 0, Role: ""},
		{Name: "Remind", Exists: 8, Unseen: 0, Role: ""},
		{Name: "Lists/golang", Exists: 234, Unseen: 0, Role: ""},
		{Name: "Lists/rust", Exists: 89, Unseen: 0, Role: ""},
	}
}

func findLineContaining(lines []string, substr string) string {
	for _, line := range lines {
		if strings.Contains(line, substr) {
			return line
		}
	}
	return ""
}

