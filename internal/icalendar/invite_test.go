package icalendar

import (
	"errors"
	"os"
	"testing"
	"time"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestParseInvite_GoogleRequest(t *testing.T) {
	inv, err := ParseInvite(readFixture(t, "google_request.ics"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Summary != "Team Sync" {
		t.Errorf("Summary = %q, want %q", inv.Summary, "Team Sync")
	}
	if inv.Method != "REQUEST" {
		t.Errorf("Method = %q, want REQUEST", inv.Method)
	}
	if inv.Location != "Google Meet" {
		t.Errorf("Location = %q, want %q", inv.Location, "Google Meet")
	}
	if inv.Organizer != "Alice Smith" {
		t.Errorf("Organizer = %q, want %q", inv.Organizer, "Alice Smith")
	}
	if inv.AttendeeCount != 2 {
		t.Errorf("AttendeeCount = %d, want 2", inv.AttendeeCount)
	}
	want := time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC)
	if !inv.Start.Equal(want) {
		t.Errorf("Start = %v, want %v", inv.Start, want)
	}
	wantEnd := time.Date(2026, 5, 15, 15, 0, 0, 0, time.UTC)
	if !inv.End.Equal(wantEnd) {
		t.Errorf("End = %v, want %v", inv.End, wantEnd)
	}
}

func TestParseInvite_OutlookTZID(t *testing.T) {
	inv, err := ParseInvite(readFixture(t, "outlook_request_tzid.ics"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Summary != "Quarterly Review" {
		t.Errorf("Summary = %q, want %q", inv.Summary, "Quarterly Review")
	}
	if inv.Method != "REQUEST" {
		t.Errorf("Method = %q, want REQUEST", inv.Method)
	}
	if inv.Organizer != "Bob Jones" {
		t.Errorf("Organizer = %q, want %q", inv.Organizer, "Bob Jones")
	}
	if inv.AttendeeCount != 1 {
		t.Errorf("AttendeeCount = %d, want 1", inv.AttendeeCount)
	}
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	want := time.Date(2026, 6, 1, 9, 0, 0, 0, la)
	if !inv.Start.Equal(want) {
		t.Errorf("Start = %v, want %v", inv.Start.In(la), want)
	}
}

func TestParseInvite_RecurringWeekly(t *testing.T) {
	inv, err := ParseInvite(readFixture(t, "recurring_weekly.ics"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Recurrence != "Every week" {
		t.Errorf("Recurrence = %q, want %q", inv.Recurrence, "Every week")
	}
	if inv.Summary != "Weekly Standup" {
		t.Errorf("Summary = %q, want %q", inv.Summary, "Weekly Standup")
	}
}

func TestParseInvite_AllDay(t *testing.T) {
	inv, err := ParseInvite(readFixture(t, "all_day.ics"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Summary != "Company Holiday" {
		t.Errorf("Summary = %q, want %q", inv.Summary, "Company Holiday")
	}
	// All-day parsed as local; just check the date components.
	if inv.Start.Year() != 2026 || inv.Start.Month() != 5 || inv.Start.Day() != 14 {
		t.Errorf("Start date = %v, want 2026-05-14", inv.Start)
	}
	if inv.End.Year() != 2026 || inv.End.Month() != 5 || inv.End.Day() != 15 {
		t.Errorf("End date = %v, want 2026-05-15", inv.End)
	}
}

func TestParseInvite_Cancel(t *testing.T) {
	inv, err := ParseInvite(readFixture(t, "cancel.ics"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Method != "CANCEL" {
		t.Errorf("Method = %q, want CANCEL", inv.Method)
	}
	if inv.Organizer != "Alice Smith" {
		t.Errorf("Organizer = %q, want Alice Smith", inv.Organizer)
	}
}

func TestParseInvite_MultiEventFirstOnly(t *testing.T) {
	inv, err := ParseInvite(readFixture(t, "multi_event_publish.ics"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Method != "PUBLISH" {
		t.Errorf("Method = %q, want PUBLISH", inv.Method)
	}
	if inv.Summary != "First Event" {
		t.Errorf("Summary = %q, want %q (first event only)", inv.Summary, "First Event")
	}
	if inv.Location != "Room A" {
		t.Errorf("Location = %q, want Room A", inv.Location)
	}
}

func TestParseInvite_Malformed(t *testing.T) {
	_, err := ParseInvite(readFixture(t, "malformed.ics"))
	if !errors.Is(err, ErrNoEvent) {
		t.Errorf("err = %v, want ErrNoEvent", err)
	}
}

func TestParseInvite_Empty(t *testing.T) {
	_, err := ParseInvite(nil)
	if !errors.Is(err, ErrNoEvent) {
		t.Errorf("err = %v, want ErrNoEvent", err)
	}
	_, err = ParseInvite([]byte{})
	if !errors.Is(err, ErrNoEvent) {
		t.Errorf("empty slice: err = %v, want ErrNoEvent", err)
	}
}

func TestParseInvite_NoSummary(t *testing.T) {
	ics := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nDTSTART:20260515T140000Z\r\nDTEND:20260515T150000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	inv, err := ParseInvite(ics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Summary != "(no title)" {
		t.Errorf("Summary = %q, want %q", inv.Summary, "(no title)")
	}
}

func TestParseInvite_OrganizerMailto(t *testing.T) {
	ics := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nDTSTART:20260515T140000Z\r\nDTEND:20260515T150000Z\r\nSUMMARY:Test\r\nORGANIZER:mailto:organizer@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	inv, err := ParseInvite(ics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Organizer != "organizer@example.com" {
		t.Errorf("Organizer = %q, want %q", inv.Organizer, "organizer@example.com")
	}
}
