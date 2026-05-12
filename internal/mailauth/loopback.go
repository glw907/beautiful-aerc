package mailauth

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type consentResult struct {
	code, state string
	err         error
}

// runConsentServer starts an HTTP server bound to a free port in portRange
// ({0,0} for kernel-assigned). Returns the running server, the redirect URL
// ("http://127.0.0.1:<port>/callback"), and a channel that receives exactly
// one callback result. Caller must call srv.Shutdown when done.
func runConsentServer(portRange [2]int) (srv *http.Server, redirect string, done <-chan consentResult, err error) {
	ln, err := listen(portRange)
	if err != nil {
		return nil, "", nil, err
	}

	ch := make(chan consentResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			ch <- consentResult{err: errors.New("oauth callback: " + e)}
		} else {
			ch <- consentResult{code: q.Get("code"), state: q.Get("state")}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<p>You can close this window.</p>")
	})

	srv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      5 * time.Second,
	}
	addr := ln.Addr().(*net.TCPAddr)
	redirect = fmt.Sprintf("http://127.0.0.1:%d/callback", addr.Port)

	go srv.Serve(ln) //nolint:errcheck

	return srv, redirect, ch, nil
}

func listen(portRange [2]int) (net.Listener, error) {
	if portRange[0] == 0 {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	for port := portRange[0]; port <= portRange[1]; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return ln, nil
		}
	}
	return nil, fmt.Errorf("oauth consent: no free port in range %d–%d", portRange[0], portRange[1])
}
