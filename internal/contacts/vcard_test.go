package contacts

import (
	"strings"
	"testing"
)

func TestParseVCard_Person(t *testing.T) {
	src := `BEGIN:VCARD
VERSION:3.0
UID:abc-123
REV:20260101T120000Z
FN:Geoff Wright
N:Wright;Geoff;;;
EMAIL;TYPE=PREF:geoff@907.life
EMAIL:work@example.com
TEL;TYPE=CELL:+15555550100
ORG:907 Life
TITLE:Captain
NOTE:Test note
END:VCARD
`
	got, err := ParseVCard(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if got.Skip {
		t.Fatal("person card flagged Skip")
	}
	if got.UID != "abc-123" {
		t.Errorf("UID = %q", got.UID)
	}
	if got.Contact.Kind != KindPerson || got.Contact.Name != "Geoff Wright" {
		t.Errorf("kind/name = %v %q", got.Contact.Kind, got.Contact.Name)
	}
	if len(got.Contact.Emails) != 2 || got.Contact.Emails[0].Address != "geoff@907.life" {
		t.Errorf("emails = %+v (PREF should sort first)", got.Contact.Emails)
	}
	if len(got.Contact.Phones) != 1 || got.Contact.Phones[0].E164 != "+15555550100" {
		t.Errorf("phones = %+v", got.Contact.Phones)
	}
	if got.Rev != "20260101T120000Z" {
		t.Errorf("Rev = %q", got.Rev)
	}
}

func TestParseVCard_Group_Skipped(t *testing.T) {
	src := `BEGIN:VCARD
VERSION:4.0
UID:g1
KIND:group
FN:Team
END:VCARD
`
	got, err := ParseVCard(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Skip {
		t.Fatal("group should be flagged Skip")
	}
}

func TestParseVCard_Org(t *testing.T) {
	src := `BEGIN:VCARD
VERSION:3.0
UID:o1
FN:Acme Co.
ORG:Acme Co.
EMAIL:hello@acme.example
END:VCARD
`
	got, err := ParseVCard(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if got.Contact.Kind != KindOrg {
		t.Errorf("Kind = %v; want KindOrg", got.Contact.Kind)
	}
	if got.Contact.Org != "Acme Co." || got.Contact.Name != "Acme Co." {
		t.Errorf("name/org = %q %q", got.Contact.Name, got.Contact.Org)
	}
}

func TestParseVCard_PrefSemantics_v4(t *testing.T) {
	// vCard 4.0: PREF=1..100, lower wins; vCard 3.0: TYPE=PREF.
	src := `BEGIN:VCARD
VERSION:4.0
UID:v4
FN:Test
EMAIL;PREF=2:second@x
EMAIL;PREF=1:first@x
END:VCARD
`
	got, err := ParseVCard(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if got.Contact.Emails[0].Address != "first@x" {
		t.Errorf("PREF=1 should sort first; got %+v", got.Contact.Emails)
	}
}
