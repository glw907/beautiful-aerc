package jmap

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestIDValid covers JT-41. RFC 8620 section 1.2 fixes both bounds
// and the character class, and the "#" of a creation-id reference
// (section 5.3) falls outside that class on purpose.
func TestIDValid(t *testing.T) {
	cases := []struct {
		name string
		id   ID
		want bool
	}{
		{"empty", "", false},
		{"one character", "a", true},
		{"255 characters", ID(strings.Repeat("a", 255)), true},
		{"256 characters", ID(strings.Repeat("a", 256)), false},
		{"whole alphabet", "AZaz09-_", true},
		{"creation id reference", "#k1", false},
		{"dot", "a.b", false},
		{"slash", "a/b", false},
		{"space", "a b", false},
		{"plus", "a+b", false},
		{"base64 pad", "a=", false},
		{"non-ASCII", "café", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.id.Valid(); got != c.want {
				t.Errorf("ID(%q).Valid() = %v, want %v", c.id, got, c.want)
			}
		})
	}
}

// TestIDMarshalAcceptsCreationReference covers JT-41's second half.
// go-jmap validated in MarshalJSON and had to disable the check
// because it rejected the sharp form; poplar does not validate there,
// so a creation-id reference reaches the wire intact.
func TestIDMarshalAcceptsCreationReference(t *testing.T) {
	set := &EmailSubmissionSet{
		Account: "A13824",
		Create: map[ID]*EmailSubmission{
			"k1490": {IdentityID: "I1", EmailID: "#k1"},
		},
	}

	data, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"accountId":"A13824","create":{"k1490":{"identityId":"I1","emailId":"#k1"}}}`
	if string(data) != want {
		t.Errorf("Marshal = %s, want %s", data, want)
	}
}

// TestDateMarshalsUTC covers JT-35 for the Date type itself. RFC 8620
// section 1.4 requires a UTCDate to carry a "Z" offset and to omit a
// zero fractional second.
func TestDateMarshalsUTC(t *testing.T) {
	plus8 := time.FixedZone("plus8", 8*60*60)

	cases := []struct {
		name string
		in   time.Time
		want string
	}{
		{
			// RFC 8620 section 1.4's own worked example.
			name: "positive offset",
			in:   time.Date(2014, time.October, 30, 14, 12, 0, 0, plus8),
			want: `"2014-10-30T06:12:00Z"`,
		},
		{
			name: "already UTC",
			in:   time.Date(2014, time.October, 30, 6, 12, 0, 0, time.UTC),
			want: `"2014-10-30T06:12:00Z"`,
		},
		{
			name: "fractional second kept",
			in:   time.Date(2014, time.October, 30, 6, 12, 0, 500000000, time.UTC),
			want: `"2014-10-30T06:12:00.5Z"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(Date(c.in))
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != c.want {
				t.Errorf("Marshal = %s, want %s", data, c.want)
			}
		})
	}
}

// TestDateFieldsMarshalUTC covers JT-35 across every type in the
// package that carries a date. A field typed time.Time rather than
// Date would send the caller's own offset and pass unnoticed.
func TestDateFieldsMarshalUTC(t *testing.T) {
	plus8 := time.FixedZone("plus8", 8*60*60)
	local := Date(time.Date(2014, time.October, 30, 14, 12, 0, 0, plus8))
	const utc = `"2014-10-30T06:12:00Z"`

	cases := []struct {
		name  string
		value any
		field string
	}{
		{"Email.receivedAt", &Email{ReceivedAt: &local}, "receivedAt"},
		{"Email.sentAt", &Email{SentAt: &local}, "sentAt"},
		{"Email.smimeVerifiedAt", &Email{SMIMEVerifiedAt: &local}, "smimeVerifiedAt"},
		{"EmailImportItem.receivedAt", &EmailImportItem{ReceivedAt: &local}, "receivedAt"},
		{"EmailFilterCondition.before", &EmailFilterCondition{Before: &local}, "before"},
		{"EmailFilterCondition.after", &EmailFilterCondition{After: &local}, "after"},
		{"EmailSubmission.sendAt", &EmailSubmission{SendAt: &local}, "sendAt"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(c.value)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			want := `"` + c.field + `":` + utc
			if !strings.Contains(string(data), want) {
				t.Errorf("Marshal = %s, want it to contain %s", data, want)
			}
		})
	}
}

// TestDateUnmarshal proves a server timestamp survives a round trip
// through Date and comes back as the same instant.
func TestDateUnmarshal(t *testing.T) {
	var d Date
	if err := json.Unmarshal([]byte(`"2014-10-30T14:12:00+08:00"`), &d); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	want := time.Date(2014, time.October, 30, 6, 12, 0, 0, time.UTC)
	if !d.Time().Equal(want) {
		t.Errorf("Time() = %v, want %v", d.Time(), want)
	}

	if err := json.Unmarshal([]byte(`"not a date"`), &d); err == nil {
		t.Error("Unmarshal of a non-date returned no error")
	}
}

// TestOptionalBooleanStates proves a *bool field carries the three
// states the plain bool it replaced could not: absent, explicitly
// false, and explicitly true.
func TestOptionalBooleanStates(t *testing.T) {
	cases := []struct {
		name string
		in   *bool
		want string
	}{
		{"absent", nil, `{}`},
		{"explicit false", new(false), `{"hasAttachment":false}`},
		{"explicit true", new(true), `{"hasAttachment":true}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(&EmailFilterCondition{HasAttachment: c.in})
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != c.want {
				t.Errorf("Marshal = %s, want %s", data, c.want)
			}
		})
	}
}
