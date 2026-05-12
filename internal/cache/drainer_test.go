package cache

import (
	"context"
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

func (a *Account) pauseDrainerForTest() { a.drainerPaused.Store(true) }

func (a *Account) startBurstForTest(n int) {
	a.burstTotal.Store(int32(n))
	a.burstDone.Store(0)
}

func (a *Account) recordBurstDoneForTest(n int) {
	for range n {
		next := a.burstDone.Add(1)
		if next >= a.burstTotal.Load() {
			a.burstTotal.Store(0)
			a.burstDone.Store(0)
			return
		}
	}
}

func TestOutboxDrainProgress(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	// Idle: no progress.
	if pct, ok := a.OutboxDrainProgress(); ok || pct != 0 {
		t.Fatalf("idle: got (%d, %v), want (0, false)", pct, ok)
	}

	// Queue three sends (drainer paused).
	a.pauseDrainerForTest()
	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"a@example.com"}}
	mime := []byte("hi\r\n")
	for range 3 {
		if _, err := a.QueueOutbound(context.Background(), "Inbox", env, mime, 0, ""); err != nil {
			t.Fatalf("queue: %v", err)
		}
	}
	a.startBurstForTest(3)

	if pct, ok := a.OutboxDrainProgress(); !ok || pct != 0 {
		t.Fatalf("burst start: got (%d, %v), want (0, true)", pct, ok)
	}

	a.recordBurstDoneForTest(2)
	if pct, ok := a.OutboxDrainProgress(); !ok || pct != 66 {
		t.Fatalf("after 2/3: got (%d, %v), want (66, true)", pct, ok)
	}

	a.recordBurstDoneForTest(1)
	if pct, ok := a.OutboxDrainProgress(); ok || pct != 0 {
		t.Fatalf("burst empty: got (%d, %v), want (0, false)", pct, ok)
	}
}
