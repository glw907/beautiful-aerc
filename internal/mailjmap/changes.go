// SPDX-License-Identifier: MIT

package mailjmap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"

	"github.com/glw907/poplar/internal/mail"
)

// Changes implements mail.ChangeTracker.
//
// JMAP Email/changes is account-scoped (RFC 8621 §4.3); the folder
// argument is therefore ignored. The cache routes the returned
// Email IDs through FetchHeaders to recover per-mailbox membership
// via the message_mailboxes junction. The SyncToken is the JMAP
// Email type's state string encoded as UTF-8 bytes.
//
// On cannotCalculateChanges the function returns
// mail.ErrCannotCalculateChanges; the caller (cache syncer) responds
// with the re-anchor path from spec §D.4.
func (b *Backend) Changes(ctx context.Context, _ string, since mail.SyncToken) (mail.ChangeSet, mail.SyncToken, error) {
	b.mu.Lock()
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()

	state := string(since)
	var out mail.ChangeSet
	for {
		if err := ctx.Err(); err != nil {
			return mail.ChangeSet{}, since, err
		}
		req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
		req.Invoke(&email.Changes{Account: accountID, SinceState: state})
		resp, err := b.do(req)
		if err != nil {
			if isCannotCalculateChanges(err) {
				return mail.ChangeSet{}, since, mail.ErrCannotCalculateChanges
			}
			return mail.ChangeSet{}, since, fmt.Errorf("email/changes: %w", err)
		}
		var cr *email.ChangesResponse
		for _, inv := range resp.Responses {
			if r, ok := inv.Args.(*email.ChangesResponse); ok {
				cr = r
				break
			}
		}
		if cr == nil {
			return mail.ChangeSet{}, since, errors.New("email/changes: no response")
		}
		out.Added = append(out.Added, idsToUIDs(cr.Created)...)
		out.Modified = append(out.Modified, idsToUIDs(cr.Updated)...)
		out.Removed = append(out.Removed, idsToUIDs(cr.Destroyed)...)
		state = cr.NewState
		if !cr.HasMoreChanges {
			break
		}
	}
	return out, mail.SyncToken(state), nil
}

// isCannotCalculateChanges sniffs a JMAP method-error name.
// Substring match on the rendered error string is the workaround
// until go-jmap exposes a typed method-error value (upstream issue);
// the JSON wire shape places the kind under "type".
func isCannotCalculateChanges(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "cannotCalculateChanges") ||
		strings.Contains(s, "CannotCalculateChanges")
}
