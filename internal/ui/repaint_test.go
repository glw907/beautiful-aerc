package ui_test

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
)

// newTestApp returns an App wired to a fresh, empty storetest pool
// (so its mail placeholder's real store facts are the same zero
// counts fixtures.Mail pins), with Init's whole Cmd tree drained and
// a 100×30 WindowSizeMsg applied: the live wiring a running
// tea.Program produces before its first paint, reproduced here since
// no test in this package runs a real Program.
func newTestApp(t *testing.T) ui.App {
	t.Helper()
	reads := storetest.OpenReadPool(t, store.DefaultWriterConfig())
	deps := ui.Deps{
		Store:   reads,
		Theme:   theme.New(ui.DefaultDark, theme.ProfileTrueColor),
		Profile: theme.ProfileTrueColor,
		Account: "geoff@907.life",
	}
	app := ui.NewApp(deps)
	app = runCmd(t, app, app.Init())
	return updateApp(t, app, tea.WindowSizeMsg{Width: 100, Height: 30})
}

// updateApp applies msg to app and drains whatever Cmd it returns.
func updateApp(t *testing.T, app ui.App, msg tea.Msg) ui.App {
	t.Helper()
	updated, cmd := app.Update(msg)
	return runCmd(t, updated.(ui.App), cmd) //nolint:errcheck // App's Update always returns an App; the assertion's panic is the message
}

// runCmd executes cmd and feeds every message it produces, including
// each sub-command of a tea.BatchMsg recursively, back into
// app.Update: a nil cmd is the recursion's base case. Draining the
// batch this way is what actually lets App's background-color
// query time out for real inside TestRender_NeverAnsweringTerminalStaysDark
// (BackgroundColorWait, no answer ever delivered), rather than this
// test skipping the mechanism it claims to prove.
func runCmd(t *testing.T, app ui.App, cmd tea.Cmd) ui.App {
	t.Helper()
	if cmd == nil {
		return app
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			app = runCmd(t, app, sub)
		}
		return app
	}
	return updateApp(t, app, msg)
}

// TestRender_NeverAnsweringTerminalStaysDark is the carried
// never-answering-terminal test: a real App, driven through its Init
// and a WindowSizeMsg with no tea.BackgroundColorMsg ever delivered,
// renders exactly the gallery's committed dark file for the mail
// fixture at the same size: the terminal that never answers gets
// exactly the frame the gallery already pins.
func TestRender_NeverAnsweringTerminalStaysDark(t *testing.T) {
	app := newTestApp(t)
	assertMatchesGalleryFile(t, "mail-100x30-truecolor-dark", app.View().Content)
}

// TestRender_BackgroundColorRepaintGoldenPair is the carried golden
// pair, driven through a real App: the pre-answer dark render matches
// the gallery's committed dark file, and delivering a real
// tea.BackgroundColorMsg{Color: color.White} through app.Update
// repaints App.View().Content to match the gallery's committed light
// file, byte for byte.
func TestRender_BackgroundColorRepaintGoldenPair(t *testing.T) {
	app := newTestApp(t)
	assertMatchesGalleryFile(t, "mail-100x30-truecolor-dark", app.View().Content)

	app = updateApp(t, app, tea.BackgroundColorMsg{Color: color.White})
	assertMatchesGalleryFile(t, "mail-100x30-truecolor-light", app.View().Content)
}

// TestGallery_TwoSweepsByteIdentical is QA-7's assertion: two
// full in-process sweeps of the gallery matrix return byte-identical
// output, case for case, independent of any committed file.
func TestGallery_TwoSweepsByteIdentical(t *testing.T) {
	first := sweepGallery()
	second := sweepGallery()
	if len(first) != len(second) {
		t.Fatalf("sweep sizes differ: %d vs %d", len(first), len(second))
	}
	for name, a := range first {
		b, ok := second[name]
		if !ok {
			t.Errorf("%s: present in the first sweep, absent from the second", name)
			continue
		}
		if a != b {
			t.Errorf("%s: the two sweeps diverged", name)
		}
	}
}

// sweepGallery renders every galleryCases × its profiles point
// once, keyed by the same name checkGallery persists under.
func sweepGallery() map[string]string {
	out := make(map[string]string)
	for _, c := range galleryCases {
		for _, sz := range c.sizes {
			for _, p := range c.profiles() {
				name := c.fixture.Name + "-" + sz.String() + "-" + p.name
				out[name] = galleryRender(c, sz, p.theme)
			}
		}
	}
	return out
}

// assertMatchesGalleryFile asserts got, escaped the same way the
// gallery escapes its renders, equals the committed gallery file
// name.txt.
func assertMatchesGalleryFile(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(galleryDir, name+".txt")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed gallery render %s: %v", path, err)
	}
	if escapeGalleryOutput(got) != string(want) {
		t.Errorf("render does not match committed gallery file %s", path)
	}
}
