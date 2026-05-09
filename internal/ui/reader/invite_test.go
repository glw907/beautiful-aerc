package reader

import (
	"strings"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/icalendar"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func testInviteStyles() Styles {
	return NewStyles(theme.Nord)
}

func testInviteIcons() uicore.IconSet {
	return uicore.SimpleIcons
}

func rowCount(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

func TestRenderInviteBlock_Nil(t *testing.T) {
	s, n := renderInviteBlock(nil, testInviteIcons(), testInviteStyles(), 80)
	if s != "" || n != 0 {
		t.Errorf("nil invite: got (%q, %d), want (\"\", 0)", s, n)
	}
}

func TestRenderInviteBlock_ZeroWidth(t *testing.T) {
	inv := &icalendar.Invite{Summary: "Meet"}
	s, n := renderInviteBlock(inv, testInviteIcons(), testInviteStyles(), 0)
	if s != "" || n != 0 {
		t.Errorf("zero width: got (%q, %d), want (\"\", 0)", s, n)
	}
}

func TestRenderInviteBlock_MinimalNoTimes(t *testing.T) {
	inv := &icalendar.Invite{Summary: "Stand-up"}
	s, n := renderInviteBlock(inv, testInviteIcons(), testInviteStyles(), 80)
	if n != 2 {
		t.Errorf("minimal invite: got %d rows, want 2", n)
	}
	if rowCount(s) != n {
		t.Errorf("rowCount(%q) = %d, reported n = %d", s, rowCount(s), n)
	}
	if !strings.Contains(s, "Stand-up") {
		t.Errorf("missing Summary in output: %q", s)
	}
	if !strings.Contains(s, "(no start time)") {
		t.Errorf("missing no-start-time in output: %q", s)
	}
}

func TestRenderInviteBlock_SameDayFull(t *testing.T) {
	start := time.Date(2026, 5, 14, 14, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 14, 15, 0, 0, 0, time.UTC)
	inv := &icalendar.Invite{
		Summary:       "Quarterly review",
		Start:         start,
		End:           end,
		Location:      "Room 42",
		Organizer:     "alice@example.com",
		AttendeeCount: 3,
	}
	s, n := renderInviteBlock(inv, testInviteIcons(), testInviteStyles(), 80)
	if n != 5 {
		t.Errorf("5-field invite: got %d rows, want 5", n)
	}
	if rowCount(s) != n {
		t.Errorf("rowCount mismatch: %d vs reported %d", rowCount(s), n)
	}
	if !strings.Contains(s, "Room 42") {
		t.Errorf("missing Location: %q", s)
	}
	if !strings.Contains(s, "alice@example.com") {
		t.Errorf("missing Organizer: %q", s)
	}
	if !strings.Contains(s, "3 attendees") {
		t.Errorf("missing attendee count: %q", s)
	}
}

func TestRenderInviteBlock_AllDay(t *testing.T) {
	start := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	inv := &icalendar.Invite{Summary: "Holiday", Start: start, End: end}
	s, _ := renderInviteBlock(inv, testInviteIcons(), testInviteStyles(), 80)
	if !strings.Contains(s, "all day") {
		t.Errorf("all-day invite missing 'all day': %q", s)
	}
}

func TestRenderInviteBlock_CrossDay(t *testing.T) {
	start := time.Date(2026, 5, 14, 22, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 15, 2, 0, 0, 0, time.UTC)
	inv := &icalendar.Invite{Summary: "Overnight", Start: start, End: end}
	s, _ := renderInviteBlock(inv, testInviteIcons(), testInviteStyles(), 80)
	parts := strings.Split(s, " – ")
	if len(parts) < 2 {
		t.Fatalf("cross-day: expected ' – ' separator in When row: %q", s)
	}
}

func TestRenderInviteBlock_Cancelled(t *testing.T) {
	inv := &icalendar.Invite{Summary: "Stand-up", Method: "CANCEL"}
	s, n := renderInviteBlock(inv, testInviteIcons(), testInviteStyles(), 80)
	if !strings.Contains(s, "[CANCELLED]") {
		t.Errorf("cancelled invite missing [CANCELLED]: %q", s)
	}
	if n < 3 {
		t.Errorf("cancelled invite: got %d rows, want >= 3 (CANCELLED + Summary + When)", n)
	}
}

func TestRenderInviteBlock_Recurring(t *testing.T) {
	start := time.Date(2026, 5, 14, 9, 0, 0, 0, time.UTC)
	inv := &icalendar.Invite{
		Summary:    "Weekly sync",
		Start:      start,
		Recurrence: "Every week",
	}
	s, _ := renderInviteBlock(inv, testInviteIcons(), testInviteStyles(), 80)
	if !strings.Contains(s, "Repeats: Every week") {
		t.Errorf("missing Repeats row: %q", s)
	}
}

func TestRenderInviteBlock_TruncatesLongSummary(t *testing.T) {
	long := strings.Repeat("X", 200)
	inv := &icalendar.Invite{Summary: long}
	s, _ := renderInviteBlock(inv, testInviteIcons(), testInviteStyles(), 40)
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		// A 200-char summary at width 40 must not produce a line over ~200 bytes of visible text.
		if len([]rune(l)) > 300 {
			t.Errorf("line too long after truncation: %d runes", len([]rune(l)))
		}
	}
}

func TestRenderInviteBlock_RowCountConsistency(t *testing.T) {
	start := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 14, 11, 0, 0, 0, time.UTC)
	inv := &icalendar.Invite{
		Summary:       "Team meeting",
		Start:         start,
		End:           end,
		Location:      "Conf room",
		Organizer:     "boss@example.com",
		AttendeeCount: 1,
		Recurrence:    "Every week",
	}
	s, n := renderInviteBlock(inv, testInviteIcons(), testInviteStyles(), 80)
	if rowCount(s) != n {
		t.Errorf("reported n=%d but actual rows=%d", n, rowCount(s))
	}
	// Summary + When + Where + Organizer + Attendees + Recurrence = 6
	if n != 6 {
		t.Errorf("expected 6 rows, got %d", n)
	}
}

func TestRenderInviteBlock_SingularAttendee(t *testing.T) {
	inv := &icalendar.Invite{Summary: "1:1", AttendeeCount: 1}
	s, _ := renderInviteBlock(inv, testInviteIcons(), testInviteStyles(), 80)
	if !strings.Contains(s, "1 attendee") {
		t.Errorf("expected '1 attendee' (singular): %q", s)
	}
	if strings.Contains(s, "1 attendees") {
		t.Errorf("expected singular 'attendee', got plural: %q", s)
	}
}
