package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
)

// waitForLog polls log for substr up to a short timeout, since a
// checkpoint's outcome can log after the job that triggered it has
// already returned to its caller.
func waitForLog(t *testing.T, log *uerrtest.Buffer, substr string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(log.String(), substr) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("log output = %q, want a line containing %q", log.String(), substr)
}

// newTestWriter opens a fresh migrated store file and returns a
// Writer over it, along with the file's path for tests that inspect
// the WAL file directly.
func newTestWriter(t *testing.T, cfg WriterConfig) (*Writer, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", dsn(path, connReadWrite))
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	w, err := NewWriter(db, cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w, path
}

// TestWriterSerializes proves that many concurrent submit callers
// each get their job run in the order their send reached the
// interactive lane, and that no two jobs ever run at once.
func TestWriterSerializes(t *testing.T) {
	w, _ := newTestWriter(t, DefaultWriterConfig())

	// A first job blocks the writer so every submitter below is
	// guaranteed to be queued, in launch order, before any of them
	// runs.
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = w.submit(context.Background(), func(*sql.Tx) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started

	const n = 20
	var active atomic.Bool
	var mu sync.Mutex
	var executed []int

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := w.submit(context.Background(), func(*sql.Tx) error {
				if !active.CompareAndSwap(false, true) {
					t.Error("two jobs ran concurrently")
				}
				mu.Lock()
				executed = append(executed, i)
				mu.Unlock()
				time.Sleep(time.Millisecond)
				active.Store(false)
				return nil
			})
			if err != nil {
				t.Errorf("submit(%d): %v", i, err)
			}
		}(i)
		// Give goroutine i time to block on the channel send before
		// launching i+1, so the queue order below is the launch order.
		time.Sleep(5 * time.Millisecond)
	}

	close(release)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	want := make([]int, n)
	for i := range want {
		want[i] = i
	}
	if len(executed) != n {
		t.Fatalf("executed %d jobs, want %d", len(executed), n)
	}
	for i := range executed {
		if executed[i] != want[i] {
			t.Fatalf("execution order = %v, want arrival order %v", executed, want)
		}
	}
}

// TestInteractivePreemption shows an interactive job admitted while
// a chunked bulk job is in flight, with the wait bounded by the
// current chunk rather than the whole queued bulk backlog. It asserts
// that twice over. The chunk count is the durable one: the job runs
// once the chunk it arrived during finishes, whatever a chunk costs.
// The wall-clock bound is the same claim in milliseconds, and it
// holds only where a millisecond means what the writer spent, so it
// sits out a race build, whose instrumentation lands squarely inside
// the window being measured (the post-chunk commit and its PASSIVE
// checkpoint).
func TestInteractivePreemption(t *testing.T) {
	w, _ := newTestWriter(t, DefaultWriterConfig())

	const chunkWork = 20 * time.Millisecond
	const chunks = 10
	const admissionCeiling = 15 * time.Millisecond // under one chunk's remaining time

	var completed atomic.Int64
	bulkDone := make(chan error, 1)
	go func() {
		for range chunks {
			err := w.submitBulk(context.Background(), func(*sql.Tx) error {
				time.Sleep(chunkWork)
				completed.Add(1)
				return nil
			})
			if err != nil {
				bulkDone <- err
				return
			}
		}
		bulkDone <- nil
	}()

	time.Sleep(chunkWork / 2) // let the bulk job start its first chunk

	inFlight := completed.Load()
	start := time.Now()
	var admittedAfter int64
	if err := w.submit(context.Background(), func(*sql.Tx) error {
		admittedAfter = completed.Load()
		return nil
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	waited := time.Since(start)

	if crossed := admittedAfter - inFlight; crossed > 1 {
		t.Errorf("interactive job waited out %d bulk chunks, want at most the one in flight; the backlog behind it is %d",
			crossed, chunks)
	}

	if waited > admissionCeiling && !raceEnabled {
		t.Errorf("interactive admission took %v, want under %v; %d queued bulk chunks would be %v",
			waited, admissionCeiling, chunks, chunks*chunkWork)
	}

	if err := <-bulkDone; err != nil {
		t.Fatalf("bulk chunk failed: %v", err)
	}
}

// The backfill subordination policy (ADR-0003 revision 2: check
// RecentInteractiveActivity before each bulk chunk rather than the
// lane's queue depth, which is empty exactly when the writer is busy)
// is exercised against internal/sync's production chunk loop, the
// policy's first real caller, not a closure here. See
// internal/sync's TestBackfillSubordination.

// TestDiskFullInjection simulates SQLITE_FULL with max_page_count,
// the standard portable stand-in for an actual full disk, and proves
// the failing transaction commits nothing and surfaces as a
// uerr.Error under ClassStoreLocal, the class uerr documents for a
// full disk.
func TestDiskFullInjection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", dsn(path, connReadWrite))
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(`PRAGMA max_page_count = 40`); err != nil {
		t.Fatalf("set max_page_count: %v", err)
	}

	w, err := NewWriter(db, DefaultWriterConfig())
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	big := strings.Repeat("x", 4096)
	writeErr := w.submit(context.Background(), func(tx *sql.Tx) error {
		for i := range 5000 {
			if _, err := tx.Exec(
				`INSERT INTO account (slug, backend_kind, address, data) VALUES (?, ?, ?, ?)`,
				fmt.Sprintf("acct-%d", i), "jmap", "user@example.com", big); err != nil {
				return err
			}
		}
		return nil
	})
	if writeErr == nil {
		t.Fatal("submit succeeded past max_page_count, want a disk-full failure")
	}

	uerrtest.AssertClass(t, writeErr, uerr.ClassStoreLocal)

	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM account`).Scan(&rows); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if rows != 0 {
		t.Errorf("accounts = %d, want 0 (the failed transaction must not have committed partially)", rows)
	}
}

// TestSubmitWaitsAfterAdmission proves a caller whose context is
// cancelled after its job is admitted still learns the job's true
// outcome instead of seeing ctx.Err() for a write that commits
// anyway: with a durable intent row behind submit, that mismatch
// would make a retry double-apply the mutation.
func TestSubmitWaitsAfterAdmission(t *testing.T) {
	w, _ := newTestWriter(t, DefaultWriterConfig())

	ctx, cancel := context.WithCancel(context.Background())
	admitted := make(chan struct{})

	submitDone := make(chan error, 1)
	go func() {
		submitDone <- w.submit(ctx, func(tx *sql.Tx) error {
			close(admitted) // fn only runs once the job is off the lane and Begin has succeeded
			time.Sleep(20 * time.Millisecond)
			_, err := tx.Exec(`INSERT INTO account (slug, backend_kind, address) VALUES ('acct', 'jmap', 'a@example.com')`)
			return err
		})
	}()

	<-admitted
	cancel()

	if err := <-submitDone; err != nil {
		t.Fatalf("submit = %v, want the admitted write to succeed despite ctx cancelling mid-flight", err)
	}
}

// TestWriteCeilingWarning proves execute logs a warning when a
// transaction runs past WriterConfig.WriteCeiling, the admission
// ceiling CO-6's kill harness and ADR-0003 revision 2 name, even
// though the writer does not cancel the transaction itself.
func TestWriteCeilingWarning(t *testing.T) {
	log := uerrtest.CaptureDefault(t)

	cfg := DefaultWriterConfig()
	cfg.WriteCeiling = 5 * time.Millisecond
	w, _ := newTestWriter(t, cfg)

	if err := w.submit(context.Background(), func(*sql.Tx) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	waitForLog(t, log, "admission ceiling")
}

// TestCheckpointFailureLogged proves a failed checkpoint reaches the
// log instead of vanishing behind a discarded error: repeated
// checkpoint failure means unbounded WAL growth ending in a disk-full
// the user does see, with nothing tracing back to the cause.
func TestCheckpointFailureLogged(t *testing.T) {
	log := uerrtest.CaptureDefault(t)

	w, _ := newTestWriter(t, DefaultWriterConfig())
	// Closing the connection out from under the running writer makes
	// every subsequent operation on it fail, including the checkpoint
	// runBulk always runs after a chunk.
	_ = w.db.Close()

	_ = w.submitBulk(context.Background(), func(*sql.Tx) error { return nil })

	waitForLog(t, log, "checkpoint failed")
}
