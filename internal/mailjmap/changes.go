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
// On a nil since token Changes runs the per-folder baseline pull —
// Email/query paged by inMailbox, plus a follow-up Email/get to
// capture the current type-state. Email/changes itself has no
// "give me everything that exists now" mode (RFC 8621 §4.3), so an
// empty sinceState would return an empty delta and lose the initial
// population.
//
// For a non-nil since the call is account-scoped Email/changes
// (folder argument ignored). The cache routes returned Email IDs
// through FetchHeaders to recover per-mailbox membership via the
// message_mailboxes junction. The SyncToken is the JMAP Email
// type's state string encoded as UTF-8 bytes.
//
// On cannotCalculateChanges the function returns
// mail.ErrCannotCalculateChanges; the caller (cache syncer) responds
// with the re-anchor path from spec §D.4.
func (b *Backend) Changes(ctx context.Context, folder string, since mail.SyncToken) (mail.ChangeSet, mail.SyncToken, error) {
	if since == nil {
		return b.baselinePull(ctx, folder)
	}

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
			return mail.ChangeSet{}, since, fmt.Errorf("changes since %q: %v", state, err)
		}
		var cr *email.ChangesResponse
		for _, inv := range resp.Responses {
			if r, ok := inv.Args.(*email.ChangesResponse); ok {
				cr = r
				break
			}
		}
		if cr == nil {
			return mail.ChangeSet{}, since, errors.New("changes: empty response")
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

// baselinePullPageSize is the per-page limit for Email/query during
// the initial folder pull. Sized to land any inbox under that count
// in a single round-trip while keeping per-page latency bounded;
// also the chunk size FetchHeaders uses for the follow-up Email/get.
const baselinePullPageSize = 500

// stateProbeID is a deliberately non-existent Email id sent with
// every baseline-pull request: JSON omitempty drops a nil/empty IDs
// slice and Fastmail then returns a state-less Email/get response.
// The id lands in notFound; the response carries the type-state
// the next Changes call needs.
const stateProbeID jmap.ID = "_poplar_state_probe_"

func (b *Backend) baselinePull(ctx context.Context, folder string) (mail.ChangeSet, mail.SyncToken, error) {
	b.mu.Lock()
	entry, ok := b.folders[folder]
	accountID := b.session.PrimaryAccounts[jmapmail.URI]
	b.mu.Unlock()
	if !ok {
		return mail.ChangeSet{}, nil, fmt.Errorf("baseline pull: unknown folder %q", folder)
	}

	var added []mail.UID
	var state string
	var position int64
	for {
		if err := ctx.Err(); err != nil {
			return mail.ChangeSet{}, nil, err
		}
		req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
		req.Invoke(&email.Query{
			Account: accountID,
			Filter: &email.FilterCondition{
				InMailbox: jmap.ID(entry.id),
			},
			Sort: []*email.SortComparator{
				{Property: "receivedAt", IsAscending: false},
			},
			Position:       position,
			Limit:          baselinePullPageSize,
			CalculateTotal: true,
		})
		// Piggyback the state probe so a single-page pull (the common
		// case) finishes in one roundtrip. Multi-page pulls overwrite
		// state on each iteration; the last page wins.
		req.Invoke(&email.Get{
			Account:    accountID,
			IDs:        []jmap.ID{stateProbeID},
			Properties: []string{"id"},
		})
		resp, err := b.do(req)
		if err != nil {
			return mail.ChangeSet{}, nil, fmt.Errorf("baseline query: %w", err)
		}
		var qr *email.QueryResponse
		var gr *email.GetResponse
		for _, inv := range resp.Responses {
			switch r := inv.Args.(type) {
			case *email.QueryResponse:
				qr = r
			case *email.GetResponse:
				gr = r
			}
		}
		if qr == nil {
			return mail.ChangeSet{}, nil, errors.New("baseline pull: no Email/query response")
		}
		if added == nil {
			added = make([]mail.UID, 0, qr.Total)
		}
		added = append(added, idsToUIDs(qr.IDs)...)
		if gr != nil && gr.State != "" {
			state = gr.State
		}
		position += int64(len(qr.IDs))
		if len(qr.IDs) == 0 || uint64(position) >= qr.Total {
			break
		}
	}
	if state == "" {
		return mail.ChangeSet{}, nil, errors.New("baseline pull: no Email/get state")
	}
	return mail.ChangeSet{Added: added}, mail.SyncToken(state), nil
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
