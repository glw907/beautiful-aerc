package jmap

import (
	"encoding/json"
	"fmt"
)

var maskedEmailCaps = []string{
	"urn:ietf:params:jmap:core",
	"https://www.fastmail.com/dev/maskedemail",
}

// MaskedEmail represents a Fastmail masked email address.
type MaskedEmail struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	State     string `json:"state"`
	ForDomain string `json:"forDomain"`
}

// GetMaskedEmails fetches all masked email addresses for the account.
func GetMaskedEmails(s *Session) ([]MaskedEmail, error) {
	calls := []any{
		[]any{"MaskedEmail/get", map[string]any{
			"accountId": s.AccountID,
			"ids":       nil,
		}, "0"},
	}

	resp, err := s.CallWith(maskedEmailCaps, calls)
	if err != nil {
		return nil, fmt.Errorf("fetching masked emails: %w", err)
	}

	raw, err := methodData(resp, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing masked email response: %w", err)
	}

	var data struct {
		List []MaskedEmail `json:"list"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decoding masked email list: %w", err)
	}

	return data.List, nil
}

// DeleteMaskedEmail soft-deletes a masked email by setting its state
// to "deleted". Future mail to this address will bounce.
func DeleteMaskedEmail(s *Session, id string) error {
	calls := []any{
		[]any{"MaskedEmail/set", map[string]any{
			"accountId": s.AccountID,
			"update": map[string]any{
				id: map[string]any{
					"state": "deleted",
				},
			},
		}, "0"},
	}

	resp, err := s.CallWith(maskedEmailCaps, calls)
	if err != nil {
		return fmt.Errorf("deleting masked email: %w", err)
	}

	raw, err := methodData(resp, 0)
	if err != nil {
		return fmt.Errorf("parsing delete response: %w", err)
	}

	var data struct {
		Updated    map[string]any `json:"updated"`
		NotUpdated map[string]any `json:"notUpdated"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("decoding set response: %w", err)
	}

	if len(data.NotUpdated) > 0 {
		return fmt.Errorf("failed to delete masked email %s", id)
	}

	return nil
}
