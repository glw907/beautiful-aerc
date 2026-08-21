package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
)

// This file is the teatest flow suite (task 12, survey amendment A's
// closing requirement): every test here drives a real *tea.Program
// end to end, deliberately kept small, five scripts total, and
// nothing the render seam's goldens or the gallery already cover
// at the model level. The teatest swap path: charmbracelet's teatest
// lives under the experimental x/exp path, so a bubbletea bump it has
// not caught up with swaps for a hand-rolled harness over
// tea.WithInput/tea.WithOutput piping keystrokes in and reading
// rendered frames back, the same virtual-terminal shape teatest
// itself wraps. Only the five functions in this file move: nothing
// else in the module imports teatest, and every committed gallery
// file is plain text produced by ui.Render directly, so a swap here
// touches no golden.

// waitForFirstFrame blocks until tm has rendered at least one frame:
// teatest.NewTestModel sends its initial tea.WindowSizeMsg
// through Program.Send after starting Program.Run in a goroutine, so
// a Type or Send issued immediately after NewTestModel returns can
// race that size message. App.Update needs a WindowSizeMsg before
// LayoutMode carries a real width and height, and pushing Confirm (a
// natural-size, clamped-and-centered box) onto the stack before one
// ever arrives renders against a zero-size layout.
func waitForFirstFrame(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "Mail")
	}, teatest.WithDuration(3*time.Second))
}

// TestST2_OfflineStartReachesTheInteractivePlaceholderWithStoreCounts
// is ST-2's acceptance case: a real *tea.Program built over
// ui.NewApp against a store fixture holding real messages and
// mailboxes reaches an interactive frame naming both the store's
// counts and, once told the connection never came up, "Offline" on
// the status line, driven end to end through teatest rather than a
// bare Update call.
func TestST2_OfflineStartReachesTheInteractivePlaceholderWithStoreCounts(t *testing.T) {
	w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
	accountID := storetest.Insert(t, w,
		`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`, "a", "jmap", "a@example.com")
	storetest.Insert(t, w, `INSERT INTO mailbox (account_id, name) VALUES (?, ?)`, accountID, "Inbox")
	storetest.Insert(t, w, `INSERT INTO message (account_id, received_at) VALUES (?, ?)`, accountID, 1000)
	storetest.Insert(t, w, `INSERT INTO message (account_id, received_at) VALUES (?, ?)`, accountID, 2000)
	storetest.Insert(t, w, `INSERT INTO message (account_id, received_at) VALUES (?, ?)`, accountID, 3000)

	app := ui.NewApp(ui.Deps{Store: reads, Theme: theme.New(true, theme.ProfileTrueColor), Profile: theme.ProfileTrueColor})
	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(100, 30),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.TrueColor)))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "3 messages in 1 folders")
	}, teatest.WithDuration(3*time.Second))

	// The bridge's ST-2 send: a connect that never came up.
	tm.Send(ui.SyncStateMsg{State: ui.SyncStateOffline})

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "Offline")
	}, teatest.WithDuration(3*time.Second))

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestQuitPath_EmptyOutboxExitsCleanAndConfirmsWithQueuedIntents
// scripts F7's acceptance case end to end through a real
// *tea.Program: q with an empty outbox and no undo window quits
// straight through, and q with a queued outbox shows the modal
// confirm and quits once y answers it. 'n' staying and 'y' discarding
// the offer without invoking it are pinned precisely, Cmd by Cmd,
// against App.Update directly in internal/ui's
// TestApp_QuitWithOpenUndoWindowConfirms and
// TestApp_ConfirmOnStack_AnswersYesNoAndPops; a diffing terminal
// renderer's incremental output is the wrong medium to reprove a
// disappearance against, so this test's job is only what those
// cannot cover: a real *tea.Program actually renders F7's text and
// actually quits on y.
func TestQuitPath_EmptyOutboxExitsCleanAndConfirmsWithQueuedIntents(t *testing.T) {
	t.Run("empty outbox quits clean", func(t *testing.T) {
		reads := storetest.OpenReadPool(t, store.DefaultWriterConfig())
		app := ui.NewApp(ui.Deps{Store: reads, Theme: theme.New(true, theme.ProfileTrueColor), Profile: theme.ProfileTrueColor})
		tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(100, 30))
		waitForFirstFrame(t, tm)

		tm.Type("q")
		tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	})

	t.Run("queued outbox shows the confirm, y quits", func(t *testing.T) {
		reads := storetest.OpenReadPool(t, store.DefaultWriterConfig())
		app := ui.NewApp(ui.Deps{Store: reads, Theme: theme.New(true, theme.ProfileTrueColor), Profile: theme.ProfileTrueColor})
		tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(100, 30))
		waitForFirstFrame(t, tm)

		tm.Send(ui.OutboxCountMsg{Queued: 2})
		tm.Type("q")

		teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
			return strings.Contains(string(b), "Quit with 2 unsent messages?")
		}, teatest.WithDuration(3*time.Second))

		tm.Type("y")
		tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	})
}

// TestFlow_SurfaceSwitchRoundTrip is UX-4's round trip
// (TestApp_DigitSurfaceSwitchRoundTrip's model-level proof,
// internal/ui/app_test.go) driven through a real *tea.Program: every
// digit switches to its surface's title text, and the 1->2->3->4->1
// cycle ends back on Mail.
func TestFlow_SurfaceSwitchRoundTrip(t *testing.T) {
	reads := storetest.OpenReadPool(t, store.DefaultWriterConfig())
	app := ui.NewApp(ui.Deps{Store: reads, Theme: theme.New(true, theme.ProfileTrueColor), Profile: theme.ProfileTrueColor})
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(100, 30))
	waitForFirstFrame(t, tm)

	steps := []struct{ key, want string }{
		{"2", "Calendar"},
		{"3", "People"},
		{"4", "Config"},
		{"1", "Mail"},
	}
	for _, s := range steps {
		tm.Type(s.key)
		teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
			return strings.Contains(string(b), s.want)
		}, teatest.WithDuration(3*time.Second))
	}

	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// groundBackgroundSGR is the exact standalone SGR sequence
// theme.New(isDark, ProfileTrueColor)'s GroundBase paints
// (theme.Blank, no foreground alongside it): a blank Main row, most
// of any placeholder's frame, carries this sequence verbatim.
func groundBackgroundSGR(isDark bool) string {
	hex := theme.GroundHex(theme.GroundBase, isDark)
	r, _ := strconv.ParseInt(hex[0:2], 16, 32)
	g, _ := strconv.ParseInt(hex[2:4], 16, 32)
	b, _ := strconv.ParseInt(hex[4:6], 16, 32)
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

// TestFlow_NeverAnsweringTerminalStaysDark is task 2's
// never-answering-terminal case, driven through a real *tea.Program
// rather than the direct App.Update harness
// TestRender_NeverAnsweringTerminalStaysDark already covers
// (repaint_test.go, internal/ui): teatest's virtual terminal
// never answers App.Init's tea.RequestBackgroundColor query, so the
// BackgroundColorWait this test sleeps past is the real timeout path
// a live terminal that never replies takes, not a message this test
// injects itself. tm.Output() drains destructively, and
// BackgroundColorTimeoutMsg's correct handler repaints nothing
// (app.go), so this reads the raw accumulated stream once, after the
// sleep, rather than through waitForFirstFrame's "Mail" wait, which
// would already have consumed the one frame carrying the dark SGR:
// asserting nothing about color would still pass even with the
// timeout branch mutated to repaint light, so the color check is
// what makes this a real mutation guard rather than theatre.
// tea.WithColorProfile(colorprofile.TrueColor) pins
// the comparison against environment-dependent downsampling, the same
// reason the ST-2 test above sets it.
func TestFlow_NeverAnsweringTerminalStaysDark(t *testing.T) {
	reads := storetest.OpenReadPool(t, store.DefaultWriterConfig())
	app := ui.NewApp(ui.Deps{Store: reads, Theme: theme.New(true, theme.ProfileTrueColor), Profile: theme.ProfileTrueColor})
	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(100, 30),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.TrueColor)))

	time.Sleep(2 * ui.BackgroundColorWait)

	out, err := io.ReadAll(tm.Output())
	if err != nil {
		t.Fatalf("read program output: %v", err)
	}
	dark, light := groundBackgroundSGR(true), groundBackgroundSGR(false)
	if !strings.Contains(string(out), dark) {
		t.Fatalf("no frame past BackgroundColorWait carried the dark ground's background SGR %q", dark)
	}
	if strings.Contains(string(out), light) {
		t.Errorf("output carries the light ground's background SGR %q; an unanswered query must never repaint light", light)
	}

	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
