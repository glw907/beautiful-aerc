package compose

import (
	"github.com/charmbracelet/bubbles/key"
	mailcompose "github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/config"
)

// Schedule is the compose key for the schedule-send picker. ADR-0076 exempts
// text-entry surfaces from the modifier-free rule.
var Schedule = key.NewBinding(
	key.WithKeys("ctrl+l"),
	key.WithHelp("^L", "schedule send"),
)

// IdentitiesFromConfig converts a config identity slice into the compose
// mirror type. Pure value copy; keeps internal/compose free of the config
// dependency.
func IdentitiesFromConfig(in []config.Identity) []mailcompose.Identity {
	out := make([]mailcompose.Identity, len(in))
	for i, ci := range in {
		sigs := make([]mailcompose.Signature, len(ci.Signatures))
		for j, cs := range ci.Signatures {
			sigs[j] = mailcompose.Signature{Name: cs.Name, Text: cs.Text}
		}
		out[i] = mailcompose.Identity{Name: ci.Name, Email: ci.Email, Signatures: sigs}
	}
	return out
}
