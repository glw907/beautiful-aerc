package contacts

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"

	"github.com/emersion/go-vcard"
)

// Parsed is one vCard mapped to the projection plus metadata for
// the cache. Cards with KIND:group carry Skip=true; callers drop
// them from the ingest path.
type Parsed struct {
	UID     string
	Rev     string
	Raw     []byte // round-trip bytes, stored as-is
	Contact Contact
	Skip    bool
}

// ParseVCard reads one vCard from r and returns its projection.
func ParseVCard(r io.Reader) (Parsed, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return Parsed{}, fmt.Errorf("read vcard: %w", err)
	}
	dec := vcard.NewDecoder(bytes.NewReader(buf))
	card, err := dec.Decode()
	if err != nil {
		return Parsed{}, fmt.Errorf("decode vcard: %w", err)
	}
	out := Parsed{
		UID: card.Value(vcard.FieldUID),
		Rev: card.Value(vcard.FieldRevision),
		Raw: buf,
	}
	if card.Kind() == vcard.KindGroup {
		out.Skip = true
		return out, nil
	}
	out.Contact = mapContact(card)
	return out, nil
}

func mapContact(card vcard.Card) Contact {
	c := Contact{
		Name:  card.PreferredValue(vcard.FieldFormattedName),
		Org:   card.PreferredValue(vcard.FieldOrganization),
		Title: card.PreferredValue(vcard.FieldTitle),
		Note:  card.PreferredValue(vcard.FieldNote),
	}
	if name := card.Name(); name != nil {
		c.Family = name.FamilyName
		c.Given = name.GivenName
	}
	// Cards with ORG and no N component are treated as organizations.
	if c.Org != "" && c.Family == "" && c.Given == "" {
		c.Kind = KindOrg
	}
	c.Emails = collectEmails(card)
	c.Phones = collectPhones(card)
	return c
}

type sortedField struct {
	value string
	label string
	pref  int
}

// fieldsSorted returns fields sorted by PREF ascending. vCard 3.0
// TYPE=PREF maps to PREF=1; missing PREF sorts last.
func fieldsSorted(card vcard.Card, key string) []sortedField {
	var out []sortedField
	for _, f := range card[key] {
		s := sortedField{value: f.Value, pref: math.MaxInt}
		if p := f.Params.Get(vcard.ParamPreferred); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				s.pref = n
			} else {
				s.pref = 1
			}
		} else if f.Params.HasType("pref") {
			s.pref = 1
		}
		s.label = primaryLabel(f.Params)
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].pref < out[j].pref
	})
	return out
}

func collectEmails(card vcard.Card) []Email {
	rows := fieldsSorted(card, vcard.FieldEmail)
	out := make([]Email, 0, len(rows))
	for _, r := range rows {
		out = append(out, Email{Address: r.value, Label: r.label})
	}
	return out
}

func collectPhones(card vcard.Card) []Phone {
	rows := fieldsSorted(card, vcard.FieldTelephone)
	out := make([]Phone, 0, len(rows))
	for _, r := range rows {
		out = append(out, Phone{E164: r.value, Label: r.label})
	}
	return out
}

func primaryLabel(p vcard.Params) string {
	for _, t := range p.Types() {
		switch t {
		case "home", "work", "cell", "mobile", "fax":
			return t
		}
	}
	return ""
}
