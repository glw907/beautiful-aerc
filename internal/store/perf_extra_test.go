//go:build !race

// The QA-1/2/3 perf harness is excluded from the race build: race
// instrumentation costs 2-20x time and 5-10x memory, so a p95 gate
// asserted under it would measure the detector instead of the store
// (build machine section 2). CI's `go test -race ./...` job never
// links this file.
package store

import "context"

// PerfBody fetches messageID's body content directly, standing in for
// a body-read API pass 3 has not built yet: enough for the QA-2
// harness's reader-open class to exercise a real, non-scalar read.
// Exported so the QA-2/QA-3 tests (package store_test, so they can
// share storetest's measurement helpers without an import cycle back
// into this package) can call it; nothing outside this pass's perf
// tests does.
func (p *ReadPool) PerfBody(ctx context.Context, messageID int64) ([]byte, error) {
	var content []byte
	err := p.db.QueryRowContext(ctx, `SELECT content FROM body WHERE message_id = ?`, messageID).Scan(&content)
	return content, err
}

// PerfSearch runs an FTS5 prefix query against message_fts, standing
// in for the search grammar pass 3 builds: enough for the QA-2
// harness's incremental-search class to exercise the index under
// typing.
func (p *ReadPool) PerfSearch(ctx context.Context, query string) error {
	rows, err := p.db.QueryContext(ctx, `SELECT rowid FROM message_fts WHERE message_fts MATCH ? LIMIT 50`, query)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
	}
	return rows.Err()
}

// PerfSearchFiltered runs an FTS5 MATCH query, newest-first the way a
// mailbox viewport orders a search result, joined to message_mailbox
// and scoped to mailboxID when it is non-zero: the QA-3 harness's
// operator-filtered class, standing in for the search grammar's
// mailbox: filter (pass 3).
//
// The ORDER BY is on message_fts's own rowid (message.id), not the
// joined mailbox row's received_at: id is assigned in received order,
// so rowid DESC is newest-first, and FTS5's virtual table supports an
// ORDER BY rowid scan natively. Ordering by the joined received_at
// column instead forces a full temp b-tree sort of every match before
// LIMIT can apply, measured at ~250ms against a mailbox holding most
// of a 100k-message index versus under 1ms for the rowid form.
func (p *ReadPool) PerfSearchFiltered(ctx context.Context, match string, mailboxID int64) error {
	query := `SELECT rowid FROM message_fts WHERE message_fts MATCH ? ORDER BY rowid DESC LIMIT 50`
	args := []any{match}
	if mailboxID != 0 {
		query = `
			SELECT mm.message_id FROM message_fts
			JOIN message_mailbox mm ON mm.message_id = message_fts.rowid
			WHERE message_fts MATCH ? AND mm.mailbox_id = ?
			ORDER BY message_fts.rowid DESC
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
