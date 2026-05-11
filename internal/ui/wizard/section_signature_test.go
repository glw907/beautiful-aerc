package wizard

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/theme"
	wizdomain "github.com/glw907/poplar/internal/wizard"
)

func newSignatureSectionForTest(t *testing.T) (*signatureSection, *Model) {
	t.Helper()
	parent := &Model{
		State:  wizdomain.Model{},
		Theme:  theme.Themes["one-dark"],
		Styles: NewStyles(theme.Themes["one-dark"]),
	}
	return newSignatureSection(parent), parent
}

func TestSignatureSection_EscSkipsWithEmptyBody(t *testing.T) {
	s, parent := newSignatureSectionForTest(t)
	_, cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc must emit a command")
	}
	msg := cmd()
	if _, ok := msg.(AdvanceMsg); !ok {
		t.Fatalf("Esc msg = %T, want AdvanceMsg", msg)
	}
	if parent.State.Signature != "" {
		t.Errorf("Signature = %q, want empty", parent.State.Signature)
	}
}

func TestSignatureSection_CtrlXSavesAndAdvances(t *testing.T) {
	s, parent := newSignatureSectionForTest(t)
	s.editor = s.editor.WithValue("Geoff\ngeoff@907.life")
	_, cmd := s.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+X must emit a command")
	}
	msg := cmd()
	if _, ok := msg.(AdvanceMsg); !ok {
		t.Fatalf("Ctrl+X msg = %T, want AdvanceMsg", msg)
	}
	if parent.State.Signature != "Geoff\ngeoff@907.life" {
		t.Errorf("Signature = %q, want catkin body", parent.State.Signature)
	}
}

func TestSignatureSection_CtrlPGoesBack(t *testing.T) {
	s, _ := newSignatureSectionForTest(t)
	_, cmd := s.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+P must emit a command")
	}
	if _, ok := cmd().(BackMsg); !ok {
		t.Fatalf("Ctrl+P msg = %T, want BackMsg", cmd())
	}
}

func TestSignatureSection_OtherKeysReachCatkin(t *testing.T) {
	s, _ := newSignatureSectionForTest(t)
	s.editor, _ = s.editor.WithFocus()
	before := s.editor.Value()
	s.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if s.editor.Value() == before {
		t.Errorf("editor did not receive 'a' keypress")
	}
}

func TestSignatureSection_ViewShowsChromeAndDescription(t *testing.T) {
	s, _ := newSignatureSectionForTest(t)
	v := s.View()
	for _, want := range []string{
		"Email signature",
		"Markdown is supported and will be rendered as HTML on send.",
		"-- ",
		"^X save",
		"Esc skip",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("View missing %q in:\n%s", want, v)
		}
	}
}
