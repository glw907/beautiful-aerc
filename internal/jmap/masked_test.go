package jmap

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetMaskedEmails(t *testing.T) {
	s := testSession(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"methodResponses": []any{
				[]any{"MaskedEmail/get", map[string]any{
					"accountId": "acct-123",
					"list": []any{
						map[string]any{
							"id":        "me-1",
							"email":     "abc123@fastmail.com",
							"state":     "enabled",
							"forDomain": "example.com",
						},
						map[string]any{
							"id":        "me-2",
							"email":     "def456@fastmail.com",
							"state":     "disabled",
							"forDomain": "shop.com",
						},
					},
				}, "0"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	emails, err := GetMaskedEmails(s)
	if err != nil {
		t.Fatalf("GetMaskedEmails: %v", err)
	}
	if len(emails) != 2 {
		t.Fatalf("expected 2 masked emails, got %d", len(emails))
	}
	if emails[0].ID != "me-1" {
		t.Errorf("emails[0].ID = %q, want %q", emails[0].ID, "me-1")
	}
	if emails[0].Email != "abc123@fastmail.com" {
		t.Errorf("emails[0].Email = %q, want %q", emails[0].Email, "abc123@fastmail.com")
	}
	if emails[1].State != "disabled" {
		t.Errorf("emails[1].State = %q, want %q", emails[1].State, "disabled")
	}
}

func TestGetMaskedEmailsEmpty(t *testing.T) {
	s := testSession(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"methodResponses": []any{
				[]any{"MaskedEmail/get", map[string]any{
					"accountId": "acct-123",
					"list":      []any{},
				}, "0"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	emails, err := GetMaskedEmails(s)
	if err != nil {
		t.Fatalf("GetMaskedEmails: %v", err)
	}
	if len(emails) != 0 {
		t.Fatalf("expected 0 masked emails, got %d", len(emails))
	}
}

func TestDeleteMaskedEmail(t *testing.T) {
	var gotBody map[string]any
	s := testSession(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		resp := map[string]any{
			"methodResponses": []any{
				[]any{"MaskedEmail/set", map[string]any{
					"accountId": "acct-123",
					"updated": map[string]any{
						"me-1": nil,
					},
				}, "0"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	err := DeleteMaskedEmail(s, "me-1")
	if err != nil {
		t.Fatalf("DeleteMaskedEmail: %v", err)
	}

	// Verify the request included the maskedemail capability
	using, ok := gotBody["using"].([]any)
	if !ok {
		t.Fatal("missing using field in request")
	}
	found := false
	for _, cap := range using {
		if cap == "https://www.fastmail.com/dev/maskedemail" {
			found = true
			break
		}
	}
	if !found {
		t.Error("request missing maskedemail capability")
	}
}

func TestDeleteMaskedEmailNotUpdated(t *testing.T) {
	s := testSession(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"methodResponses": []any{
				[]any{"MaskedEmail/set", map[string]any{
					"accountId": "acct-123",
					"notUpdated": map[string]any{
						"me-bad": map[string]any{
							"type":        "notFound",
							"description": "masked email not found",
						},
					},
				}, "0"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	err := DeleteMaskedEmail(s, "me-bad")
	if err == nil {
		t.Fatal("expected error for not-updated response")
	}
}
