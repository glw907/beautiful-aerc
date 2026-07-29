package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/glw907/poplar/internal/platform"
	"github.com/glw907/poplar/internal/store"
)

// readySignal is an io.Writer that closes ready the first time it
// sees run's "poplar is running" line, so a test can cancel run's
// context only once startup has actually finished, rather than racing
// it with a context cancelled before run begins.
type readySignal struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	once  sync.Once
	ready chan struct{}
}

func newReadySignal() *readySignal {
	return &readySignal{ready: make(chan struct{})}
}

func (r *readySignal) Write(p []byte) (int, error) {
	r.mu.Lock()
	n, err := r.buf.Write(p)
	r.mu.Unlock()
	if strings.Contains(string(p), "poplar is running") {
		r.once.Do(func() { close(r.ready) })
	}
	return n, err
}

func (r *readySignal) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

// runToCompletion starts run in the background, waits for its "poplar
// is running" line, cancels its context, and returns run's outcome
// along with everything it printed.
func runToCompletion(t *testing.T, dbPath string, f flags) (*readySignal, error) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := newReadySignal()
	done := make(chan error, 1)
	go func() { done <- run(ctx, dbPath, f, out) }()

	select {
	case <-out.ready:
	case err := <-done:
		t.Fatalf("run exited before reaching startup, output %q: %v", out.String(), err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for run to start")
	}

	cancel()
	return out, <-done
}

// TestRunStartsAndShutsDownCleanly proves a normal run opens and
// migrates the store, then marks a clean shutdown on the way out, so
// the next run's NeedsIntegrityCheck skips its check.
func TestRunStartsAndShutsDownCleanly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	if _, err := runToCompletion(t, dbPath, flags{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	if _, err := os.Stat(dbPath + ".clean-shutdown"); err != nil {
		t.Errorf("clean-shutdown marker missing after a graceful run: %v", err)
	}
	if store.NeedsIntegrityCheck(dbPath, false) {
		t.Error("NeedsIntegrityCheck after a clean run = true, want false")
	}
}

// TestRunRebuildsIndexOnFlag proves the --rebuild-index flag reaches
// store.RebuildIndex through the writer.
func TestRunRebuildsIndexOnFlag(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	out, err := runToCompletion(t, dbPath, flags{rebuildIndex: true})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), "rebuilding full-text index") {
		t.Errorf("output = %q, want a rebuild-index status line", out.String())
	}
}

// TestRunRefusesSecondInstance proves run's own startup path surfaces
// AcquireInstanceLock's refusal instead of swallowing it (SY-7).
func TestRunRefusesSecondInstance(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	lock, err := platform.AcquireInstanceLock(dbPath)
	if err != nil {
		t.Fatalf("AcquireInstanceLock: %v", err)
	}
	defer func() { _ = lock.Release() }()

	var out bytes.Buffer
	if err := run(context.Background(), dbPath, flags{}, &out); err == nil {
		t.Fatal("run succeeded against a locked store, want refusal")
	}
}

// TestPrepareStoreRecoversFromFailedMigration proves prepareStore
// rebuilds the store when Migrate fails, rather than propagating the
// failure and leaving startup dead in the water (SY-8).
func TestPrepareStoreRecoversFromFailedMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "store.db")

	seed, err := store.OpenWriteConn(dbPath)
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	if err := store.Migrate(seed); err != nil {
		t.Fatalf("seed Migrate: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'a', 'jmap', 'a@example.com')`); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO outbox (id, account_id, kind, payload, created_at) VALUES (1, 1, 'move-messages', '{}', 0)`); err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	if _, err := seed.Exec(`UPDATE schema_version SET version = 0`); err != nil {
		t.Fatalf("force a re-migration attempt: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed connection: %v", err)
	}

	var out bytes.Buffer
	db, err := prepareStore(context.Background(), dbPath, false, &out)
	if err != nil {
		t.Fatalf("prepareStore: %v", err)
	}
	defer func() { _ = db.Close() }()

	if !strings.Contains(out.String(), "rebuilding from local data") {
		t.Errorf("output = %q, want a rebuild status line", out.String())
	}

	var outboxCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id = 1`).Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	if outboxCount != 1 {
		t.Errorf("outbox rows after recovery = %d, want 1 (the preserved row must survive the rebuild)", outboxCount)
	}
}
