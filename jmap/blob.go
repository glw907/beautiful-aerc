package jmap

// BlobCopy copies binary data between accounts (RFC 8620 section
// 6.3). The blobs keep their octets and get new ids, because a blob id
// is scoped to the account that holds it.
type BlobCopy struct {
	// From is the account the blobs are read from. A server that does
	// not hold it answers the whole call with fromAccountNotFound.
	From ID `json:"fromAccountId,omitempty"`

	// Account is the account the copies land in.
	Account ID `json:"accountId,omitempty"`

	BlobIDs []ID `json:"blobIds,omitempty"`
}

func (*BlobCopy) Name() string { return "Blob/copy" }

func (*BlobCopy) Requires() []URI { return nil }

// BlobCopyResponse answers a BlobCopy. Copied and NotCopied are
// independent maps keyed by the id in the source account, so a call
// that moved one blob and refused two fills both.
type BlobCopyResponse struct {
	From ID `json:"fromAccountId,omitempty"`

	Account ID `json:"accountId,omitempty"`

	// Copied maps each source blob id to the id it took in Account.
	Copied map[ID]ID `json:"copied,omitempty"`

	// NotCopied says, per source blob id, why that one stayed behind.
	NotCopied map[ID]*SetError `json:"notCopied,omitempty"`
}
