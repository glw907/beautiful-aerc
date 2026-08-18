package store

import (
	"database/sql"
	"slices"
	"testing"
)

// TestExplainQueryPlan holds a golden EXPLAIN QUERY PLAN detail per
// hot query (ADR-0002's index set): a plan that degrades to a table
// scan produces different detail text and fails the exact-match
// assertion below.
func TestExplainQueryPlan(t *testing.T) {
	db := openMigratedTestDB(t)

	tests := []struct {
		name  string
		query string
		args  []any
		want  []string
	}{
		{
			"mailbox list",
			queryMailboxList,
			[]any{1},
			[]string{"SEARCH message_mailbox USING COVERING INDEX idx_message_mailbox_list (mailbox_id=?)"},
		},
		{
			"cross-folder thread",
			queryThreadAcrossFolders,
			[]any{1, "t"},
			[]string{"SEARCH message USING COVERING INDEX idx_message_thread (account_id=? AND thread_key=?)"},
		},
		{
			"unread partial index",
			queryMailboxUnread,
			[]any{1},
			[]string{"SEARCH message_mailbox USING COVERING INDEX idx_message_mailbox_unread (mailbox_id=?)"},
		},
		{
			"outbox dispatch",
			queryOutboxDispatch,
			[]any{"pending", 100},
			[]string{"SEARCH outbox USING COVERING INDEX idx_outbox_dispatch (state=? AND next_attempt_at<?)"},
		},
		{
			"outbox eligibility probe",
			queryOutboxEligible,
			[]any{100, 1},
			[]string{
				"SCAN CONSTANT ROW",
				"SCALAR SUBQUERY 1",
				"SEARCH outbox USING INDEX idx_outbox_dispatch (state=? AND next_attempt_at<?)",
			},
		},
		{
			"occurrence by range",
			queryOccurrenceByRange,
			[]any{1, 2},
			[]string{"SEARCH occurrence USING INDEX idx_occurrence_start_utc (start_utc>? AND start_utc<?)"},
		},
		{
			"occurrence by local date",
			queryOccurrenceByLocalDate,
			[]any{"2026-07-28"},
			[]string{"SEARCH occurrence USING INDEX idx_occurrence_local_date (local_date=?)"},
		},
		{
			"mailbox list forward",
			queryMailboxListForward,
			[]any{1, 100, 100, 5, 50},
			[]string{"SEARCH message_mailbox USING COVERING INDEX idx_message_mailbox_list (mailbox_id=? AND received_at<?)"},
		},
		{
			"mailbox list backward",
			queryMailboxListBackward,
			[]any{1, 100, 100, 5, 50},
			[]string{"SEARCH message_mailbox USING COVERING INDEX idx_message_mailbox_list (mailbox_id=? AND received_at>?)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := explainPlan(t, db, tt.query, tt.args...)
			if !slices.Equal(got, tt.want) {
				t.Errorf("plan = %v, want %v", got, tt.want)
			}
		})
	}
}

// explainPlan returns the detail column of EXPLAIN QUERY PLAN for
// query against args, one entry per plan row.
func explainPlan(t *testing.T, db *sql.DB, query string, args ...any) []string {
	t.Helper()

	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain query plan %q: %v", query, err)
	}
	defer func() { _ = rows.Close() }()

	var details []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan explain row: %v", err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate explain rows: %v", err)
	}
	return details
}
