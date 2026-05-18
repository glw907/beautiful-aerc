package mailjmap

import (
	"net/http"
	"net/http/httputil"

	"github.com/glw907/poplar/internal/logctx"
)

// loggingTransport dumps every request and response through
// logctx.WireWriter so wire-trace captures the JMAP HTTP exchange.
type loggingTransport struct{ inner http.RoundTripper }

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if dump, err := httputil.DumpRequestOut(req, true); err == nil {
		w := logctx.WireWriter{Component: "jmap"}
		_, _ = w.Write(dump)
	}
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if dump, err := httputil.DumpResponse(resp, true); err == nil {
		w := logctx.WireWriter{Component: "jmap"}
		_, _ = w.Write(dump)
	}
	return resp, nil
}
