// Forked from aerc (git.sr.ht/~rjarry/aerc) — MIT License
package imap

import (
	"github.com/glw907/beautiful-aerc/internal/aercfork/worker/types"
)

func (imapw *IMAPWorker) handleRemoveDirectory(msg *types.RemoveDirectory) {
	if err := imapw.client.Delete(msg.Directory); err != nil {
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
