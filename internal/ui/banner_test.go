package ui

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

const bannerTestWidth = 100

// TestRenderBanner_ShowsGlyphMessageAndDismissHint proves the
// exemplar's own banner_strip composition: the warn glyph, the
// banner's own message, and "Esc dismiss" plus the dismiss glyph
// right-aligned.
func TestRenderBanner_ShowsGlyphMessageAndDismissHint(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	banner := Banner{Active: true, Message: "No keyring found, so your token is stored in a plain file."}

	got := ansi.Strip(renderBanner(banner, th, bannerTestWidth))
	if !strings.Contains(got, th.Glyphs().WarnMarker) {
		t.Errorf("renderBanner() = %q, want the warn marker glyph", got)
	}
	if !strings.Contains(got, banner.Message) {
		t.Errorf("renderBanner() = %q, want the banner's own message", got)
	}
	if !strings.Contains(got, "Esc dismiss") {
		t.Errorf("renderBanner() = %q, want the dismiss hint", got)
	}
	if !strings.Contains(got, th.Glyphs().Dismiss) {
		t.Errorf("renderBanner() = %q, want the dismiss glyph", got)
	}
}

// TestRenderBanner_ExactWidth mirrors the status line and footer's own
// width-invariant test: the rendered row's display width is exactly
// width, a long message truncated with the ellipsis token rather than
// overflowing.
func TestRenderBanner_ExactWidth(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	long := Banner{Active: true, Message: strings.Repeat("a very long banner message ", 10)}

	for _, width := range []int{60, 80, 100, 120} {
		got := ansi.StringWidth(ansi.Strip(renderBanner(long, th, width)))
		if got != width {
			t.Errorf("renderBanner(%d) display width = %d, want %d", width, got, width)
		}
	}
}

// TestBannerHitSpans_TargetsTheDismissGlyph proves ADR-0017's
// character grain: the returned span's Rect lands exactly on the
// dismiss glyph's own rendered column.
func TestBannerHitSpans_TargetsTheDismissGlyph(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	banner := Banner{Active: true, Message: "No keyring found."}
	bannerRow := image.Rect(0, 1, bannerTestWidth, 2)

	row := ansi.Strip(renderBanner(banner, th, bannerTestWidth))
	spans := BannerHitSpans(banner, StateDigitsSwitch, th, bannerRow)

	if len(spans) != 1 {
		t.Fatalf("BannerHitSpans() returned %d spans, want 1", len(spans))
	}
	span := spans[0]
	if span.Target != PointerBannerDismiss {
		t.Errorf("Target = %v, want PointerBannerDismiss", span.Target)
	}
	if span.Verb.Help().Desc != GrammarKeys.Back.Help().Desc {
		t.Errorf("Verb = %v, want GrammarKeys.Back", span.Verb)
	}
	cells := []rune(row)
	x := span.Rect.Min.X - bannerRow.Min.X
	want := []rune(th.Glyphs().Dismiss)[0]
	if x < 0 || x >= len(cells) || cells[x] != want {
		t.Errorf("span column %d = %q, want the dismiss glyph %q\nrow: %q", x, string(cells[x]), want, row)
	}
}

// TestBannerHitSpans_NoneWhenInactive proves an inactive banner offers
// no dismiss span to click.
func TestBannerHitSpans_NoneWhenInactive(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	bannerRow := image.Rect(0, 1, bannerTestWidth, 2)

	if got := BannerHitSpans(Banner{}, StateDigitsSwitch, th, bannerRow); got != nil {
		t.Errorf("BannerHitSpans(inactive) = %v, want none", got)
	}
}

// TestApp_BannerMsgShowsBanner proves App absorbs BannerMsg into its
// own model state and grows the layout a banner row (recomputeLayout).
func TestApp_BannerMsgShowsBanner(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))

	updated, _ := app.Update(BannerMsg{Message: "No keyring found."})
	app = mustApp(t, updated)

	if !app.banner.Active || app.banner.Message != "No keyring found." {
		t.Errorf("banner = %+v, want it active with the message", app.banner)
	}
	if !app.layout.BannerRow {
		t.Error("layout.BannerRow = false after BannerMsg, want the layout regrown with a banner row")
	}
}

// TestApp_EscDismissesBannerBeforeBack proves design decision 2: the
// first Esc with a banner showing dismisses it without popping the
// stack; the next Esc behaves normally.
func TestApp_EscDismissesBannerBeforeBack(t *testing.T) {
	resetRegistry(t)
	Register[*fakeModal](ScreenEntry{SwitchState: StateModal})

	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(BannerMsg{Message: "No keyring found."})))
	app.stack = append(app.stack, &fakeModal{})

	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	app = mustApp(t, first(app.Update(esc)))
	if app.banner.Active {
		t.Fatal("banner still active after the first Esc, want it dismissed")
	}
	if len(app.stack) != 1 {
		t.Fatalf("stack length after the first Esc = %d, want 1 (the banner dismiss must not pop it)", len(app.stack))
	}

	app = mustApp(t, first(app.Update(esc)))
	if len(app.stack) != 0 {
		t.Errorf("stack length after the second Esc = %d, want 0 (ordinary back)", len(app.stack))
	}
}

// fakeEntryScreen is a stubbed StatePrintableEntry Screen (the brief's
// own "stubbed entry-state flag", since pass 2 registers no real
// text-entry screen yet): pushed onto the stack, it stands in for a
// focused text-entry context.
type fakeEntryScreen struct{}

func (f *fakeEntryScreen) Init() tea.Cmd { return nil }

func (f *fakeEntryScreen) Update(tea.Msg) (tea.Model, tea.Cmd) { return f, nil }

func (f *fakeEntryScreen) View() tea.View { return tea.NewView("entry") }

func (f *fakeEntryScreen) Entry() ScreenEntry {
	return ScreenEntry{SwitchState: StatePrintableEntry}
}

// TestApp_FocusedTextEntryConsumesNoBannerKeypress proves a focused
// text-entry context's own Esc reaches its normal back handling
// untouched, rather than being consumed by the banner: the banner
// stays showing, and the stack pops exactly as it would with no
// banner active at all.
func TestApp_FocusedTextEntryConsumesNoBannerKeypress(t *testing.T) {
	resetRegistry(t)
	Register[*fakeEntryScreen](ScreenEntry{SwitchState: StatePrintableEntry})

	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})))
	app = mustApp(t, first(app.Update(BannerMsg{Message: "No keyring found."})))
	app.stack = append(app.stack, &fakeEntryScreen{})

	app = mustApp(t, first(app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})))

	if !app.banner.Active {
		t.Error("banner dismissed by Esc while a text-entry context was in front, want it left showing")
	}
	if len(app.stack) != 0 {
		t.Errorf("stack length after Esc reached the entry context = %d, want 0 (ordinary back popped it)", len(app.stack))
	}
}
