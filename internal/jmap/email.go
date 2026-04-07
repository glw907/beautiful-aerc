package jmap

import (
	"encoding/json"
	"fmt"
)

// QueryInbox returns message IDs matching a search filter in the Inbox.
func QueryInbox(s *Session, search string) ([]string, error) {
	calls := []any{
		[]any{"Mailbox/query", map[string]any{
			"accountId": s.AccountID,
			"filter": map[string]any{
				"role": "inbox",
			},
		}, "0"},
		[]any{"Email/query", map[string]any{
			"accountId": s.AccountID,
			"filter": map[string]any{
				"inMailbox": "#0",
				"text":      search,
			},
			"limit": 500,
		}, "1"},
	}

	resp, err := s.Call(calls)
	if err != nil {
		return nil, fmt.Errorf("querying inbox: %w", err)
	}

	mbRaw, err := methodData(resp, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing mailbox query: %w", err)
	}
	var mbData struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(mbRaw, &mbData); err != nil {
		return nil, fmt.Errorf("decoding mailbox ids: %w", err)
	}
	if len(mbData.IDs) == 0 {
		return nil, fmt.Errorf("inbox not found")
	}

	emRaw, err := methodData(resp, 1)
	if err != nil {
		return nil, fmt.Errorf("parsing email query: %w", err)
	}
	var emData struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(emRaw, &emData); err != nil {
		return nil, fmt.Errorf("decoding email ids: %w", err)
	}

	return emData.IDs, nil
}

// MoveEmails updates the mailbox assignment for the given message IDs,
// removing them from srcMailbox and adding them to dstMailbox.
func MoveEmails(s *Session, emailIDs []string, srcMailbox, dstMailbox string) error {
	if len(emailIDs) == 0 {
		return nil
	}

	update := make(map[string]any)
	for _, id := range emailIDs {
		update[id] = map[string]any{
			"mailboxIds/" + srcMailbox: nil,
			"mailboxIds/" + dstMailbox: true,
		}
	}

	calls := []any{
		[]any{"Email/set", map[string]any{
			"accountId": s.AccountID,
			"update":    update,
		}, "0"},
	}

	resp, err := s.Call(calls)
	if err != nil {
		return fmt.Errorf("moving emails: %w", err)
	}

	var result struct {
		MethodResponses []json.RawMessage `json:"methodResponses"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("decoding move response: %w", err)
	}

	return nil
}
