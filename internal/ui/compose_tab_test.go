// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gomail "github.com/emersion/go-message/mail"
	"github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/theme"
)

func newTestCompose(t *testing.T) *ComposeTab {
	t.Helper()
	styles := NewStyles(theme.Nord)
	c := NewComposeTab(styles, theme.Nord, "geoff@907.life", SimpleIcons)
	c.SetSize(80, 24)
	return c
}

func keyMsgFromString(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func sendKey(c *ComposeTab, k string) *ComposeTab {
	next, _ := c.Update(keyMsgFromString(k))
	return next
}

func TestComposeTab_View_HonorsAssignedWidth(t *testing.T) {
	c := newTestCompose(t)
	c.SetSize(60, 20)
	for i, line := range strings.Split(c.View(), "\n") {
		if w := lipgloss.Width(line); w != 60 {
			t.Fatalf("line %d width = %d, want 60: %q", i, w, line)
		}
	}
}

func TestComposeTab_View_HasHeaderRows(t *testing.T) {
	c := newTestCompose(t)
	v := c.View()
	for _, want := range []string{"From:", "To:", "Cc:", "Bcc:", "Subject:"} {
		if !strings.Contains(v, want) {
			t.Fatalf("View missing %q\n%s", want, v)
		}
	}
}

func TestComposeTab_TabCyclesFields(t *testing.T) {
	c := newTestCompose(t)
	want := []int{composeFocusCc, composeFocusBcc, composeFocusSubject, composeFocusBody, composeFocusTo}
	for i, w := range want {
		c = sendKey(c, "tab")
		if c.focus != w {
			t.Fatalf("step %d: focus = %d, want %d", i, c.focus, w)
		}
	}
}

func TestComposeTab_ShiftTabCyclesBackward(t *testing.T) {
	c := newTestCompose(t)
	c = sendKey(c, "shift+tab")
	if c.focus != composeFocusBody {
		t.Fatalf("Shift+Tab from To should wrap to Body, got %d", c.focus)
	}
}

func TestComposeTab_EscFromBodyReturnsToSubject(t *testing.T) {
	c := newTestCompose(t)
	c.focus = composeFocusBody
	c.editor.Focus()
	c.to.Blur()
	c = sendKey(c, "esc")
	if c.focus != composeFocusSubject {
		t.Fatalf("Esc from Body should focus Subject, got %d", c.focus)
	}
}

func TestComposeTab_EscFromHeaderReturnsToBody(t *testing.T) {
	c := newTestCompose(t)
	c.focus = composeFocusTo
	c = sendKey(c, "esc")
	if c.focus != composeFocusBody {
		t.Fatalf("Esc from header should focus Body, got %d", c.focus)
	}
}

func gomailAddress(addr string) gomail.Address {
	return gomail.Address{Address: addr}
}

func TestComposeTab_DraftReflectsInputs(t *testing.T) {
	c := newTestCompose(t)
	c.to.SetValue("alice@example.com, bob@example.com")
	c.cc.SetValue("c@example.com")
	c.subject.SetValue("hi")
	c.editor.SetValue("hello world")

	d, err := c.Draft()
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if len(d.To) != 2 || d.To[0].Address != "alice@example.com" {
		t.Fatalf("To not parsed: %+v", d.To)
	}
	if len(d.Cc) != 1 || d.Cc[0].Address != "c@example.com" {
		t.Fatalf("Cc not parsed: %+v", d.Cc)
	}
	if d.Subject != "hi" || d.Body != "hello world" {
		t.Fatalf("subject/body wrong: %q %q", d.Subject, d.Body)
	}
	if d.From.Address != "geoff@907.life" {
		t.Fatalf("From wrong: %+v", d.From)
	}
}

func TestComposeTab_DraftBadAddressFails(t *testing.T) {
	c := newTestCompose(t)
	c.to.SetValue("not an address")
	if _, err := c.Draft(); err == nil {
		t.Fatalf("want parse error on bad address, got nil")
	}
}

func TestComposeTab_IsDirty(t *testing.T) {
	c := newTestCompose(t)
	if c.IsDirty() {
		t.Fatalf("fresh compose should not be dirty")
	}
	c.editor.SetValue("hi")
	if !c.IsDirty() {
		t.Fatalf("body content should mark dirty")
	}
}

func TestComposeTab_Seed(t *testing.T) {
	c := newTestCompose(t)
	d := compose.Draft{
		Subject: "Re: hi",
		Body:    "> original\n\n",
	}
	d.To = append(d.To, gomailAddress("alice@example.com"))
	c.Seed(d)
	if c.subject.Value() != "Re: hi" {
		t.Fatalf("subject not seeded: %q", c.subject.Value())
	}
	if c.editor.Value() != "> original\n\n" {
		t.Fatalf("body not seeded: %q", c.editor.Value())
	}
	if c.to.Value() != "alice@example.com" {
		t.Fatalf("To not seeded: %q", c.to.Value())
	}
}

func TestComposeTab_CtrlXEmitsSendMsg(t *testing.T) {
	c := newTestCompose(t)
	c.to.SetValue("alice@example.com")
	c.subject.SetValue("hi")
	c.editor.SetValue("body")

	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if cmd == nil {
		t.Fatal("Ctrl+X should return a Cmd that emits ComposeSendMsg")
	}
	msg := cmd()
	send, ok := msg.(ComposeSendMsg)
	if !ok {
		t.Fatalf("want ComposeSendMsg, got %T", msg)
	}
	if send.Draft.Subject != "hi" {
		t.Fatalf("send carries wrong draft: %+v", send.Draft)
	}
}

func TestComposeTab_CtrlCEmitsCancelMsg(t *testing.T) {
	c := newTestCompose(t)
	c.editor.SetValue("dirty")

	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl+C should return a Cmd")
	}
	msg := cmd()
	cancel, ok := msg.(ComposeCancelMsg)
	if !ok {
		t.Fatalf("want ComposeCancelMsg, got %T", msg)
	}
	if !cancel.Dirty {
		t.Fatalf("dirty draft should set Dirty=true")
	}
}

func TestComposeTab_CtrlXBadAddressInlinesError(t *testing.T) {
	c := newTestCompose(t)
	c.to.SetValue("not an address")
	_, cmd := c.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if cmd != nil {
		t.Fatalf("Ctrl+X with bad address should not emit send")
	}
	if c.err == "" {
		t.Fatalf("inline err row should be set")
	}
}
