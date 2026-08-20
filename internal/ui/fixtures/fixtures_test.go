package fixtures

import (
	"testing"
	"time"
)

// TestClockPinnedAgainstProcessTZ proves amendment D's clock/TZ pin:
// Clock, read through TZ, renders the same instant regardless of the
// process's own TZ environment variable or time.Local, since TZ is a
// fixed offset rather than a name the machine resolves.
func TestClockPinnedAgainstProcessTZ(t *testing.T) {
	originalLocal := time.Local
	t.Cleanup(func() { time.Local = originalLocal })

	t.Setenv("TZ", "UTC")
	time.Local = time.UTC
	first := Clock.In(TZ).Format(time.RFC3339)

	t.Setenv("TZ", "Asia/Tokyo")
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		time.Local = loc
	}
	second := Clock.In(TZ).Format(time.RFC3339)

	if first != second {
		t.Errorf("Clock.In(TZ) drifted with the process TZ: %q vs %q, want the pinned zone to ignore it", first, second)
	}
}

// TestFixturesHaveDistinctNames proves every fixture's Name is
// unique: the gallery sweep and the render seam's tests both key a
// committed file off Name, so a collision would silently overwrite
// one fixture's render with another's.
func TestFixturesHaveDistinctNames(t *testing.T) {
	all := []Fixture{Mail, Calendar, Contacts, Config, Floor, Short}
	seen := make(map[string]bool)
	for _, fx := range all {
		if seen[fx.Name] {
			t.Errorf("duplicate fixture name %q", fx.Name)
		}
		seen[fx.Name] = true
	}
}
