package store

// Hot-query SQL, one constant per query ADR-0002's index set exists
// for. The read API issues these verbatim rather than re-spelling
// the SQL, so TestExplainQueryPlan's goldens hold for every caller: a
// schema change that breaks one of these plans fails here instead of
// at QA-2 or QA-3.
const (
	// queryMailboxList is the folder list query: message ids in a
	// mailbox, newest first. idx_message_mailbox_list covers it, so
	// it never touches the message table.
	queryMailboxList = `SELECT message_id FROM message_mailbox WHERE mailbox_id = ? ORDER BY received_at DESC, message_id`

	// queryThreadAcrossFolders is TH-2's cross-folder thread query:
	// every message sharing a thread_key within an account, in
	// arrival order.
	queryThreadAcrossFolders = `SELECT id FROM message WHERE account_id = ? AND thread_key = ? ORDER BY received_at`

	// queryMailboxUnread is LT-6's next-unread query: unread message
	// ids in a mailbox, newest first. idx_message_mailbox_unread is
	// the partial index over message_mailbox.unread.
	queryMailboxUnread = `SELECT message_id FROM message_mailbox WHERE mailbox_id = ? AND unread = 1 ORDER BY received_at DESC, message_id`

	// queryOutboxDispatch is the outbox drainer's pickup query:
	// dispatchable rows in a given state, earliest attempt first.
	queryOutboxDispatch = `SELECT id FROM outbox WHERE state = ? AND next_attempt_at <= ? ORDER BY next_attempt_at`

	// queryOccurrenceByRange is the calendar view's window query:
	// occurrences starting within [from, to).
	queryOccurrenceByRange = `SELECT event_id FROM occurrence WHERE start_utc >= ? AND start_utc < ?`

	// queryOccurrenceByLocalDate is the day-view query: occurrences
	// falling on a given local calendar date.
	queryOccurrenceByLocalDate = `SELECT event_id FROM occurrence WHERE local_date = ?`
)
