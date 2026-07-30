package dav

import (
	_ "a/jmap"             // want `a/internal/backend/dav must not import a/jmap: only internal/backend/jmapsource may import jmap`
	_ "a/jmap/eventsource" // want `a/internal/backend/dav must not import a/jmap/eventsource: only internal/backend/jmapsource may import jmap`
)
