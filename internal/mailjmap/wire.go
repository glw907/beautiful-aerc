package mailjmap

import (
	"log/slog"
	"net/http"
	"net/http/httputil"

	"github.com/glw907/poplar/internal/logctx"
)

type loggingTransport struct {
	inner http.RoundTripper
	w     logctx.WireWriter
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	enabled := slog.Default().Enabled(req.Context(), slog.LevelDebug)
	if enabled {
		if dump, err := httputil.DumpRequestOut(req, true); err == nil {
			_, _ = t.w.Write(dump)
		}
	}
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if enabled {
		if dump, err := httputil.DumpResponse(resp, true); err == nil {
			_, _ = t.w.Write(dump)
		}
	}
	return resp, nil
}
