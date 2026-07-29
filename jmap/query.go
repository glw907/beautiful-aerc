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

// omitNullFilter returns nil when f marshals to JSON null, which is
// what a typed nil produces: a nil *EmailFilterCondition held in a
// Filter interface is non-nil to Go and null on the wire. RFC 8620
// never blesses "filter": null, and Stalwart rejected the form before
// v0.16.10, so the property is left out instead.
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
	return f, nil
}

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
