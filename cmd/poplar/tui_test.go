package main

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/outbox"
	"github.com/glw907/poplar/internal/platform"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
	"github.com/glw907/poplar/internal/ui"
)

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
	err = runInteractive(context.Background(), dbPath, flags{}, &out, io.Discard, noopConnector)
	if err == nil {
		t.Fatal("runInteractive succeeded against a locked store, want refusal")
	}
	// Asserting err != nil alone would pass for any store failure at
	// all; the actionable pid message SY-7 promises lives specifically
	// behind ClassInstanceLocked (fix round 1 finding 9, m5).
	_ = uerrtest.AssertClass(t, err, uerr.ClassInstanceLocked)
}

// TestStartEnginesRetryingInteractiveBridgesTheOfflineCase proves
// ST-2's own wiring against real reconnect-then-sync traffic, not a
// seeded message this test then finds in its own log (fix round 1
// finding 4, M1: the prior version pushed SyncStateMsg{Offline} into
// its own log before calling the code under test, so the assertion
// passed even with an empty function body). connect fails once, then
// succeeds; startEnginesRetryingInteractive's own retryConnect loop
// reaches the live backend, and its bridgeSyncHealth observer (set
// before RunPush's goroutine starts) reports the real Syncing/Synced
// transition a pushed notification drives, with StoreChangedMsg
// behind Synced and no Offline message anywhere in this loop's own
// traffic: initialSyncMsg is runInteractive's own call, not this
// loop's, and TestInitialSyncMsg covers it directly.
func TestStartEnginesRetryingInteractiveBridgesTheOfflineCase(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())

		refused := errors.New("dial tcp: connection refused")
		notify := make(chan backend.Notification, 1)
		var attempts atomic.Int64
		connect := func(context.Context) (backend.Backend, string, error) {
			if attempts.Add(1) == 1 {
				return nil, "", refused
			}
			var be backendtest.Fake
			be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
				return backend.ChangeSet{}, nil
			}
			be.PushSource = &backendtest.FakePush{ListenFunc: func(context.Context) (<-chan backend.Notification, error) {
				return notify, nil
			}}
			return &be, "test-account", nil
		}

		var log msgLog
		ctx, cancel := context.WithCancel(context.Background())
		wg := startEnginesRetryingInteractive(ctx, w, reads, connect, refused, log.send)

		// Past both dial backoffs: engine.go's dialBackoffMin/Max are
		// 500ms/30s, full jitter, so attempt 0's wait is at most 500ms
		// and attempt 1's at most 1s; 2s clears both with room to
		// spare.
		time.Sleep(2 * time.Second)
		synctest.Wait()

		if got := attempts.Load(); got != 2 {
			t.Fatalf("connect calls = %d, want 2 (one failure, one reconnect)", got)
		}
		if got := log.snapshot(); len(got) != 0 {
			t.Fatalf("messages after reconnect alone = %#v, want none: a live stream with no traffic yet sends nothing", got)
		}

		notify <- backend.Notification{}
		// Past CoalesceWindow (200ms) and InteractiveQuiet (1s):
		// ensureAccount's own insert, moments earlier, is itself an
		// interactive-lane write the bulk lane's flush subordinates
		// behind (ADR-0003 revision 2).
		time.Sleep(2 * time.Second)
		synctest.Wait()

		want := []tea.Msg{
			ui.SyncStateMsg{State: ui.SyncStateSyncing},
			ui.SyncStateMsg{State: ui.SyncStateSynced},
			ui.StoreChangedMsg{},
		}
		if got := log.snapshot(); !slices.Equal(got, want) {
			t.Fatalf("messages = %#v, want %#v", got, want)
		}

		cancel()
		wg.Wait()
	})
}

// TestInitialSyncMsg pins the seam finding 4 extracted: nil connectErr
// (the first connect already succeeded) sends nothing, since
// bridgeSyncHealth's own observer takes over from there; any other
// error reports ST-2's own Offline state, the one sync.State itself
// has no room for.
func TestInitialSyncMsg(t *testing.T) {
	if got := initialSyncMsg(nil); got != nil {
		t.Errorf("initialSyncMsg(nil) = %#v, want nil", got)
	}

	err := errors.New("dial tcp: connection refused")
	want := ui.SyncStateMsg{State: ui.SyncStateOffline}
	if got := initialSyncMsg(err); got != want {
		t.Errorf("initialSyncMsg(err) = %#v, want %#v", got, want)
	}
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
