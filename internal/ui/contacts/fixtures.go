package contacts

import (
	"sort"
	"strings"
)

// Fixtures returns the canonical Pass 9.1 mockup pool. Stable order;
// tests assert against indices.
func Fixtures() []Contact {
	return []Contact{
		{
			Kind: KindPerson, Name: "Alice Chen", Given: "Alice", Family: "Chen",
			Org: "ACME", Title: "Senior Engineer",
			Emails: []Email{
				{Address: "alice@example.com", Label: "work"},
				{Address: "a.chen@personal.io", Label: "home"},
			},
			Phones: []Phone{
				{E164: "+15555550100", Label: "mobile"},
				{E164: "+15555550199", Label: "work"},
			},
			Note: "Met at GopherCon 2024.\nCares about error messages.",
		},
		{
			Kind:   KindPerson,
			Name:   "Bob Iyer",
			Given:  "Bob",
			Family: "Iyer",
			Emails: []Email{{Address: "bob@iyer.dev"}},
			Phones: []Phone{{E164: "+15555550101", Label: "mobile"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Carol Nguyen",
			Given:  "Carol",
			Family: "Nguyen",
			Org:    "Widgets Inc.",
			Title:  "Product Manager",
			Emails: []Email{
				{Address: "carol@widgets.example", Label: "work"},
				{Address: "carol.nguyen@gmail.example", Label: "home"},
				{Address: "carol@personal.example"},
			},
			Phones: []Phone{{E164: "+15555550102", Label: "mobile"}},
		},
		{
			Kind:   KindPerson,
			Name:   "David Park",
			Given:  "David",
			Family: "Park",
			Emails: []Email{{Address: "dpark@example.org", Label: "work"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Elena Vasquez",
			Given:  "Elena",
			Family: "Vasquez",
			Org:    "Cloudops",
			Title:  "SRE",
			Emails: []Email{
				{Address: "elena@cloudops.example", Label: "work"},
				{Address: "e.vasquez@home.example", Label: "home"},
			},
			Phones: []Phone{
				{E164: "+15555550105", Label: "work"},
				{E164: "+15555550106", Label: "mobile"},
			},
		},
		{
			Kind:   KindPerson,
			Name:   "Frank Osei",
			Given:  "Frank",
			Family: "Osei",
			Emails: []Email{{Address: "frank.osei@example.net"}},
			Note:   "Prefers async communication.",
		},
		{
			Kind:   KindPerson,
			Name:   "Grace Liu",
			Given:  "Grace",
			Family: "Liu",
			Org:    "Acme Research",
			Title:  "Principal Scientist",
			Emails: []Email{{Address: "grace@acme-research.example", Label: "work"}},
			Phones: []Phone{{E164: "+15555550107", Label: "work"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Hiroshi Tanaka",
			Given:  "Hiroshi",
			Family: "Tanaka",
			Emails: []Email{
				{Address: "h.tanaka@example.jp", Label: "work"},
				{Address: "hiroshi.personal@example.jp", Label: "home"},
			},
		},
		{
			Kind:   KindOrg,
			Name:   "IANA Registry",
			Emails: []Email{{Address: "iana@iana.example"}},
			Phones: []Phone{{E164: "+15555550108"}},
			Note:   "Standards body.",
		},
		{
			Kind:   KindPerson,
			Name:   "Jasmine Brooks",
			Given:  "Jasmine",
			Family: "Brooks",
			Emails: []Email{{Address: "jasmine@brooks.example", Label: "home"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Kenji Mori",
			Given:  "Kenji",
			Family: "Mori",
			Org:    "Softworks",
			Title:  "Staff Engineer",
			Emails: []Email{{Address: "kenji@softworks.example", Label: "work"}},
			Phones: []Phone{{E164: "+15555550109", Label: "mobile"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Laura Schmidt",
			Given:  "Laura",
			Family: "Schmidt",
			Emails: []Email{
				{Address: "laura@schmidt.example"},
				{Address: "l.schmidt@work.example", Label: "work"},
			},
		},
		{
			Kind:   KindPerson,
			Name:   "Rafael Méndez",
			Given:  "Rafael",
			Family: "Méndez",
			Emails: []Email{{Address: "rafael.mendez@example.mx", Label: "work"}},
			Phones: []Phone{{E164: "+525555550110", Label: "mobile"}},
			Note:   "Unicode name; tests non-ASCII display and sort.",
		},
		{
			Kind:   KindPerson,
			Name:   "Nadia Petrov",
			Given:  "Nadia",
			Family: "Petrov",
			Emails: []Email{{Address: "nadia@petrov.example"}},
		},
		{
			Kind:   KindOrg,
			Name:   "OpenInfra Foundation",
			Emails: []Email{{Address: "info@openinfra.example"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Priya Sharma",
			Given:  "Priya",
			Family: "Sharma",
			Org:    "DevCloud",
			Title:  "Engineering Manager",
			Emails: []Email{
				{Address: "priya@devcloud.example", Label: "work"},
				{Address: "priya.home@example.in", Label: "home"},
			},
			Phones: []Phone{
				{E164: "+15555550111", Label: "work"},
				{E164: "+15555550112", Label: "mobile"},
				{E164: "+15555550113", Label: "fax"},
			},
		},
		{
			Kind:   KindOrg,
			Name:   "QuickSupport Ltd.",
			Emails: []Email{{Address: "support@quicksupport.example"}},
			Phones: []Phone{{E164: "+15555550114"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Rosa Ferreira",
			Given:  "Rosa",
			Family: "Ferreira",
			Emails: []Email{{Address: "rosa.ferreira@example.br"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Samuel Okafor",
			Given:  "Samuel",
			Family: "Okafor",
			Org:    "BridgeTech",
			Title:  "Director of Operations",
			Emails: []Email{{Address: "s.okafor@bridgetech.example", Label: "work"}},
			Phones: []Phone{{E164: "+15555550115", Label: "work"}},
			Note:   "Prefers calendar invites over email.",
		},
		{
			Kind:   KindPerson,
			Name:   "Tomás García",
			Given:  "Tomás",
			Family: "García",
			Emails: []Email{{Address: "tomas.garcia@example.es", Label: "home"}},
		},
		{
			Kind: KindOrg,
			Name: "Upstream Systems",
			Emails: []Email{
				{Address: "info@upstream.example"},
				{Address: "support@upstream.example"},
			},
			Phones: []Phone{{E164: "+15555550116"}},
			Note:   "Vendor for the build-pipeline contract.",
		},
		{
			Kind:   KindPerson,
			Name:   "Victoria Holst",
			Given:  "Victoria",
			Family: "Holst",
			Emails: []Email{{Address: "v.holst@example.dk", Label: "work"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Wei Zhang",
			Given:  "Wei",
			Family: "Zhang",
			Org:    "MegaCorp",
			Title:  "Distinguished Engineer",
			Emails: []Email{
				{Address: "wei.zhang@megacorp.example", Label: "work"},
				{Address: "weizhang@personal.example", Label: "home"},
			},
			Phones: []Phone{{E164: "+15555550117", Label: "mobile"}},
		},
		{
			Kind:   KindOrg,
			Name:   "Xenon Networks",
			Emails: []Email{{Address: "hello@xenon.example"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Yuki Kobayashi",
			Given:  "Yuki",
			Family: "Kobayashi",
			Emails: []Email{{Address: "yuki@kobayashi.example"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Zara Ahmed",
			Given:  "Zara",
			Family: "Ahmed",
			Emails: []Email{{Address: "zara.ahmed@example.ae", Label: "work"}},
			Note:   "Key account contact.",
		},
		{
			Kind:   KindPerson,
			Name:   "Bartholomew Featherington-Smythe",
			Given:  "Bartholomew",
			Family: "Featherington-Smythe",
			Org:    "Antiquated Institutions of Higher Renown",
			Title:  "Associate Deputy Vice Chancellor for Academic Affairs",
			Emails: []Email{{Address: "b.featherington-smythe@aihr.example", Label: "work"}},
		},
		{
			Kind:   KindOrg,
			Name:   "International Consortium for Advanced Distributed Systems Research",
			Emails: []Email{{Address: "contact@icadsr.example"}},
		},
		{
			Kind:   KindPerson,
			Name:   "Alex Rivera",
			Given:  "Alex",
			Family: "Rivera",
			Emails: []Email{{Address: "alex@rivera.example"}},
			Phones: []Phone{{E164: "+15555550118", Label: "mobile"}},
		},
		{
			Kind:   KindOrg,
			Name:   "ACME Support",
			Emails: []Email{{Address: "support@acme.com"}},
			Phones: []Phone{{E164: "+15555550199"}},
			Note:   "Vendor for the\nbuild-pipeline contract.",
		},
	}
}

// LookupByEmail finds the contact whose email list contains addr (case-insensitive).
func LookupByEmail(all []Contact, addr string) (Contact, bool) {
	lc := strings.ToLower(addr)
	for _, c := range all {
		for _, e := range c.Emails {
			if strings.ToLower(e.Address) == lc {
				return c, true
			}
		}
	}
	return Contact{}, false
}

// FixtureSuggestions returns up to 7 autocomplete suggestions whose
// lowercase Name or Email starts with prefix, sorted by Name then Email.
// Each contact expands to one suggestion per email.
func FixtureSuggestions(prefix string) []Suggestion {
	if len(prefix) < 2 {
		return nil
	}
	lp := strings.ToLower(prefix)

	var out []Suggestion
	for _, c := range Fixtures() {
		for _, e := range c.Emails {
			if !strings.HasPrefix(strings.ToLower(c.Name), lp) &&
				!strings.HasPrefix(strings.ToLower(e.Address), lp) {
				continue
			}
			org := c.Org
			if c.Kind == KindOrg {
				org = ""
			}
			out = append(out, Suggestion{
				Name:  c.Name,
				Email: e.Address,
				Org:   org,
				IsOrg: c.Kind == KindOrg,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Email < out[j].Email
	})

	if len(out) > 7 {
		out = out[:7]
	}
	return out
}
