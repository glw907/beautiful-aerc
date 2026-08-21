package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/platform"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
	"github.com/glw907/poplar/internal/ui"
)

// TestHeadlessEntry proves main's routing: --headless and
// --startup-trace both select run's engine-only loop over
// runInteractive's TUI, and neither alone without the other still
// answers correctly for its case.
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

// TestRunInteractiveRefusesSecondInstance proves SY-7's refusal is
// re-asserted at the TUI's entry path, before any TUI init: the
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
	// behind ClassInstanceLocked.
	_ = uerrtest.AssertClass(t, err, uerr.ClassInstanceLocked)
}

// TestLogFallbackBanner proves logFallbackBanner's two user-facing
// message strings, the two dispositions row 24 owes an operator: a
// working fallback names the path it engaged, and a fallback that
// failed its own trial write too says logging is degraded rather than
// naming a path nothing reaches. uerrtest.Capture keeps
// LogFallbackPath/LogDegraded's first-ever call in this test binary
// from actually opening the real state-dir log file.
func TestLogFallbackBanner(t *testing.T) {
	uerrtest.Capture(t)

	t.Run("fallback engaged", func(t *testing.T) {
		restore := uerr.SetLogFallbackForTest("/tmp/poplar.log", false)
		defer restore()

		msg, ok := logFallbackBanner()
		if !ok {
			t.Fatal("logFallbackBanner() ok = false, want true")
		}
		want := "state directory unavailable; logging to /tmp/poplar.log instead"
		if msg.Message != want {
			t.Errorf("Message = %q, want %q", msg.Message, want)
		}
	})

	t.Run("both destinations failed", func(t *testing.T) {
		restore := uerr.SetLogFallbackForTest("", true)
		defer restore()

		msg, ok := logFallbackBanner()
		if !ok {
			t.Fatal("logFallbackBanner() ok = false, want true")
		}
		want := "state directory unavailable and logging is degraded; some lines may be lost"
		if msg.Message != want {
			t.Errorf("Message = %q, want %q", msg.Message, want)
		}
	})

	t.Run("neither state", func(t *testing.T) {
		restore := uerr.SetLogFallbackForTest("", false)
		defer restore()

		if _, ok := logFallbackBanner(); ok {
			t.Error("logFallbackBanner() ok = true, want false when the log is at its normal home")
		}
	})
}

// TestInitialSyncMsg pins runInteractive's startup-connect signal: nil
// connectErr (the first connect already succeeded) sends nothing,
// since bridgeSyncHealth's observer takes over from there; any other
// error reports ST-2's Offline state, the one sync.State itself
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
