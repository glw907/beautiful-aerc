package jmap

import (
	"encoding/json"
	"testing"
)

// TestMailboxIsSubscribed covers the record half of the optional
// Boolean rule. RFC 8621 section 2 leaves the create-time default to
// the server, so omitting the property and asking for false are
// different requests and a plain bool cannot express the second.
func TestMailboxIsSubscribed(t *testing.T) {
	cases := []struct {
		name string
		in   *bool
		want string
	}{
		{"absent leaves the default to the server", nil, `{"name":"Receipts"}`},
		{"explicit false", new(false), `{"name":"Receipts","isSubscribed":false}`},
		{"explicit true", new(true), `{"name":"Receipts","isSubscribed":true}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(&Mailbox{Name: "Receipts", IsSubscribed: c.in})
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != c.want {
				t.Errorf("Marshal = %s, want %s", data, c.want)
			}
		})
	}
}

// TestMailboxDecode proves every property of a mailbox record reads
// back, so a mistyped tag cannot leave a count or a right silently
// zero.
func TestMailboxDecode(t *testing.T) {
	raw := []byte(`{"id":"MA","name":"Inbox","parentId":"","role":"inbox","sortOrder":10,` +
		`"totalEmails":1424,"unreadEmails":3,"totalThreads":1200,"unreadThreads":2,` +
		`"myRights":{"mayReadItems":true,"mayAddItems":true,"mayRemoveItems":true,` +
		`"maySetSeen":true,"maySetKeywords":true,"mayCreateChild":true,"mayRename":false,` +
		`"mayDelete":false,"maySubmit":true},"isSubscribed":true}`)

	var box Mailbox
	if err := json.Unmarshal(raw, &box); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	counts := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"SortOrder", box.SortOrder, 10},
		{"TotalEmails", box.TotalEmails, 1424},
		{"UnreadEmails", box.UnreadEmails, 3},
		{"TotalThreads", box.TotalThreads, 1200},
		{"UnreadThreads", box.UnreadThreads, 2},
	}
	for _, c := range counts {
		if c.got != c.want {
			t.Errorf("Mailbox.%s = %d, want %d", c.name, c.got, c.want)
		}
	}

	if box.Role != RoleInbox {
		t.Errorf("Role = %q, want %q", box.Role, RoleInbox)
	}
	if box.IsSubscribed == nil || !*box.IsSubscribed {
		t.Errorf("IsSubscribed = %v, want true", box.IsSubscribed)
	}
	if box.Rights == nil {
		t.Fatal("Rights is nil")
	}
	rights := []struct {
		name string
		got  bool
		want bool
	}{
		{"MayReadItems", box.Rights.MayReadItems, true},
		{"MayAddItems", box.Rights.MayAddItems, true},
		{"MayRemoveItems", box.Rights.MayRemoveItems, true},
		{"MaySetSeen", box.Rights.MaySetSeen, true},
		{"MaySetKeywords", box.Rights.MaySetKeywords, true},
		{"MayCreateChild", box.Rights.MayCreateChild, true},
		{"MayRename", box.Rights.MayRename, false},
		{"MayDelete", box.Rights.MayDelete, false},
		{"MaySubmit", box.Rights.MaySubmit, true},
	}
	for _, c := range rights {
		if c.got != c.want {
			t.Errorf("Rights.%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestMailboxSetPartialFailure proves the mailbox /set carries the
// same six independent result maps as the email one, so a rename that
// failed cannot hide behind a destroy that worked.
func TestMailboxSetPartialFailure(t *testing.T) {
	raw := []byte(`{"methodResponses":[["Mailbox/set",{"accountId":"A13824",` +
		`"oldState":"78540","newState":"78541",` +
		`"destroyed":["MB"],` +
		`"notUpdated":{"MA":{"type":"invalidProperties","properties":["name"]}},` +
		`"notDestroyed":{"MC":{"type":"mailboxHasEmail"}}},"0"]],"sessionState":"1"}`)

	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	set := argsOf[*MailboxSetResponse](t, &resp, "0")

	if len(set.Destroyed) != 1 || set.Destroyed[0] != "MB" {
		t.Errorf("Destroyed = %v, want [MB]", set.Destroyed)
	}
	if got := set.NotUpdated["MA"]; got == nil || got.Type != "invalidProperties" {
		t.Errorf("NotUpdated[MA] = %v, want an invalidProperties error", got)
	} else if len(got.Properties) != 1 || got.Properties[0] != "name" {
		t.Errorf("NotUpdated[MA].Properties = %v, want [name]", got.Properties)
	}
	if got := set.NotDestroyed["MC"]; got == nil || got.Type != "mailboxHasEmail" {
		t.Errorf("NotDestroyed[MC] = %v, want a mailboxHasEmail error", got)
	}
}
