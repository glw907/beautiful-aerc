package compose

import (
	mailcompose "github.com/glw907/poplar/internal/compose"
	"github.com/glw907/poplar/internal/config"
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
