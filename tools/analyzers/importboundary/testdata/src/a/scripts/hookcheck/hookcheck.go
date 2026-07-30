// Package hookcheck stands in for a real poplar package outside
// internal/ and cmd/ (scripts/hookcheck itself): pkgrole.Of reports
// ok=false for it, and the carve-out must still fire here, since a
// package the role system cannot classify is exactly how the
// original version of this rule went silent.
package hookcheck

import (
	_ "a/jmap" // want `a/scripts/hookcheck must not import a/jmap: only internal/backend/jmapsource may import jmap`
)
