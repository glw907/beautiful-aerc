package uicore

// SearchScope picks the breadth of the sidebar search shelf:
// current folder only, or every folder the account knows about.
type SearchScope int

const (
	// ScopeFolder filters the active folder in place.
	ScopeFolder SearchScope = iota
	// ScopeAll searches every folder via the FTS5 index.
	ScopeAll
)
