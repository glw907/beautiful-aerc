// SPDX-License-Identifier: MIT

// Package cache is poplar's local mail store. Each account maps to
// a SQLite database; the cache is the UI-facing read/write layer
// while mail.Backend / mail.ChangeTracker handle protocol I/O. See
// docs/superpowers/specs/2026-05-02-cache-0-design.md.
package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	_ "modernc.org/sqlite"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// Cache holds the per-account handles. Accounts are added with
// Open and removed with Close. Construction is the App's job.
type Cache struct {
	mu       sync.RWMutex
	accounts map[string]*Account
}

// NewCache returns an empty cache.
func NewCache() *Cache {
	return &Cache{accounts: make(map[string]*Account)}
}

// Account exposes a single account's handle. The UI reads/writes
// through this type; the backend pointer is held so the syncer
// and drainer can talk to the protocol layer.
type Account struct {
	Backend       mail.Backend
	ChangeTracker mail.ChangeTracker

	db   *sql.DB
	dir  string
	name string

	events        chan CacheEvent
	droppedEvents atomic.Uint64

	// Lifecycle handles. The drainer wakes on signal; the syncer
	// runs per-folder when wired up by the App.
	drainSignal chan struct{}
	stop        chan struct{}
	wg          sync.WaitGroup
}

// OpStatus is the lifecycle state of an outbox row. Stored as the
// underlying string in outbox.status; emitted on CacheEvent.Status.
type OpStatus string

const (
	OpPending   OpStatus = "pending"
	OpExecuting OpStatus = "executing"
	OpDone      OpStatus = "done"
	OpFailed    OpStatus = "failed"
	OpConflict  OpStatus = "conflict"
)

// OpKind is the discriminator for OpArgs. The string value is
// stored in outbox.kind and returned by OpArgs.opKind().
type OpKind string

const (
	KindMove    OpKind = "move"
	KindFlag    OpKind = "flag"
	KindDestroy OpKind = "destroy"
	KindSend    OpKind = "send"
	KindAppend  OpKind = "append"
)

// CacheEvent is the drainer→UI signal channel payload. App's
// pumpCacheCmd ranges (*Account).Events() and re-emits these as
// tea.Msg.
type CacheEvent struct {
	Account string
	OpID    int64
	Kind    OpKind
	Status  OpStatus
	Err     string // populated for conflict/failed
}

// Open returns an Account ready for reads and writes. It opens (or
// creates) the per-account SQLite database under dir, applies
// pragmas, and runs schema migrations to the current version.
//
// dir is the cache base directory; the per-account subdirectory is
// created if absent. A leading ~ is expanded to the user's home.
func Open(accountName string, backend mail.Backend, ct mail.ChangeTracker, dir string) (*Account, error) {
	expanded, err := expandHome(dir)
	if err != nil {
		return nil, fmt.Errorf("expand cache dir: %w", err)
	}
	slug := slugify(accountName)
	if slug == "" {
		return nil, fmt.Errorf("account %q produces empty cache slug", accountName)
	}
	acctDir := filepath.Join(expanded, slug)
	if err := os.MkdirAll(acctDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir cache: %w", err)
	}
	dbPath := filepath.Join(acctDir, "mail.db")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc.org/sqlite uses one connection per pragma scope by
	// default. Cap to a small pool with the same DSN so writers
	// serialize on a single connection (matches the WAL "one writer,
	// many readers" pattern).
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	a := &Account{
		Backend:       backend,
		ChangeTracker: ct,
		db:            db,
		dir:           acctDir,
		name:          accountName,
		events:        make(chan CacheEvent, 32),
		drainSignal:   make(chan struct{}, 1),
		stop:          make(chan struct{}),
	}
	return a, nil
}

// Name is the user-facing account label.
func (a *Account) Name() string { return a.name }

// AccountName proxies to the backend's display label. Sidebar header
// reads this; routes through the cache so the UI doesn't pierce
// straight to the backend pointer.
func (a *Account) AccountName() string {
	if a.Backend == nil {
		return a.name
	}
	return a.Backend.AccountName()
}

// AccountEmail proxies to the backend's resolved email address.
func (a *Account) AccountEmail() string {
	if a.Backend == nil {
		return ""
	}
	return a.Backend.AccountEmail()
}

// Dir is the per-account cache directory on disk.
func (a *Account) Dir() string { return a.dir }

// Events returns the drainer→UI signal channel.
func (a *Account) Events() <-chan CacheEvent { return a.events }

// DroppedEvents returns the count of CacheEvents the drainer has
// dropped because the events buffer was full. The UI reads this as
// a staleness signal — when it advances, do a full cache re-read
// rather than incremental Event handling.
func (a *Account) DroppedEvents() uint64 { return a.droppedEvents.Load() }

// Close stops background goroutines and closes the database.
func (a *Account) Close() error {
	select {
	case <-a.stop:
	default:
		close(a.stop)
	}
	a.wg.Wait()
	return a.db.Close()
}

// tx runs fn inside a transaction, rolling back on error.
func (a *Account) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// signalDrainer wakes the drainer if it's idle. Non-blocking — a
// pending wake-up coalesces with the new one.
func (a *Account) signalDrainer() {
	select {
	case a.drainSignal <- struct{}{}:
	default:
	}
}

// expandHome resolves dir to an absolute on-disk cache root.
// Empty input → $XDG_CACHE_HOME/poplar (or platform-equivalent).
// Otherwise the path is tilde-expanded via config.ExpandHome.
func expandHome(p string) (string, error) {
	if p == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "poplar"), nil
	}
	return config.ExpandHome(p)
}

// slugify lowercases name and reduces non-[a-z0-9-] runs to a
// single dash. Leading/trailing dashes are stripped.
func slugify(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// errClosed indicates the account was closed mid-operation.
var errClosed = errors.New("cache: account closed")
