package compose

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/glw907/poplar/internal/ui/contacts"
)

func staticSuggest(rows []contacts.Suggestion) SuggestFn {
	return func(prefix string) []contacts.Suggestion {
		if len(prefix) < 2 {
			return nil
		}
		return rows
	}
}

func TestDropdown_EmptyBelowMinPrefix(t *testing.T) {
	d := NewDropdown(staticSuggest([]contacts.Suggestion{
		{Name: "Alice Adams", Email: "alice@example.com"},
	}))
	d = d.SetPrefix("a")
	if !d.Empty() {
		t.Fatalf("dropdown should be empty for prefix < 2 chars")
	}
}

func TestDropdown_PopulatesAtMinPrefix(t *testing.T) {
	rows := []contacts.Suggestion{
		{Name: "Alice Adams", Email: "alice@example.com", Org: "Acme"},
		{Name: "Alex Brown", Email: "alex@example.com"},
	}
	d := NewDropdown(staticSuggest(rows)).SetPrefix("al")
	if d.Empty() {
		t.Fatalf("dropdown should populate at prefix length 2")
	}
	if got := strings.Count(d.View(), "\n"); got != 1 {
		t.Fatalf("expected 2 rendered rows (1 newline), got %d:\n%s", got, d.View())
	}
}

func TestDropdown_UpDownWraps(t *testing.T) {
	rows := []contacts.Suggestion{
		{Name: "A", Email: "a@x"},
		{Name: "B", Email: "b@x"},
		{Name: "C", Email: "c@x"},
	}
	d := NewDropdown(staticSuggest(rows)).SetPrefix("ab")
	if got, _ := d.Selected(); got.Name != "A" {
		t.Fatalf("initial selection = %q, want A", got.Name)
	}
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if got, _ := d.Selected(); got.Name != "C" {
		t.Fatalf("after 2 Down, selection = %q, want C", got.Name)
	}
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if got, _ := d.Selected(); got.Name != "A" {
		t.Fatalf("Down at end should wrap to A, got %q", got.Name)
	}
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if got, _ := d.Selected(); got.Name != "C" {
		t.Fatalf("Up at top should wrap to C, got %q", got.Name)
	}
}

func TestDropdown_RenderIncludesOrgSuffixForPerson(t *testing.T) {
	d := NewDropdown(staticSuggest([]contacts.Suggestion{
		{Name: "Alice Adams", Email: "alice@example.com", Org: "Acme"},
	})).SetPrefix("al")
	v := d.View()
	if !strings.Contains(v, "Alice Adams") || !strings.Contains(v, "alice@example.com") {
		t.Fatalf("render missing name/email:\n%s", v)
	}
	if !strings.Contains(v, "Acme") {
		t.Fatalf("person row should include org suffix:\n%s", v)
	}
}

func TestDropdown_OrgRowOmitsSuffix(t *testing.T) {
	d := NewDropdown(staticSuggest([]contacts.Suggestion{
		{Name: "Acme Inc.", Email: "hello@acme.com", IsOrg: true},
	})).SetPrefix("ac")
	v := d.View()
	if !strings.Contains(v, "Acme Inc.") || !strings.Contains(v, "hello@acme.com") {
		t.Fatalf("org row missing name/email:\n%s", v)
	}
	if strings.Contains(v, "·") {
		t.Fatalf("org row should not include the · suffix:\n%s", v)
	}
}

func TestDropdown_SetPrefixEmptyClears(t *testing.T) {
	rows := []contacts.Suggestion{{Name: "A", Email: "a@x"}}
	d := NewDropdown(staticSuggest(rows)).SetPrefix("ab")
	if d.Empty() {
		t.Fatalf("precondition: dropdown should populate")
	}
	d = d.SetPrefix("")
	if !d.Empty() {
		t.Fatalf("SetPrefix(\"\") should clear rows")
	}
}
