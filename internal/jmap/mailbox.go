package jmap

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Folder represents a JMAP mailbox.
type Folder struct {
	ID   string
	Name string
}

type mailbox struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Role *string `json:"role"`
}

func getMailboxes(s *Session) ([]mailbox, error) {
	calls := []any{
		[]any{"Mailbox/get", map[string]any{
			"accountId":  s.AccountID,
			"properties": []string{"name", "role"},
		}, "0"},
	}

	resp, err := s.Call(calls)
	if err != nil {
		return nil, fmt.Errorf("fetching mailboxes: %w", err)
	}

	raw, err := methodData(resp, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing mailbox response: %w", err)
	}

	var data struct {
		List []mailbox `json:"list"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decoding mailbox list: %w", err)
	}

	return data.List, nil
}

// ListFolders returns custom (non-role) mailboxes, sorted by name.
func ListFolders(s *Session) ([]Folder, error) {
	mailboxes, err := getMailboxes(s)
	if err != nil {
		return nil, err
	}

	var folders []Folder
	for _, mb := range mailboxes {
		if mb.Role != nil {
			continue
		}
		folders = append(folders, Folder{ID: mb.ID, Name: mb.Name})
	}

	sort.Slice(folders, func(i, j int) bool {
		return folders[i].Name < folders[j].Name
	})

	return folders, nil
}

// FindMailbox returns the ID of the mailbox with the given name.
func FindMailbox(s *Session, name string) (string, error) {
	mailboxes, err := getMailboxes(s)
	if err != nil {
		return "", err
	}

	for _, mb := range mailboxes {
		if mb.Name == name {
			return mb.ID, nil
		}
	}
	return "", fmt.Errorf("mailbox %q not found", name)
}
