package jmap

import (
	"encoding/json"
	"testing"
)

// TestStateChangeFansOut covers JT-28. One push covers several
// accounts, and a type name this package does not model still has to
// reach the client, or a server extension goes unnoticed.
func TestStateChangeFansOut(t *testing.T) {
	var change StateChange
	if err := json.Unmarshal(readFixture(t, "rfc8620-7.1.1-statechange.json"), &change); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if change.Type != "StateChange" {
		t.Errorf("Type = %q, want %q", change.Type, "StateChange")
	}
	if len(change.Changed) != 2 {
		t.Fatalf("Changed covers %d accounts, want 2", len(change.Changed))
	}

	first := change.Changed["a3123"]
	if len(first) != 3 {
		t.Errorf("a3123 reports %d types, want 3", len(first))
	}
	if first["Email"] != "d35ecb040aab" {
		t.Errorf("a3123 Email state = %q, want %q", first["Email"], "d35ecb040aab")
	}
	// CalendarEvent belongs to a capability this package does not
	// model, and the state still arrives.
	if first["CalendarEvent"] != "87accfac587a" {
		t.Errorf("a3123 CalendarEvent state = %q, want it kept", first["CalendarEvent"])
	}

	second := change.Changed["a43461d"]
	if second["Mailbox"] != "0af7a512ce70" {
		t.Errorf("a43461d Mailbox state = %q, want %q", second["Mailbox"], "0af7a512ce70")
	}
	if _, ok := second["Email"]; ok {
		t.Error("a43461d reported an Email state the server did not send")
	}
	if first["CalendarEvent"] == second["CalendarEvent"] {
		t.Error("the two accounts share one CalendarEvent state, so the fan-out collapsed")
	}
}
