package main

import (
	"fmt"
	"testing"

	"github.com/glw907/poplar/internal/ui"
)

// rungIndex returns sketchRungs' index of the w×h rung, failing the
// test if none matches: a guard against this file's own rung
// selection drifting from sketchRungs rather than a fixture's chosen
// size silently landing on the wrong one.
func rungIndex(t *testing.T, w, h int) int {
	t.Helper()
	for i, r := range sketchRungs {
		if r.width == w && r.height == h {
			return i
		}
	}
	t.Fatalf("no sketchRungs entry for %dx%d", w, h)
	return -1
}

// TestComputeFrame_MatchesComposeView is task 2 of pass 2c's equality
// guard, cmd/sketch's side of it: computeFrame's own output, for
// every fixture sketch cycles at a representative rung, is exactly
// what ui.ComposeView returns computed independently from the same
// fixture, the same active/stack split sketchStackFixtures records.
// Reverting computeFrame to call ui.Render directly (dropping
// ComposeView, so FullRegion and the modal/help stack treatment)
// fails every case here.
func TestComputeFrame_MatchesComposeView(t *testing.T) {
	p := sketchProfiles[0]

	for fi, f := range sketchFixtures {
		t.Run(f.Name, func(t *testing.T) {
			width, height := 100, 30
			switch f.Name {
			case "floor":
				width, height = 40, 10
			case "short":
				width, height = 100, 16
			}
			ri := rungIndex(t, width, height)

			m := sketchModel{fixture: fi, profile: 0, rung: ri}
			got, _ := m.computeFrame()

			lm := ui.ComputeLayout(width, height, f.Banner.Active)
			updated, _ := f.Build(p.theme).Update(ui.LayoutMsg{Layout: lm})
			scr, ok := updated.(ui.Screen)
			if !ok {
				t.Fatalf("Update returned %T, want ui.Screen", updated)
			}

			var active ui.Screen
			var stack []ui.Screen
			if sketchStackFixtures[f.Name] {
				stack = []ui.Screen{scr}
			} else {
				active = scr
			}
			frame := ui.ComposeView(lm, p.theme, f.Status, f.Banner, active, stack)
			status := fmt.Sprintf("fixture %s   profile %s   rung %s   ? help", f.Name, p.name, sketchRungs[ri])
			want := frame.Content + "\n" + status

			if got != want {
				t.Errorf("computeFrame() for fixture %s diverged from ui.ComposeView's own frame", f.Name)
			}
		})
	}
}
