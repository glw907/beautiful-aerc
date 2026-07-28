package search

import (
	_ "a/internal/backend" // want `internal/search must not import a/internal/backend: internal/search does not reach internal/backend`
)
