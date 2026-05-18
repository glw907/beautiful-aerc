package uicore

// BackendState tracks whether connectBackendCmd has succeeded for this
// App session. Distinct from mail.ConnState, which reflects the
// backend's own transport health once wired.
type BackendState int

const (
	BackendConnecting BackendState = iota
	BackendConnected
	BackendFailed
)
