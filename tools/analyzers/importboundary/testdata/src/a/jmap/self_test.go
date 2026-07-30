package jmap_test

// This mirrors jmap/example_test.go's real idiom: jmap's own external
// test package imports jmap to demonstrate calling it as a consumer
// would. violatesJMAPCarveOut's jmap-itself exemption must let this
// through with no diagnostic.
import (
	_ "a/jmap"
)
