//go:build !race && !(linux && (amd64 || arm64))

package main

import "testing"

// perfDropPageCache is a no-op outside linux/{amd64,arm64}: dropping a
// file's page cache without root needs an architecture-specific
// fadvise64 argument order this build doesn't implement, so QA-1's
// cold run here stays the "never opened by this process before"
// approximation instead of a genuine cache eviction.
func perfDropPageCache(t *testing.T, _ string) {
	t.Helper()
}
