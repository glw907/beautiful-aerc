// conformance provisions the Stalwart instance the conformance suite
// runs against. `make conformance` runs it twice around a container
// restart, because Stalwart v0.16 boots into a setup mode that serves
// nothing but its own Bootstrap object and leaves that mode only when
// it restarts against the configuration it just wrote.
//
// Everything here talks to the server's own management surface, which
// v0.16 moved onto JMAP under the urn:stalwart:jmap capability as
// "x:Object/method" calls. The v0.11 REST endpoints the test inventory
// cites (POST /api/account) no longer exist.
//
//	conformance -step setup   -url http://localhost:19080
//	conformance -step account -url http://localhost:19080
//	conformance -step env
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

// The credentials the suite authenticates with. They are fixed and
// public: the instance is created and destroyed by one make target and
// holds nothing but the records the tests write. The password is long
// enough to clear Stalwart's own strength check, which rejects
// anything on its common-password list. They are declared here alone,
// and `conformance -step env` is how the make target learns them: a
// second copy in the Makefile would drift the day one of them changed.
const (
	adminUser    = "admin"
	adminSecret  = "conformance"
	accountName  = "user1"
	accountEmail = "user1@conformance.test"
	accountPass  = "poplar-conformance-9f2c"
)

// setupPath is the Stalwart configuration for the conformance
// instance, in the shape v0.16.15's Bootstrap object takes.
const setupPath = "jmap/testdata/conformance/stalwart.json"

func main() {
	step := flag.String("step", "", "setup, account, or env")
	base := flag.String("url", "", "base URL of the Stalwart instance")
	flag.Parse()

	if err := run(*step, *base); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(step, base string) error {
	// The env step describes the account this program creates, so it
	// needs no instance to talk to.
	if step == "env" {
		fmt.Printf("POPLAR_JMAP_USER=%s POPLAR_JMAP_PASSWORD=%s\n", accountEmail, accountPass)
		return nil
	}
	if base == "" {
		return errors.New("no -url")
	}
	switch step {
	case "setup":
		return setup(base)
	case "account":
		return account(base)
	}
	return fmt.Errorf("unknown -step %q, want setup, account, or env", step)
}

// setup hands Stalwart its configuration. The server answers with the
// permanent administrator it provisioned, which nothing here uses: the
// suite authenticates as the recovery admin the container's
// environment pins, so no generated secret has to travel anywhere.
func setup(base string) error {
	if err := waitFor(base + "/healthz/live"); err != nil {
		return err
	}

	config, err := os.ReadFile(setupPath)
	if err != nil {
		return err
	}

	resp, err := jmapCall(base, "x:Bootstrap/set", map[string]any{
		"update": map[string]any{"singleton": json.RawMessage(config)},
	})
	if err != nil {
		return err
	}
	var result struct {
		Updated    map[string]any             `json:"updated"`
		NotUpdated map[string]json.RawMessage `json:"notUpdated"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}
	if refused, ok := result.NotUpdated["singleton"]; ok {
		return fmt.Errorf("stalwart refused the configuration: %s", refused)
	}
	if _, ok := result.Updated["singleton"]; !ok {
		return fmt.Errorf("stalwart confirmed no configuration: %s", resp)
	}
	return nil
}

// account creates the mail account the suite runs as. The fallback
// admin cannot stand in for it: its account id is synthetic, and an
// Email/query against it fails inside the server rather than
// answering.
func account(base string) error {
	if err := waitFor(base + "/.well-known/jmap"); err != nil {
		return err
	}

	if id, err := accountID(base); err != nil {
		return err
	} else if id != "" {
		return nil
	}

	domain, err := domainID(base)
	if err != nil {
		return err
	}

	resp, err := jmapCall(base, "x:Account/set", map[string]any{
		"create": map[string]any{
			"u1": map[string]any{
				"@type":    "User",
				"name":     accountName,
				"domainId": domain,
				// Both properties are ordered lists that travel as an
				// object keyed by decimal position.
				"credentials": map[string]any{
					"0": map[string]any{"@type": "Password", "secret": accountPass},
				},
				"aliases": map[string]any{
					"0": map[string]any{"enabled": true, "name": accountName, "domainId": domain},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	var result struct {
		Created    map[string]json.RawMessage `json:"created"`
		NotCreated map[string]json.RawMessage `json:"notCreated"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}
	if refused, ok := result.NotCreated["u1"]; ok {
		return fmt.Errorf("create %s: %s", accountEmail, refused)
	}
	if _, ok := result.Created["u1"]; !ok {
		return fmt.Errorf("create %s: no record came back: %s", accountEmail, resp)
	}
	return nil
}

// accountID returns the suite's account, or the empty string when the
// instance does not hold it yet. The list is never empty on a
// configured instance: setting the configuration provisions a
// permanent administrator, so the account has to be found by name.
func accountID(base string) (string, error) {
	resp, err := jmapCall(base, "x:Account/get", map[string]any{})
	if err != nil {
		return "", err
	}
	var result struct {
		List []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"list"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}
	for _, account := range result.List {
		if account.Name == accountName {
			return account.ID, nil
		}
	}
	return "", nil
}

func domainID(base string) (string, error) {
	resp, err := jmapCall(base, "x:Domain/query", map[string]any{})
	if err != nil {
		return "", err
	}
	var result struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}
	if len(result.IDs) == 0 {
		return "", errors.New("the instance holds no domain; the configuration's defaultDomain should have made one")
	}
	return result.IDs[0], nil
}

// jmapCall runs one method as the recovery admin and returns its
// arguments object. A method error comes back as an error, because
// every call here is provisioning and none of them is allowed to fail
// quietly.
func jmapCall(base, method string, args map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{
		"using": []string{"urn:ietf:params:jmap:core", "urn:stalwart:jmap"},
		"methodCalls": []any{
			[]any{method, args, "0"},
		},
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/jmap/", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(adminUser, adminSecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", method, resp.Status)
	}

	var decoded struct {
		MethodResponses []json.RawMessage `json:"methodResponses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.MethodResponses) != 1 {
		return nil, fmt.Errorf("%s answered %d times, want once", method, len(decoded.MethodResponses))
	}

	var invocation []json.RawMessage
	if err := json.Unmarshal(decoded.MethodResponses[0], &invocation); err != nil {
		return nil, err
	}
	if len(invocation) != 3 {
		return nil, fmt.Errorf("%s answered a %d-element invocation", method, len(invocation))
	}
	var name string
	if err := json.Unmarshal(invocation[0], &name); err != nil {
		return nil, err
	}
	if name != method {
		return nil, fmt.Errorf("%s answered %s: %s", method, name, invocation[1])
	}
	return invocation[1], nil
}

// waitFor polls url until it answers, which is how long the server
// takes to open its listener. The ceiling is generous because a first
// boot after configuration pulls the spam rules and the geo data
// before it starts serving.
//
// The last attempt's outcome travels into the timeout message. A
// server that answered 401 every time for three minutes and one whose
// port never opened are different faults with different fixes, and
// with the attempts discarded they read identically.
func waitFor(url string) error {
	deadline := time.Now().Add(3 * time.Minute)
	var last string
	for {
		last = attempt(url)
		if last == "" {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not answer within three minutes; the last attempt %s", url, last)
		}
		time.Sleep(time.Second)
	}
}

// attempt makes one request and describes what happened, or returns
// the empty string for the answer waitFor is waiting on.
func attempt(url string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err.Error()
	}
	req.SetBasicAuth(adminUser, adminSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return ""
	}
	return "answered " + resp.Status
}
