//go:build !race

package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// qa3MessageCount and qa3MailboxCount hold QA-3's committed query set
// to the QA-5 scale envelope: a 100k-message index.
const (
	qa3MessageCount    = 100_000
	qa3MailboxCount    = 4
	qa3RepeatsPerQuery = 25
)

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
// across four classes, drawn from perfSeedEnvelope's own vocabulary so
// every query has a known, non-accidental hit rate against the index
// it runs against. mailboxID is the mailbox the operator-filtered
// class scopes to.
func qa3Classes(mailboxID int64) []qa3Class {
	singleTerm := make([]qa3Query, 0, 6)
	for _, w := range perfCommonWords[:6] {
		singleTerm = append(singleTerm, qa3Query{match: w})
	}

	phrase := make([]qa3Query, 0, 5)
	for i := range 5 {
		phrase = append(phrase, qa3Query{match: `"` + perfCommonWords[i] + " " + perfCommonWords[i+1] + `"`})
	}

	operatorFiltered := make([]qa3Query, 0, 5)
	for _, w := range perfCommonWords[6:11] {
		operatorFiltered = append(operatorFiltered, qa3Query{match: w, mailboxID: mailboxID})
	}

	booleanNegation := make([]qa3Query, 0, 6)
	for i := range 3 {
		a, b := perfCommonWords[2*i], perfCommonWords[2*i+1]
		booleanNegation = append(booleanNegation, qa3Query{match: a + " OR " + b})
	}
	for i := range 3 {
		booleanNegation = append(booleanNegation, qa3Query{match: perfCommonWords[i] + " NOT " + perfRareWords[i%len(perfRareWords)]})
	}

	return []qa3Class{
		{name: "single_term", budget: 100 * time.Millisecond, queries: singleTerm},
		{name: "phrase", budget: 100 * time.Millisecond, queries: phrase},
		{name: "operator_filtered", budget: 200 * time.Millisecond, queries: operatorFiltered},
		{name: "boolean_negation", budget: 500 * time.Millisecond, queries: booleanNegation},
	}
}

// TestQA3Search proves each of QA-3's four query classes holds its
// per-class p95 budget against a 100k-message index: single term and
// phrase under 100ms, operator-filtered under 200ms, boolean and
// negation under 500ms.
//
// It measures raw FTS5 MATCH latency over message_fts directly; the
// search grammar that compiles a user's typed query into one (and the
// bounded-scan cap a query with no positive term owes) is pass 3's
// internal/search, not yet built.
func TestQA3Search(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	env := perfSeedEnvelope(t, path, qa3MessageCount, qa3MailboxCount)

	writer, err := Open(path, DefaultWriterConfig())
	if err != nil {
		t.Fatalf("open writer: %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	reads, err := NewReadPool(path, DefaultReadPoolSize, writer.Revision())
	if err != nil {
		t.Fatalf("open read pool: %v", err)
	}
	t.Cleanup(func() { _ = reads.Close() })

	for _, class := range qa3Classes(env.mailboxIDs[0]) {
		t.Run(class.name, func(t *testing.T) {
			i := 0
			var firstErr error

			// firstErr, not t.Fatalf, inside the closure: perfMeasure's
			// op runs on the goroutine testing.Benchmark launches
			// internally, not the one running t, and t.Fatalf must only
			// be called from the goroutine running the test.
			samples, line := perfMeasure(t, len(class.queries)*qa3RepeatsPerQuery, func() time.Duration {
				q := class.queries[i%len(class.queries)]
				i++
				start := time.Now()
				if err := reads.perfSearchFiltered(context.Background(), q.match, q.mailboxID); err != nil && firstErr == nil {
					firstErr = fmt.Errorf("query %q: %w", q.match, err)
				}
				return time.Since(start)
			})
			if firstErr != nil {
				t.Fatal(firstErr)
			}

			perfWriteArtifact(t, "QA3Search_"+class.name, line, samples)
			p95 := perfPercentile(samples, 95)
			t.Logf("QA-3 %s: p50=%s p95=%s (budget %s; spike baseline 0.9-4.5ms p95 across all classes)",
				class.name, perfPercentile(samples, 50), p95, class.budget)
			if p95 > class.budget {
				t.Errorf("%s p95 = %s, want under %s", class.name, p95, class.budget)
			}
		})
	}
}

// perfSearchFiltered runs an FTS5 MATCH query, joined to
// message_mailbox and scoped to mailboxID when it is non-zero: QA-3's
// operator-filtered class, standing in for the search grammar's
// mailbox: filter (pass 3).
func (p *ReadPool) perfSearchFiltered(ctx context.Context, match string, mailboxID int64) error {
	query := `SELECT rowid FROM message_fts WHERE message_fts MATCH ? LIMIT 50`
	args := []any{match}
	if mailboxID != 0 {
		query = `
			SELECT mm.message_id FROM message_fts
			JOIN message_mailbox mm ON mm.message_id = message_fts.rowid
			WHERE message_fts MATCH ? AND mm.mailbox_id = ?
			LIMIT 50`
		args = append(args, mailboxID)
	}

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
	}
	return rows.Err()
}
