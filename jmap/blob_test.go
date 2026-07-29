package jmap

import (
	"encoding/json"
	"testing"
)

// TestBlobCopyResponseDecodes covers JT-32's copy half. RFC 8620
// section 6.3 answers with two independent maps, and the one that
// says a blob did not travel is the one a caller reading only the
// method's error return never sees.
func TestBlobCopyResponseDecodes(t *testing.T) {
	const body = `{
	  "fromAccountId": "A1",
	  "accountId": "A2",
	  "copied": {"B1": "B9"},
	  "notCopied": {
	    "B2": {"type": "blobNotFound", "notFound": ["B2"]},
	    "B3": {"type": "overQuota", "description": "no room"}
	  }
	}`

	resp := &BlobCopyResponse{}
	if err := json.Unmarshal([]byte(body), resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if resp.From != "A1" || resp.Account != "A2" {
		t.Errorf("accounts = %q -> %q, want A1 -> A2", resp.From, resp.Account)
	}
	if got := resp.Copied["B1"]; got != "B9" {
		t.Errorf("Copied[B1] = %q, want B9", got)
	}
	if len(resp.NotCopied) != 2 {
		t.Fatalf("NotCopied holds %d entries, want 2", len(resp.NotCopied))
	}

	missing := resp.NotCopied["B2"]
	if missing.Type != "blobNotFound" {
		t.Errorf("NotCopied[B2].Type = %q, want blobNotFound", missing.Type)
	}
	if len(missing.NotFound) != 1 || missing.NotFound[0] != "B2" {
		t.Errorf("NotCopied[B2].NotFound = %v, want [B2]", missing.NotFound)
	}

	// An error type neither RFC names keeps its whole payload rather
	// than decaying to a bare string.
	quota := resp.NotCopied["B3"]
	if quota.Type != "overQuota" || quota.Description != "no room" {
		t.Errorf("NotCopied[B3] = %+v, want overQuota with its description", quota)
	}
	if len(quota.Raw) == 0 {
		t.Error("NotCopied[B3].Raw is empty; the server's own object was dropped")
	}
}

// TestBlobCopyRequiresNoMailCapability pins the capability list. RFC
// 8620 section 6.3 puts Blob/copy in the core specification, so a
// request carrying it needs nothing beyond the core URI every request
// already sends.
func TestBlobCopyRequiresNoMailCapability(t *testing.T) {
	if required := (&BlobCopy{}).Requires(); len(required) != 0 {
		t.Errorf("Requires() = %v, want none", required)
	}
}
