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

	mu       sync.Mutex
	cmd      imapClient // command connection (nil before Connect)
	idle     imapClient // idle connection
	caps     capSet
	current  string // currently-selected folder on cmd
	trash    string // resolved Trash folder name, empty before first Delete
	password string // cached PasswordCmd result, empty when cfg.Password is inline
	updates  chan mail.Update

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
}

// New constructs an unconnected Backend for cfg.
func New(cfg config.AccountConfig) *Backend {
	return &Backend{cfg: cfg}
}

func (b *Backend) AccountName() string {
	if b.cfg.Display != "" {
		return b.cfg.Display
	}
	if b.cfg.Email != "" {
		return b.cfg.Email
	}
	return b.cfg.Name
}

func (b *Backend) AccountEmail() string {
	if b.cfg.From != nil && b.cfg.From.Address != "" {
		return b.cfg.From.Address
	}
	return b.cfg.Email
}

// Updates returns a nil channel before
// Connect succeeds.
func (b *Backend) Updates() <-chan mail.Update { return b.updates }

const updatesBuffer = 64

// Connect resolves the password (running
// PasswordCmd if needed, caching the result), dials both connections,
// authenticates, negotiates capabilities, and starts the idle goroutine.
// The dial happens in auth.go. Tests bypass by setting b.cmd / b.idle
// directly and calling finishConnect.
func (b *Backend) Connect(ctx context.Context) error {
	pw, err := b.resolvedPassword()
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	cmd, err := dial(b.cfg, pw, "command")
	if err != nil {
		return fmt.Errorf("connect cmd: %w", err)
	}
	idle, err := dial(b.cfg, pw, "idle")
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
	}
	if !cs.UIDPLUS {
		return errors.New("server does not advertise UIDPLUS — required for safe deletion")
	}
	if b.cfg.GmailQuirks && !caps["X-GM-EXT-1"] {
		return errors.New("gmail account but server does not advertise X-GM-EXT-1")
	}

	updates := make(chan mail.Update, updatesBuffer)

	b.mu.Lock()
	b.caps = cs
	b.updates = updates
	b.switchCh = make(chan string, 1)
	idleCtx, cancel := context.WithCancel(ctx)
	b.idleCancel = cancel
	b.idleDone = make(chan struct{})
	b.mu.Unlock()

	go b.idleLoop(idleCtx)

	return nil
}

// Disconnect tears down the idle goroutine
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

// idleLoop, runIdleSession, pollLoop, and emit are implemented in
// idle.go.
