package uicore

import (
	"testing"

	"charm.land/bubbles/v2/key"
)

func TestGatedBinding(t *testing.T) {
	gb := GatedBinding{
		Binding:          key.NewBinding(key.WithKeys("ctrl+i"), key.WithHelp("^i", "italic")),
		RequiresKittyKbd: true,
	}
	if !gb.RequiresKittyKbd {
		t.Fatalf("tag not preserved")
	}
	if got := gb.Binding.Help().Key; got != "^i" {
		t.Fatalf("help key = %q, want %q", got, "^i")
	}
}
