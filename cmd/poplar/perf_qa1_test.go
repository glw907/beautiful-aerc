//go:build !race

// The QA-1/2/3 perf harness is excluded from the race build: race
// instrumentation costs 2-20x time and 5-10x memory, so a p95 gate
// asserted under it would measure the detector instead of the store
// (build machine section 2). CI's `go test -race ./...` job never
// links this file.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/glw907/poplar/internal/store"
)

const (
	qa1MessageCount = 100_000
	qa1WarmRuns     = 20
	qa1WarmBudget   = 200 * time.Millisecond
	qa1ColdBudget   = 500 * time.Millisecond
)

// TestQA1Startup proves exec-to-first-list-page holds QA-1's budget
// against a 100k-message store: p95 under 200ms warm (page cache
// warmed by one prior run), and under 500ms on the first exec against
// a freshly seeded store.
func TestQA1Startup(t *testing.T) {
	warmHome := t.TempDir()
	seedQA1Store(t, warmHome)

	if _, err := qa1Trace(warmHome); err != nil {
		t.Fatalf("warm-up run: %v", err)
	}

	// firstErr, not t.Fatalf, inside the closure: perfMeasure's op runs
	// on the goroutine testing.Benchmark launches internally, not the
	// one running t, and t.Fatalf must only be called from the
	// goroutine running the test.
	var firstErr error
	samples, line := perfMeasure(t, qa1WarmRuns, func() time.Duration {
		trace, err := qa1Trace(warmHome)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return time.Duration(trace.TotalNS)
	})
	if firstErr != nil {
		t.Fatalf("startup-trace run: %v", firstErr)
	}
	perfWriteArtifact(t, "QA1Startup_warm", line, samples)
	p95 := perfPercentile(samples, 95)
	t.Logf("QA-1 warm: p50=%s p95=%s (gate 200ms; spike baseline ~5ms with quick_check off the launch path)",
		perfPercentile(samples, 50), p95)
	if p95 > qa1WarmBudget {
		t.Errorf("warm p95 = %s, want under %s", p95, qa1WarmBudget)
	}

	// Cold: the first startup-trace exec against a store this process
	// has never opened before. This approximates QA-1's "caches
	// dropped" cold start rather than reproducing it: a true OS
	// page-cache drop needs root or a syscall dependency outside this
	// pass's stdlib-plus-benchstat budget, and QA-1 is still
	// provisional pending that harness.
	coldHome := t.TempDir()
	seedQA1Store(t, coldHome)
	firstErr = nil
	coldSamples, coldLine := perfMeasure(t, 1, func() time.Duration {
		trace, err := qa1Trace(coldHome)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return time.Duration(trace.TotalNS)
	})
	if firstErr != nil {
		t.Fatalf("cold startup-trace run: %v", firstErr)
	}
	perfWriteArtifact(t, "QA1Startup_cold", coldLine, coldSamples)
	cold := coldSamples[0]
	t.Logf("QA-1 cold (approximate, first exec against an unopened store): %s (gate 500ms)", cold)
	if cold > qa1ColdBudget {
		t.Errorf("cold = %s, want under %s", cold, qa1ColdBudget)
	}
}

// qa1Trace execs this test binary as poplar itself (the runMainEnvVar
// re-exec TestMain already supports) with --startup-trace against the
// store under dataHome, and parses its one JSON trace line.
func qa1Trace(dataHome string) (startupTraceResult, error) {
	cmd := exec.Command(os.Args[0], "--startup-trace") //nolint:gosec // G204: os.Args[0] is this same test binary, re-invoked as its own subprocess
	cmd.Env = append(os.Environ(), runMainEnvVar+"=1", "XDG_DATA_HOME="+dataHome)
	out, err := cmd.Output()
	if err != nil {
		return startupTraceResult{}, fmt.Errorf("run poplar --startup-trace: %w (output: %s)", err, out)
	}
	var trace startupTraceResult
	if err := json.Unmarshal(out, &trace); err != nil {
		return startupTraceResult{}, fmt.Errorf("parse trace output %q: %w", out, err)
	}
	return trace, nil
}

// seedQA1Store migrates a fresh store under dataHome/poplar/store.db
// and fills it with qa1MessageCount messages in one mailbox,
// amplifying a small vocabulary rather than requiring the private
// mail corpus (QA-5's 100k-message envelope). It marks a clean
// shutdown on return, so poplar's own startup skips the integrity
// check QA-1 measures around.
func seedQA1Store(t *testing.T, dataHome string) {
	t.Helper()

	dbDir := filepath.Join(dataHome, "poplar")
	if err := os.MkdirAll(dbDir, 0o750); err != nil {
		t.Fatalf("create %s: %v", dbDir, err)
	}
	path := filepath.Join(dbDir, "store.db")

	db, err := store.OpenWriteConn(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'perf', 'jmap', 'perf@example.com')`); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO mailbox (id, account_id, role, name) VALUES (1, 1, 'inbox', 'Inbox')`); err != nil {
		t.Fatalf("seed mailbox: %v", err)
	}

	vocab := []string{"meeting", "invoice", "update", "review", "report", "schedule", "budget", "project"}
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: a fixed seed makes this fixture's shape reproducible across runs, not a security-sensitive use
	receivedBase := time.Now().Add(-2 * 365 * 24 * time.Hour).Unix()

	const batchSize = 2000
	for batchStart := 0; batchStart < qa1MessageCount; batchStart += batchSize {
		batchEnd := min(batchStart+batchSize, qa1MessageCount)
		if err := seedQA1Batch(db, vocab, rng, receivedBase, batchStart, batchEnd); err != nil {
			t.Fatalf("seed messages %d..%d: %v", batchStart, batchEnd, err)
		}
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close seed connection: %v", err)
	}
	if err := store.MarkCleanShutdown(path); err != nil {
		t.Fatalf("mark clean shutdown: %v", err)
	}
}

func seedQA1Batch(db *sql.DB, vocab []string, rng *rand.Rand, receivedBase int64, batchStart, batchEnd int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	messageStmt, err := tx.Prepare(`INSERT INTO message (id, account_id, thread_key, received_at, subject, from_addr) VALUES (?, 1, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare message insert: %w", err)
	}
	defer func() { _ = messageStmt.Close() }()

	mailboxStmt, err := tx.Prepare(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at, unread) VALUES (?, 1, ?, 0)`)
	if err != nil {
		return fmt.Errorf("prepare message_mailbox insert: %w", err)
	}
	defer func() { _ = mailboxStmt.Close() }()

	for i := batchStart; i < batchEnd; i++ {
		id := int64(i + 1)
		receivedAt := receivedBase + int64(i)*97
		subject := vocab[rng.IntN(len(vocab))] + " " + vocab[rng.IntN(len(vocab))]
		if _, err := messageStmt.Exec(id, fmt.Sprintf("thread-%d", i/6), receivedAt, subject, "sender@example.com"); err != nil {
			return fmt.Errorf("insert message %d: %w", id, err)
		}
		if _, err := mailboxStmt.Exec(id, receivedAt); err != nil {
			return fmt.Errorf("insert message_mailbox %d: %w", id, err)
		}
	}
	return tx.Commit()
}

// perfMeasure runs op exactly count times through testing.B.Loop,
// forcing that exact iteration count with -benchtime's Nx form: QA-1
// names a fixed run count (20), not however many fit in a calibrated
// second. It captures op's own reported duration per call rather than
// B's mean, since a p95 budget needs the distribution a mean hides.
func perfMeasure(t *testing.T, count int, op func() time.Duration) (samples []time.Duration, benchLine string) {
	t.Helper()

	if err := flag.Set("test.benchtime", fmt.Sprintf("%dx", count)); err != nil {
		t.Fatalf("set benchtime: %v", err)
	}
	result := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			samples = append(samples, op())
		}
	})
	return samples, result.String()
}

// perfPercentile returns the p-th percentile (0-100) of samples,
// nearest-rank over the sorted set.
func perfPercentile(samples []time.Duration, p float64) time.Duration {
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	idx := int(p / 100 * float64(len(sorted)-1))
	return sorted[idx]
}

// perfWriteArtifact writes a baseline summary for name to t's
// ArtifactDir: the go-test-style benchmark line plus this run's
// percentiles, so a later pass can benchstat two such files against
// each other.
func perfWriteArtifact(t *testing.T, name, benchLine string, samples []time.Duration) {
	t.Helper()

	summary := fmt.Sprintf("Benchmark%s %s\nn=%d p50=%s p95=%s p99=%s max=%s\n",
		name, benchLine, len(samples),
		perfPercentile(samples, 50), perfPercentile(samples, 95), perfPercentile(samples, 99),
		perfPercentile(samples, 100))

	path := filepath.Join(t.ArtifactDir(), name+".txt")
	if err := os.WriteFile(path, []byte(summary), 0o644); err != nil { //nolint:gosec // G306: a perf artifact is diagnostic output alongside the test binary, not sensitive data
		t.Fatalf("write artifact %s: %v", path, err)
	}
}
