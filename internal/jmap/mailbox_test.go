package jmap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testSession(t *testing.T, handler http.HandlerFunc) *Session {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Session{
		AccountID: "acct-123",
		APIURL:    srv.URL,
		token:     "test-token",
	}
}

func TestListFolders(t *testing.T) {
	s := testSession(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"methodResponses": []any{
				[]any{"Mailbox/get", map[string]any{
					"list": []any{
						map[string]any{"id": "mb1", "name": "Inbox", "role": "inbox"},
						map[string]any{"id": "mb2", "name": "Sent", "role": "sent"},
						map[string]any{"id": "mb3", "name": "Notifications", "role": nil},
						map[string]any{"id": "mb4", "name": "Buccaneer 18", "role": nil},
						map[string]any{"id": "mb5", "name": "Trash", "role": "trash"},
						map[string]any{"id": "mb6", "name": "Drafts", "role": "drafts"},
					},
				}, "0"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	folders, err := ListFolders(s)
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d: %v", len(folders), folders)
	}
	if folders[0].Name != "Buccaneer 18" {
		t.Errorf("folders[0] = %q, want %q", folders[0].Name, "Buccaneer 18")
	}
	if folders[1].Name != "Notifications" {
		t.Errorf("folders[1] = %q, want %q", folders[1].Name, "Notifications")
	}
}
