// Forked from aerc (git.sr.ht/~rjarry/aerc) — MIT License
package imap

import (
	"io"

	"github.com/glw907/poplar/internal/mailworker/models"
	"github.com/glw907/poplar/internal/mailworker/worker/types"
)

func (imapw *IMAPWorker) handleCopyMessages(msg *types.CopyMessages) {
	uids := toSeqSet(msg.Uids)
	if err := imapw.client.UidCopy(uids, msg.Destination); err != nil {
		imapw.worker.PostMessage(&types.Error{
			Message: types.RespondTo(msg),
			Error:   err,
		}, nil)
	} else {
		imapw.worker.PostMessage(&types.MessagesCopied{
			Message:     types.RespondTo(msg),
			Destination: msg.Destination,
			Uids:        msg.Uids,
		}, nil)
		imapw.worker.PostMessage(&types.Done{Message: types.RespondTo(msg)}, nil)
	}
}

type appendLiteral struct {
	io.Reader
	Length int
}

func (m appendLiteral) Len() int {
	return m.Length
}

func (imapw *IMAPWorker) handleAppendMessage(msg *types.AppendMessage) {
	if err := imapw.client.Append(msg.Destination, translateFlags(msg.Flags), msg.Date,
		&appendLiteral{
			Reader: msg.Reader,
			Length: msg.Length,
		}); err != nil {
		imapw.worker.PostMessage(&types.Error{
			Message: types.RespondTo(msg),
			Error:   err,
		}, nil)
	} else {
		imapw.worker.PostMessage(&types.Done{Message: types.RespondTo(msg)}, nil)
	}
}

func (imapw *IMAPWorker) handleMoveMessages(msg *types.MoveMessages) {
	drain := imapw.drainUpdates()
	defer drain.Close()

	// Build provider-dependent EXPUNGE handler.
	imapw.BuildExpungeHandler(models.UidToUint32List(msg.Uids), false)

	uids := toSeqSet(msg.Uids)
	if err := imapw.client.UidMove(uids, msg.Destination); err != nil {
		imapw.worker.PostMessage(&types.Error{
			Message: types.RespondTo(msg),
			Error:   err,
		}, nil)
	} else {
		imapw.worker.PostMessage(&types.MessagesMoved{
			Message:     types.RespondTo(msg),
			Destination: msg.Destination,
			Uids:        msg.Uids,
		}, nil)
		imapw.worker.PostMessage(&types.Done{Message: types.RespondTo(msg)}, nil)
	}
}
