package fixtures

import (
	"testing"
)

// TestFixturesHaveDistinctNames proves every fixture's Name is
// unique: the gallery sweep and the render seam's tests both key a
// committed file off Name, so a collision would silently overwrite
// one fixture's render with another's.
func TestFixturesHaveDistinctNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, fx := range All {
		if seen[fx.Name] {
			t.Errorf("duplicate fixture name %q", fx.Name)
		}
		seen[fx.Name] = true
	}
}
