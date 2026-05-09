package reader

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/ansix"
	"github.com/glw907/poplar/internal/content"
	"github.com/glw907/poplar/internal/icalendar"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func newTestViewer() Model {
	return New(NewStyles(theme.Nord), theme.Nord, "geoff@907.life", uicore.SimpleIcons)
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
	if v.Phase() != PhaseLoading {
		t.Errorf("phase = %v, want loading", v.Phase())
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
	v = v.SetBody(blocks, content.Unsubscribe{}, nil)
	if v.Phase() != PhaseReady {
		t.Errorf("phase = %v, want ready", v.Phase())
	}
	out := v.View()
	if !strings.Contains(out, "Hello world") {
		t.Errorf("ready view missing body: %q", out)
	}
	if !strings.Contains(out, "Hi") {
		t.Errorf("ready view missing subject title: %q", out)
	}
	if !strings.Contains(out, "FROM ") {
		t.Errorf("ready view missing From label: %q", out)
	}
}

func TestViewerStaleBodyLoadedIgnored(t *testing.T) {
	// AccountTab is the layer that performs the UID guard. Viewer's
	// contract is: CurrentUID is the source of truth. SetBody is
	// idempotent and the caller must filter.
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1", From: "Alice"})
	v = v.Close().Open(mail.MessageInfo{UID: "2", From: "Bob"})
	if v.CurrentUID() != "2" {
		t.Fatalf("CurrentUID = %q, want 2", v.CurrentUID())
	}
}

func TestViewerCloseFromLoading(t *testing.T) {
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1"})
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if v.IsOpen() {
		t.Error("esc must close viewer")
	}
}

func TestViewerCloseFromReady(t *testing.T) {
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1"})
	v = v.SetBody([]content.Block{content.Paragraph{Spans: []content.Span{content.Text{Content: "body"}}}}, content.Unsubscribe{}, nil)
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if v.IsOpen() {
		t.Error("q must close viewer from ready phase")
	}
}

func TestViewerScrollPctUpdatesOnNav(t *testing.T) {
	v := newTestViewer().SetSize(80, 6).Open(mail.MessageInfo{UID: "1", Subject: "S"})
	long := strings.Repeat("alpha bravo charlie ", 40)
	v = v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{content.Text{Content: long}}},
	}, content.Unsubscribe{}, nil)
	if v.ScrollPct() != 0 {
		t.Errorf("initial scroll pct = %d, want 0", v.ScrollPct())
	}
	// Press G to jump to bottom. Scroll % should change to 100.
	v, _ = v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if pct := v.ScrollPct(); pct != 100 {
		t.Errorf("after G scroll pct = %d, want 100", pct)
	}
}

func TestViewerNumericLaunchesURL(t *testing.T) {
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1"})
	v = v.SetBody([]content.Block{content.Paragraph{Spans: []content.Span{
		content.Link{Text: "click", URL: "https://example.com/one"},
	}}}, content.Unsubscribe{}, nil)
	if len(v.Links()) != 1 {
		t.Fatalf("expected 1 harvested link, got %d", len(v.Links()))
	}
	v, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if cmd == nil {
		t.Fatal("numeric key must produce a launch cmd")
	}
	msg, ok := cmd().(LaunchURLMsg)
	if !ok {
		t.Fatalf("expected LaunchURLMsg, got %T", cmd())
	}
	if msg.URL != "https://example.com/one" {
		t.Errorf("LaunchURLMsg.URL = %q, want https://example.com/one", msg.URL)
	}
	if !v.IsOpen() {
		t.Error("link launch must not close viewer")
	}
}

func TestViewerNumericNoOpOutOfRange(t *testing.T) {
	got := ""
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1"})
	v = v.SetBody([]content.Block{content.Paragraph{Spans: []content.Span{
		content.Link{Text: "click", URL: "https://only.example"},
	}}}, content.Unsubscribe{}, nil)
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	if cmd != nil {
		if msg, ok := cmd().(LaunchURLMsg); ok {
			got = msg.URL
		}
	}
	if got != "" {
		t.Errorf("out-of-range numeric key launched a URL: %q", got)
	}
}

func TestViewerTabEmitsOpenLinkPickerMsg(t *testing.T) {
	v := newTestViewer().SetSize(80, 24)
	v = v.Open(mail.MessageInfo{UID: "uid-1"})
	v = v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{
			content.Link{Text: "click", URL: "https://a.com"},
		}},
	}, content.Unsubscribe{}, nil)

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("expected Cmd from Tab when links are harvested")
	}
	msg, ok := cmd().(OpenLinkPickerMsg)
	if !ok {
		t.Fatalf("expected OpenLinkPickerMsg, got %T", cmd())
	}
	if len(msg.Links) != 1 || msg.Links[0] != "https://a.com" {
		t.Fatalf("expected [https://a.com], got %v", msg.Links)
	}
}

func TestViewerTabNoLinksInert(t *testing.T) {
	v := newTestViewer().SetSize(80, 24)
	v = v.Open(mail.MessageInfo{UID: "uid-1"})
	v = v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{content.Text{Content: "no links"}}},
	}, content.Unsubscribe{}, nil)

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

func TestViewerHandleKey_CloseViaQ_UsesKeyMatches(t *testing.T) {
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1", From: "Alice"})
	v, _ = v.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if v.IsOpen() {
		t.Fatal("viewer should close on q")
	}
}

// TestViewerLeftPaddingGeometry verifies that every rendered line in the
// ready phase is exactly v.width display cells wide. Panel rows lead
// with the panel's BgElevated PaddingLeft cell. The panel's bottom
// border row leads with a '─' that spans the full panel width. Body
// rows lead with a BgBase gutter cell. Width is the contract. Leading
// character varies by row class.
func TestViewerLeftPaddingGeometry(t *testing.T) {
	const w, h = 80, 20
	v := newTestViewer().SetSize(w, h).Open(mail.MessageInfo{
		UID: "1", Subject: "Geometry test", From: "Alice",
	})
	v = v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{content.Text{Content: "Hello, padding world."}}},
	}, content.Unsubscribe{}, nil)

	out := v.View()
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		if got := ansix.Width(line); got != w {
			t.Errorf("line %d width = %d, want %d: %q", i, got, w, line)
		}
	}
}

func TestViewer_ChipRow_Hidden_WhenEmpty(t *testing.T) {
	v := newTestViewer()
	v = v.SetSize(80, 24)
	v = v.Open(mail.MessageInfo{UID: "u1", Subject: "x"})
	v = v.SetBody(nil, content.Unsubscribe{}, nil)
	v = v.SetAttachments(nil)
	out := v.View()
	if strings.Contains(out, "§") || strings.Contains(out, "\U000F0184") {
		t.Errorf("chip glyph present despite no attachments")
	}
}

func TestViewer_ChipRow_Visible(t *testing.T) {
	v := newTestViewer()
	v = v.SetSize(120, 40)
	v = v.Open(mail.MessageInfo{UID: "u1", Subject: "x"})
	v = v.SetBody(nil, content.Unsubscribe{}, nil)
	v = v.SetAttachments([]mail.Attachment{
		{PartID: "2", Filename: "report.pdf", Size: 2400},
	})
	out := v.View()
	if !strings.Contains(out, "report.pdf") {
		t.Errorf("chip row missing filename: %s", out)
	}
	if !strings.Contains(out, "2.3 KB") {
		t.Errorf("chip row missing size: %s", out)
	}
}

func TestViewer_AtKey_Inert_WhenEmpty(t *testing.T) {
	v := newTestViewer()
	v = v.SetSize(120, 40)
	v = v.Open(mail.MessageInfo{UID: "u1"})
	v = v.SetBody(nil, content.Unsubscribe{}, nil)
	v = v.SetAttachments(nil)
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}})
	if cmd != nil {
		t.Errorf("expected no Cmd when no attachments; got one")
	}
}

func TestViewer_AtKey_OpensPicker(t *testing.T) {
	v := newTestViewer()
	v = v.SetSize(120, 40)
	v = v.Open(mail.MessageInfo{UID: "u1"})
	v = v.SetBody(nil, content.Unsubscribe{}, nil)
	v = v.SetAttachments([]mail.Attachment{{PartID: "2", Filename: "x.pdf", Size: 1}})
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}})
	if cmd == nil {
		t.Fatal("expected OpenAttachPickerMsg Cmd")
	}
	msg := cmd()
	open, ok := msg.(OpenAttachPickerMsg)
	if !ok {
		t.Fatalf("got %T, want OpenAttachPickerMsg", msg)
	}
	if len(open.Items) != 1 || open.UID != "u1" {
		t.Errorf("unexpected payload: %+v", open)
	}
}

func TestViewer_InviteBlock_AppearsInView(t *testing.T) {
	v := newTestViewer().SetSize(80, 30).Open(mail.MessageInfo{UID: "1", Subject: "Meeting"})
	inv := &icalendar.Invite{Summary: "Quarterly sync"}
	v = v.SetBody(nil, content.Unsubscribe{}, inv)
	out := v.View()
	if !strings.Contains(out, "Quarterly sync") {
		t.Errorf("invite block missing Summary in View output: %q", out)
	}
}

func TestViewer_InviteBlock_BodyHeightAccountsForInvite(t *testing.T) {
	const w, h = 80, 30
	v := newTestViewer().SetSize(w, h).Open(mail.MessageInfo{UID: "1", Subject: "Meeting"})

	// Baseline: no invite.
	v0 := v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{content.Text{Content: "body"}}},
	}, content.Unsubscribe{}, nil)
	baseHeight := v0.inviteHeight

	// With invite.
	inv := &icalendar.Invite{Summary: "Sync"}
	v1 := v.SetBody([]content.Block{
		content.Paragraph{Spans: []content.Span{content.Text{Content: "body"}}},
	}, content.Unsubscribe{}, inv)

	if v1.inviteHeight == 0 {
		t.Fatal("expected non-zero inviteHeight when invite is set")
	}
	if v1.inviteHeight <= baseHeight {
		t.Errorf("invite height should exceed baseline: got %d vs %d", v1.inviteHeight, baseHeight)
	}
}

func TestViewer_InviteAccessor(t *testing.T) {
	v := newTestViewer().SetSize(80, 24).Open(mail.MessageInfo{UID: "1"})
	if v.Invite() != nil {
		t.Error("Invite() should be nil before SetBody")
	}
	inv := &icalendar.Invite{Summary: "Check"}
	v = v.SetBody(nil, content.Unsubscribe{}, inv)
	if v.Invite() == nil {
		t.Error("Invite() should be non-nil after SetBody with invite")
	}
	if v.Invite().Summary != "Check" {
		t.Errorf("Invite().Summary = %q, want Check", v.Invite().Summary)
	}
}
