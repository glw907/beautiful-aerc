package outbox

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/uerr"
)

// defaultWireBatch is the per-request object limit assumed for a
// backend that reports no MaxObjectsInSet at all, so a batch is
// bounded even against a server that named no bound.
const defaultWireBatch = 50

// moveChunkMessages is the store-side bound on one bulk-move chunk,
// and it is a transaction budget rather than a wire one: the chunk's
// enqueue runs one prior-mailbox point query per message inside a
// single interactive transaction, and its later claim runs one
// message point query per message inside another. Measured against a
// migrated store on the development machine, one 250-message chunk
// cost 6.0ms to enqueue, an eighth of ADR-0003's 50ms admission
// ceiling, which leaves room for a machine several times slower. A
// server's MaxObjectsInSet is a bound on one request and says nothing
// about either transaction: Fastmail reports 4096, at which one chunk
// held the writer for 87ms.
const moveChunkMessages = 250

// newUndoGroup returns a fresh random undo_group id: every Enqueue*
// call mints one, single intents and bulk chunks alike, so a caller
// always has a group key to hand Undo regardless of how many rows the
// action produced.
func newUndoGroup() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("outbox: read random undo group: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// enqueueSingle marshals payload and inserts one intent of kind,
// under a fresh undo group of its own, dispatch-eligible immediately.
// Every single-row Enqueue*Tx entry point goes through it; a bulk move
// does not, since its chunks share one group and hold for the undo
// window.
func enqueueSingle(tx *sql.Tx, accountID int64, kind Kind, payload any, now time.Time) (intentID int64, undoGroup string, err error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, "", err
	}
	undoGroup = newUndoGroup()
	intentID, err = insertRow(tx, accountID, kind, raw, undoGroup, 0, now, now)
	return intentID, undoGroup, err
}

// enqueueInOwnTx runs enqueue, one of the Enqueue*Tx entry points, in
// its own interactive-lane transaction, the Writer-level shape every
// single-intent Enqueue call shares.
func enqueueInOwnTx(ctx context.Context, w *store.Writer, enqueue func(*sql.Tx) (int64, string, error)) (intentID int64, undoGroup string, err error) {
	err = w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		var txErr error
		intentID, undoGroup, txErr = enqueue(tx)
		return txErr
	})
	return intentID, undoGroup, err
}

// EnqueueCreateMailboxTx enqueues a KindCreateMailbox intent for a
// mailbox named name under parentMailboxID (an existing mailbox row,
// or 0 for the root) or, offline, under parentRef (another queued
// KindCreateMailbox intent's own id) inside tx, so a caller commits it
// alongside its own local mutation in one writer transaction.
func EnqueueCreateMailboxTx(tx *sql.Tx, accountID int64, name string, parentMailboxID, parentRef int64, now time.Time) (intentID int64, undoGroup string, err error) {
	return enqueueSingle(tx, accountID, KindCreateMailbox,
		CreateMailboxPayload{Name: name, ParentMailboxID: parentMailboxID, ParentRef: parentRef}, now)
}

// EnqueueCreateMailbox is EnqueueCreateMailboxTx's Writer-level
// convenience for a caller with no local mutation of its own to pair
// it with: it opens its own interactive-lane transaction.
func EnqueueCreateMailbox(ctx context.Context, w *store.Writer, accountID int64, name string, parentMailboxID, parentRef int64, now time.Time) (intentID int64, undoGroup string, err error) {
	return enqueueInOwnTx(ctx, w, func(tx *sql.Tx) (int64, string, error) {
		return EnqueueCreateMailboxTx(tx, accountID, name, parentMailboxID, parentRef, now)
	})
}

// EnqueueRenameMailboxTx enqueues a KindRenameMailbox intent renaming
// mailboxID (an existing mailbox row) to name inside tx.
func EnqueueRenameMailboxTx(tx *sql.Tx, accountID, mailboxID int64, name string, now time.Time) (intentID int64, undoGroup string, err error) {
	return enqueueSingle(tx, accountID, KindRenameMailbox,
		RenameMailboxPayload{MailboxID: mailboxID, Name: name}, now)
}

// EnqueueRenameMailbox is EnqueueRenameMailboxTx's Writer-level
// convenience: it opens its own interactive-lane transaction.
func EnqueueRenameMailbox(ctx context.Context, w *store.Writer, accountID, mailboxID int64, name string, now time.Time) (intentID int64, undoGroup string, err error) {
	return enqueueInOwnTx(ctx, w, func(tx *sql.Tx) (int64, string, error) {
		return EnqueueRenameMailboxTx(tx, accountID, mailboxID, name, now)
	})
}

// EnqueueDeleteMailboxTx enqueues a KindDeleteMailbox intent removing
// mailboxID inside tx.
func EnqueueDeleteMailboxTx(tx *sql.Tx, accountID, mailboxID int64, now time.Time) (intentID int64, undoGroup string, err error) {
	return enqueueSingle(tx, accountID, KindDeleteMailbox,
		DeleteMailboxPayload{MailboxID: mailboxID}, now)
}

// EnqueueDeleteMailbox is EnqueueDeleteMailboxTx's Writer-level
// convenience: it opens its own interactive-lane transaction.
func EnqueueDeleteMailbox(ctx context.Context, w *store.Writer, accountID, mailboxID int64, now time.Time) (intentID int64, undoGroup string, err error) {
	return enqueueInOwnTx(ctx, w, func(tx *sql.Tx) (int64, string, error) {
		return EnqueueDeleteMailboxTx(tx, accountID, mailboxID, now)
	})
}

// EnqueueMoveMessagesChunkTx enqueues one chunk of a bulk move inside
// tx: messageIDs' move to destMailboxID (an existing mailbox row) or,
// offline, destRef (a queued KindCreateMailbox intent's own id), part
// of undoGroup at chunkSeq, held until holdUntil (UX-9's undo
// window). Each message's current mailbox is read and recorded as its
// prior state before the move, so a compensating move can restore it
// without a second read once this intent's row is gone. A caller
// pairs this with its own local mutation in the same transaction;
// EnqueueMoveMessagesBulk calls it once per chunk, each its own
// transaction, so one chunk's write stays within the writer's
// admission ceiling regardless of how many chunks the whole action
// produces.
func EnqueueMoveMessagesChunkTx(tx *sql.Tx, accountID int64, messageIDs []int64, destMailboxID, destRef int64, undoGroup string, chunkSeq int, holdUntil, now time.Time) (intentID int64, err error) {
	prior := make(map[int64]int64, len(messageIDs))
	for _, msgID := range messageIDs {
		mailboxID, err := currentMailboxID(tx, msgID)
		if err != nil {
			return 0, err
		}
		prior[msgID] = mailboxID
	}
	payload, err := json.Marshal(MoveMessagesPayload{
		MessageIDs:      messageIDs,
		DestMailboxID:   destMailboxID,
		DestRef:         destRef,
		PriorMailboxIDs: prior,
	})
	if err != nil {
		return 0, err
	}
	return insertRow(tx, accountID, KindMoveMessages, payload, undoGroup, chunkSeq, holdUntil, now)
}

// EnqueueMoveMessagesBulk enqueues messageIDs' move to destMailboxID
// (an existing mailbox row) or, offline, destRef (a queued
// KindCreateMailbox intent's own id), split into moveChunkSize
// chunks, sharing one undo_group and ordered by chunk_seq (LT-3's
// bulk-action shape). Each chunk commits in its own writer
// transaction, so one chunk's write (its prior-mailbox reads plus its
// insert) stays within the writer's admission ceiling no matter how
// many messages the whole action covers. undoable holds
// each chunk queued for UndoWindow before it becomes dispatch-eligible
// (UX-9); a non-undoable move (a sync-driven or system action) is
// eligible immediately.
//
// A chunk that fails to enqueue undoes every chunk this call already
// committed before returning err, so a caller checking err can treat
// the whole call as never having run. The one gap is a non-undoable
// move: a chunk a concurrent DispatchOnce pass already claimed before
// undo runs survives it, and intentIDs then names those escaped
// chunks (err carries the same detail, and it is logged).
func EnqueueMoveMessagesBulk(ctx context.Context, w *store.Writer, accountID int64, messageIDs []int64, destMailboxID, destRef int64, be backend.Backend, undoable bool, now time.Time) (undoGroup string, intentIDs []int64, err error) {
	if (destMailboxID == 0) == (destRef == 0) {
		return "", nil, fmt.Errorf("outbox: move messages: exactly one of destMailboxID, destRef must be set")
	}
	hold := time.Duration(0)
	if undoable {
		hold = UndoWindow
	}
	holdUntil := now.Add(hold)
	undoGroup = newUndoGroup()

	for seq, chunk := range chunks(messageIDs, moveChunkSize(be)) {
		var id int64
		chunkErr := w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
			var txErr error
			id, txErr = EnqueueMoveMessagesChunkTx(tx, accountID, chunk, destMailboxID, destRef, undoGroup, seq, holdUntil, now)
			return txErr
		})
		if chunkErr != nil {
			intentIDs, err = compensateFailedChunk(ctx, w, intentIDs, seq, chunkErr)
			return undoGroup, intentIDs, err
		}
		intentIDs = append(intentIDs, id)
	}
	return undoGroup, intentIDs, nil
}

// compensateFailedChunk runs when chunk seq fails to enqueue after
// committed's chunks already landed: it undoes every one of them so
// the call leaves nothing behind for a caller that treats chunkErr as
// "nothing happened". committed is all still queued (a chunk this
// call just wrote holds past its own commit, so nothing but a
// concurrent DispatchOnce pass on a non-undoable move can claim it
// first), so undo ordinarily catches every one; it returns whichever
// ids that pass did catch, if any, as the true result of the call,
// logged through uerr since that is a bulk move partly taking effect
// despite the error the caller sees.
func compensateFailedChunk(ctx context.Context, w *store.Writer, committed []int64, seq int, chunkErr error) ([]int64, error) {
	if len(committed) == 0 {
		return nil, fmt.Errorf("outbox: move messages: chunk %d: %w", seq, chunkErr)
	}
	annihilated, undoErr := Undo(ctx, w, committed)
	if undoErr != nil {
		return committed, fmt.Errorf("outbox: move messages: chunk %d: %w (rollback of %d earlier chunk(s) failed: %v)", seq, chunkErr, len(committed), undoErr)
	}
	escaped := escapedIDs(committed, annihilated)
	if len(escaped) == 0 {
		return nil, fmt.Errorf("outbox: move messages: chunk %d: %w (%d earlier chunk(s) rolled back)", seq, chunkErr, len(committed))
	}
	err := fmt.Errorf("outbox: move messages: chunk %d: %w (%d of %d earlier chunk(s) already dispatched, not rolled back)", seq, chunkErr, len(escaped), len(committed))
	_ = uerr.New("outbox.enqueue_move", idStrings(escaped), uerr.ClassStoreLocal, err)
	return escaped, err
}

// escapedIDs returns every id in committed that annihilated does not
// cover: the chunks Undo's queued-only guard could not catch because
// a concurrent DispatchOnce pass had already claimed them.
func escapedIDs(committed, annihilated []int64) []int64 {
	caught := make(map[int64]bool, len(annihilated))
	for _, id := range annihilated {
		caught[id] = true
	}
	var escaped []int64
	for _, id := range committed {
		if !caught[id] {
			escaped = append(escaped, id)
		}
	}
	return escaped
}

// idStrings renders ids as decimal strings for uerr's redaction-safe
// IDs field, which takes strings regardless of whether the id names a
// server or an internal row.
func idStrings(ids []int64) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = strconv.FormatInt(id, 10)
	}
	return out
}

// wireBatchLimit returns the most objects one request to be may
// carry: its reported MaxObjectsInSet, or defaultWireBatch when it
// reported none.
func wireBatchLimit(be backend.Backend) int {
	if n := be.Capabilities().Limits.MaxObjectsInSet; n > 0 {
		return n
	}
	return defaultWireBatch
}

// moveChunkSize returns the most messages one bulk-move chunk
// carries. A chunk is both one store transaction and one request, so
// it is bounded by whichever of the two budgets is smaller.
func moveChunkSize(be backend.Backend) int {
	return min(moveChunkMessages, wireBatchLimit(be))
}

func chunks(ids []int64, size int) [][]int64 {
	var out [][]int64
	for len(ids) > 0 {
		n := min(size, len(ids))
		out = append(out, ids[:n])
		ids = ids[n:]
	}
	return out
}
