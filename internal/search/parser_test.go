package search

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  Query
	}{
		{name: "empty", input: "", want: Query{}},
		{name: "whitespace only", input: "   ", want: Query{}},
		{name: "bare term", input: "hello", want: Query{Terms: []string{"hello"}}},
		{name: "two terms", input: "alpha beta", want: Query{Terms: []string{"alpha", "beta"}}},
		{name: "from operator", input: "from:alice@x.com", want: Query{From: []string{"alice@x.com"}}},
		{name: "subject operator", input: "subject:invoice", want: Query{Subject: []string{"invoice"}}},
		{name: "in operator", input: "in:Inbox", want: Query{In: []string{"Inbox"}}},
		{name: "has:attachment", input: "has:attachment", want: Query{HasAttachment: true}},
		{name: "has:attachments plural", input: "has:attachments", want: Query{HasAttachment: true}},
		{
			name:  "mixed",
			input: `from:alice subject:"q3 review" pelican`,
			want: Query{
				From:    []string{"alice"},
				Subject: []string{"q3 review"},
				Terms:   []string{"pelican"},
			},
		},
		{
			name:  "quoted phrase as bare term",
			input: `"project pelican"`,
			want:  Query{Terms: []string{"project pelican"}},
		},
		{
			name:  "case-insensitive operator key",
			input: "FROM:bob",
			want:  Query{From: []string{"bob"}},
		},
		{
			name:  "unknown operator falls through",
			input: "label:work",
			want:  Query{Terms: []string{"label:work"}},
		},
		{
			name:  "has:foo not attachment falls through",
			input: "has:badges",
			want:  Query{Terms: []string{"has:badges"}},
		},
		{
			name:  "trailing colon falls through",
			input: "from:",
			want:  Query{Terms: []string{"from:"}},
		},
		{
			name:  "multiple from",
			input: "from:alice from:bob",
			want:  Query{From: []string{"alice", "bob"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q):\n  got  %#v\n  want %#v", tc.input, got, tc.want)
			}
		})
	}
}

func TestQueryIsZero(t *testing.T) {
	if !(Query{}).IsZero() {
		t.Error("zero Query should report IsZero")
	}
	if (Query{Terms: []string{"x"}}).IsZero() {
		t.Error("Query with terms should not be zero")
	}
	if (Query{HasAttachment: true}).IsZero() {
		t.Error("Query with HasAttachment should not be zero")
	}
}
