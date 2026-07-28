package calendar

import (
	_ "a/internal/outbox" // want `internal/calendar must not import a/internal/outbox: internal/calendar does not reach internal/outbox`
)
