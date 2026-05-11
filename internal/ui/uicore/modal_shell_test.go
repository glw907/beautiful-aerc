package uicore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

const goldensDir = "testdata/goldens"

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldensDir, name)
	if os.Getenv("UPDATE_GOLDENS") == "1" {
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
		gotLines := strings.Split(got, "\n")
		wantLines := strings.Split(want, "\n")
		for i := range min(len(gotLines), len(wantLines)) {
			if gotLines[i] != wantLines[i] {
				t.Fatalf("golden %s mismatch at line %d:\n  got  %q\n  want %q",
					name, i+1, gotLines[i], wantLines[i])
			}
		}
		t.Fatalf("golden %s mismatch: got %d lines, want %d lines",
			name, len(gotLines), len(wantLines))
	}
}

// buildShellBox is a helper that exercises ModalShell.Box with the given
// parameters and returns the raw box string (no overlay compositing).
func buildShellBox(title string, bodyRows, footerRows []string, contentW int) string {
	var s ModalShell
	return s.Box(title, bodyRows, footerRows, contentW)
}

// lineWidths returns the display-cell width of each line in s, measured
// with lipgloss.Width. Used for modal box width-contract assertions.
func lineWidths(s string) ([]string, []int) {
	lines := strings.Split(s, "\n")
	widths := make([]int, len(lines))
	for i, l := range lines {
		widths[i] = lipgloss.Width(l)
	}
	return lines, widths
}

// TestModalShell_OpenWithOpen verifies IsOpen / WithOpen lifecycle.
func TestModalShell_OpenWithOpen(t *testing.T) {
	var s ModalShell
	if s.IsOpen() {
		t.Fatal("zero-value ModalShell should not be open")
	}
	s = s.WithOpen(true)
	if !s.IsOpen() {
		t.Fatal("should be open after WithOpen(true)")
	}
	s = s.WithOpen(false)
	if s.IsOpen() {
		t.Fatal("should be closed after WithOpen(false)")
	}
}

// TestModalShell_SetSize verifies Width / Height accessors after SetSize.
func TestModalShell_SetSize(t *testing.T) {
	var s ModalShell
	s = s.SetSize(80, 24)
	if s.Width() != 80 || s.Height() != 24 {
		t.Fatalf("SetSize(80,24): got (%d,%d), want (80,24)", s.Width(), s.Height())
	}
	s = s.SetSize(120, 40)
	if s.Width() != 120 || s.Height() != 40 {
		t.Fatalf("SetSize(120,40): got (%d,%d), want (120,40)", s.Width(), s.Height())
	}
}

// TestModalShell_BoxWidth verifies that every line of Box output has equal
// width equal to contentW+2.
func TestModalShell_BoxWidth(t *testing.T) {
	cases := []struct {
		name       string
		title      string
		bodyRows   []string
		footerRows []string
		contentW   int
	}{
		{
			name:       "single body row",
			title:      "Hello",
			bodyRows:   []string{"one row here        "},
			footerRows: []string{"footer row here     "},
			contentW:   20,
		},
		{
			name:       "empty body rows",
			title:      "Empty",
			bodyRows:   nil,
			footerRows: []string{"press y             "},
			contentW:   20,
		},
		{
			name:       "no footer rows",
			title:      "No Footer",
			bodyRows:   []string{"body only           "},
			footerRows: nil,
			contentW:   20,
		},
		{
			name:       "multi body row",
			title:      "Multi",
			bodyRows:   []string{"line one            ", "line two            ", "line three          "},
			footerRows: []string{"hint row            "},
			contentW:   20,
		},
		{
			// contentW==2 exposes the off-by-one in title truncation:
			// maxTitleW-2 == -1 without the clamp, producing a 5-cell top border.
			name:       "minimum contentW",
			title:      "X",
			bodyRows:   []string{"  "},
			footerRows: []string{"  "},
			contentW:   2,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			box := buildShellBox(tc.title, tc.bodyRows, tc.footerRows, tc.contentW)
			lines, widths := lineWidths(box)
			if len(lines) == 0 {
				t.Fatal("box produced no lines")
			}
			want := tc.contentW + 2
			for i, w := range widths {
				if w != want {
					t.Errorf("line %d width = %d, want %d: %q", i, w, want, lines[i])
				}
			}
		})
	}
}

// TestModalShell_TitleTruncation verifies that an overly-long title is
// truncated so the top border stays at contentW+2 cells.
func TestModalShell_TitleTruncation(t *testing.T) {
	longTitle := "This Is A Very Long Title That Exceeds The Available Space Completely"
	contentW := 20
	box := buildShellBox(longTitle, []string{"body row            "}, []string{"foot row            "}, contentW)
	lines, widths := lineWidths(box)
	if len(lines) == 0 {
		t.Fatal("box produced no lines")
	}
	want := contentW + 2
	for i, w := range widths {
		if w != want {
			t.Errorf("line %d width = %d, want %d: %q", i, w, want, lines[i])
		}
	}
	// Verify the top border does not start with the full title.
	topBorder := lines[0]
	if strings.Contains(topBorder, longTitle) {
		t.Errorf("top border contains untrimmed title: %q", topBorder)
	}
}

// TestModalShell_BoxGolden captures golden output for Box at two typical
// sizes used in the overlay golden tests. Each golden records the raw box
// string (before overlay compositing) so the test is self-contained.
func TestModalShell_BoxGolden(t *testing.T) {
	cases := []struct {
		name       string
		title      string
		bodyRows   []string
		footerRows []string
		contentW   int
	}{
		{
			// Matches a 28-cell-wide modal (contentW=26) at a narrow terminal.
			name:  "modal_shell_narrow.txt",
			title: "Confirm",
			bodyRows: []string{
				"Delete all messages?      ",
			},
			footerRows: []string{
				"[y] yes   [n] no          ",
			},
			contentW: 26,
		},
		{
			// Matches a 60-cell-wide modal (contentW=58) at a typical terminal.
			name:  "modal_shell_wide.txt",
			title: "Empty Trash",
			bodyRows: []string{
				"Permanently delete all 42 messages in Trash?             ",
			},
			footerRows: []string{
				"[y] yes   [n] no   [esc] cancel                          ",
			},
			contentW: 58,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := buildShellBox(tc.title, tc.bodyRows, tc.footerRows, tc.contentW)
			checkGolden(t, tc.name, got)
		})
	}
}
