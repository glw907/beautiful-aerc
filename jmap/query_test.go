package jmap

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestComparatorIsAscending covers JT-42. RFC 8620 section 5.5
// defaults isAscending to true, so a plain Go bool sends false for
// every caller who never set it and turns the sort upside down.
// go-jmap had exactly that, and its own test asserted the wrong
// output as correct.
func TestComparatorIsAscending(t *testing.T) {
	cases := []struct {
		name string
		sort *Comparator
		want string
	}{
		{
			name: "unset omits the property and takes the ascending default",
			sort: &Comparator{Property: "name"},
			want: `{"property":"name"}`,
		},
		{
			name: "explicit false sorts descending",
			sort: &Comparator{Property: "receivedAt", IsAscending: new(false)},
			want: `{"property":"receivedAt","isAscending":false}`,
		},
		{
			name: "explicit true is sent as written",
			sort: &Comparator{Property: "subject", IsAscending: new(true)},
			want: `{"property":"subject","isAscending":true}`,
		},
		{
			name: "collation and keyword ride along",
			sort: &Comparator{
				Property:  "hasKeyword",
				Keyword:   "$flagged",
				Collation: ASCIICasemap,
			},
			want: `{"property":"hasKeyword","collation":"i;ascii-casemap","keyword":"$flagged"}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(c.sort)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != c.want {
				t.Errorf("Marshal = %s, want %s", data, c.want)
			}
		})
	}
}

// TestFilterOperatorNesting covers JT-43. go-jmap's entire email
// filter coverage was one assertion that an empty condition marshals
// to "{}".
func TestFilterOperatorNesting(t *testing.T) {
	cases := []struct {
		name   string
		filter Filter
		want   string
	}{
		{
			name:   "an empty condition",
			filter: &EmailFilterCondition{},
			want:   `{}`,
		},
		{
			name: "AND of two conditions",
			filter: &FilterOperator{
				Operator: OperatorAND,
				Conditions: []Filter{
					&EmailFilterCondition{InMailbox: "MA"},
					&EmailFilterCondition{HasKeyword: "$flagged"},
				},
			},
			want: `{"operator":"AND","conditions":[{"inMailbox":"MA"},{"hasKeyword":"$flagged"}]}`,
		},
		{
			name: "NOT of one condition",
			filter: &FilterOperator{
				Operator:   OperatorNOT,
				Conditions: []Filter{&EmailFilterCondition{HasKeyword: "$seen"}},
			},
			want: `{"operator":"NOT","conditions":[{"hasKeyword":"$seen"}]}`,
		},
		{
			name: "an operator with no condition",
			filter: &FilterOperator{
				Operator: OperatorOR,
			},
			want: `{"operator":"OR","conditions":null}`,
		},
		{
			name: "OR nested two deep inside an AND",
			filter: &FilterOperator{
				Operator: OperatorAND,
				Conditions: []Filter{
					&EmailFilterCondition{InMailbox: "MA"},
					&FilterOperator{
						Operator: OperatorOR,
						Conditions: []Filter{
							&EmailFilterCondition{From: "a@example.com"},
							&FilterOperator{
								Operator: OperatorNOT,
								Conditions: []Filter{
									&EmailFilterCondition{HasAttachment: new(false)},
								},
							},
						},
					},
				},
			},
			want: `{"operator":"AND","conditions":[{"inMailbox":"MA"},` +
				`{"operator":"OR","conditions":[{"from":"a@example.com"},` +
				`{"operator":"NOT","conditions":[{"hasAttachment":false}]}]}]}`,
		},
		{
			name: "a mailbox condition tree",
			filter: &FilterOperator{
				Operator: OperatorAND,
				Conditions: []Filter{
					&MailboxFilterCondition{ParentID: "MA"},
					&MailboxFilterCondition{HasAnyRole: new(false), IsSubscribed: new(true)},
				},
			},
			want: `{"operator":"AND","conditions":[{"parentId":"MA"},` +
				`{"hasAnyRole":false,"isSubscribed":true}]}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(c.filter)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != c.want {
				t.Errorf("Marshal = %s, want %s", data, c.want)
			}
		})
	}
}

// TestQueryOmitsANilFilter covers DV-03. Stalwart rejected
// "filter": null until v0.16.10 and RFC 8620 never blesses the form,
// so poplar never produces it.
func TestQueryOmitsANilFilter(t *testing.T) {
	for _, q := range []Method{
		&EmailQuery{Account: "A13824"},
		&MailboxQuery{Account: "A13824"},
	} {
		data, err := json.Marshal(q)
		if err != nil {
			t.Fatalf("Marshal %s: %v", q.Name(), err)
		}
		if want := `{"accountId":"A13824"}`; string(data) != want {
			t.Errorf("%s marshalled to %s, want %s", q.Name(), data, want)
		}
	}
}

// TestQueryOmitsATypedNilFilter is the other half of DV-03. The
// idiom below is ordinary Go, and the interface it produces is
// non-nil to the language and null on the wire.
func TestQueryOmitsATypedNilFilter(t *testing.T) {
	var email *EmailFilterCondition
	var mailbox *MailboxFilterCondition
	var operator *FilterOperator

	cases := []struct {
		name  string
		query Method
	}{
		{"email query, nil condition", &EmailQuery{Account: "A1", Filter: email}},
		{"email query, nil operator", &EmailQuery{Account: "A1", Filter: operator}},
		{"mailbox query, nil condition", &MailboxQuery{Account: "A1", Filter: mailbox}},
		{"mailbox query, nil operator", &MailboxQuery{Account: "A1", Filter: operator}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(c.query)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if want := `{"accountId":"A1"}`; string(data) != want {
				t.Errorf("Marshal = %s, want %s", data, want)
			}
		})
	}

	// A typed nil one slot deeper, inside the conditions array, is
	// the same hazard: the filter itself is set, so the top-level
	// guard never sees it.
	nested, err := json.Marshal(&EmailQuery{
		Account: "A1",
		Filter: &FilterOperator{
			Operator: OperatorAND,
			Conditions: []Filter{
				&EmailFilterCondition{InMailbox: "MA"},
				email,
				operator,
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"accountId":"A1","filter":{"operator":"AND",` +
		`"conditions":[{"inMailbox":"MA"}]}}`
	if string(nested) != want {
		t.Errorf("Marshal = %s, want %s", nested, want)
	}

	// A filter that is actually set still travels, through the same
	// marshaler.
	set, err := json.Marshal(&EmailQuery{
		Account: "A1",
		Filter:  &EmailFilterCondition{InMailbox: "MA"},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got := `{"accountId":"A1","filter":{"inMailbox":"MA"}}`; string(set) != got {
		t.Errorf("Marshal = %s, want %s", set, got)
	}
}

// TestQueryAnchorPaging covers JT-44. The anchor form pages from a
// known id rather than an offset, so a concurrent change cannot shift
// the window, and an anchor the query no longer matches is an error
// with its own type.
func TestQueryAnchorPaging(t *testing.T) {
	query := &EmailQuery{
		Account:      "A13824",
		Anchor:       "M1ba2ffb0d2b7a1a4",
		AnchorOffset: -5,
		Limit:        20,
	}
	data, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"accountId":"A13824","anchor":"M1ba2ffb0d2b7a1a4","anchorOffset":-5,"limit":20}`
	if string(data) != want {
		t.Errorf("Marshal = %s, want %s", data, want)
	}

	raw := []byte(`{"methodResponses":[["error",{"type":"anchorNotFound"},"0"]],"sessionState":"1"}`)
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	methodErr := argsOf[*MethodError](t, &resp, "0")
	if !errors.Is(methodErr, ErrAnchorNotFound) {
		t.Errorf("%v did not match ErrAnchorNotFound", methodErr)
	}
}

// TestQueryCollapseThreads covers JT-45. Sending the argument is
// poplar's whole job here: a server that ignores it has a bug poplar
// surfaces rather than papers over (DV-07).
func TestQueryCollapseThreads(t *testing.T) {
	on, err := json.Marshal(&EmailQuery{Account: "A13824", CollapseThreads: true})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `{"accountId":"A13824","collapseThreads":true}`; string(on) != want {
		t.Errorf("Marshal = %s, want %s", on, want)
	}

	off, err := json.Marshal(&EmailQuery{Account: "A13824", CollapseThreads: false})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `{"accountId":"A13824"}`; string(off) != want {
		t.Errorf("Marshal = %s, want %s: the RFC default is already false", off, want)
	}
}

// TestQueryResponseKeepsServerOrder covers DV-02. The order of an
// optional Boolean sort is unspecified, two servers disagree on it,
// and poplar reorders nothing.
func TestQueryResponseKeepsServerOrder(t *testing.T) {
	raw := []byte(`{"methodResponses":[["Email/query",` +
		`{"accountId":"A13824","queryState":"q1","position":0,"ids":["M3","M1","M2"],"total":3},` +
		`"0"]],"sessionState":"1"}`)

	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	query := argsOf[*EmailQueryResponse](t, &resp, "0")

	want := []ID{"M3", "M1", "M2"}
	if len(query.IDs) != len(want) {
		t.Fatalf("IDs = %v, want %v", query.IDs, want)
	}
	for i, id := range want {
		if query.IDs[i] != id {
			t.Fatalf("IDs = %v, want %v in the server's order", query.IDs, want)
		}
	}
	if query.Total != 3 {
		t.Errorf("Total = %d, want 3", query.Total)
	}
}
