// SPDX-License-Identifier: MIT

package mailjmap

import (
	"sync"

	"git.sr.ht/~rockorager/go-jmap"
)

// fakeClient is a jmapClient for offline tests. Set respond to
// control what Do returns; inspect sent to assert outgoing requests.
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

// fakeResponse constructs a *jmap.Response whose Responses slice
// contains the given invocations. Tasks 10–14 use this to inject
// canned method responses into fakeClient.
func fakeResponse(invocations ...*jmap.Invocation) *jmap.Response {
	return &jmap.Response{Responses: invocations}
}
