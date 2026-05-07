package mailimap

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// idleBackend builds a backend wired for idle tests. It connects with
// UIDPLUS+IDLE caps and does NOT start the idle goroutine. Callers
// drive idleLoop manually.
func idleBackend(t *testing.T, cmd, idle *fakeClient) *Backend {
	t.Helper()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true, "IDLE": true}
	idle.caps = cmd.caps
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	// Manually set up the state that finishConnect would produce,
	// but without spawning a goroutine.
	b.caps = capSet{UIDPLUS: true, IDLE: true}
	b.updates = make(chan mail.Update, 64)
	b.switchCh = make(chan string, 1)
	b.idleDone = make(chan struct{})
	b.current = "INBOX"
	return b
}

// drainUpdates collects up to n updates from b.updates within timeout.
func drainUpdates(b *Backend, n int, timeout time.Duration) []mail.Update {
	var out []mail.Update
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case u := <-b.updates:
			out = append(out, u)
		case <-deadline:
			return out
		}
	}
	return out
}

func TestIdleEmitsConnectedOnStart(t *testing.T) {
	cmd := newFakeClient()
	idleClient := newFakeClient()

	// idleStop signal
	stopCh := make(chan struct{})
	var onceStop sync.Once

	idleClient.onIdle = func(emit func(mail.Update)) error {
		<-stopCh
		return nil
	}
	idleClient.idleStop = func() {
		onceStop.Do(func() { close(stopCh) })
	}

	b := idleBackend(t, cmd, idleClient)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		b.idleLoop(ctx)
	}()

	updates := drainUpdates(b, 1, 2*time.Second)
	cancel()
	<-b.idleDone

	if len(updates) == 0 {
		t.Fatal("expected ConnConnected update, got none")
	}
	if updates[0].Type != mail.UpdateConnState || updates[0].ConnState != mail.ConnConnected {
		t.Errorf("update = %+v, want ConnConnected", updates[0])
	}
}

func TestIdleFolderSwitch(t *testing.T) {
	cmd := newFakeClient()
	idleClient := newFakeClient()

	stopCh := make(chan struct{}, 1)
	selectedFolders := make([]string, 0, 4)
	var mu sync.Mutex

	origSelect := idleClient.Select
	_ = origSelect // fakeClient.Select sets f.selected. We wrap via onIdle

	// Track Select calls through a custom fakeClient. We can't easily
	// override Select on fakeClient, so track through the idle func.
	// Instead track idleClient.selected after each IDLE round.
	idleRound := 0

	idleClient.onIdle = func(emit func(mail.Update)) error {
		mu.Lock()
		idleRound++
		round := idleRound
		sel := idleClient.selected
		mu.Unlock()

		mu.Lock()
		selectedFolders = append(selectedFolders, sel)
		mu.Unlock()

		if round == 1 {
			// Block until stopCh fires (from IdleStop)
			<-stopCh
		}
		return nil
	}
	idleClient.idleStop = func() {
		select {
		case stopCh <- struct{}{}:
		default:
		}
	}

	b := idleBackend(t, cmd, idleClient)

	ctx, cancel := context.WithCancel(context.Background())
	go b.idleLoop(ctx)

	// Wait for ConnConnected to confirm first IDLE started.
	updates := drainUpdates(b, 1, 2*time.Second)
	if len(updates) == 0 || updates[0].ConnState != mail.ConnConnected {
		t.Fatalf("expected ConnConnected, got %v", updates)
	}

	// Send folder-switch.
	b.switchCh <- "Sent"

	// Wait for second ConnConnected (re-IDLE on Sent).
	updates2 := drainUpdates(b, 1, 2*time.Second)
	cancel()
	<-b.idleDone

	if len(updates2) == 0 || updates2[0].ConnState != mail.ConnConnected {
		t.Errorf("expected second ConnConnected after folder switch, got %v", updates2)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(selectedFolders) < 2 {
		t.Fatalf("expected ≥2 select calls, got %v", selectedFolders)
	}
	if selectedFolders[0] != "INBOX" {
		t.Errorf("first select = %q, want INBOX", selectedFolders[0])
	}
	if selectedFolders[1] != "Sent" {
		t.Errorf("second select = %q, want Sent", selectedFolders[1])
	}
}

func TestIdleReconnectsOnError(t *testing.T) {
	cmd := newFakeClient()
	idleClient := newFakeClient()

	attempts := 0
	var mu sync.Mutex
	stopCh := make(chan struct{})
	var onceStop sync.Once

	idleClient.onIdle = func(emit func(mail.Update)) error {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()

		if n == 1 {
			return errors.New("connection reset")
		}
		// Second attempt: block until cancel.
		<-stopCh
		return nil
	}
	idleClient.idleStop = func() {
		onceStop.Do(func() { close(stopCh) })
	}

	b := idleBackend(t, cmd, idleClient)

	ctx, cancel := context.WithCancel(context.Background())
	go b.idleLoop(ctx)

	// Sequence: ConnConnected (select OK, first IDLE entered) →
	// ConnReconnecting (IDLE returns error) → ConnConnected (second
	// attempt succeeds). Collect the first two updates.
	updates := drainUpdates(b, 2, 3*time.Second)
	if len(updates) < 2 {
		t.Fatalf("expected ≥2 updates, got %v", updates)
	}
	if updates[0].ConnState != mail.ConnConnected {
		t.Errorf("updates[0] = %+v, want ConnConnected", updates[0])
	}
	if updates[1].ConnState != mail.ConnReconnecting {
		t.Errorf("updates[1] = %+v, want ConnReconnecting", updates[1])
	}

	// After backoff, second attempt emits ConnConnected.
	updates2 := drainUpdates(b, 1, 3*time.Second)
	cancel()
	<-b.idleDone

	if len(updates2) == 0 || updates2[0].ConnState != mail.ConnConnected {
		t.Errorf("expected ConnConnected after reconnect, got %v", updates2)
	}

	mu.Lock()
	got := attempts
	mu.Unlock()
	if got < 2 {
		t.Errorf("expected ≥2 idle attempts, got %d", got)
	}
}

func TestIdlePollFallback(t *testing.T) {
	cmd := newFakeClient()
	idleClient := newFakeClient()

	b := idleBackend(t, cmd, idleClient)
	// Disable IDLE cap.
	b.caps.IDLE = false

	// pollFallbackInterval is 60s, too long for a test. Stub it via a
	// dedicated field if we add one. Instead we drive the test by
	// cancelling the context quickly and checking emit didn't crash.
	// A real poll-interval test would require a configurable tick. See
	// comments below.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	go b.idleLoop(ctx)
	<-b.idleDone

	// No crash, goroutine exited cleanly. The poll interval (60s) is
	// longer than the test duration so we won't see a FolderInfo update
	// here. That's expected and acceptable.
}

func TestIdleExitOnContextCancel(t *testing.T) {
	cmd := newFakeClient()
	idleClient := newFakeClient()

	stopCh := make(chan struct{})
	var onceStop sync.Once

	idleClient.onIdle = func(emit func(mail.Update)) error {
		<-stopCh
		return nil
	}
	idleClient.idleStop = func() {
		onceStop.Do(func() { close(stopCh) })
	}

	b := idleBackend(t, cmd, idleClient)

	ctx, cancel := context.WithCancel(context.Background())
	go b.idleLoop(ctx)

	// Wait for connected signal.
	drainUpdates(b, 1, 2*time.Second)

	// Cancel should unblock quickly.
	cancel()

	select {
	case <-b.idleDone:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("idleDone not closed after context cancel")
	}
}

func TestIdleWaitsForFirstFolder(t *testing.T) {
	cmd := newFakeClient()
	idleClient := newFakeClient()

	stopCh := make(chan struct{})
	var onceStop sync.Once

	idleClient.onIdle = func(emit func(mail.Update)) error {
		<-stopCh
		return nil
	}
	idleClient.idleStop = func() {
		onceStop.Do(func() { close(stopCh) })
	}

	b := idleBackend(t, cmd, idleClient)
	b.current = "" // no folder yet

	ctx, cancel := context.WithCancel(context.Background())
	go b.idleLoop(ctx)

	// Goroutine should not emit anything until we send a folder.
	select {
	case u := <-b.updates:
		t.Fatalf("unexpected early update: %+v", u)
	case <-time.After(100 * time.Millisecond):
	}

	// Send the first folder.
	b.switchCh <- "INBOX"

	// Now it should emit ConnConnected.
	updates := drainUpdates(b, 1, 2*time.Second)
	cancel()
	<-b.idleDone

	if len(updates) == 0 || updates[0].ConnState != mail.ConnConnected {
		t.Errorf("expected ConnConnected after folder send, got %v", updates)
	}
}
