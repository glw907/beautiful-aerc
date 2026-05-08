package contacts

import (
	"strings"
	"testing"
	"time"
)

func TestBuildVCard_PersonMinimal(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	c := Contact{
		Kind:   KindPerson,
		Given:  "Ada",
		Family: "Lovelace",
		Name:   "Ada Lovelace",
		Emails: []Email{{Address: "ada@example.org", Label: "work"}},
	}
	got, err := BuildVCard(c, "uid-1", now)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, want := range []string{
		"BEGIN:VCARD", "VERSION:4.0", "UID:uid-1",
		"FN:Ada Lovelace", "N:Lovelace;Ada;;;",
		"EMAIL;PREF=1;TYPE=work:ada@example.org",
		"REV:2026-05-07T12:00:00Z", "END:VCARD",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestBuildVCard_OrgKind(t *testing.T) {
	c := Contact{Kind: KindOrg, Name: "ACME", Org: "ACME"}
	got, err := BuildVCard(c, "uid-org", time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "KIND:org") {
		t.Errorf("KindOrg missing KIND:org line:\n%s", got)
	}
}

func TestPatchVCard_PreservesUnknownFields(t *testing.T) {
	stored := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u1\r\n" +
		"FN:Ada Lovelace\r\nN:Lovelace;Ada;;;\r\n" +
		"EMAIL;PREF=1:ada@example.org\r\n" +
		"BDAY:18151210\r\nX-CUSTOM:keep-me\r\n" +
		"END:VCARD\r\n")
	c := Contact{
		Kind:   KindPerson,
		Given:  "Ada",
		Family: "Lovelace",
		Name:   "Ada Lovelace",
		Note:   "added note",
		Emails: []Email{{Address: "ada@example.org", Label: ""}},
	}
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	got, err := PatchVCard(stored, c, now)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, want := range []string{"BDAY:18151210", "X-CUSTOM:keep-me", "NOTE:added note"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q after patch:\n%s", want, s)
		}
	}
	if !strings.Contains(s, "REV:2026-05-07T12:00:00Z") {
		t.Errorf("REV not bumped:\n%s", s)
	}
}

func TestPatchVCard_AddRemoveEmailsKeepLabels(t *testing.T) {
	stored := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u1\r\n" +
		"FN:Ada\r\nN:;Ada;;;\r\n" +
		"EMAIL;PREF=1;TYPE=_$!<Work>!$_:ada@old.example\r\n" +
		"EMAIL;TYPE=home:ada@home.example\r\n" +
		"END:VCARD\r\n")
	c := Contact{
		Kind:  KindPerson,
		Given: "Ada",
		Name:  "Ada",
		Emails: []Email{
			{Address: "ada@home.example", Label: "home"},
			{Address: "ada@new.example", Label: "work"},
		},
	}
	got, err := PatchVCard(stored, c, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if strings.Contains(s, "ada@old.example") {
		t.Errorf("removed email still present:\n%s", s)
	}
	if !strings.Contains(s, "EMAIL;PREF=1;TYPE=home:ada@home.example") {
		t.Errorf("retained row missing or PREF not promoted:\n%s", s)
	}
	if !strings.Contains(s, "EMAIL;TYPE=work:ada@new.example") {
		t.Errorf("added row missing canonical TYPE:\n%s", s)
	}
}
