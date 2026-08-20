package ui

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/key"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/theme"
)

// Surface identifies one of poplar's four unified surfaces (C6,
// design language section 8): the four digit destinations.
type Surface int

// The four surfaces, in surface-switch digit order.
const (
	SurfaceMail Surface = iota
	SurfaceCalendar
	SurfaceContacts
	SurfaceConfig
)

// LayoutMsg reports App's current LayoutMode to every surface screen,
// recomputed once per tea.WindowSizeMsg (design language section 9):
// the one struct every component consumes rather than deriving its
// own chrome margins.
type LayoutMsg struct {
	Layout LayoutMode
}

// ThemeMsg reports App's current theme to every surface screen,
// resent whenever the runtime theme changes (technical design section
// 12's repaint path). Each screen is constructed with the theme
// App.NewApp starts on, so this message is what carries a later
// tea.BackgroundColorMsg answer's rebuilt theme to a screen that is
// not currently active.
type ThemeMsg struct {
	Theme theme.Theme
}

// placeholderKeys is the keymap every surface placeholder shares
// (wireframe F1): quit is the only legal key of the screen's own.
// Help's hint is pinned by the footer itself, never a screen's own
// FooterPriority, and the surface digits are App's own global
// concern, never a screen's.
type placeholderKeys struct{}

func (placeholderKeys) ShortHelp() []key.Binding  { return []key.Binding{GrammarKeys.Quit} }
func (placeholderKeys) FullHelp() [][]key.Binding { return [][]key.Binding{{GrammarKeys.Quit}} }

// placeholderFooterPriority is the committed footer hint order every
// surface placeholder shares (decision 8): quit, the only hint beyond
// the eternally pinned "? help".
var placeholderFooterPriority = []key.Binding{GrammarKeys.Quit}

// composePlaceholder centers title and facts within layout's content
// pane, on the base ground (decision 11: base carries every content
// pane). facts is empty while a load Cmd has not yet returned, or on
// a surface with no facts to show. Every surface placeholder shares
// this composition (wireframe F1/F8).
func composePlaceholder(th theme.Theme, layout LayoutMode, title, facts string) tea.View {
	rect := layout.Content().Rect
	width, height := rect.Dx(), rect.Dy()

	body := th.TypeStyle(theme.TypeTitle, theme.GroundBase).Render(title)
	if facts != "" {
		body += "\n\n" + th.TypeStyle(theme.TypeValue, theme.GroundBase).Render(facts)
	}

	block := th.Center(th.Style(theme.RoleFg, theme.GroundBase).Width(width).Height(height)).Render(body)
	return tea.NewView(block)
}

// formatCount renders n grouped by thousands ("36,102"): the one
// formatter every screen that shows a raw store count uses (F11), so
// a grouped count never drifts screen to screen. The wireframes
// render every count grouped; the placeholders are its first callers,
// and the status line (a later task) is its next.
func formatCount(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")

	var grouped strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(r)
	}

	if neg {
		return "-" + grouped.String()
	}
	return grouped.String()
}

// MailPlaceholder is the mail surface's pass-2 placeholder (wireframe
// F1): the surface name and live message/mailbox counts, read
// through the store's read pool. Pass 3 replaces it with the real
// mail list.
type MailPlaceholder struct {
	store  *store.ReadPool
	theme  theme.Theme
	layout LayoutMode
	stats  store.MailStats
	loaded bool
}

func newMailPlaceholder(reads *store.ReadPool, th theme.Theme) MailPlaceholder {
	return MailPlaceholder{store: reads, theme: th}
}

// NewMailPlaceholder returns a MailPlaceholder already carrying
// stats, as if its own store load had already returned. It is the
// fixtures package's own entry point: a fixture has no
// store.ReadPool to load from (amendment D's no-I/O rule), so it
// pins its facts here instead of through Init's Cmd.
func NewMailPlaceholder(th theme.Theme, stats store.MailStats) MailPlaceholder {
	return MailPlaceholder{theme: th, stats: stats, loaded: true}
}

type mailStatsMsg struct {
	stats store.MailStats
	err   error
}

func (m MailPlaceholder) Init() tea.Cmd {
	reads := m.store
	return func() tea.Msg {
		stats, err := reads.MailStats(context.Background())
		return mailStatsMsg{stats: stats, err: err}
	}
}

func (m MailPlaceholder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	return next, cmd
}

// update's Cmd result is always nil until task 6's StoreChangedMsg
// refresh gives it one to return; the signature carries the contract
// now so that task's change is additive.
//
//nolint:unparam // Cmd stays nil until task 6's StoreChangedMsg refresh
func (m MailPlaceholder) update(msg tea.Msg) (MailPlaceholder, tea.Cmd) {
	switch msg := msg.(type) {
	case LayoutMsg:
		m.layout = msg.Layout
	case ThemeMsg:
		m.theme = msg.Theme
	case mailStatsMsg:
		if msg.err != nil {
			slog.Warn("mail placeholder: load store counts", "error", msg.err)
			return m, nil
		}
		m.stats, m.loaded = msg.stats, true
	}
	return m, nil
}

func (m MailPlaceholder) View() tea.View {
	facts := ""
	if m.loaded {
		facts = fmt.Sprintf("%s messages in %s folders", formatCount(m.stats.Messages), formatCount(m.stats.Mailboxes))
	}
	return composePlaceholder(m.theme, m.layout, "Mail", facts)
}

// Entry implements Screen.
func (MailPlaceholder) Entry() ScreenEntry { return mailPlaceholderEntry() }

func mailPlaceholderEntry() ScreenEntry {
	return ScreenEntry{
		Name:           "mail list",
		Keys:           placeholderKeys{},
		FooterPriority: placeholderFooterPriority,
		SwitchState:    StateDigitsSwitch,
	}
}

func init() {
	Register[MailPlaceholder](mailPlaceholderEntry())
}

// CalendarPlaceholder is the calendar surface's pass-2 placeholder
// (wireframe F8): the surface name and the live event count, read
// through the store's read pool.
type CalendarPlaceholder struct {
	store  *store.ReadPool
	theme  theme.Theme
	layout LayoutMode
	events int64
	loaded bool
}

func newCalendarPlaceholder(reads *store.ReadPool, th theme.Theme) CalendarPlaceholder {
	return CalendarPlaceholder{store: reads, theme: th}
}

// NewCalendarPlaceholder mirrors NewMailPlaceholder for the calendar
// surface's own fact.
func NewCalendarPlaceholder(th theme.Theme, events int64) CalendarPlaceholder {
	return CalendarPlaceholder{theme: th, events: events, loaded: true}
}

type eventCountMsg struct {
	count int64
	err   error
}

func (c CalendarPlaceholder) Init() tea.Cmd {
	reads := c.store
	return func() tea.Msg {
		count, err := reads.EventCount(context.Background())
		return eventCountMsg{count: count, err: err}
	}
}

func (c CalendarPlaceholder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := c.update(msg)
	return next, cmd
}

//nolint:unparam // Cmd stays nil until task 6's StoreChangedMsg refresh
func (c CalendarPlaceholder) update(msg tea.Msg) (CalendarPlaceholder, tea.Cmd) {
	switch msg := msg.(type) {
	case LayoutMsg:
		c.layout = msg.Layout
	case ThemeMsg:
		c.theme = msg.Theme
	case eventCountMsg:
		if msg.err != nil {
			slog.Warn("calendar placeholder: load event count", "error", msg.err)
			return c, nil
		}
		c.events, c.loaded = msg.count, true
	}
	return c, nil
}

func (c CalendarPlaceholder) View() tea.View {
	facts := ""
	if c.loaded {
		facts = fmt.Sprintf("%s events", formatCount(c.events))
	}
	return composePlaceholder(c.theme, c.layout, "Calendar", facts)
}

// Entry implements Screen.
func (CalendarPlaceholder) Entry() ScreenEntry { return calendarPlaceholderEntry() }

func calendarPlaceholderEntry() ScreenEntry {
	return ScreenEntry{
		Name:           "calendar agenda",
		Keys:           placeholderKeys{},
		FooterPriority: placeholderFooterPriority,
		SwitchState:    StateDigitsSwitch,
	}
}

func init() {
	Register[CalendarPlaceholder](calendarPlaceholderEntry())
}

// ContactsPlaceholder is the contacts surface's pass-2 placeholder
// (wireframe F8, "People"). It states plainly that no contacts read
// surface exists yet (a plan non-goal) rather than growing one; pass
// 5 wires the real store surface.
type ContactsPlaceholder struct {
	theme  theme.Theme
	layout LayoutMode
}

func newContactsPlaceholder(th theme.Theme) ContactsPlaceholder {
	return ContactsPlaceholder{theme: th}
}

func (c ContactsPlaceholder) Init() tea.Cmd { return nil }

func (c ContactsPlaceholder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := c.update(msg)
	return next, cmd
}

//nolint:unparam // Cmd stays nil until task 6's StoreChangedMsg refresh
func (c ContactsPlaceholder) update(msg tea.Msg) (ContactsPlaceholder, tea.Cmd) {
	switch msg := msg.(type) {
	case LayoutMsg:
		c.layout = msg.Layout
	case ThemeMsg:
		c.theme = msg.Theme
	}
	return c, nil
}

func (c ContactsPlaceholder) View() tea.View {
	// "People" is wireframe F8's own reviewed copy: the state name
	// ("contact list", ScreenEntry.Name below) and the display title
	// are deliberately different fields.
	return composePlaceholder(c.theme, c.layout, "People", "contacts sync lands with pass 5")
}

// Entry implements Screen.
func (ContactsPlaceholder) Entry() ScreenEntry { return contactsPlaceholderEntry() }

func contactsPlaceholderEntry() ScreenEntry {
	return ScreenEntry{
		Name:           "contact list",
		Keys:           placeholderKeys{},
		FooterPriority: placeholderFooterPriority,
		SwitchState:    StateDigitsSwitch,
	}
}

func init() {
	Register[ContactsPlaceholder](contactsPlaceholderEntry())
}

// ConfigPlaceholder is the config surface's pass-2 placeholder
// (wireframe F8). The real config surface (ST-3's alpine-style setup
// screens) lands with pass 2b.
type ConfigPlaceholder struct {
	theme  theme.Theme
	layout LayoutMode
}

func newConfigPlaceholder(th theme.Theme) ConfigPlaceholder {
	return ConfigPlaceholder{theme: th}
}

func (c ConfigPlaceholder) Init() tea.Cmd { return nil }

func (c ConfigPlaceholder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := c.update(msg)
	return next, cmd
}

//nolint:unparam // Cmd stays nil until task 6's StoreChangedMsg refresh
func (c ConfigPlaceholder) update(msg tea.Msg) (ConfigPlaceholder, tea.Cmd) {
	switch msg := msg.(type) {
	case LayoutMsg:
		c.layout = msg.Layout
	case ThemeMsg:
		c.theme = msg.Theme
	}
	return c, nil
}

func (c ConfigPlaceholder) View() tea.View {
	return composePlaceholder(c.theme, c.layout, "Config", "the config surface lands with pass 2b")
}

// Entry implements Screen.
func (ConfigPlaceholder) Entry() ScreenEntry { return configPlaceholderEntry() }

func configPlaceholderEntry() ScreenEntry {
	return ScreenEntry{
		Name:           "config sections",
		Keys:           placeholderKeys{},
		FooterPriority: placeholderFooterPriority,
		SwitchState:    StateDigitsSwitch,
	}
}

func init() {
	Register[ConfigPlaceholder](configPlaceholderEntry())
}
