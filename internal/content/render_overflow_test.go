// SPDX-License-Identifier: MIT

package content

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/glw907/poplar/internal/theme"
)

// TestFootnoteURLNoOverflow is the regression test for F1: footnote URL
// lines must not exceed the requested width. A long unsubscribe URL
// (the Dave Johnson message failure mode) was rendered verbatim without
// wrapping, causing the viewport to emit lines wider than maxBodyWidth
// and displacing content into the sidebar column.
func TestFootnoteURLNoOverflow(t *testing.T) {
	longURL := "https://lists.example.org/mailman/options/asc-membership-committee/someone%40example.com?password_reminder=really-long-token-that-pushes-the-line-over-72-cells"
	blocks := []Block{
		Paragraph{Spans: []Span{
			Text{Content: "To unsubscribe, visit "},
			Link{Text: "this link", URL: longURL},
		}},
	}

	for _, w := range []int{40, 72, 100} {
		t.Run(fmt.Sprintf("width=%d", w), func(t *testing.T) {
			out, urls := RenderBodyWithFootnotes(blocks, theme.Nord, w)
			if len(urls) != 1 {
				t.Fatalf("expected 1 url, got %d", len(urls))
			}
			if !strings.Contains(ansi.Strip(out), "[^1]:") {
				t.Errorf("footnote entry missing from output:\n%s", out)
			}
			widthCap := w
			if widthCap > maxBodyWidth {
				widthCap = maxBodyWidth
			}
			for i, line := range strings.Split(out, "\n") {
				cw := lipgloss.Width(line)
				if cw > widthCap {
					t.Errorf("line %d exceeds cap %d (width=%d): %q", i+1, widthCap, cw, line)
				}
			}
		})
	}
}

// TestRenderFixturesNoOverflow is a property-style test: every markdown
// fixture in testdata/ rendered at widths 40, 72, 100 must produce no
// line wider than the effective cap (min(width, maxBodyWidth)).
func TestRenderFixturesNoOverflow(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	if len(entries) == 0 {
		t.Skip("no fixtures in testdata/")
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := e.Name()
		path := filepath.Join("testdata", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		blocks := ParseBlocks(string(data))

		for _, w := range []int{40, 72, 100} {
			t.Run(fmt.Sprintf("%s/width=%d", name, w), func(t *testing.T) {
				widthCap := w
				if widthCap > maxBodyWidth {
					widthCap = maxBodyWidth
				}
				out, _ := RenderBodyWithFootnotes(blocks, theme.Nord, w)
				for i, line := range strings.Split(out, "\n") {
					cw := lipgloss.Width(line)
					if cw > widthCap {
						t.Errorf("line %d exceeds cap %d (width=%d): %q",
							i+1, widthCap, cw, line)
					}
				}
			})
		}
	}
}
