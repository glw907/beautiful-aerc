package ui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/theme"
)

func testTheme() theme.Theme {
	return theme.New(true, theme.ProfileTrueColor)
}

func testLayoutMsg() LayoutMsg {
	return LayoutMsg{Layout: ComputeLayout(100, 30, false)}
}

// TestMailPlaceholder_InitLoadsStoreCounts proves the Init-time load:
// Init returns a Cmd that reads the store's read pool, and Update
// absorbs the result into the composed view.
func TestMailPlaceholder_InitLoadsStoreCounts(t *testing.T) {
	reads := storetest.OpenReadPool(t, store.DefaultWriterConfig())
	ctx := context.Background()
	if _, err := reads.MailStats(ctx); err != nil {
		t.Fatalf("MailStats on an empty store: %v", err)
	}

	m := newMailPlaceholder(reads, testTheme())
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned a nil Cmd, want the store-count load")
	}

	msg, ok := cmd().(mailStatsMsg)
	if !ok {
		t.Fatalf("Init()'s Cmd yielded %#v, want mailStatsMsg", msg)
	}
	if msg.err != nil {
		t.Fatalf("mailStatsMsg.err = %v, want nil", msg.err)
	}

	m = m.update(msg)
	m = m.update(testLayoutMsg())
	if !m.loaded {
		t.Fatal("mail placeholder did not mark itself loaded")
	}

	got := m.View().Content
	if !strings.Contains(got, "Mail") {
		t.Errorf("view = %q, want it to contain the surface name %q", got, "Mail")
	}
	if !strings.Contains(got, "0 messages in 0 folders") {
		t.Errorf("view = %q, want the empty store's counts", got)
	}
}

// TestMailPlaceholder_ViewBeforeLoadOmitsFacts proves the view before
// any load result has arrived shows the surface name alone, never a
// zero-value fact line that would misstate the store.
func TestMailPlaceholder_ViewBeforeLoadOmitsFacts(t *testing.T) {
	reads := storetest.OpenReadPool(t, store.DefaultWriterConfig())
	m := newMailPlaceholder(reads, testTheme())
	m = m.update(testLayoutMsg())

	got := m.View().Content
	if !strings.Contains(got, "Mail") {
		t.Errorf("view = %q, want it to contain the surface name %q", got, "Mail")
	}
	if strings.Contains(got, "messages in") {
		t.Errorf("view = %q, want no fact line before the load Cmd returns", got)
	}
}

// TestCalendarPlaceholder_InitLoadsEventCount mirrors the mail
// placeholder's Init-time load for the calendar surface's own fact.
func TestCalendarPlaceholder_InitLoadsEventCount(t *testing.T) {
	reads := storetest.OpenReadPool(t, store.DefaultWriterConfig())
	c := newCalendarPlaceholder(reads, testTheme())

	msg, ok := c.Init()().(eventCountMsg)
	if !ok {
		t.Fatalf("Init()'s Cmd yielded %#v, want eventCountMsg", msg)
	}
	if msg.err != nil {
		t.Fatalf("eventCountMsg.err = %v, want nil", msg.err)
	}

	c = c.update(msg)
	c = c.update(testLayoutMsg())
	got := c.View().Content
	if !strings.Contains(got, "Calendar") || !strings.Contains(got, "0 events") {
		t.Errorf("view = %q, want the surface name and the empty store's event count", got)
	}
}

// TestContactsPlaceholder_StatesNoStoreSurfacePlainly proves the
// contacts placeholder's own non-goal: it names the absence of a
// contacts read surface rather than growing one.
func TestContactsPlaceholder_StatesNoStoreSurfacePlainly(t *testing.T) {
	c := newContactsPlaceholder(testTheme())
	c = c.update(testLayoutMsg())

	got := c.View().Content
	if !strings.Contains(got, "Contacts") {
		t.Errorf("view = %q, want it to contain the surface name %q", got, "Contacts")
	}
	if !strings.Contains(got, "pass 5") {
		t.Errorf("view = %q, want it to name when contacts sync lands", got)
	}
}

// TestConfigPlaceholder_NamesWhenItLands mirrors the contacts case for
// the config surface's own 2b non-goal.
func TestConfigPlaceholder_NamesWhenItLands(t *testing.T) {
	c := newConfigPlaceholder(testTheme())
	c = c.update(testLayoutMsg())

	got := c.View().Content
	if !strings.Contains(got, "Config") {
		t.Errorf("view = %q, want it to contain the surface name %q", got, "Config")
	}
	if !strings.Contains(got, "pass 2b") {
		t.Errorf("view = %q, want it to name when the config surface lands", got)
	}
}

// TestPlaceholderScreens_RegisteredWithTheirOwnSwitchState proves all
// four surface placeholders register as StateDigitsSwitch (F8: "each
// is a registered screen so UX-4's round trip and the grammar test
// cover all four surfaces from this pass on").
func TestPlaceholderScreens_RegisteredWithTheirOwnSwitchState(t *testing.T) {
	tests := []struct {
		name  string
		entry ScreenEntry
	}{
		{"mail", mailPlaceholderEntry()},
		{"calendar", calendarPlaceholderEntry()},
		{"contacts", contactsPlaceholderEntry()},
		{"config", configPlaceholderEntry()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.entry.SwitchState != StateDigitsSwitch {
				t.Errorf("%s: SwitchState = %v, want StateDigitsSwitch", tt.name, tt.entry.SwitchState)
			}
			if tt.entry.Name == "" {
				t.Errorf("%s: Name is empty", tt.name)
			}
		})
	}
}

// TestScreen_TypeAssertion proves every placeholder type satisfies
// the Screen interface, the registry's own registration contract.
func TestScreen_TypeAssertion(t *testing.T) {
	var _ Screen = MailPlaceholder{}
	var _ Screen = CalendarPlaceholder{}
	var _ Screen = ContactsPlaceholder{}
	var _ Screen = ConfigPlaceholder{}
	var _ tea.Model = MailPlaceholder{}
}
