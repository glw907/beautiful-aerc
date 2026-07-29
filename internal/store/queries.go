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

	// queryMailboxListForward is the read API's keyset-paginated
	// mailbox list, paging toward older mail: rows before a cursor in
	// idx_message_mailbox_list's received_at DESC, message_id ASC
	// order.
	//
	// The redundant "received_at <= ?" conjunct is load-bearing. With
	// bound parameters, rather than literal constants, SQLite cannot
	// derive an index seek bound from "received_at < ? OR
	// (received_at = ? AND message_id > ?)" alone, and degrades to a
	// full scan of the mailbox filtering row by row. The plain
	// conjunct restores the seek, verified against
	// TestExplainQueryPlan's golden.
	//
	// The inner OR breaks ties on message_id so two rows sharing a
	// received_at are still a total order. The column set is scalar
	// only. The list path never touches message.data.
	queryMailboxListForward = `SELECT message_id, received_at FROM message_mailbox WHERE mailbox_id = ? AND received_at <= ? AND (received_at < ? OR message_id > ?) ORDER BY received_at DESC, message_id ASC LIMIT ?`

	// queryMailboxListBackward is the same window paging toward newer
	// mail: rows after a cursor, walked ascending so the same index
	// still covers it, with the same redundant-bound shape
	// queryMailboxListForward's comment explains. The caller reverses
	// the rows into display order.
	queryMailboxListBackward = `SELECT message_id, received_at FROM message_mailbox WHERE mailbox_id = ? AND received_at >= ? AND (received_at > ? OR message_id < ?) ORDER BY received_at ASC, message_id DESC LIMIT ?`

	// queryMessageSummaryByID is the read API's companion to the
	// mailbox list: the scalar columns LT-1's row needs beyond
	// MailboxRow's id and date, selected by message id. The caller
	// appends one "?" per id and closes the IN clause. The column set
	// never selects message.data.
	queryMessageSummaryByID = `SELECT id, subject, from_addr, flags, has_attachment, thread_key FROM message WHERE id IN (`

	// queryMailboxFirstID is FirstMailboxID's lookup: the lowest-id
	// mailbox in the store, for a caller with no onboarding flow yet to
	// name one explicitly.
	queryMailboxFirstID = `SELECT id FROM mailbox ORDER BY id LIMIT 1`
)
