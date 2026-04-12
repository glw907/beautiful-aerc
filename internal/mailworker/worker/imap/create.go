// Forked from aerc (git.sr.ht/~rjarry/aerc) — MIT License
package imap

import (
	"github.com/glw907/poplar/internal/mailworker/worker/types"
)

func (imapw *IMAPWorker) handleCreateDirectory(msg *types.CreateDirectory) {
	if err := imapw.client.Create(msg.Directory); err != nil {
		if msg.Quiet {
			return
		}
		imapw.worker.PostMessage(&types.Error{
			Message: types.RespondTo(msg),
			Error:   err,
		}, nil)
	} else {
		imapw.worker.PostMessage(&types.Done{Message: types.RespondTo(msg)}, nil)
	}
}
