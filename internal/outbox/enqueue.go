package outbox

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/store"
)

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

// EnqueueCreateMailbox enqueues a KindCreateMailbox intent for a
// mailbox named name under parentMailboxID (an existing mailbox row,
// or 0 for the root) or, offline, under parentRef (another queued
// KindCreateMailbox intent's own id).
func EnqueueCreateMailbox(ctx context.Context, w *store.Writer, accountID int64, name string, parentMailboxID, parentRef int64, now time.Time) (intentID int64, undoGroup string, err error) {
	payload, err := json.Marshal(CreateMailboxPayload{Name: name, ParentMailboxID: parentMailboxID, ParentRef: parentRef})
	if err != nil {
		return 0, "", err
	}
	undoGroup = newUndoGroup()
	err = w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		var txErr error
		intentID, txErr = insertRow(tx, accountID, KindCreateMailbox, payload, undoGroup, 0, now, now)
		return txErr
	})
	return intentID, undoGroup, err
}

// EnqueueRenameMailbox enqueues a KindRenameMailbox intent renaming
// mailboxID (an existing mailbox row) to name.
func EnqueueRenameMailbox(ctx context.Context, w *store.Writer, accountID, mailboxID int64, name string, now time.Time) (intentID int64, undoGroup string, err error) {
	payload, err := json.Marshal(RenameMailboxPayload{MailboxID: mailboxID, Name: name})
	if err != nil {
		return 0, "", err
	}
	undoGroup = newUndoGroup()
	err = w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		var txErr error
		intentID, txErr = insertRow(tx, accountID, KindRenameMailbox, payload, undoGroup, 0, now, now)
		return txErr
	})
	return intentID, undoGroup, err
}

// EnqueueDeleteMailbox enqueues a KindDeleteMailbox intent removing
// mailboxID.
func EnqueueDeleteMailbox(ctx context.Context, w *store.Writer, accountID, mailboxID int64, now time.Time) (intentID int64, undoGroup string, err error) {
	payload, err := json.Marshal(DeleteMailboxPayload{MailboxID: mailboxID})
	if err != nil {
		return 0, "", err
	}
	undoGroup = newUndoGroup()
	err = w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		var txErr error
		intentID, txErr = insertRow(tx, accountID, KindDeleteMailbox, payload, undoGroup, 0, now, now)
		return txErr
	})
	return intentID, undoGroup, err
}

// EnqueueMoveMessagesBulk enqueues messageIDs' move to destMailboxID
// (an existing mailbox row) or, offline, destRef (a queued
// KindCreateMailbox intent's own id), split into chunks of at most
// maxPerChunk messages sharing one undo_group and ordered by
// chunk_seq: LT-3's bulk-action shape, sized under the backend's
// maxObjectsInSet. undoable holds each chunk queued for UndoWindow
// before it becomes dispatch-eligible (UX-9); a non-undoable move
// (a sync-driven or system action) is eligible immediately.
//
// Each message's current mailbox is read and recorded as its prior
// state before the move, so a compensating move can restore it
// without a second read once this intent's row is gone.
func EnqueueMoveMessagesBulk(ctx context.Context, w *store.Writer, accountID int64, messageIDs []int64, destMailboxID, destRef int64, maxPerChunk int, undoable bool, now time.Time) (undoGroup string, intentIDs []int64, err error) {
	if (destMailboxID == 0) == (destRef == 0) {
		return "", nil, fmt.Errorf("outbox: move messages: exactly one of destMailboxID, destRef must be set")
	}
	if maxPerChunk <= 0 {
		maxPerChunk = len(messageIDs)
	}
	hold := time.Duration(0)
	if undoable {
		hold = UndoWindow
	}
	undoGroup = newUndoGroup()

	err = w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		for seq, chunk := range chunks(messageIDs, maxPerChunk) {
			prior := make(map[int64]int64, len(chunk))
			for _, msgID := range chunk {
				mailboxID, err := currentMailboxID(tx, msgID)
				if err != nil {
					return err
				}
				prior[msgID] = mailboxID
			}
			payload, err := json.Marshal(MoveMessagesPayload{
				MessageIDs:      chunk,
				DestMailboxID:   destMailboxID,
				DestRef:         destRef,
				PriorMailboxIDs: prior,
			})
			if err != nil {
				return err
			}
			id, err := insertRow(tx, accountID, KindMoveMessages, payload, undoGroup, seq, now.Add(hold), now)
			if err != nil {
				return err
			}
			intentIDs = append(intentIDs, id)
		}
		return nil
	})
	return undoGroup, intentIDs, err
}

// chunks splits ids into consecutive slices of at most size elements
// each.
func chunks(ids []int64, size int) [][]int64 {
	var out [][]int64
	for len(ids) > 0 {
		n := min(size, len(ids))
		out = append(out, ids[:n])
		ids = ids[n:]
	}
	return out
}
