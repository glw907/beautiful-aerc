package reader

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/theme"
)

func newTestLinkPicker(t *testing.T) LinkPicker {
	t.Helper()
	p := NewLinkPicker(NewStyles(theme.Nord), ansix.NewMeasurer(1))
	p = p.SetSize(80, 24)
	return p
}

func TestLinkPickerOpenSetsCursor(t *testing.T) {
	p := newTestLinkPicker(t)
	links := []string{"https://a.com", "https://b.com", "https://c.com"}
	p = p.Open(links)
	if !p.IsOpen() {
		t.Fatal("picker should be open after Open()")
	}
	if p.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", p.Cursor())
	}
}

func TestLinkPickerCursorBounds(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com", "https://b.com"})

	p, _ = p.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if p.Cursor() != 0 {
		t.Fatalf("k from row 0: cursor = %d, want 0", p.Cursor())
	}
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if p.Cursor() != 1 {
		t.Fatalf("j from row 0: cursor = %d, want 1", p.Cursor())
	}
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if p.Cursor() != 1 {
		t.Fatalf("j from last row: cursor = %d, want 1", p.Cursor())
	}
}

func TestLinkPickerEnterEmitsLaunchAndClose(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com", "https://b.com"})
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	got := collectMsgs(cmd)
	if !containsLaunchURL(got, "https://b.com") {
		t.Fatalf("expected LaunchURLMsg{https://b.com}, got %v", got)
	}
	if !containsClosed(got) {
		t.Fatalf("expected LinkPickerClosedMsg, got %v", got)
	}
}

func TestLinkPickerNumericLaunchInRange(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com", "https://b.com", "https://c.com"})

	_, cmd := p.Update(tea.KeyPressMsg{Code: '2', Text: "2"})

	got := collectMsgs(cmd)
	if !containsLaunchURL(got, "https://b.com") {
		t.Fatalf("expected LaunchURLMsg{https://b.com}, got %v", got)
	}
}

func TestLinkPickerNumericOutOfRangeInert(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})

	_, cmd := p.Update(tea.KeyPressMsg{Code: '5', Text: "5"})

	if cmd != nil {
		t.Fatalf("out-of-range numeric should be inert, got cmd=%v", cmd)
	}
}

func TestLinkPickerEscCloses(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got := collectMsgs(cmd)
	if !containsClosed(got) {
		t.Fatalf("expected LinkPickerClosedMsg from Esc, got %v", got)
	}
}

func TestLinkPickerTabCloses(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	got := collectMsgs(cmd)
	if !containsClosed(got) {
		t.Fatalf("expected LinkPickerClosedMsg from Tab, got %v", got)
	}
}

func TestLinkPickerQSwallowed(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})
	p2, cmd := p.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		t.Fatalf("q should be swallowed, got cmd=%v", cmd)
	}
	if !p2.IsOpen() {
		t.Fatal("q should not close picker")
	}
}

// collectMsgs runs cmd and returns the resulting messages. Handles
// tea.Batch by walking the batch tree.
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

func containsLaunchURL(msgs []tea.Msg, url string) bool {
	for _, m := range msgs {
		if l, ok := m.(LaunchURLMsg); ok && l.URL == url {
			return true
		}
	}
	return false
}

func containsClosed(msgs []tea.Msg) bool {
	for _, m := range msgs {
		if _, ok := m.(LinkPickerClosedMsg); ok {
			return true
		}
	}
	return false
}

func TestLinkPickerRowFormatFixed(t *testing.T) {
	// Picker uses fixed "[N] " prefix (4 chars); no dynamic padding needed since
	// the digit-key surface caps at 9 items.
	links := make([]string, 9)
	for i := range links {
		links[i] = "https://a.com"
	}
	p := newTestLinkPicker(t)
	p = p.SetSize(80, 24).Open(links)
	out := p.View()
	if !strings.Contains(out, "[1]") {
		t.Fatalf("expected '[1]' in output, got:\n%s", out)
	}
	if strings.Contains(out, " [1]") {
		t.Fatalf("expected no leading-space pad in fixed-format picker, got:\n%s", out)
	}
}

func TestLinkPickerRowFormatNoPad(t *testing.T) {
	links := make([]string, 9)
	for i := range links {
		links[i] = "https://a.com"
	}
	p := newTestLinkPicker(t)
	p = p.SetSize(80, 24).Open(links)
	out := p.View()
	if strings.Contains(out, " [1]") {
		t.Fatalf("expected no leading-space pad in 9-link picker, got:\n%s", out)
	}
	if !strings.Contains(out, "[1]") {
		t.Fatalf("expected '[1]' in output, got:\n%s", out)
	}
}

func TestLinkPickerPreviewShowsFullURL(t *testing.T) {
	long := "https://example.com/some/very/long/path/that/wraps?query=value"
	p := newTestLinkPicker(t)
	p = p.SetSize(80, 24).Open([]string{"https://a.com", long})
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	out := p.View()
	if !strings.Contains(out, "example.com/some/very/long") {
		t.Fatalf("preview should expose full URL prefix, got:\n%s", out)
	}
}

func TestLinkPicker_listModel_cursorAdvances(t *testing.T) {
	p := NewLinkPicker(NewStyles(theme.Nord), ansix.NewMeasurer(1)).Open([]string{"https://a", "https://b", "https://c"})
	p = p.SetSize(60, 12)

	if got := p.Cursor(); got != 0 {
		t.Fatalf("initial cursor = %d, want 0", got)
	}
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j'})
	if got := p.Cursor(); got != 1 {
		t.Fatalf("cursor after j = %d, want 1", got)
	}
	p, _ = p.Update(tea.KeyPressMsg{Code: 'k'})
	if got := p.Cursor(); got != 0 {
		t.Fatalf("cursor after k = %d, want 0", got)
	}
}

func TestLinkPicker_listModel_enterLaunchesCursor(t *testing.T) {
	p := NewLinkPicker(NewStyles(theme.Nord), ansix.NewMeasurer(1)).Open([]string{"https://a", "https://b"})
	p = p.SetSize(60, 12)
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j'})

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on cursor produced no Cmd")
	}
	msgs := collectMsgs(cmd)
	if !containsLaunchURL(msgs, "https://b") {
		t.Fatalf("LaunchURLMsg.URL mismatch; got msgs %v", msgs)
	}
}
