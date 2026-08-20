package fixtures

import (
	"testing"
	"time"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
)

// renderMail builds the Mail fixture and renders it through the seam
// at a fixed 100×30 truecolor-dark point.
func renderMail(t *testing.T) string {
	t.Helper()
	th := theme.New(true, theme.ProfileTrueColor)
	lm := ui.ComputeLayout(100, 30, false)
	scr := Mail.Build(th)
	updated, _ := scr.Update(ui.LayoutMsg{Layout: lm})
	return ui.Render(updated.(ui.Screen), lm, th, Mail.Status).Content //nolint:errcheck // a Screen's own Update always returns a Screen; the assertion's panic is the message
}

// TestClockPinnedAgainstProcessTZ proves amendment D's clock/TZ pin
// against the render seam itself, not just the standard library: a
// fixture rendered through the seam is byte-identical whether the
// process's own TZ environment variable and time.Local point at UTC
// or Tokyo, since TZ is a fixed offset rather than a name the machine
// resolves. No pass-2 screen renders a date yet, so this also stands
// as the contract a later date-bearing fixture must keep holding.
func TestClockPinnedAgainstProcessTZ(t *testing.T) {
	originalLocal := time.Local
	t.Cleanup(func() { time.Local = originalLocal })

	t.Setenv("TZ", "UTC")
	time.Local = time.UTC
	first := renderMail(t)

	t.Setenv("TZ", "Asia/Tokyo")
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		time.Local = loc
	}
	second := renderMail(t)

	if first != second {
		t.Errorf("the seam's render drifted with the process TZ, want the pinned Clock/TZ to leave it byte-identical:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestFixturesHaveDistinctNames proves every fixture's Name is
// unique: the gallery sweep and the render seam's tests both key a
// committed file off Name, so a collision would silently overwrite
// one fixture's render with another's.
func TestFixturesHaveDistinctNames(t *testing.T) {
	all := []Fixture{
		Mail, MailLoaded, MailSyncing, MailOffline, MailBackingOff,
		Calendar, Contacts, Config, Floor, Short,
	}
	seen := make(map[string]bool)
	for _, fx := range all {
		if seen[fx.Name] {
			t.Errorf("duplicate fixture name %q", fx.Name)
		}
		seen[fx.Name] = true
	}
}
