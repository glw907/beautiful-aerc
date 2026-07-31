package jmap

import "encoding/json"

// A Mailbox is a named set of messages, presented as a folder or a
// label (RFC 8621 section 2). Every message belongs to at least one.
type Mailbox struct {
	ID ID `json:"id,omitempty"`

	// Name is the mailbox's name within its parent. It is unique among
	// its siblings.
	Name string `json:"name,omitempty"`

	// ParentID is empty at the top level.
	ParentID ID `json:"parentId,omitempty"`

	// Role identifies a mailbox by purpose rather than by its
	// localised name.
	Role Role `json:"role,omitempty"`

	// SortOrder is a hint for where to place the mailbox among its
	// siblings. Equal values sort by name.
	SortOrder uint64 `json:"sortOrder,omitempty"`

	TotalEmails   uint64 `json:"totalEmails,omitempty"`
	UnreadEmails  uint64 `json:"unreadEmails,omitempty"`
	TotalThreads  uint64 `json:"totalThreads,omitempty"`
	UnreadThreads uint64 `json:"unreadThreads,omitempty"`

	// Rights is what the user may do in this mailbox.
	Rights *Rights `json:"myRights,omitempty"`

	// IsSubscribed reports whether the user asked to see the mailbox.
	// Nil on a create leaves the choice to the server, whose default
	// RFC 8621 section 2 leaves open; new(false) asks for a mailbox
	// the user has not subscribed to.
	IsSubscribed *bool `json:"isSubscribed,omitempty"`
}

// Rights is a mailbox's access control list (RFC 8621 section 2).
type Rights struct {
	MayReadItems   bool `json:"mayReadItems,omitempty"`
	MayAddItems    bool `json:"mayAddItems,omitempty"`
	MayRemoveItems bool `json:"mayRemoveItems,omitempty"`
	MaySetSeen     bool `json:"maySetSeen,omitempty"`
	MaySetKeywords bool `json:"maySetKeywords,omitempty"`
	MayCreateChild bool `json:"mayCreateChild,omitempty"`
	MayRename      bool `json:"mayRename,omitempty"`
	MayDelete      bool `json:"mayDelete,omitempty"`
	MaySubmit      bool `json:"maySubmit,omitempty"`
}

// A Role marks a mailbox with a common purpose, independent of the
// name the user sees (RFC 8621 section 2, over the IMAP mailbox name
// attributes of RFC 8457).
type Role string

// The roles RFC 8621 section 2 names.
const (
	RoleAll           Role = "all"
	RoleArchive       Role = "archive"
	RoleDrafts        Role = "drafts"
	RoleFlagged       Role = "flagged"
	RoleHasChildren   Role = "haschildren"
	RoleHasNoChildren Role = "hasnochildren"
	RoleImportant     Role = "important"
	RoleInbox         Role = "inbox"
	RoleJunk          Role = "junk"
	RoleMarked        Role = "marked"
	RoleNoInferiors   Role = "noinferiors"
	RoleNonExistent   Role = "nonexistent"
	RoleNoSelect      Role = "noselect"
	RoleRemote        Role = "remote"
	RoleSent          Role = "sent"
	RoleSubscribed    Role = "subscribed"
	RoleTrash         Role = "trash"
	RoleUnmarked      Role = "unmarked"
)

// MailboxGet fetches mailboxes by id (RFC 8621 section 2.1). Empty
// IDs and ReferenceIDs together ask for every mailbox in the account.
type MailboxGet struct {
	Account ID `json:"accountId,omitempty"`

	IDs []ID `json:"ids,omitempty"`

	// Properties limits the properties the server returns. A property
	// left out comes back as its Go zero value, which is why the
	// optional Booleans are pointers.
	Properties []string `json:"properties,omitempty"`

	// ReferenceIDs takes the ids from an earlier call's result instead
	// of IDs. Setting both is an invalidArguments error.
	ReferenceIDs *ResultReference `json:"#ids,omitempty"`
}

func (*MailboxGet) Name() string { return "Mailbox/get" }

func (*MailboxGet) Requires() []URI { return []URI{MailURI} }

// MailboxGetResponse answers a MailboxGet.
type MailboxGetResponse struct {
	Account ID `json:"accountId,omitempty"`

	// State is the mailbox state this list reflects, to pass to a
	// later MailboxChanges.
	State string `json:"state,omitempty"`

	List []*Mailbox `json:"list,omitempty"`

	// NotFound echoes the requested ids the account does not hold.
	// A server that omits it has said nothing, which is not the same
	// as saying nothing was missing.
	NotFound []ID `json:"notFound,omitempty"`
}

// MailboxChanges lists what moved since a state (RFC 8621 section
// 2.2).
type MailboxChanges struct {
	Account ID `json:"accountId,omitempty"`

	SinceState string `json:"sinceState,omitempty"`

	// MaxChanges caps one page. Zero leaves the size to the server.
	MaxChanges uint64 `json:"maxChanges,omitempty"`
}

func (*MailboxChanges) Name() string { return "Mailbox/changes" }

func (*MailboxChanges) Requires() []URI { return []URI{MailURI} }

// MailboxChangesResponse answers a MailboxChanges.
type MailboxChangesResponse struct {
	Account ID `json:"accountId,omitempty"`

	OldState string `json:"oldState,omitempty"`

	// NewState is where to resume. It advances by one page at a time
	// while HasMoreChanges is true.
	NewState string `json:"newState,omitempty"`

	// HasMoreChanges reports that another page waits at NewState.
	HasMoreChanges bool `json:"hasMoreChanges,omitempty"`

	Created   []ID `json:"created,omitempty"`
	Updated   []ID `json:"updated,omitempty"`
	Destroyed []ID `json:"destroyed,omitempty"`

	// UpdatedProperties, when present, names the only properties that
	// changed on every id in Updated, so a client can refetch less.
	UpdatedProperties []string `json:"updatedProperties,omitempty"`
}

// MailboxQuery lists mailbox ids matching a filter, in a sort order
// (RFC 8621 section 2.3).
type MailboxQuery struct {
	Account ID `json:"accountId,omitempty"`

	// Filter is left out entirely when nil. A server is within its
	// rights to reject an explicit null.
	Filter Filter `json:"filter,omitempty"`

	Sort []*Comparator `json:"sort,omitempty"`

	// Position is the offset of the first id to return. A negative
	// value counts back from the end.
	Position int64 `json:"position,omitempty"`

	// Anchor pages from a known id instead of an offset, so a
	// concurrent change cannot shift the window. An anchor the query
	// no longer matches fails with anchorNotFound.
	Anchor ID `json:"anchor,omitempty"`

	// AnchorOffset moves the window relative to Anchor.
	AnchorOffset int64 `json:"anchorOffset,omitempty"`

	Limit uint64 `json:"limit,omitempty"`

	// CalculateTotal asks for the full match count, which a server may
	// find expensive.
	CalculateTotal bool `json:"calculateTotal,omitempty"`

	// SortAsTree orders children under their parents.
	SortAsTree bool `json:"sortAsTree,omitempty"`

	// FilterAsTree keeps a mailbox only when every ancestor matches
	// too.
	FilterAsTree bool `json:"filterAsTree,omitempty"`
}

func (*MailboxQuery) Name() string { return "Mailbox/query" }

func (*MailboxQuery) Requires() []URI { return []URI{MailURI} }

// MarshalJSON implements json.Marshaler. It drops a Filter holding a
// typed nil rather than sending "filter": null, and carries the
// filter it keeps as bytes so the tree is marshalled once.
func (m MailboxQuery) MarshalJSON() ([]byte, error) {
	filter, err := omitNullFilter(m.Filter)
	if err != nil {
		return nil, err
	}

	type mailboxQuery MailboxQuery
	out := mailboxQuery(m)
	out.Filter = filter
	return json.Marshal(out)
}

// MailboxQueryResponse answers a MailboxQuery.
type MailboxQueryResponse struct {
	Account ID `json:"accountId,omitempty"`

	QueryState string `json:"queryState,omitempty"`

	CanCalculateChanges bool `json:"canCalculateChanges,omitempty"`

	Position uint64 `json:"position,omitempty"`

	IDs []ID `json:"ids,omitempty"`

	Total uint64 `json:"total,omitempty"`

	Limit uint64 `json:"limit,omitempty"`
}

// MailboxSet creates, updates, and destroys mailboxes (RFC 8621
// section 2.5). Each record succeeds or fails on its own.
type MailboxSet struct {
	Account ID `json:"accountId,omitempty"`

	// IfInState refuses the whole call unless the account is still at
	// this state.
	IfInState string `json:"ifInState,omitempty"`

	// Create maps a creation id, which later calls reference with a
	// leading "#", to the mailbox to create.
	Create map[ID]*Mailbox `json:"create,omitempty"`

	Update map[ID]Patch `json:"update,omitempty"`

	Destroy []ID `json:"destroy,omitempty"`

	// OnDestroyRemoveEmails allows destroying a mailbox that still
	// holds messages, removing them from it. Without it the server
	// answers mailboxHasEmail.
	OnDestroyRemoveEmails bool `json:"onDestroyRemoveEmails,omitempty"`
}

func (*MailboxSet) Name() string { return "Mailbox/set" }

func (*MailboxSet) Requires() []URI { return []URI{MailURI} }

// MailboxSetResponse answers a MailboxSet. The six result maps are
// independent: a call that created one mailbox and failed to create
// two more fills both Created and NotCreated.
type MailboxSetResponse struct {
	Account ID `json:"accountId,omitempty"`

	OldState string `json:"oldState,omitempty"`
	NewState string `json:"newState,omitempty"`

	// Created maps each creation id to the server's version of the
	// record, carrying at least the id it assigned.
	Created map[ID]*Mailbox `json:"created,omitempty"`

	Updated map[ID]*Mailbox `json:"updated,omitempty"`

	Destroyed []ID `json:"destroyed,omitempty"`

	NotCreated   map[ID]*SetError `json:"notCreated,omitempty"`
	NotUpdated   map[ID]*SetError `json:"notUpdated,omitempty"`
	NotDestroyed map[ID]*SetError `json:"notDestroyed,omitempty"`
}

// A MailboxFilterCondition matches mailboxes in a MailboxQuery (RFC
// 8621 section 2.3).
type MailboxFilterCondition struct {
	// ParentID matches mailboxes directly under this one. Empty leaves
	// the condition out of the filter, which constrains nothing, so
	// this cannot express the "parentId": null section 2.3 defines for
	// the top level. A caller wanting the top level narrows on what it
	// can express and reads parentId off the records it gets back.
	ParentID ID `json:"parentId,omitempty"`

	// Name matches a mailbox whose name contains the given string.
	// Section 2.3 words it that way where it makes parentId and role
	// match "exactly", so this is a substring condition and a caller
	// needing an exact name confirms it itself.
	Name string `json:"name,omitempty"`

	Role Role `json:"role,omitempty"`

	// HasAnyRole nil places no constraint. new(true) matches only
	// mailboxes that carry a role, new(false) only those that do not.
	HasAnyRole *bool `json:"hasAnyRole,omitempty"`

	// IsSubscribed nil places no constraint.
	IsSubscribed *bool `json:"isSubscribed,omitempty"`
}

func (*MailboxFilterCondition) isFilter() {}
