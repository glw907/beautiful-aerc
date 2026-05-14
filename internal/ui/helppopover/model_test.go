package helppopover

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/theme"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func TestHelpPopover_AccountGroupsCoverage(t *testing.T) {
	wantGroups := []string{
		"Navigate", "Triage", "Reply",
		"Search", "Select", "Threads",
		"Go To",
	}
	if len(accountGroups) != len(wantGroups) {
		t.Fatalf("accountGroups: got %d groups, want %d",
			len(accountGroups), len(wantGroups))
	}
	for i, want := range wantGroups {
		if accountGroups[i].title != want {
			t.Errorf("accountGroups[%d].title = %q, want %q",
				i, accountGroups[i].title, want)
		}
	}
}

func TestHelpPopover_ViewerGroupsCoverage(t *testing.T) {
	wantGroups := []string{"Navigate", "Triage", "Reply"}
	if len(viewerGroups) != len(wantGroups) {
		t.Fatalf("viewerGroups: got %d groups, want %d",
			len(viewerGroups), len(wantGroups))
	}
	for i, want := range wantGroups {
		if viewerGroups[i].title != want {
			t.Errorf("viewerGroups[%d].title = %q, want %q",
				i, viewerGroups[i].title, want)
		}
	}
}

func TestHelpPopover_AccountViewContent(t *testing.T) {
	styles := NewStyles(theme.Nord)
	h := New(styles, Account)

	view := stripANSI(h.SetSize(80, 24).View())

	if !strings.Contains(view, "Message List") {
		t.Error("account popover: missing title 'Message List'")
	}

	for _, want := range []string{
		"Navigate", "Triage", "Reply",
		"Search", "Select", "Threads", "Go To",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("account popover: missing group heading %q", want)
		}
	}

	for _, want := range []string{
		"j/k", "up/down",
		"d", "delete",
		"r", "reply",
		"/", "search",
		"v", "select",
		"F", "fold all",
		"I", "inbox", "T", "trash",
		"Enter", "open", "?", "close",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("account popover: missing %q", want)
		}
	}

	for _, want := range []string{"╭", "╮", "╰", "╯"} {
		if !strings.Contains(view, want) {
			t.Errorf("account popover: missing border char %q", want)
		}
	}
}

func TestHelpPopover_ViewerViewContent(t *testing.T) {
	styles := NewStyles(theme.Nord)
	h := New(styles, Viewer)

	view := stripANSI(h.SetSize(80, 24).View())

	if !strings.Contains(view, "Message Viewer") {
		t.Error("viewer popover: missing title 'Message Viewer'")
	}

	for _, want := range []string{
		"j/k", "scroll",
		"␣/b", "page d/u",
		"1-9", "open link",
		"Tab", "link picker",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("viewer popover: missing %q", want)
		}
	}

	// Account-only groups must NOT appear.
	for _, missing := range []string{"Search", "Select", "Threads", "Go To"} {
		if strings.Contains(view, missing) {
			t.Errorf("viewer popover: should not contain %q", missing)
		}
	}

	for _, want := range []string{"╭", "╮", "╰", "╯"} {
		if !strings.Contains(view, want) {
			t.Errorf("viewer popover: missing border char %q", want)
		}
	}
}

func TestHelpPopover_Styles(t *testing.T) {
	styles := NewStyles(theme.Nord)

	if !styles.HelpKey.GetBold() {
		t.Error("HelpKey style must be bold")
	}
	if styles.Dim.GetBold() {
		t.Error("Dim style must not be bold")
	}
	if !styles.HelpGroupHeader.GetBold() {
		t.Error("HelpGroupHeader style must be bold")
	}

	view := stripANSI(New(styles, Account).SetSize(120, 30).View())
	for _, want := range []string{"j/k", "up/down", "d", "delete", "Reply"} {
		if !strings.Contains(view, want) {
			t.Errorf("account popover missing %q", want)
		}
	}
}

func TestPopoverFiltersGatedBindings(t *testing.T) {
	styles := NewStyles(theme.Nord)

	var caps tea.KeyboardEnhancementsMsg // protocol absent (Flags == 0)
	m := New(styles, Compose).WithKbdCaps(caps).SetSize(80, 40)
	out := stripANSI(m.View())
	if strings.Contains(out, "^I") || strings.Contains(out, "italic") {
		t.Fatalf("gated chord rendered with protocol absent:\n%s", out)
	}

	caps.Flags = 1 // kittyDisambiguateEscapeCodes bit
	m = New(styles, Compose).WithKbdCaps(caps).SetSize(80, 40)
	out = stripANSI(m.View())
	if !strings.Contains(out, "italic") {
		t.Fatalf("gated chord missing with protocol active:\n%s", out)
	}
}

// BenchmarkHelpPopoverBox_Cold measures the full Box rebuild cost (dirty cache).
// One new Model per iteration.
func BenchmarkHelpPopoverBox_Cold(b *testing.B) {
	styles := NewStyles(theme.Nord)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h := New(styles, Account)
		_, _ = h.Box(120, 40)
	}
}

// BenchmarkHelpPopoverBox_Warm measures the cache-hit path for Box.
// After the first call the cache is clean. Subsequent calls return the
// stored strings without rebuilding the lipgloss layout.
func BenchmarkHelpPopoverBox_Warm(b *testing.B) {
	styles := NewStyles(theme.Nord)
	h := New(styles, Account)
	_, _ = h.Box(120, 40) // warm the cache
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = h.Box(120, 40)
	}
}

// BenchmarkHelpPopoverView measures the cost of repeated View calls.
// View still calls lipgloss.Place on each call. The cache saves the
// expensive Box rebuild (lipgloss layout + string assembly).
func BenchmarkHelpPopoverView(b *testing.B) {
	styles := NewStyles(theme.Nord)
	h := New(styles, Account).SetSize(120, 40)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = h.SetSize(120, 40).View()
	}
}

// TestHelpPopover_VerticallyCentered locks in the F3 acceptance: the
// popover's blank-row margins above and below the box are equal (±1).
// Prior regression rendered the box pinned ~1 row from the top.
func TestHelpPopover_VerticallyCentered(t *testing.T) {
	styles := NewStyles(theme.Nord)
	for _, dims := range []struct{ w, h int }{{120, 30}, {120, 40}, {160, 50}} {
		view := stripANSI(New(styles, Account).SetSize(dims.w, dims.h).View())
		lines := strings.Split(view, "\n")
		var first, last int = -1, -1
		for i, ln := range lines {
			if strings.TrimSpace(ln) != "" {
				if first < 0 {
					first = i
				}
				last = i
			}
		}
		if first < 0 {
			t.Fatalf("%dx%d: empty view", dims.w, dims.h)
		}
		topBlank := first
		botBlank := len(lines) - 1 - last
		if diff := topBlank - botBlank; diff < -1 || diff > 1 {
			t.Errorf("%dx%d: top blank %d vs bottom blank %d (diff %d, want |diff|<=1)",
				dims.w, dims.h, topBlank, botBlank, diff)
		}
	}
}
