// SPDX-License-Identifier: MIT

package mailjmap

import (
	"sync"

	"git.sr.ht/~rockorager/go-jmap"
)

// fakeClient is a jmapClient for offline tests. Set respond to
// control what Do returns. Inspect sent to assert outgoing requests.
type fakeClient struct {
	mu      sync.Mutex
	sent    []*jmap.Request
	respond func(req *jmap.Request) (*jmap.Response, error)
}

func (f *fakeClient) Do(req *jmap.Request) (*jmap.Response, error) {
	f.mu.Lock()
	f.sent = append(f.sent, req)
	f.mu.Unlock()
	if f.respond == nil {
		return &jmap.Response{}, nil
	}
	return f.respond(req)
}

// fakeResponse wraps invocations into a *jmap.Response so fakeClient
// can hand them back as canned method responses.
func fakeResponse(invocations ...*jmap.Invocation) *jmap.Response {
	return &jmap.Response{Responses: invocations}
}
