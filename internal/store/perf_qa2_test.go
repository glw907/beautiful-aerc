//go:build perf && !race

// The QA-1/2/3 perf harness builds only under the perf tag, and never
// under race. The tag keeps the harness out of `go test ./...`, which
// schedules packages in parallel: under that load QA-1's single-sample
// cold-start gate measured 907ms against its 500ms budget, where it
// measures 26-42ms alone. `make perf` runs the harness by itself under
// -p 1 instead (build machine section 2). Race instrumentation costs
// 2-20x time and 5-10x memory, so a p95 asserted under it would measure
// the detector rather than the store, and CI's `go test -race ./...`
// job never links this file either.
package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
)

// qa2ScriptLen is the length of QA-2's scripted session; the store it
// replays against is storetest's perf corpus, whose scale the
// POPLAR_PERF_FULL environment variable selects.
const (
	qa2ScriptLen = 500

	qa2P95Budget = 25 * time.Millisecond
	qa2P99Budget = 40 * time.Millisecond
)

// qa2Op names one class of scripted interaction QA-2's 500-operation
// mix draws from: 60% list movement, 15% folder switch, 15% reader
// open of a cached body, 10% incremental search keystrokes.
type qa2Op int

const (
	qa2OpList qa2Op = iota
	qa2OpSwitch
	qa2OpReader
	qa2OpSearch
)

// qa2Script returns a fixed-seed shuffle of qa2ScriptLen ops in QA-2's
// 60/15/15/10 mix, so every call replays the same scripted session.
func qa2Script() []qa2Op {
	listN, switchN, readerN := qa2ScriptLen*60/100, qa2ScriptLen*15/100, qa2ScriptLen*15/100
	searchN := qa2ScriptLen - listN - switchN - readerN

	// Built in a fixed order (never ranging over a map) so the fixed
	// seed below always shuffles the same starting sequence into the
	// same scripted session.
	script := make([]qa2Op, 0, qa2ScriptLen)
	for _, class := range []struct {
		op qa2Op
		n  int
	}{{qa2OpList, listN}, {qa2OpSwitch, switchN}, {qa2OpReader, readerN}, {qa2OpSearch, searchN}} {
		for range class.n {
			script = append(script, class.op)
		}
	}
	rand.New(rand.NewPCG(3, 4)).Shuffle(len(script), func(i, j int) { script[i], script[j] = script[j], script[i] }) //nolint:gosec // G404: a fixed seed makes the scripted session reproducible, not a security-sensitive use
	return script
}

// TestQA2Interaction proves the store's read path holds QA-2's
// interaction-latency budget (p95 under 25ms, p99 under 40ms) over a
// scripted 500-operation session against the seeded perf corpus, both
// quiescent and while a bulk backfill runs concurrently on the
// writer's bulk lane (ADR-0003's concurrency design is what QA-2
// requires to hold under that load).
func TestQA2Interaction(t *testing.T) {
	log := uerrtest.CaptureDefault(t)

	path := filepath.Join(t.TempDir(), "store.db")
	env := storetest.SeedPerfEnvelope(t, path)

	writer, err := store.Open(path, store.DefaultWriterConfig())
	if err != nil {
		t.Fatalf("open writer: %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	reads, err := store.NewReadPool(path, store.DefaultReadPoolSize, writer.Revision())
	if err != nil {
		t.Fatalf("open read pool: %v", err)
	}
	t.Cleanup(func() { _ = reads.Close() })

	sess := &qa2Session{reads: reads, mailboxIDs: env.MailboxIDs, messageIDs: env.MessageIDs, script: qa2Script()}

	quiescent, busy := sess.run(t)
	storetest.WriteBaseline(t, "testdata/perf-baselines", env.BaselineName("QA2Interaction_quiescent"), busy.line, quiescent)
	qa2AssertBudget(t, "quiescent", quiescent)
	if got := busy.count.Load(); got != 0 {
		t.Errorf("quiescent SQLITE_BUSY count = %d, want 0", got)
	}

	backfill := startQA2Backfill(writer, env.MessageIDs)
	underWrite, busy := sess.run(t)
	if err := backfill.stop(); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	storetest.WriteBaseline(t, "testdata/perf-baselines", env.BaselineName("QA2Interaction_under_write"), busy.line, underWrite)
	qa2AssertBudget(t, "under_write", underWrite)
	if got := busy.count.Load(); got != 0 {
		t.Errorf("under-write SQLITE_BUSY count = %d, want 0 (WAL readers must not block on the writer)", got)
	}

	// A batch under a millisecond would let this case pass even if
	// reads blocked on the writer, the same weak-discriminator failure
	// mode task 3's write-blocking test was flagged for: this backfill
	// must actually cost real time on the writer for the concurrent
	// assertion above to mean anything. The first batch, not the mean,
	// is what proves that: it queues behind nothing (no prior batch's
	// post-commit checkpoint to wait out), so its wall time approximates
	// the transaction's own execute time. The mean includes that
	// checkpoint wait and would clear a 1ms floor even for a
	// near-instant batch.
	batches := backfill.batches.Load()
	if batches == 0 {
		t.Fatal("backfill ran zero batches; the under-write case measured nothing concurrent")
	}
	firstBatch := time.Duration(backfill.firstBatchNS.Load())
	if firstBatch < time.Millisecond {
		t.Errorf("first backfill batch = %s, want at least 1ms of real write pressure", firstBatch)
	}

	// qa2BackfillBatch is sized to keep the writer's own transaction
	// (what the admission ceiling gates) under WriteCeiling; the log
	// capture above is that claim's proof, not this Logf line.
	if strings.Contains(log.String(), "admission ceiling") {
		t.Errorf("backfill logged an admission ceiling warning; qa2BackfillBatch's row counts are too large for this corpus:\n%s", log.String())
	}

	meanBatch := time.Duration(backfill.totalBatchNS.Load() / batches)
	t.Logf("backfill: %d batches, %d rows mutated, first batch %s (execute time, unqueued), mean batch %s (wall time, includes queueing behind the prior batch's post-commit checkpoint)",
		batches, backfill.rowsMutated.Load(), firstBatch, meanBatch)
}

// qa2AssertBudget checks samples against QA-2's gate: p95 under 25ms,
// p99 under 40ms.
func qa2AssertBudget(t *testing.T, label string, samples []time.Duration) {
	t.Helper()

	p95, p99 := storetest.Percentile(samples, 95), storetest.Percentile(samples, 99)
	t.Logf("QA-2 %s: p50=%s p95=%s p99=%s (spike baseline 22-25ms p95)", label, storetest.Percentile(samples, 50), p95, p99)
	if p95 > qa2P95Budget {
		t.Errorf("%s p95 = %s, want under %s", label, p95, qa2P95Budget)
	}
	if p99 > qa2P99Budget {
		t.Errorf("%s p99 = %s, want under %s", label, p99, qa2P99Budget)
	}
}

// qa2Session holds the fixtures a scripted QA-2 run replays against:
// the read pool, the mailboxes and messages storetest.SeedPerfEnvelope
// built, and the fixed script every run (quiescent and under-write)
// replays identically.
type qa2Session struct {
	reads      *store.ReadPool
	mailboxIDs []int64
	messageIDs []int64
	script     []qa2Op
}

// qa2BusyTracker counts SQLITE_BUSY / "database is locked" errors a
// scripted run's reads hit, alongside the go-test-style benchmark
// line storetest.Measure returned for the run.
type qa2BusyTracker struct {
	count atomic.Int64
	line  string
}

// run replays sess's script once through storetest.Measure, returning
// the per-operation latencies and a busy-error tracker.
//
// A non-busy error is recorded rather than failed on the spot:
// storetest.Measure's op runs on the goroutine testing.Benchmark
// launches internally, not the one running t, and t.Fatalf must only
// be called from the goroutine running the test (testing.T's own
// documented contract). run reports the first such error itself, back
// on t's own goroutine, once storetest.Measure has returned.
func (sess *qa2Session) run(t *testing.T) ([]time.Duration, *qa2BusyTracker) {
	t.Helper()

	busy := &qa2BusyTracker{}
	currentMailbox := sess.mailboxIDs[0]
	cursor := store.MailboxCursor{}
	i := 0
	var firstErr error

	samples, line := storetest.Measure(t, len(sess.script), func() time.Duration {
		op := sess.script[i%len(sess.script)]
		i++
		start := time.Now()
		err := sess.runOp(op, &currentMailbox, &cursor)
		dur := time.Since(start)
		if err != nil {
			if isBusyError(err) {
				busy.count.Add(1)
			} else if firstErr == nil {
				firstErr = fmt.Errorf("op %d: %w", i, err)
			}
		}
		return dur
	})
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	busy.line = line
	return samples, busy
}

// runOp runs one scripted op against sess.reads, threading
// list-movement state (currentMailbox, cursor) across calls so a
// list op continues scrolling rather than always re-fetching page one.
func (sess *qa2Session) runOp(op qa2Op, currentMailbox *int64, cursor *store.MailboxCursor) error {
	ctx := context.Background()
	switch op {
	case qa2OpList:
		page, err := sess.reads.ListMailboxForward(ctx, *currentMailbox, *cursor, 50)
		if err != nil {
			return err
		}
		if len(page.Rows) == 0 {
			*cursor = store.MailboxCursor{}
			return nil
		}
		last := page.Rows[len(page.Rows)-1]
		*cursor = store.MailboxCursor{ReceivedAt: last.ReceivedAt, MessageID: last.MessageID}
		return nil

	case qa2OpSwitch:
		*currentMailbox = sess.mailboxIDs[rand.IntN(len(sess.mailboxIDs))] //nolint:gosec // G404: folder choice, not a security-sensitive use
		*cursor = store.MailboxCursor{}
		_, err := sess.reads.ListMailboxForward(ctx, *currentMailbox, *cursor, 50)
		return err

	case qa2OpReader:
		id := sess.messageIDs[rand.IntN(len(sess.messageIDs))] //nolint:gosec // G404: sample choice, not a security-sensitive use
		_, err := sess.reads.PerfBody(ctx, id)
		return err

	default: // qa2OpSearch
		// Prefix lengths start at 2, matching message_fts's prefix='2
		// 3 4' index: a 1-character prefix has no prefix-index entry
		// and falls back to a full vocabulary scan, which is not the
		// as-you-type case a 2-character minimum trigger avoids. The
		// draw caps at 4 deliberately, matching the index rather than
		// by coincidence: length-5 coverage is an open pass-gate
		// question (measured 972ms unindexed), not settled here.
		term := storetest.CommonWords[rand.IntN(len(storetest.CommonWords))] //nolint:gosec // G404: sample choice, not a security-sensitive use
		prefixLen := 2 + rand.IntN(max(1, min(3, len(term)-2)))              //nolint:gosec // G404: sample choice, not a security-sensitive use
		return sess.reads.PerfSearch(ctx, term[:prefixLen]+"*")
	}
}

// isBusyError reports whether err is SQLite's busy/locked error, the
// one failure QA-2's concurrent case must never see (the spike's own
// baseline: zero SQLITE_BUSY under concurrent write). The store's read
// path wraps every failure in uerr.Error, whose Error() returns only
// the fixed class sentence and drops the driver's own message, so the
// check unwraps to the cause before matching.
func isBusyError(err error) bool {
	var uerrErr uerr.Error
	if errors.As(err, &uerrErr) && uerrErr.Cause != nil {
		err = uerrErr.Cause
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "database is locked")
}

// qa2Backfill is a bulk-lane writer loop that keeps mutating the store
// until stopped: QA-2's "20k-body backfill runs concurrently" case
// needs sustained write pressure for the whole scripted session, not
// one write that finishes before the reads do.
type qa2Backfill struct {
	stopCh       chan struct{}
	done         chan struct{}
	batches      atomic.Int64
	totalBatchNS atomic.Int64
	firstBatchNS atomic.Int64 // set once, from the batch that queued behind no prior checkpoint
	rowsMutated  atomic.Int64
	err          error
}

// startQA2Backfill starts the backfill goroutine over writer, cycling
// through messageIDs.
func startQA2Backfill(writer *store.Writer, messageIDs []int64) *qa2Backfill {
	bf := &qa2Backfill{stopCh: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(bf.done)
		rng := rand.New(rand.NewPCG(7, 8)) //nolint:gosec // G404: a fixed seed makes the backfill's own mutation order reproducible, not a security-sensitive use
		for {
			select {
			case <-bf.stopCh:
				return
			default:
			}
			start := time.Now()
			rows, err := qa2BackfillBatch(writer, messageIDs, rng)
			if err != nil {
				bf.err = err
				return
			}
			elapsed := time.Since(start)
			if bf.batches.Load() == 0 {
				bf.firstBatchNS.Store(elapsed.Nanoseconds())
			}
			bf.batches.Add(1)
			bf.totalBatchNS.Add(elapsed.Nanoseconds())
			bf.rowsMutated.Add(int64(rows))
		}
	}()
	return bf
}

// stop signals the backfill goroutine, waits for it to exit, and
// returns the write error that stopped it early, if any. stop runs
// before the caller's t.Cleanup, while the writer is still very much
// alive, so a mid-session write failure is a real result to report,
// not a shutdown artifact to swallow.
func (bf *qa2Backfill) stop() error {
	close(bf.stopCh)
	<-bf.done
	return bf.err
}

// TestIsBusyError proves isBusyError sees through uerr.Error's
// wrapping to the driver's own SQLITE_BUSY message. uerr.Error.Error()
// returns only its fixed class sentence, so a check against err.Error()
// directly can never match a busy error ListMailboxForward or
// ListMailboxBackward returns, which is exactly what made QA-2's
// busy.count == 0 assertions vacuous.
func TestIsBusyError(t *testing.T) {
	wrapped := uerr.New("store.read", nil, uerr.ClassStoreLocal, errors.New("sqlite: SQLITE_BUSY: database is locked"))
	if !isBusyError(wrapped) {
		t.Error("isBusyError(wrapped busy error) = false, want true")
	}
	if isBusyError(errors.New("no such table: message")) {
		t.Error("isBusyError(unrelated error) = true, want false")
	}
}

// TestQA2BackfillSurfacesWriteError proves a mid-session write failure
// on the backfill goroutine reaches the caller through stop(), rather
// than the goroutine exiting silently and the under-write case
// measuring zero real write pressure without anyone noticing.
//
// It opens its own writer rather than storetest.OpenWriter, closing it
// once before starting the backfill: OpenWriter's own t.Cleanup would
// double-close it, and Close panics on a channel already closed.
func TestQA2BackfillSurfacesWriteError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	writer, err := store.Open(path, store.DefaultWriterConfig())
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	bf := startQA2Backfill(writer, []int64{1})
	// The closed writer fails the backfill goroutine's very first batch
	// attempt, so waiting for it to exit on its own, before this test
	// ever touches bf.stopCh, is what makes the failure path
	// deterministic rather than racing stop()'s own cooperative signal
	// against the goroutine's first write attempt.
	<-bf.done
	if err := bf.stop(); err == nil {
		t.Fatal("stop() = nil, want the write error from a closed writer")
	}
	if got := bf.batches.Load(); got != 0 {
		t.Errorf("batches = %d, want 0 (the writer was closed before the first batch)", got)
	}
}

// qa2BackfillBatch runs one bulk-lane transaction: flagUpdates
// individual flag updates plus bodyUpserts body upserts, one
// statement per row rather than a single bulk UPDATE, the same
// per-row shape a real sync backfill writes in. That per-statement
// shape is also what keeps the batch's cost real: a single bulk
// statement would run fast enough to prove nothing about write
// pressure. The row counts are sized to stay under ADR-0003's 50ms
// admission ceiling, the same conforming-client shape a real bulk sync
// chunks its own writes to.
//
// The gate corpus is what binds the sizing, not the full envelope. A
// first batch at the full envelope sits around 21ms run to run, where
// gate scale is both larger and far more variable: 20-35ms on a quiet
// machine and past 70ms under load. So a row count chosen against the
// envelope's steady figure would trip the ceiling on the corpus every
// commit actually runs.
func qa2BackfillBatch(writer *store.Writer, messageIDs []int64, rng *rand.Rand) (int, error) {
	const flagUpdates, bodyUpserts = 300, 40
	rows := 0
	err := writer.Apply(context.Background(), func(tx *sql.Tx) error {
		for range flagUpdates {
			id := messageIDs[rng.IntN(len(messageIDs))]
			if _, err := tx.Exec(`UPDATE message SET flags = (flags + 1) & 15 WHERE id = ?`, id); err != nil {
				return err
			}
			rows++
		}
		for range bodyUpserts {
			id := messageIDs[rng.IntN(len(messageIDs))]
			content := "backfilled body for message " + strconv.FormatInt(id, 10)
			if _, err := tx.Exec(`INSERT INTO body (message_id, content, fetched_at) VALUES (?, ?, ?) ON CONFLICT(message_id) DO UPDATE SET content = excluded.content, fetched_at = excluded.fetched_at`,
				id, content, time.Now().Unix()); err != nil {
				return err
			}
			rows++
		}
		return nil
	})
	return rows, err
}
