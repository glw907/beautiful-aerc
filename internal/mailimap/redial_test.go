package mailimap

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func TestClassifyErr_ConnectionDead(t *testing.T) {
	tests := []struct {
		name string
		in   error
	}{
		{"eof", io.EOF},
		{"closed-pipe", io.ErrClosedPipe},
		{"net-closed", net.ErrClosed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyErr(tc.in)
			if !errors.Is(got, mail.ErrConnection) {
				t.Errorf("classifyErr(%v) = %v, want errors.Is ErrConnection", tc.in, got)
			}
		})
	}
}

func TestCmdClient_RedialsAfterDrop(t *testing.T) {
	first := newFakeClient()
	second := newFakeClient()

	b := New(config.AccountConfig{Name: "t"}, nil)
	b.cmd = first
	b.connCtx = context.Background()
	called := 0
	b.dialFn = func(_ context.Context, role string) (imapClient, error) {
		if role != "command" {
			t.Fatalf("dialFn role = %q, want command", role)
		}
		called++
		return second, nil
	}

	// Existing cached client returns without dialing.
	got, err := b.cmdClient()
	if err != nil {
		t.Fatalf("cmdClient: %v", err)
	}
	if got != first {
		t.Fatalf("first cmdClient = %v, want %v", got, first)
	}
	if called != 0 {
		t.Errorf("dialFn called = %d, want 0 (cached)", called)
	}

	// Drop the cached client.
	b.dropCmd(first)
	if b.cmd != nil {
		t.Fatal("dropCmd: b.cmd not nil after drop")
	}

	// Next call dials.
	got, err = b.cmdClient()
	if err != nil {
		t.Fatalf("cmdClient (redial): %v", err)
	}
	if got != second {
		t.Errorf("redialed cmd = %v, want %v", got, second)
	}
	if called != 1 {
		t.Errorf("dialFn called = %d, want 1", called)
	}
}

func TestCmdClient_RedialReselectsCurrent(t *testing.T) {
	first := newFakeClient()
	second := newFakeClient()

	b := New(config.AccountConfig{Name: "t"}, nil)
	b.cmd = first
	b.current = "INBOX"
	b.connCtx = context.Background()
	b.dialFn = func(_ context.Context, _ string) (imapClient, error) { return second, nil }

	b.dropCmd(first)
	if _, err := b.cmdClient(); err != nil {
		t.Fatalf("cmdClient: %v", err)
	}
	if second.selected != "INBOX" {
		t.Errorf("redialed cmd not re-selected: selected = %q, want INBOX", second.selected)
	}
}

func TestFlag_DropsCmdOnConnectionError(t *testing.T) {
	cmd := newFakeClient()
	cmd.storeFn = func(_ []mail.UID, _ string, _ any) error { return io.EOF }
	idle := newFakeClient()

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	b.connCtx = context.Background()
	called := 0
	b.dialFn = func(_ context.Context, _ string) (imapClient, error) {
		called++
		return newFakeClient(), nil
	}

	err := b.Flag([]mail.UID{"1"}, mail.FlagSeen, true)
	if !errors.Is(err, mail.ErrConnection) {
		t.Fatalf("Flag err = %v, want ErrConnection", err)
	}
	if b.cmd != nil {
		t.Error("Flag did not drop b.cmd after connection error")
	}

	// Next call should redial.
	if err := b.Flag([]mail.UID{"1"}, mail.FlagSeen, true); err == nil {
		// fresh fake's storeFn is nil, so the second Flag should
		// succeed.
		// nothing to do
	}
	if called != 1 {
		t.Errorf("dialFn called = %d, want 1", called)
	}
}

// Auth failures on cmd-path redial propagate as mail.ErrAuth so the
// cache drainer routes the op to OpConflict auth-failure.
func TestCmdClient_AuthDialFailure(t *testing.T) {
	cmd := newFakeClient()
	cmd.storeFn = func(_ []mail.UID, _ string, _ any) error { return io.EOF }
	idle := newFakeClient()

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	b.connCtx = context.Background()
	b.dialFn = func(_ context.Context, _ string) (imapClient, error) {
		return nil, mail.WrapSentinel(errors.New("AUTHENTICATIONFAILED"), mail.ErrAuth)
	}

	// First Flag triggers the io.EOF path → drops cmd → redials → ErrAuth.
	_ = b.Flag([]mail.UID{"1"}, mail.FlagSeen, true)
	if b.cmd != nil {
		t.Fatal("Flag did not drop b.cmd after connection error")
	}
	err := b.Flag([]mail.UID{"1"}, mail.FlagSeen, true)
	if !errors.Is(err, mail.ErrAuth) {
		t.Fatalf("Flag err = %v, want ErrAuth", err)
	}
}

func TestIdleLoop_RedialsOnConnectionError(t *testing.T) {
	cmd := newFakeClient()
	idle := newFakeClient()

	stopCh := make(chan struct{})
	var onceStop sync.Once

	attempts := 0
	var mu sync.Mutex
	idle.onIdle = func(_ func(mail.Update)) error {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n == 1 {
			return io.EOF
		}
		<-stopCh
		return nil
	}
	idle.idleStop = func() { onceStop.Do(func() { close(stopCh) }) }

	b := idleBackend(t, cmd, idle)
	// Override dialFn so we can verify a redial happens. The first
	// idle pointer is dropped on EOF; dialFn returns the same fake so
	// the second session can run.
	dials := 0
	b.dialFn = func(_ context.Context, role string) (imapClient, error) {
		if role == "idle" {
			dials++
			return idle, nil
		}
		return cmd, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	go b.idleLoop(ctx)

	// Expect ConnConnected, then ConnReconnecting on EOF, then a fresh
	// ConnConnected after the redial.
	updates := drainUpdates(b, 3, 5*time.Second)
	cancel()
	<-b.idleDone

	if len(updates) < 3 {
		t.Fatalf("expected ≥3 updates, got %v", updates)
	}
	if updates[0].ConnState != mail.ConnConnected {
		t.Errorf("updates[0] = %v, want ConnConnected", updates[0])
	}
	if updates[1].ConnState != mail.ConnReconnecting {
		t.Errorf("updates[1] = %v, want ConnReconnecting", updates[1])
	}
	if updates[2].ConnState != mail.ConnConnected {
		t.Errorf("updates[2] = %v, want ConnConnected after redial", updates[2])
	}

	if dials < 1 {
		t.Errorf("expected ≥1 idle redial, got %d", dials)
	}
}
