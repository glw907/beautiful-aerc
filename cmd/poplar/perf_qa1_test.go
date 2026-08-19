//go:build !race

// The QA-1/2/3 perf harness is excluded from the race build: race
// instrumentation costs 2-20x time and 5-10x memory, so a p95 gate
// asserted under it would measure the detector instead of the store
// (build machine section 2). CI's `go test -race ./...` job never
// links this file.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"

	"github.com/glw907/poplar/internal/store/storetest"
)

const (
	qa1WarmRuns   = 20
	qa1WarmBudget = 200 * time.Millisecond
	qa1ColdBudget = 500 * time.Millisecond

	// qa1ExpectedRows is the first page size --startup-trace lists: the
	// perf corpus's inbox always holds far more than one page, so a
	// warm or cold run returning fewer rows is a broken seed or a
	// wrong mailbox, not a fast one.
	qa1ExpectedRows = 50
)

// TestQA1Startup proves exec-to-first-list-page holds QA-1's budget
// against the seeded perf corpus: p95 under 200ms warm (page cache
// warmed by one prior run), and under 500ms on the first exec against
// a store whose page cache this test evicts first.
func TestQA1Startup(t *testing.T) {
	warmHome := t.TempDir()
	env := seedQA1Store(t, warmHome)

	if _, _, err := qa1Trace(warmHome); err != nil {
		t.Fatalf("warm-up run: %v", err)
	}

	// firstErr, not t.Fatalf, inside the closure: storetest.Measure's
	// op runs on the goroutine testing.Benchmark launches internally,
	// not the one running t, and t.Fatalf must only be called from the
	// goroutine running the test.
	var firstErr error
	samples, line := storetest.Measure(t, qa1WarmRuns, func() time.Duration {
		trace, elapsed, err := qa1Trace(warmHome)
		switch {
		case err != nil && firstErr == nil:
			firstErr = err
		case err == nil && trace.Rows != qa1ExpectedRows && firstErr == nil:
			firstErr = fmt.Errorf("warm run returned %d rows, want %d", trace.Rows, qa1ExpectedRows)
		}
		return elapsed
	})
	if firstErr != nil {
		t.Fatalf("startup-trace run: %v", firstErr)
	}
	storetest.WriteBaseline(t, "testdata/perf-baselines", env.BaselineName("QA1Startup_warm"), line, samples)
	p95 := storetest.Percentile(samples, 95)
	t.Logf("QA-1 warm: p50=%s p95=%s (gate 200ms; spike baseline ~5ms with quick_check off the launch path)",
		storetest.Percentile(samples, 50), p95)
	if p95 > qa1WarmBudget {
		t.Errorf("warm p95 = %s, want under %s", p95, qa1WarmBudget)
	}

	// Cold: the first startup-trace exec against a store whose page
	// cache perfDropPageCache has just evicted, so this process's own
	// seeding moments earlier can't leave the read a hot-cache replay.
	coldHome := t.TempDir()
	seedQA1Store(t, coldHome)
	perfDropPageCache(t, filepath.Join(coldHome, "poplar", "store.db"))
	firstErr = nil
	coldSamples, coldLine := storetest.Measure(t, 1, func() time.Duration {
		trace, elapsed, err := qa1Trace(coldHome)
		switch {
		case err != nil && firstErr == nil:
			firstErr = err
		case err == nil && trace.Rows != qa1ExpectedRows && firstErr == nil:
			firstErr = fmt.Errorf("cold run returned %d rows, want %d", trace.Rows, qa1ExpectedRows)
		}
		return elapsed
	})
	if firstErr != nil {
		t.Fatalf("cold startup-trace run: %v", firstErr)
	}
	storetest.WriteBaseline(t, "testdata/perf-baselines", env.BaselineName("QA1Startup_cold"), coldLine, coldSamples)
	cold := coldSamples[0]
	t.Logf("QA-1 cold (page cache evicted before exec): %s (gate 500ms)", cold)
	if cold > qa1ColdBudget {
		t.Errorf("cold = %s, want under %s", cold, qa1ColdBudget)
	}
}

// qa1Trace execs this test binary as poplar itself (the runMainEnvVar
// re-exec TestMain already supports) with --startup-trace against the
// store under dataHome, and parses its one JSON trace line. The
// returned duration is the wall clock around the exec itself, QA-1's
// named origin, not run's own in-process start time: --startup-trace's
// TotalNS excludes process spawn, Go runtime init, and dynamic
// linking.
func qa1Trace(dataHome string) (startupTraceResult, time.Duration, error) {
	cmd := exec.Command(os.Args[0], "--startup-trace") //nolint:gosec // G204: os.Args[0] is this same test binary, re-invoked as its own subprocess
	cmd.Env = append(os.Environ(), runMainEnvVar+"=1", "XDG_DATA_HOME="+dataHome)
	start := time.Now()
	out, err := cmd.Output()
	elapsed := time.Since(start)
	if err != nil {
		return startupTraceResult{}, 0, fmt.Errorf("run poplar --startup-trace: %w (output: %s)", err, out)
	}
	var trace startupTraceResult
	if err := json.Unmarshal(out, &trace); err != nil {
		return startupTraceResult{}, 0, fmt.Errorf("parse trace output %q: %w", out, err)
	}
	return trace, elapsed, nil
}

// seedQA1Store places storetest's perf corpus under
// dataHome/poplar/store.db, where poplar's own startup path finds it.
func seedQA1Store(t *testing.T, dataHome string) storetest.PerfEnvelope {
	t.Helper()

	dbDir := filepath.Join(dataHome, "poplar")
	if err := os.MkdirAll(dbDir, 0o750); err != nil {
		t.Fatalf("create %s: %v", dbDir, err)
	}
	return storetest.SeedPerfEnvelope(t, filepath.Join(dbDir, "store.db"))
}
