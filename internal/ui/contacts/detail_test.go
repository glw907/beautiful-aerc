// SPDX-License-Identifier: MIT

package contacts

import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/theme"
)

func TestRenderDetailCard(t *testing.T) {
	styles := NewStyles(theme.OneDark)
	const width = 40

	tests := []struct {
		name    string
		contact Contact
		want    []string
		banned  []string
	}{
		{
			name: "person with title org emails phones note",
			contact: Contact{
				Kind:   KindPerson,
				Name:   "Alice Chen",
				Given:  "Alice",
				Family: "Chen",
				Org:    "ACME",
				Title:  "Senior Engineer",
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
			want: []string{
				"Alice Chen",
				"Senior Engineer · ACME",
				"alice@example.com",
				"(work, primary)",
				"a.chen@personal.io",
				"(home)",
				"+1 555-0100",
				"(mobile, primary)",
				"Met at GopherCon 2024.",
			},
		},
		{
			name: "org with email phone note",
			contact: Contact{
				Kind:   KindOrg,
				Name:   "ACME Support",
				Emails: []Email{{Address: "support@acme.com"}},
				Phones: []Phone{{E164: "+15555550199"}},
				Note:   "Vendor for the\nbuild-pipeline contract.",
			},
			want: []string{
				"ACME Support",
				"support@acme.com",
				"(primary)",
				"Vendor for the",
			},
			banned: []string{
				"Senior",
				" · ",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderDetailCard(tc.contact, width, styles)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q in:\n%s", w, got)
				}
			}
			for _, b := range tc.banned {
				if strings.Contains(got, b) {
					t.Errorf("banned %q found in:\n%s", b, got)
				}
			}
		})
	}
}
