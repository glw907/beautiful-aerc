package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
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

// TestInteractivePreemption proves an interactive job runs at the
// first chunk boundary after it queues, however deep the bulk backlog
// sitting behind it. Each round parks an interactive submit on the
// writer's lane while one bulk chunk is in flight and dozens more wait
// their turn, releases that chunk, and requires the interactive job to
// be the very next thing the writer runs.
//
// The claim is admission order, not elapsed time: a wall-clock bound
// measures the machine's load as much as the writer's policy, which is
// what made an earlier form of this test fail roughly one full-module
// run in three. Repeating the round is what keeps it sensitive to a
// revert. Without run's non-blocking interactive check the writer's
// select picks between two ready lanes at random, so one round passes
// half the time and preemptionRounds of them do not.
func TestInteractivePreemption(t *testing.T) {
	const preemptionRounds = 20
	const bulkChunks = 2 * preemptionRounds

	w, _ := newTestWriter(t, DefaultWriterConfig())

	admitted := make(chan string)
	release := make(chan struct{})
	quiet := make(chan struct{})
	// A failed round leaves the writer parked inside a bulk chunk,
	// where newTestWriter's own Close would wait on it forever. This
	// cleanup was registered later, so it runs first and lets the
	// backlog drain.
	drain := sync.OnceFunc(func() { close(quiet); close(release) })
	t.Cleanup(drain)

	// The chunk errors are collected rather than reported on the spot:
	// a t.Errorf from one of these goroutines after the test goroutine
	// has left panics the whole package binary, and every other test in
	// internal/store then reports nothing at all.
	chunkErrs := make(chan error, bulkChunks)
	var wg sync.WaitGroup
	for range bulkChunks {
		wg.Go(func() {
			if err := w.submitBulk(context.Background(), func(*sql.Tx) error {
				select {
				case admitted <- "bulk":
				case <-quiet:
				}
				<-release
				return nil
			}); err != nil {
				chunkErrs <- err
			}
		})
	}

	// One chunk admitted, the rest queued behind it: the state every
	// round below starts from and returns the writer to.
	if got := <-admitted; got != "bulk" {
		t.Fatalf("first admission = %q, want a bulk chunk", got)
	}

	interactive := make(chan error, 1)
	for round := range preemptionRounds {
		go submitOnInteractiveLane(w, admitted, interactive)
		awaitParkedSubmit(t)

		release <- struct{}{}
		if got := <-admitted; got != "interactive" {
			t.Fatalf("round %d: the writer admitted a %s chunk with an interactive job already queued and %d more chunks behind it",
				round, got, bulkChunks-round-2)
		}
		if err := <-interactive; err != nil {
			t.Fatalf("round %d: submit: %v", round, err)
		}
		if got := <-admitted; got != "bulk" {
			t.Fatalf("round %d: the writer admitted %q instead of resuming its backlog", round, got)
		}
	}

	drain()
	wg.Wait()
	close(chunkErrs)
	for err := range chunkErrs {
		// A chunk still queued when the writer closes is the shape of a
		// torn-down round, not a failure of the lane under test.
		if !errors.Is(err, errWriterClosed) {
			t.Errorf("submitBulk: %v", err)
		}
	}
}

// submitOnInteractiveLane submits a job that announces itself on
// admitted, and reports the submit's own outcome on result. It is a
// named function rather than a closure so that awaitParkedSubmit can
// find its frame in a stack dump: a bulk submitter parked on the other
// lane is otherwise identical.
func submitOnInteractiveLane(w *Writer, admitted chan<- string, result chan<- error) {
	result <- w.submit(context.Background(), func(*sql.Tx) error {
		admitted <- "interactive"
		return nil
	})
}

// awaitParkedSubmit blocks until a submitOnInteractiveLane goroutine
// is parked offering its job to the writer's interactive lane. A
// submitter that has not reached that offer yet leaves the lane empty,
// and the writer then picks the next bulk chunk quite correctly;
// waiting for the park is what separates that scheduling artifact from
// a real preemption failure. The state is stable rather than raced:
// the writer is inside a bulk transaction, receiving from nothing, so
// a parked submitter stays parked until the test releases the chunk.
//
// enqueue offers the job inside a select, which parks the goroutine in
// select rather than chan send; both spellings are the same fact.
func awaitParkedSubmit(t *testing.T) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for {
		for g := range strings.SplitSeq(perfGoroutineDump(), "\n\ngoroutine ") {
			parked := strings.Contains(g, "[select") || strings.Contains(g, "[chan send")
			// The trailing paren pins the frame to the function itself:
			// its own closure would show as submitOnInteractiveLane.func1.
			if parked && strings.Contains(g, "store.submitOnInteractiveLane(") {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("no interactive submit parked on the writer's lane")
		}
		time.Sleep(time.Millisecond)
	}
}

// The backfill subordination policy (ADR-0003 revision 2: check
// RecentInteractiveActivity before each bulk chunk rather than the
// lane's queue depth, which is empty exactly when the writer is busy)
// is exercised against internal/sync's production chunk loop, the
// policy's first real caller, not a closure here. See
// internal/sync's TestBackfillSubordination.

// perfGoroutineDump returns every goroutine's stack, growing the
// buffer until runtime.Stack reports it had room to finish: a fixed
// buffer silently truncates, and a truncated dump reads as a submitter
// that never parked.
func perfGoroutineDump() string {
	buf := make([]byte, 1<<16)
	for {
		if n := runtime.Stack(buf, true); n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}

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

	_ = uerrtest.AssertClass(t, writeErr, uerr.ClassStoreLocal)

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
