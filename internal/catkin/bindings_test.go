package catkin

import (
	"testing"
)

func TestChordSet_GatedSubset(t *testing.T) {
	chords := ChordSet()
	gated := map[string]bool{
		"^B": true, "^I": true, "^K": true, "^L": true, "^Q": true, "^@": true,
	}
	seen := map[string]bool{}
	for _, c := range chords {
		k := c.Binding.Help().Key
		seen[k] = true
		if gated[k] && !c.RequiresKittyKbd {
			t.Errorf("chord %q expected RequiresKittyKbd=true", k)
		}
	}
	for k := range gated {
		if !seen[k] {
			t.Errorf("chord %q missing from ChordSet", k)
		}
	}
}
