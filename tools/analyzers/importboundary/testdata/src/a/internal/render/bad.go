package render

import (
	_ "a/internal/backend"  // want `internal/render must not import a/internal/backend: internal/render does not reach internal/backend`
	_ "a/internal/keyring"  // want `internal/render must not import a/internal/keyring: internal/render does not reach internal/keyring`
	_ "a/internal/outbox"   // want `internal/render must not import a/internal/outbox: internal/render does not reach internal/outbox`
	_ "a/internal/platform" // want `internal/render must not import a/internal/platform: internal/render does not reach internal/platform`
	_ "a/internal/store"    // want `internal/render must not import a/internal/store: internal/render does not reach internal/store`
	_ "a/internal/sync"     // want `internal/render must not import a/internal/sync: internal/render does not reach internal/sync`
)
