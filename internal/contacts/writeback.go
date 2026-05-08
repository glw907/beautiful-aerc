package contacts

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-vcard"
)

// BuildVCard encodes a fresh card from c. Used for new contacts that
// have no stored blob to patch.
func BuildVCard(c Contact, uid string, now time.Time) ([]byte, error) {
	card := vcard.Card{}
	card.SetValue(vcard.FieldVersion, "4.0")
	card.SetValue(vcard.FieldUID, uid)
	applyOwnedFields(card, c, now)
	if c.Kind == KindOrg {
		card.SetValue(vcard.FieldKind, "org")
	}
	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(card); err != nil {
		return nil, fmt.Errorf("encode vcard: %w", err)
	}
	return buf.Bytes(), nil
}

// PatchVCard decodes stored, mutates only the fields poplar models,
// and re-encodes. Unknown fields (BDAY, ADR, X-*, PHOTO, …) survive.
func PatchVCard(stored []byte, c Contact, now time.Time) ([]byte, error) {
	dec := vcard.NewDecoder(bytes.NewReader(stored))
	card, err := dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("decode stored vcard: %w", err)
	}
	applyOwnedFields(card, c, now)
	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(card); err != nil {
		return nil, fmt.Errorf("re-encode vcard: %w", err)
	}
	return buf.Bytes(), nil
}

// applyOwnedFields mutates card in place: rewrites FN, N, ORG, TITLE,
// NOTE, EMAIL, TEL, REV. Other keys are untouched.
func applyOwnedFields(card vcard.Card, c Contact, now time.Time) {
	if c.Name != "" {
		card.SetValue(vcard.FieldFormattedName, c.Name)
	}
	if c.Family != "" || c.Given != "" {
		card.SetName(&vcard.Name{FamilyName: c.Family, GivenName: c.Given})
	} else {
		delete(card, vcard.FieldName)
	}
	setOrDelete(card, vcard.FieldOrganization, c.Org)
	setOrDelete(card, vcard.FieldTitle, c.Title)
	setOrDelete(card, vcard.FieldNote, c.Note)

	mergeRows(card, vcard.FieldEmail, emailValues(c.Emails), emailTypes(c.Emails))
	mergeRows(card, vcard.FieldTelephone, phoneValues(c.Phones), phoneTypes(c.Phones))

	card.SetValue(vcard.FieldRevision, now.UTC().Format(time.RFC3339))
}

func setOrDelete(card vcard.Card, key, val string) {
	if val == "" {
		delete(card, key)
		return
	}
	card.SetValue(key, val)
}

// mergeRows replaces card[key] with one row per value, preserving
// existing TYPE params for retained rows (matched by case-insensitive
// value equality) and assigning newType for added rows. Index 0 gets
// PREF=1; others have PREF cleared.
func mergeRows(card vcard.Card, key string, values, newTypes []string) {
	old := card[key]
	indexOld := func(v string) int {
		for i, f := range old {
			if strings.EqualFold(f.Value, v) {
				return i
			}
		}
		return -1
	}
	out := make([]*vcard.Field, 0, len(values))
	for i, v := range values {
		var f *vcard.Field
		if idx := indexOld(v); idx >= 0 {
			f = old[idx]
			delete(f.Params, vcard.ParamPreferred)
		} else {
			f = &vcard.Field{Value: v, Params: vcard.Params{}}
			if newTypes[i] != "" {
				f.Params.Set(vcard.ParamType, newTypes[i])
			}
		}
		if i == 0 {
			f.Params.Set(vcard.ParamPreferred, "1")
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		delete(card, key)
		return
	}
	card[key] = out
}

func emailValues(es []Email) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Address
	}
	return out
}

func emailTypes(es []Email) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = strings.ToLower(e.Label)
	}
	return out
}

func phoneValues(ps []Phone) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.E164
	}
	return out
}

func phoneTypes(ps []Phone) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = strings.ToLower(p.Label)
	}
	return out
}
