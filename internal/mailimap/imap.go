// SPDX-License-Identifier: MIT

// Package mailimap implements mail.Backend over IMAP4rev1 using
// emersion/go-imap. Capabilities are negotiated at Connect; UIDPLUS
// is required, MOVE / SPECIAL-USE / IDLE are used opportunistically.
//
// A Backend owns two physical IMAP connections: a synchronous
// "command" connection used by every mail.Backend method, and an
// "idle" connection that runs in a goroutine emitting mail.Update
// values. Both share the auth path defined in auth.go.
package mailimap

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// Backend is one IMAP account.
type Backend struct {
	cfg config.AccountConfig

	mu      sync.Mutex
	cmd     imapClient // command connection (nil before Connect)
	idle    imapClient // idle connection
	caps    capSet
	current string // currently-selected folder on cmd
	updates chan mail.Update

	idleCancel context.CancelFunc
	idleDone   chan struct{}
	switchCh   chan string // folder-switch signal to idle goroutine
}

// capSet records the capabilities advertised by the server. UIDPLUS
// is required and Connect refuses to proceed without it.
type capSet struct {
	UIDPLUS    bool
	MOVE       bool
	IDLE       bool
	SpecialUse bool
	XGM        bool // X-GM-EXT-1, set by Pass 8.1 when GmailQuirks is on
}

// New constructs an unconnected Backend for cfg.
func New(cfg config.AccountConfig) *Backend {
	return &Backend{cfg: cfg}
}

// AccountName satisfies mail.Backend.
func (b *Backend) AccountName() string {
	if b.cfg.Display != "" {
		return b.cfg.Display
	}
	if b.cfg.Email != "" {
		return b.cfg.Email
	}
	return b.cfg.Name
}

// AccountEmail satisfies mail.Backend.
func (b *Backend) AccountEmail() string {
	if b.cfg.From != nil && b.cfg.From.Address != "" {
		return b.cfg.From.Address
	}
	return b.cfg.Email
}

// Updates satisfies mail.Backend. Returns a nil channel before
// Connect succeeds.
func (b *Backend) Updates() <-chan mail.Update { return b.updates }

const updatesBuffer = 64

// Connect satisfies mail.Backend. It dials both connections,
// authenticates, negotiates capabilities, and starts the idle
// goroutine. The dial happens in auth.go; tests bypass by setting
// b.cmd / b.idle directly and calling finishConnect.
func (b *Backend) Connect(ctx context.Context) error {
	cmd, err := dialCommand(b.cfg)
	if err != nil {
		return fmt.Errorf("connect cmd: %w", err)
	}
	idle, err := dialIdle(b.cfg)
	if err != nil {
		_ = cmd.Logout()
		return fmt.Errorf("connect idle: %w", err)
	}
	b.mu.Lock()
	b.cmd = cmd
	b.idle = idle
	b.mu.Unlock()

	return b.finishConnect(ctx)
}

// finishConnect runs the post-dial bringup: capability negotiation,
// channel setup, idle-goroutine spawn. Split out so unit tests can
// drive it with fakes.
func (b *Backend) finishConnect(ctx context.Context) error {
	caps, err := b.cmd.Capabilities()
	if err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}
	cs := capSet{
		UIDPLUS:    caps["UIDPLUS"],
		MOVE:       caps["MOVE"],
		IDLE:       caps["IDLE"],
		SpecialUse: caps["SPECIAL-USE"],
		XGM:        caps["X-GM-EXT-1"],
	}
	if !cs.UIDPLUS {
		return errors.New("server does not advertise UIDPLUS — required for safe deletion")
	}

	updates := make(chan mail.Update, updatesBuffer)

	b.mu.Lock()
	b.caps = cs
	b.updates = updates
	b.switchCh = make(chan string, 1)
	idleCtx, cancel := context.WithCancel(context.Background())
	b.idleCancel = cancel
	b.idleDone = make(chan struct{})
	b.mu.Unlock()

	go b.idleLoop(idleCtx)

	return nil
}

// Disconnect satisfies mail.Backend. Tears down the idle goroutine
// then logs out both connections. Returns the first non-nil error.
func (b *Backend) Disconnect() error {
	b.mu.Lock()
	cancel := b.idleCancel
	done := b.idleDone
	cmd := b.cmd
	idle := b.idle
	b.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}

	var firstErr error
	if cmd != nil {
		if err := cmd.Logout(); err != nil {
			firstErr = err
		}
	}
	if idle != nil {
		if err := idle.Logout(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// idleLoop, runIdleSession, pollLoop, handleUnilateral, and emit
// are implemented in idle.go.
