// Package eventsource is a stub standing in for a subpackage jmap
// might grow (jmap/eventsource, say): the carve-out must cover it the
// same as the top-level jmap package, not only an exact path match.
// It imports jmap itself, the library wiring on itself that
// violatesJMAPCarveOut's jmap-itself exemption must let through with
// no diagnostic.
package eventsource

import (
	_ "a/jmap"
)
