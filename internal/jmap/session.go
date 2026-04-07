package jmap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const DefaultSessionURL = "https://api.fastmail.com/jmap/session"

// Session holds JMAP connection state.
type Session struct {
	AccountID string
	APIURL    string
	token     string
}

// NewSession authenticates with the JMAP server and discovers the
// account ID and API URL.
func NewSession(token, sessionURL string) (*Session, error) {
	req, err := http.NewRequest("GET", sessionURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating session request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("session request failed: %s", resp.Status)
	}

	var sess struct {
		PrimaryAccounts map[string]string `json:"primaryAccounts"`
		APIURL          string            `json:"apiUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sess); err != nil {
		return nil, fmt.Errorf("decoding session: %w", err)
	}

	accountID := sess.PrimaryAccounts["urn:ietf:params:jmap:mail"]
	if accountID == "" {
		return nil, fmt.Errorf("no mail account found in session")
	}

	return &Session{
		AccountID: accountID,
		APIURL:    sess.APIURL,
		token:     token,
	}, nil
}

// CallWith makes a JMAP API request with the given capabilities and
// method calls, returning the raw response body.
func (s *Session) CallWith(capabilities []string, methodCalls []any) (json.RawMessage, error) {
	body := map[string]any{
		"using":       capabilities,
		"methodCalls": methodCalls,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequest("POST", s.APIURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating API request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API request failed: %s", resp.Status)
	}

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading API response: %w", err)
	}

	return json.RawMessage(respData), nil
}

// Call makes a JMAP API request with standard mail capabilities.
func (s *Session) Call(methodCalls []any) (json.RawMessage, error) {
	return s.CallWith([]string{
		"urn:ietf:params:jmap:core",
		"urn:ietf:params:jmap:mail",
	}, methodCalls)
}

// methodData extracts the data payload from a JMAP method response at
// the given index. JMAP responses are arrays of [methodName, data, callId]
// triples; this returns the data element.
func methodData(resp json.RawMessage, index int) (json.RawMessage, error) {
	var result struct {
		MethodResponses []json.RawMessage `json:"methodResponses"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if index >= len(result.MethodResponses) {
		return nil, fmt.Errorf("expected at least %d method responses, got %d", index+1, len(result.MethodResponses))
	}

	var triple []json.RawMessage
	if err := json.Unmarshal(result.MethodResponses[index], &triple); err != nil {
		return nil, fmt.Errorf("decoding method response: %w", err)
	}
	if len(triple) < 2 {
		return nil, fmt.Errorf("malformed method response")
	}

	return triple[1], nil
}
