package store

// Writer applies mutations to the store. Only internal/store
// constructs one.
type Writer struct{}

// NewWriter constructs the store's writer.
func NewWriter() *Writer { return &Writer{} }

// WriteMessage persists a message via the writer.
func WriteMessage(w *Writer, id string) error { return nil }

// Intent is the sanctioned mutation request from outside the
// store.
type Intent struct{ Kind string }

// Apply is the intent gateway: the only write entry point callers
// outside internal/store may reach.
func Apply(intent Intent) error { return nil }

// Dispatch queues an intent for the writer.
func Dispatch(intent Intent) error { return nil }
