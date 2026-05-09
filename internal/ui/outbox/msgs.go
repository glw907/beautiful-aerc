package outbox

import "github.com/glw907/poplar/internal/cache"

// CancelMsg requests the App cancel the named outbox op.
type CancelMsg struct{ OpID int64 }

// RescheduleMsg requests the App open the schedule picker pre-filled
// with the row's current time. Initial is "2006-01-02 15:04".
type RescheduleMsg struct {
	OpID    int64
	Initial string
}

// EditAsDraftMsg requests the App cancel the op and open compose
// seeded from the linked draft.
type EditAsDraftMsg struct {
	OpID  int64
	Draft *cache.DraftRow
}

// CloseMsg requests the App return to the previous folder.
type CloseMsg struct{}
