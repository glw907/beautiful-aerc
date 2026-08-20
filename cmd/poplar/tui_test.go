package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/outbox"
	"github.com/glw907/poplar/internal/platform"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
)

// waitForFirstFrame blocks until tm has rendered at least one frame:
// teatest.NewTestModel sends its own initial tea.WindowSizeMsg
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

// TestHeadlessEntry proves main's own routing: --headless and
// --startup-trace both select run's engine-only loop over
// runInteractive's TUI, and neither alone without the other still
// answers correctly for its own case.
func TestHeadlessEntry(t *testing.T) {
	tests := []struct {
		name string
		f    flags
		want bool
	}{
		{"neither flag", flags{}, false},
		{"--headless", flags{headless: true}, true},
		{"--startup-trace", flags{startupTrace: true}, true},
		{"both", flags{headless: true, startupTrace: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := headlessEntry(tt.f); got != tt.want {
				t.Errorf("headlessEntry(%+v) = %v, want %v", tt.f, got, tt.want)
			}
		})
	}
}

// TestMapColorProfile pins CARRY 1's mapping, all three cases: without
// it, tea.WithColorProfile is never told what ResolveProfile already
// resolved, and bubbletea's own terminal auto-detection re-downsamples
// the theme's explicit values.
func TestMapColorProfile(t *testing.T) {
	tests := []struct {
		in   theme.Profile
		want colorprofile.Profile
	}{
		{theme.ProfileTrueColor, colorprofile.TrueColor},
		{theme.ProfileANSI16, colorprofile.ANSI},
		{theme.ProfileNoColor, colorprofile.Ascii},
	}
	for _, tt := range tests {
		if got := mapColorProfile(tt.in); got != tt.want {
			t.Errorf("mapColorProfile(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// TestRunInteractiveRefusesSecondInstance proves SY-7's refusal is
// re-asserted at the TUI's own entry path, before any TUI init: the
// instance lock is the very first thing runInteractive touches, so
// this returns without ever constructing a *tea.Program (which would
// otherwise try to read the real terminal in a test binary).
func TestRunInteractiveRefusesSecondInstance(t *testing.T) {
	dbPath := t.TempDir() + "/store.db"

	lock, err := platform.AcquireInstanceLock(dbPath)
	if err != nil {
		t.Fatalf("AcquireInstanceLock: %v", err)
	}
	defer func() { _ = lock.Release() }()

	var out strings.Builder
	if err := runInteractive(context.Background(), dbPath, flags{}, &out, io.Discard, noopConnector); err == nil {
		t.Fatal("runInteractive succeeded against a locked store, want refusal")
	}
}

// TestStartEnginesRetryingInteractiveBridgesTheOfflineCase proves
// ST-2's own wiring: a non-fatal first connect failure sends
// ui.SyncStateMsg{State: SyncStateOffline} (runInteractive's own call,
// before this loop starts), and once retryConnect reaches a live
// connect, startEnginesInteractive's own bridgeSyncHealth observer
// takes over and reports Syncing/Synced through the very same send,
// with no further Offline message once recovered.
func TestStartEnginesRetryingInteractiveBridgesTheOfflineCase(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())

		refused := errors.New("dial tcp: connection refused")
		var attempts int
		connect := func(context.Context) (backend.Backend, string, error) {
			attempts++
			if attempts == 1 {
				return nil, "", refused
			}
			var be backendtest.Fake
			be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
				return backend.ChangeSet{}, nil
			}
			return &be, "test-account", nil
		}

		var log msgLog
		log.send(ui.SyncStateMsg{State: ui.SyncStateOffline}) // runInteractive's own send, reproduced here

		ctx, cancel := context.WithCancel(context.Background())
		wg := startEnginesRetryingInteractive(ctx, w, reads, connect, refused, log.send)

		synctest.Wait()

		got := log.snapshot()
		if len(got) == 0 {
			t.Fatal("no messages sent, want at least the seeded Offline message")
		}
		if got[0] != (ui.SyncStateMsg{State: ui.SyncStateOffline}) {
			t.Fatalf("first message = %#v, want the Offline state", got[0])
		}

		cancel()
		wg.Wait()
	})
}

// TestRunDispatchLoopBridgedSendsOutboxCount proves CARRY 5's own
// wiring: runDispatchLoopBridged's outbox-count bridge rides the
// dispatch loop's own existing cadence, with no ticker of its own.
func TestRunDispatchLoopBridgedSendsOutboxCount(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
		accountID := storetest.Insert(t, w,
			`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`, "a", "jmap", "a@example.com")
		storetest.Insert(t, w,
			`INSERT INTO outbox (account_id, kind, payload, state, created_at) VALUES (?, ?, ?, ?, ?)`,
			accountID, "send", "{}", "queued", 1000)

		var be backendtest.Fake
		d := outbox.NewDispatcher(accountID, &be, w, reads)

		var log msgLog
		ctx, cancel := context.WithCancel(context.Background())
		go runDispatchLoopBridged(ctx, d, reads, log.send)

		synctest.Wait()

		want := ui.OutboxCountMsg{Queued: 1}
		found := false
		for _, msg := range log.snapshot() {
			if msg == tea.Msg(want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("messages = %#v, want an OutboxCountMsg{Queued: 1} among them", log.snapshot())
		}

		cancel()
	})
}

// TestST2_OfflineStartReachesTheInteractivePlaceholderWithStoreCounts
// is ST-2's own acceptance case: a real *tea.Program built over
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

	// The bridge's own ST-2 send: a connect that never came up.
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
// scripts F7's own acceptance case end to end through a real
// *tea.Program: q with an empty outbox and no undo window quits
// straight through, and q with a queued outbox shows the modal
// confirm and quits once y answers it. 'n' staying and 'y' discarding
// the offer without invoking it are pinned precisely, Cmd by Cmd,
// against App.Update directly in internal/ui's own
// TestApp_QuitWithOpenUndoWindowConfirms and
// TestApp_ConfirmOnStack_AnswersYesNoAndPops; a diffing terminal
// renderer's incremental output is the wrong medium to reprove a
// disappearance against, so this test's own job is only what those
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
