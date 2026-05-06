// SPDX-License-Identifier: MIT

package ui

// Golden-output baseline tests for the four overlay surfaces and the
// AccountTab full view. Run with UPDATE_GOLDENS=1 to regenerate all
// golden files. Subsequent runs assert byte-for-byte equality so that
// refactor steps (ModalShell extraction, SidebarColumn extraction, render
// caches) leave rendering unchanged.
//
// Surfaces × sizes (12 files total):
//
//   confirm_80x24.txt, confirm_120x40.txt
//   linkpicker_80x24.txt, linkpicker_120x40.txt
//   movepicker_80x24.txt, movepicker_120x40.txt
//   helppopover_account_80x24.txt, helppopover_account_120x40.txt
//   helppopover_viewer_80x24.txt, helppopover_viewer_120x40.txt
//   accounttab_80x24.txt, accounttab_120x40.txt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/movepicker"
)

const goldensDir = "testdata/goldens"

// goldenUpdate returns true when the caller asked to regenerate goldens.
func goldenUpdate() bool {
	return os.Getenv("UPDATE_GOLDENS") == "1"
}

// checkGolden compares got against the golden file at
// testdata/goldens/<name>. When UPDATE_GOLDENS=1 it writes the file
// instead of comparing.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldensDir, name)
	if goldenUpdate() {
		if err := os.MkdirAll(goldensDir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", goldensDir, err)
		}
		if err := os.WriteFile(path, []byte(got), 0644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with UPDATE_GOLDENS=1 to create)", path, err)
	}
	want := string(data)
	if got != want {
		// Show a short diff hint at the first diverging line.
		gotLines := strings.Split(got, "\n")
		wantLines := strings.Split(want, "\n")
		for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
			if gotLines[i] != wantLines[i] {
				t.Fatalf("golden %s mismatch at line %d:\n  got  %q\n  want %q",
					name, i+1, gotLines[i], wantLines[i])
			}
		}
		// Lengths differ.
		t.Fatalf("golden %s mismatch: got %d lines, want %d lines",
			name, len(gotLines), len(wantLines))
	}
}

// blankBackground returns a width×height string of spaces separated by
// newlines, suitable as a PlaceOverlay background.
func blankBackground(w, h int) string {
	row := strings.Repeat(" ", w)
	rows := make([]string, h)
	for i := range rows {
		rows[i] = row
	}
	return strings.Join(rows, "\n")
}

// compositeOverlay renders box onto a blank w×h background using the
// same PlaceOverlay + centerOverlay path that App uses.
func compositeOverlay(box string, w, h int) string {
	x, y := centerOverlay(box, w, h)
	return PlaceOverlay(x, y, box, blankBackground(w, h))
}

// TestGolden_ConfirmModal captures the rendered confirm modal at both
// design-polish sizes.
func TestGolden_ConfirmModal(t *testing.T) {
	styles := NewStyles(theme.Themes[theme.DefaultThemeName])
	req := testReq() // from confirm_modal_test.go

	cases := []struct {
		name string
		w, h int
	}{
		{"confirm_80x24.txt", 80, 24},
		{"confirm_120x40.txt", 120, 40},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			m := NewConfirmModal(styles)
			m = m.Open(req)
			m = m.SetSize(tc.w, tc.h)
			box := m.Box(tc.w, tc.h)
			got := compositeOverlay(box, tc.w, tc.h)
			checkGolden(t, tc.name, got)
		})
	}
}

// TestGolden_LinkPicker captures the rendered link picker with a
// representative 3-URL set (mix of short and long URLs).
func TestGolden_LinkPicker(t *testing.T) {
	styles := NewStyles(theme.Nord)
	links := []string{
		"https://go.dev",
		"https://pkg.go.dev/github.com/charmbracelet/bubbletea",
		"https://example.com/some/very/long/path/that/wraps?query=value&another=param",
	}

	cases := []struct {
		name string
		w, h int
	}{
		{"linkpicker_80x24.txt", 80, 24},
		{"linkpicker_120x40.txt", 120, 40},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := NewLinkPicker(styles)
			p = p.SetSize(tc.w, tc.h)
			p = p.Open(links)
			box := p.Box(tc.w, tc.h)
			got := compositeOverlay(box, tc.w, tc.h)
			checkGolden(t, tc.name, got)
		})
	}
}

func goldenMovePickerFolders() []mail.FolderEntry {
	return []mail.FolderEntry{
		{Display: "Inbox", Provider: "INBOX", Group: mail.GroupPrimary},
		{Display: "Drafts", Provider: "Drafts", Group: mail.GroupPrimary},
		{Display: "Sent", Provider: "Sent", Group: mail.GroupPrimary},
		{Display: "Archive", Provider: "Archive", Group: mail.GroupDisposal},
		{Display: "Trash", Provider: "Trash", Group: mail.GroupDisposal},
		{Display: "Receipts/2026", Provider: "Receipts/2026", Group: mail.GroupCustom},
		{Display: "Receipts/2025", Provider: "Receipts/2025", Group: mail.GroupCustom},
	}
}

func goldenNewMovePicker() movepicker.Model {
	th := theme.Themes[theme.DefaultThemeName]
	return movepicker.New(movepicker.Styles{
		Dim:           lipgloss.NewStyle().Foreground(th.FgDim),
		MsgListCursor: lipgloss.NewStyle().Foreground(th.AccentPrimary),
	})
}

// TestGolden_MovePicker captures the rendered move picker with
// sampleFolders(), no query, cursor at 0.
func TestGolden_MovePicker(t *testing.T) {
	cases := []struct {
		name string
		w, h int
	}{
		{"movepicker_80x24.txt", 80, 24},
		{"movepicker_120x40.txt", 120, 40},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := goldenNewMovePicker()
			p = p.Open(nil, "", goldenMovePickerFolders())
			p = p.SetSize(tc.w, tc.h)
			box := p.Box(tc.w, tc.h)
			got := compositeOverlay(box, tc.w, tc.h)
			checkGolden(t, tc.name, got)
		})
	}
}

// TestGolden_HelpPopover captures both HelpAccount and HelpViewer
// contexts at both design-polish sizes.
func TestGolden_HelpPopover(t *testing.T) {
	styles := NewStyles(theme.Nord)

	cases := []struct {
		name string
		ctx  HelpContext
		w, h int
	}{
		{"helppopover_account_80x24.txt", HelpAccount, 80, 24},
		{"helppopover_account_120x40.txt", HelpAccount, 120, 40},
		{"helppopover_viewer_80x24.txt", HelpViewer, 80, 24},
		{"helppopover_viewer_120x40.txt", HelpViewer, 120, 40},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := NewHelpPopover(styles, tc.ctx)
			h = h.SetSize(tc.w, tc.h)
			// HelpPopover.View already centers with lipgloss.Place.
			got := h.View(tc.w, tc.h)
			checkGolden(t, tc.name, got)
		})
	}
}

// TestGolden_AccountTab captures the full AccountTab.View() at both
// design-polish sizes. The sidebar-column refactor (Task 7) must leave
// this output byte-for-byte equal.
func TestGolden_AccountTab(t *testing.T) {
	cases := []struct {
		name string
		w, h int
	}{
		{"accounttab_80x24.txt", 80, 24},
		{"accounttab_120x40.txt", 120, 40},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tab := newLoadedTab(t, tc.w, tc.h)
			got := tab.View()
			checkGolden(t, tc.name, got)
		})
	}
}
