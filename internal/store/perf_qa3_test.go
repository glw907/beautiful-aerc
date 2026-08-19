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
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// qa3RepeatsPerQuery is how many times each committed query runs
// against the seeded perf corpus.
const qa3RepeatsPerQuery = 25

// qa3Query is one committed benchmark query: an FTS5 MATCH expression,
// plus a mailbox id for the operator-filtered class (0 elsewhere).
type qa3Query struct {
	match     string
	mailboxID int64
}

// qa3Class is one of QA-3's four query classes: single term, phrase,
// operator-filtered, and boolean/negation, each with its own p95
// budget.
type qa3Class struct {
	name    string
	budget  time.Duration
	queries []qa3Query
}

// qa3Classes is QA-3's committed benchmark set: at least 20 queries
// across four classes, drawn from storetest.SeedPerfEnvelope's own
// common/medium/rare vocabulary so every query has a known,
// non-accidental hit rate against the index it runs against, spanning
// high, moderate, and low selectivity rather than one uniform
// frequency tier that every query matches nearly the whole index.
// mailboxID is the mailbox the operator-filtered class scopes to.
func qa3Classes(mailboxID int64) []qa3Class {
	singleTerm := make([]qa3Query, 0, 6+len(storetest.MediumWords)+len(storetest.RareWords))
	for _, w := range storetest.CommonWords[:6] {
		singleTerm = append(singleTerm, qa3Query{match: w})
	}
	for _, w := range storetest.MediumWords {
		singleTerm = append(singleTerm, qa3Query{match: w})
	}
	for _, w := range storetest.RareWords {
		singleTerm = append(singleTerm, qa3Query{match: w})
	}

	phrase := make([]qa3Query, 0, 5)
	for i := range 5 {
		phrase = append(phrase, qa3Query{match: `"` + storetest.CommonWords[i] + " " + storetest.CommonWords[i+1] + `"`})
	}

	operatorFiltered := make([]qa3Query, 0, 5)
	for _, w := range storetest.CommonWords[6:11] {
		operatorFiltered = append(operatorFiltered, qa3Query{match: w, mailboxID: mailboxID})
	}

	booleanNegation := make([]qa3Query, 0, 6)
	for i := range 3 {
		a, b := storetest.CommonWords[2*i], storetest.CommonWords[2*i+1]
		booleanNegation = append(booleanNegation, qa3Query{match: a + " OR " + b})
	}
	for i := range 3 {
		booleanNegation = append(booleanNegation, qa3Query{match: storetest.CommonWords[i] + " NOT " + storetest.RareWords[i%len(storetest.RareWords)]})
	}

	return []qa3Class{
		{name: "single_term", budget: 100 * time.Millisecond, queries: singleTerm},
		{name: "phrase", budget: 100 * time.Millisecond, queries: phrase},
		{name: "operator_filtered", budget: 200 * time.Millisecond, queries: operatorFiltered},
		{name: "boolean_negation", budget: 500 * time.Millisecond, queries: booleanNegation},
	}
}

// TestQA3Search proves each of QA-3's four query classes holds its
// per-class p95 budget against the seeded index: single term and
// phrase under 100ms, operator-filtered under 200ms, boolean and
// negation under 500ms.
//
// It measures raw FTS5 MATCH latency over message_fts directly; the
// search grammar that compiles a user's typed query into one (and the
// bounded-scan cap a query with no positive term owes) is pass 3's
// internal/search, not yet built.
func TestQA3Search(t *testing.T) {
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

	for _, class := range qa3Classes(env.MailboxIDs[0]) {
		t.Run(class.name, func(t *testing.T) {
			i := 0
			var firstErr error

			// firstErr, not t.Fatalf, inside the closure:
			// storetest.Measure's op runs on the goroutine
			// testing.Benchmark launches internally, not the one running
			// t, and t.Fatalf must only be called from the goroutine
			// running the test.
			samples, line := storetest.Measure(t, len(class.queries)*qa3RepeatsPerQuery, func() time.Duration {
				q := class.queries[i%len(class.queries)]
				i++
				start := time.Now()
				if err := reads.PerfSearchFiltered(context.Background(), q.match, q.mailboxID); err != nil && firstErr == nil {
					firstErr = fmt.Errorf("query %q: %w", q.match, err)
				}
				return time.Since(start)
			})
			if firstErr != nil {
				t.Fatal(firstErr)
			}

			storetest.WriteBaseline(t, "testdata/perf-baselines", env.BaselineName("QA3Search_"+class.name), line, samples)
			p95 := storetest.Percentile(samples, 95)
			t.Logf("QA-3 %s: p50=%s p95=%s (budget %s; spike reference 0.9-4.5ms p95 across all classes, measured at the full envelope on a quiet machine)",
				class.name, storetest.Percentile(samples, 50), p95, class.budget)
			if p95 > class.budget {
				t.Errorf("%s p95 = %s, want under %s", class.name, p95, class.budget)
			}
		})
	}
}
