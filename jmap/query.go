package jmap

import "encoding/json"

// A Filter narrows a /query. It is either a [FilterOperator] or one
// of the per-record condition types, and every condition in one query
// belongs to the record type being queried.
type Filter interface {
	isFilter()
}

// An Operator combines the conditions of a [FilterOperator].
type Operator string

// The operators RFC 8620 section 5.5 defines.
const (
	// OperatorAND matches when every condition matches.
	OperatorAND Operator = "AND"

	// OperatorOR matches when at least one condition matches.
	OperatorOR Operator = "OR"

	// OperatorNOT matches when no condition matches.
	OperatorNOT Operator = "NOT"
)

// A FilterOperator joins filters (RFC 8620 section 5.5). It is itself
// a [Filter], so operators nest.
type FilterOperator struct {
	Operator   Operator `json:"operator"`
	Conditions []Filter `json:"conditions"`
}

func (*FilterOperator) isFilter() {}

// MarshalJSON implements json.Marshaler. It drops a condition holding
// a typed nil for the same reason [EmailQuery] drops a null filter,
// one slot deeper: a null in the conditions array is a form RFC 8620
// never blesses, and a server within its rights to reject it says so
// about the whole query rather than about the slot.
//
// Each condition is marshalled exactly once, into bytes the encoder
// then copies. Marshaling a condition to inspect it and handing the
// value back for the encoder to marshal again costs 2^depth, which a
// nested filter reaches long before it looks large.
//
// The shadow is a plain conversion of the type it stands for, so a
// property added to FilterOperator travels without an edit here.
func (o FilterOperator) MarshalJSON() ([]byte, error) {
	type filterOperator FilterOperator
	out := filterOperator(o)

	// RFC 8620 types conditions as an array, so an operator with none
	// left sends [] rather than null, which is a different type.
	out.Conditions = []Filter{}
	for _, condition := range o.Conditions {
		data, err := omitNullFilter(condition)
		if err != nil {
			return nil, err
		}
		if data == nil {
			continue
		}
		out.Conditions = append(out.Conditions, data)
	}
	return json.Marshal(out)
}

// omitNullFilter marshals f and returns the bytes, or nil when there
// is nothing to send: a nil *EmailFilterCondition held in a Filter
// interface is non-nil to Go and null on the wire. RFC 8620 never
// blesses "filter": null, and Stalwart rejected the form before
// v0.16.10, so the property is left out instead.
//
// Returning a rawFilter rather than f itself keeps the caller from
// marshaling the same tree a second time to send it.
func omitNullFilter(f Filter) (Filter, error) {
	if f == nil {
		return nil, nil
	}
	data, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	if string(data) == "null" {
		return nil, nil
	}
	return rawFilter(data), nil
}

// marshalFiltered runs omitNullFilter on filter and hands the result
// to build, which returns the shadow type; json.Marshal then encodes
// it. It is the MarshalJSON body [EmailQuery], [MailboxQuery],
// [MailboxQueryChanges], and [EmailQueryChanges] share: each drops a
// Filter holding a typed nil rather than sending "filter": null, and
// carries the filter it keeps as bytes so the tree is marshalled
// once.
func marshalFiltered[T any](filter Filter, build func(Filter) T) ([]byte, error) {
	f, err := omitNullFilter(filter)
	if err != nil {
		return nil, err
	}
	return json.Marshal(build(f))
}

// A rawFilter is an already-marshalled filter standing in a Filter
// slot. It lets a shadow type stay a plain conversion of the type it
// stands for, which keeps the property order and drops nothing when a
// property is added, while still marshaling the tree once.
type rawFilter json.RawMessage

func (rawFilter) isFilter() {}

// MarshalJSON implements json.Marshaler.
func (r rawFilter) MarshalJSON() ([]byte, error) { return json.RawMessage(r), nil }

// A Comparator is one term of a /query sort (RFC 8620 section 5.5).
// Terms apply in order, each breaking the ties of the one before it.
type Comparator struct {
	// Property names the record property to compare. A server
	// advertises the ones it sorts on, for mail in
	// [Mail].EmailQuerySortOptions.
	Property string `json:"property"`

	// IsAscending nil sorts ascending, section 5.5's default, and
	// new(false) reverses the term. A plain bool would send false for
	// every caller who never thought about the field, turning every
	// unconsidered sort upside down.
	IsAscending *bool `json:"isAscending,omitempty"`

	// Collation names the RFC 4790 algorithm for ordering strings.
	// Empty leaves the choice to the server.
	Collation CollationAlgo `json:"collation,omitempty"`

	// Keyword is the keyword an Email/query sort on hasKeyword,
	// allInThreadHaveKeyword, or someInThreadHaveKeyword compares (RFC
	// 8621 section 4.4.2). Other records have no use for it.
	Keyword string `json:"keyword,omitempty"`
}
