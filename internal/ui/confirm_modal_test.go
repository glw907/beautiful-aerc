// SPDX-License-Identifier: MIT

package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

func newTestConfirmModal() ConfirmModal {
	t := theme.Themes[theme.DefaultThemeName]
	return NewConfirmModal(NewStyles(t))
}

func testReq() ConfirmRequest {
	return ConfirmRequest{
		Title: "Empty Trash",
		Body:  "Permanently delete all 42 messages in Trash?",
		OnYes: func() tea.Msg {
			return EmptyFolderConfirmedMsg{Folder: "Trash", Source: "Trash"}
		},
	}
}

func containsMsg(msgs []tea.Msg, target tea.Msg) bool {
	for _, m := range msgs {
		if m == target {
			return true
		}
	}
	return false
}

func containsMsgType(msgs []tea.Msg, target interface{}) bool {
	targetType := func() string {
		switch target.(type) {
		case EmptyFolderConfirmedMsg:
			return "EmptyFolderConfirmedMsg"
		case ConfirmModalClosedMsg:
			return "ConfirmModalClosedMsg"
		default:
			return ""
		}
	}()
	for _, m := range msgs {
		var got string
		switch m.(type) {
		case EmptyFolderConfirmedMsg:
			got = "EmptyFolderConfirmedMsg"
		case ConfirmModalClosedMsg:
			got = "ConfirmModalClosedMsg"
		}
		if got == targetType {
			return true
		}
	}
	return false
}

func TestConfirmModal_OpenClose(t *testing.T) {
	m := newTestConfirmModal()
	if m.IsOpen() {
		t.Fatal("new modal should be closed")
	}
	m = m.Open(testReq())
	if !m.IsOpen() {
		t.Fatal("modal should be open after Open")
	}
	m = m.Close()
	if m.IsOpen() {
		t.Fatal("modal should be closed after Close")
	}
}

func TestConfirmModal_YesEmitsOnYesAndCloses(t *testing.T) {
	m := newTestConfirmModal()
	m = m.Open(testReq())
	m = m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	msgs := drainBatch(cmd)

	if !containsMsgType(msgs, EmptyFolderConfirmedMsg{}) {
		t.Error("expected EmptyFolderConfirmedMsg in batch")
	}
	if !containsMsgType(msgs, ConfirmModalClosedMsg{}) {
		t.Error("expected ConfirmModalClosedMsg in batch")
	}
}

func TestConfirmModal_NoAndEscClose(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("n")},
		{Type: tea.KeyEscape},
	} {
		m := newTestConfirmModal()
		m = m.Open(testReq())
		_, cmd := m.Update(key)
		msgs := drainBatch(cmd)

		if !containsMsgType(msgs, ConfirmModalClosedMsg{}) {
			t.Errorf("key %q: expected ConfirmModalClosedMsg", key)
		}
		if containsMsgType(msgs, EmptyFolderConfirmedMsg{}) {
			t.Errorf("key %q: must NOT emit EmptyFolderConfirmedMsg", key)
		}
	}
}

func TestConfirmModal_QSwallowed(t *testing.T) {
	m := newTestConfirmModal()
	m = m.Open(testReq())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd != nil {
		t.Errorf("q should be swallowed (nil cmd), got %v", cmd)
	}
}

func TestConfirmModal_ViewWidthContract(t *testing.T) {
	m := newTestConfirmModal()
	m = m.Open(testReq())
	m = m.SetSize(80, 24)

	box := m.Box(80, 24)
	lines, _ := splitLines(box)
	if len(lines) == 0 {
		t.Fatal("box produced no lines")
	}
	want := lipgloss.Width(lines[0])
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w > 80 {
			t.Errorf("line %d width = %d, want ≤80: %q", i, w, line)
		}
		if w != want {
			t.Errorf("line %d width = %d, want %d (border alignment): %q", i, w, want, line)
		}
	}
}
