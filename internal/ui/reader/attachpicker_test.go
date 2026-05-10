package reader

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

func newTestAttachPicker(t *testing.T) AttachPicker {
	t.Helper()
	return NewAttachPicker(NewStyles(theme.Nord), uicore.SimpleIcons).SetSize(120, 40)
}

func TestAttachPicker_OpenClose(t *testing.T) {
	p := newTestAttachPicker(t)
	if p.IsOpen() {
		t.Fatal("new picker should be closed")
	}
	p = p.Open("u1", []mail.Attachment{{PartID: "2", Filename: "x.pdf", Size: 10}})
	if !p.IsOpen() {
		t.Fatal("Open should set open")
	}
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc should emit close cmd")
	}
	if _, ok := cmd().(AttachPickerClosedMsg); !ok {
		t.Errorf("got %T, want AttachPickerClosedMsg", cmd())
	}
}

func TestAttachPicker_OpenAction(t *testing.T) {
	p := newTestAttachPicker(t).Open("u1",
		[]mail.Attachment{{PartID: "2", Filename: "x.pdf", Size: 10}})
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should emit a Cmd")
	}
	if cmd() == nil {
		t.Fatal("batch nil")
	}
}

func TestAttachPicker_SaveAction(t *testing.T) {
	p := newTestAttachPicker(t).Open("u1",
		[]mail.Attachment{{PartID: "2", Filename: "x.pdf", Size: 10}})
	_, cmd := p.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if cmd == nil {
		t.Fatal("s should emit a Cmd")
	}
}

func TestAttachPicker_DigitOpensIndex(t *testing.T) {
	p := newTestAttachPicker(t).Open("u1", []mail.Attachment{
		{PartID: "1", Filename: "a", Size: 1},
		{PartID: "2", Filename: "b", Size: 2},
	})
	_, cmd := p.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	if cmd == nil {
		t.Fatal("digit should emit a Cmd")
	}
}

func TestAttachPicker_RenderContainsFilename(t *testing.T) {
	p := newTestAttachPicker(t).Open("u1",
		[]mail.Attachment{{PartID: "2", Filename: "report.pdf", Size: 2400}})
	out := p.View()
	if !strings.Contains(out, "report.pdf") {
		t.Errorf("View missing filename: %s", out)
	}
	if !strings.Contains(out, "2.3 KB") {
		t.Errorf("View missing size: %s", out)
	}
}

func TestAttachPicker_listModel_cursorAdvances(t *testing.T) {
	st := NewStyles(theme.Nord)
	icons := uicore.SimpleIcons
	atts := []mail.Attachment{
		{Filename: "a.pdf", Size: 1234},
		{Filename: "b.png", Size: 5678},
	}
	p := NewAttachPicker(st, icons).Open("u1", atts)
	p = p.SetSize(60, 12)

	if got := p.Cursor(); got != 0 {
		t.Fatalf("initial cursor = %d, want 0", got)
	}
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j'})
	if got := p.Cursor(); got != 1 {
		t.Fatalf("cursor after j = %d, want 1", got)
	}
}

func TestAttachPicker_listModel_enterOpensCursor(t *testing.T) {
	st := NewStyles(theme.Nord)
	icons := uicore.SimpleIcons
	atts := []mail.Attachment{
		{Filename: "a.pdf", Size: 1234},
		{Filename: "b.png", Size: 5678},
	}
	p := NewAttachPicker(st, icons).Open("u7", atts)
	p = p.SetSize(60, 12)
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j'})

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msgs := collectMsgs(cmd)
	var got OpenAttachmentMsg
	for _, m := range msgs {
		if open, ok := m.(OpenAttachmentMsg); ok {
			got = open
		}
	}
	if got.Att.Filename != "b.png" || got.UID != mail.UID("u7") {
		t.Fatalf("OpenAttachmentMsg = %+v, want UID=u7 Filename=b.png", got)
	}
}
