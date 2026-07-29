package jmap

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// MailboxQueryChanges brings a cached MailboxQuery result up to date
// (RFC 8621 section 2.4, over RFC 8620 section 5.6). Filter and Sort
// repeat what the original query used; a server given different ones
// answers about a different query.
type MailboxQueryChanges struct {
	Account ID `json:"accountId,omitempty"`

	// Filter is left out entirely when nil, for the reason
	// [MailboxQuery].Filter is.
	Filter Filter `json:"filter,omitempty"`

	Sort []*Comparator `json:"sort,omitempty"`

	// SinceQueryState is the queryState the cached result carries.
	SinceQueryState string `json:"sinceQueryState,omitempty"`

	// MaxChanges caps one page. A query with more answers
	// tooManyChanges rather than truncating.
	MaxChanges uint64 `json:"maxChanges,omitempty"`

	// UpToID is the highest-index id the client cached, which lets a
	// server skip changes beyond it. A server ignores it unless the
	// filter and sort are both on immutable properties.
	UpToID ID `json:"upToId,omitempty"`

	// CalculateTotal asks for the full match count, which a server may
	// find expensive.
	CalculateTotal bool `json:"calculateTotal,omitempty"`
}

func (*MailboxQueryChanges) Name() string { return "Mailbox/queryChanges" }

func (*MailboxQueryChanges) Requires() []URI { return []URI{MailURI} }

// MarshalJSON implements json.Marshaler.
func (m MailboxQueryChanges) MarshalJSON() ([]byte, error) {
	filter, err := omitNullFilter(m.Filter)
	if err != nil {
		return nil, err
	}

	type mailboxQueryChanges MailboxQueryChanges
	out := mailboxQueryChanges(m)
	out.Filter = filter
	return json.Marshal(out)
}

// MailboxQueryChangesResponse answers a MailboxQueryChanges.
type MailboxQueryChangesResponse struct {
	Account ID `json:"accountId,omitempty"`

	OldQueryState string `json:"oldQueryState,omitempty"`
	NewQueryState string `json:"newQueryState,omitempty"`

	// Total arrives only when the call asked for it.
	Total uint64 `json:"total,omitempty"`

	// Removed lists ids no longer in the results. A server may name
	// ids that were not there either, and may name one that is in
	// Added too when a mutable property moved it.
	Removed []ID `json:"removed,omitempty"`

	// Added lists ids and the index each takes in the new results,
	// lowest index first. [Splice] applies the pair.
	Added []AddedItem `json:"added,omitempty"`
}

// EmailQueryChanges brings a cached EmailQuery result up to date (RFC
// 8621 section 4.5, over RFC 8620 section 5.6).
type EmailQueryChanges struct {
	Account ID `json:"accountId,omitempty"`

	// Filter is left out entirely when nil, for the reason
	// [EmailQuery].Filter is.
	Filter Filter `json:"filter,omitempty"`

	Sort []*Comparator `json:"sort,omitempty"`

	SinceQueryState string `json:"sinceQueryState,omitempty"`

	MaxChanges uint64 `json:"maxChanges,omitempty"`

	UpToID ID `json:"upToId,omitempty"`

	CalculateTotal bool `json:"calculateTotal,omitempty"`

	// CollapseThreads repeats what the original query used. A value
	// that disagrees with it describes a different result list.
	CollapseThreads bool `json:"collapseThreads,omitempty"`
}

func (*EmailQueryChanges) Name() string { return "Email/queryChanges" }

func (*EmailQueryChanges) Requires() []URI { return []URI{MailURI} }

// MarshalJSON implements json.Marshaler.
func (m EmailQueryChanges) MarshalJSON() ([]byte, error) {
	filter, err := omitNullFilter(m.Filter)
	if err != nil {
		return nil, err
	}

	type emailQueryChanges EmailQueryChanges
	out := emailQueryChanges(m)
	out.Filter = filter
	return json.Marshal(out)
}

// EmailQueryChangesResponse answers an EmailQueryChanges.
type EmailQueryChangesResponse struct {
	Account ID `json:"accountId,omitempty"`

	OldQueryState string `json:"oldQueryState,omitempty"`
	NewQueryState string `json:"newQueryState,omitempty"`

	Total uint64 `json:"total,omitempty"`

	Removed []ID `json:"removed,omitempty"`

	Added []AddedItem `json:"added,omitempty"`
}

// An AddedItem places one id at an index in the new query results
// (RFC 8620 section 5.6).
type AddedItem struct {
	ID ID `json:"id"`

	// Index counts rows in the new state, so the item at index 0 is
	// the first result.
	Index uint64 `json:"index"`
}

// Splice applies a /queryChanges result to a cached list of query
// results and returns the new list, leaving cached untouched (RFC 8620
// section 5.6). It splices out every removed id, then splices in each
// added item in turn, each insertion shifting the rows below it down.
//
// A client that cached only part of the results holds a sparse list.
// The empty id stands for a row it never fetched: RFC 8620 section
// 1.2's alphabet has no zero-length id, so a hole cannot be mistaken
// for a record.
//
// Splice does not truncate or extend to the response's total, because
// a cached window is shorter than the results by design and the caller
// is the one that knows which it holds.
func Splice(cached []ID, removed []ID, added []AddedItem) ([]ID, error) {
	// RFC 8620 section 5.6 requires the added array sorted lowest
	// index first, and every insertion shifts the rows below it, so
	// applying an unsorted array lands ids in rows the server never
	// meant and the listing disagrees without saying so.
	if !slices.IsSortedFunc(added, func(a, b AddedItem) int { return cmp.Compare(a.Index, b.Index) }) {
		return nil, errors.New("added items are not in ascending index order")
	}

	out := slices.DeleteFunc(slices.Clone(cached), func(id ID) bool {
		return slices.Contains(removed, id)
	})
	for _, item := range added {
		if item.Index > uint64(len(out)) {
			return nil, fmt.Errorf("added id %q takes index %d, past the end of %d cached results", item.ID, item.Index, len(out))
		}
		out = slices.Insert(out, int(item.Index), item.ID) //nolint:gosec // G115: the bound above rejects an index above len(out), which is itself an int
	}
	return out, nil
}
