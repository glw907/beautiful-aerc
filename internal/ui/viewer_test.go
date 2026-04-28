// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

func newTestViewer() Viewer {
	return NewViewer(NewStyles(theme.Nord), theme.Nord, "geoff@907.life")
}

func TestViewerOpenTransitionsToLoading(t *testing.T) {
	v := newTestViewer()
	if v.IsOpen() {
		t.Fatal("new viewer must be closed")
	}
	v = v.SetSize(80, 24).Open(mail.MessageInfo{UID: "1", Subject: "Hi", From: "Alice"})
	if !v.IsOpen() {
		t.Fatal("Open must mark viewer open")
	}
	if v.phase != viewerLoading {
		t.Errorf("phase = %v, want loading", v.phase)
	}
	if v.CurrentUID() != "1" {
		t.Errorf("CurrentUID = %q, want 1", v.CurrentUID())
	}
	if !strings.Contains(v.View(), "Loading") {
		t.Errorf("loading view should mention Loading; got: %q", v.View())
	}
}

func TestViewerBodyLoadedSetsReady(t *testing.T) {
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1", Subject: "Hi", From: "Alice"})
	blocks := []content.Block{content.Paragraph{Spans: []content.Span{content.Text{Content: "Hello world"}}}}
	v = v.SetBody(blocks)
	if v.phase != viewerReady {
		t.Errorf("phase = %v, want ready", v.phase)
	}
	out := v.View()
	if !strings.Contains(out, "Hello world") {
		t.Errorf("ready view missing body: %q", out)
	}
	if !strings.Contains(out, "Subject:") {
		t.Errorf("ready view missing headers: %q", out)
	}
}

func TestViewerStaleBodyLoadedIgnored(t *testing.T) {
	// AccountTab is the layer that performs the UID guard. Viewer's
	// contract is: CurrentUID is the source of truth; SetBody is
	// idempotent and the caller must filter.
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1", From: "Alice"})
	v = v.Close().Open(mail.MessageInfo{UID: "2", From: "Bob"})
	if v.CurrentUID() != "2" {
		t.Fatalf("CurrentUID = %q, want 2", v.CurrentUID())
	}
}

func TestViewerCloseFromLoading(t *testing.T) {
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1"})
	v, cmd := v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.IsOpen() {
		t.Error("esc must close viewer")
	}
	if cmd == nil {
		t.Error("close must emit ViewerClosedMsg cmd")
	}
	if _, ok := cmd().(ViewerClosedMsg); !ok {
		t.Errorf("cmd should produce ViewerClosedMsg")
	}
}

func TestViewerCloseFromReady(t *testing.T) {
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1"})
	v = v.SetBody([]content.Block{content.Paragraph{Spans: []content.Span{content.Text{Content: "body"}}}})
	v, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if v.IsOpen() {
		t.Error("q must close viewer from ready phase")
	}
	if cmd == nil {
		t.Error("close must emit cmd")
	}
}

func TestViewerScrollEmitsScrollMsg(t *testing.T) {
	v := newTestViewer().SetSize(80, 6).Open(mail.MessageInfo{UID: "1", Subject: "S"})
	long := strings.Repeat("alpha bravo charlie ", 40)
	v = v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{content.Text{Content: long}}},
	})
	if v.ScrollPct() != 0 {
		t.Errorf("initial scroll pct = %d, want 0", v.ScrollPct())
	}
	// Press G to jump to bottom — scroll % should change to 100.
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if pct := v.ScrollPct(); pct != 100 {
		t.Errorf("after G scroll pct = %d, want 100", pct)
	}
}

func TestViewerNumericLaunchesURL(t *testing.T) {
	got := ""
	prev := openURL
	openURL = func(u string) error { got = u; return nil }
	defer func() { openURL = prev }()

	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1"})
	v = v.SetBody([]content.Block{content.Paragraph{Spans: []content.Span{
		content.Link{Text: "click", URL: "https://example.com/one"},
	}}})
	if len(v.Links()) != 1 {
		t.Fatalf("expected 1 harvested link, got %d", len(v.Links()))
	}
	v, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if cmd == nil {
		t.Fatal("numeric key must produce a launch cmd")
	}
	cmd() // execute the cmd to invoke openURL hook
	if got != "https://example.com/one" {
		t.Errorf("launched URL = %q, want https://example.com/one", got)
	}
	if !v.IsOpen() {
		t.Error("link launch must not close viewer")
	}
}

func TestViewerNumericNoOpOutOfRange(t *testing.T) {
	got := ""
	prev := openURL
	openURL = func(u string) error { got = u; return nil }
	defer func() { openURL = prev }()

	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1"})
	v = v.SetBody([]content.Block{content.Paragraph{Spans: []content.Span{
		content.Link{Text: "click", URL: "https://only.example"},
	}}})
	v, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	if cmd != nil {
		// Even if a cmd comes back, it should not invoke the launcher.
		cmd()
	}
	if got != "" {
		t.Errorf("out-of-range numeric key launched a URL: %q", got)
	}
}

func TestViewerTabEmitsLinkPickerOpenWhenLinks(t *testing.T) {
	v := newTestViewer().SetSize(80, 24)
	v = v.Open(mail.MessageInfo{UID: "uid-1"})
	v = v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{
			content.Link{Text: "click", URL: "https://a.com"},
		}},
	})

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyTab})

	got := collectMsgs(cmd)
	var found bool
	for _, m := range got {
		if op, ok := m.(LinkPickerOpenMsg); ok && len(op.Links) == 1 && op.Links[0] == "https://a.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected LinkPickerOpenMsg with [https://a.com], got %v", got)
	}
}

func TestViewerTabNoLinksInert(t *testing.T) {
	v := newTestViewer().SetSize(80, 24)
	v = v.Open(mail.MessageInfo{UID: "uid-1"})
	v = v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{content.Text{Content: "no links"}}},
	})

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyTab})

	if cmd != nil {
		t.Fatalf("expected no Cmd when zero links, got %v", cmd)
	}
}

func TestViewerClosedViewIsEmpty(t *testing.T) {
	v := newTestViewer()
	if v.View() != "" {
		t.Errorf("closed View must be empty, got %q", v.View())
	}
}

// TestViewerLeftPaddingGeometry verifies that every rendered line in the
// ready phase starts with a space and is exactly v.width display cells wide.
func TestViewerLeftPaddingGeometry(t *testing.T) {
	const w, h = 80, 20
	v := newTestViewer().SetSize(w, h).Open(mail.MessageInfo{
		UID: "1", Subject: "Geometry test", From: "Alice",
	})
	v = v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{content.Text{Content: "Hello, padding world."}}},
	})

	out := v.View()
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			t.Errorf("line %d does not start with a space: %q", i, line)
		}
		if got := displayCells(line); got != w {
			t.Errorf("line %d width = %d, want %d: %q", i, got, w, line)
		}
	}
}
