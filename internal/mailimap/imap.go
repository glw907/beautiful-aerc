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
