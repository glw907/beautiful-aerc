package ui

import (
	_ "a/internal/backend" // want `internal/ui must not import a/internal/backend: internal/ui does not reach internal/backend`
	_ "a/internal/outbox"  // want `internal/ui must not import a/internal/outbox: internal/ui does not reach internal/outbox`
	_ "a/internal/sync"    // want `internal/ui must not import a/internal/sync: internal/ui does not reach internal/sync`
)
