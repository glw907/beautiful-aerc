package when

import (
	_ "a/internal/store" // want `internal/when must not import a/internal/store: internal/when does not reach internal/store`
)
