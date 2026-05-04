# Go stdlib comment exemplars

Verbatim excerpts for style-guide synthesis. Every excerpt includes
file path and line range so the synthesis pass can spot-check.

---

## net/http

### Package doc

```go
// /usr/local/go/src/net/http/doc.go:1–96 (full file)

/*
Package http provides HTTP client and server implementations.

[Get], [Head], [Post], and [PostForm] make HTTP (or HTTPS) requests:

	resp, err := http.Get("http://example.com/")
	...
	resp, err := http.Post("http://example.com/upload", "image/jpeg", &buf)
	...
	resp, err := http.PostForm("http://example.com/form",
		url.Values{"key": {"Value"}, "id": {"123"}})

The caller must close the response body when finished with it:

	resp, err := http.Get("http://example.com/")
	if err != nil {
		// handle error
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	// ...

# Clients and Transports

For control over HTTP client headers, redirect policy, and other
settings, create a [Client]:

	client := &http.Client{
		CheckRedirect: redirectPolicyFunc,
	}

	resp, err := client.Get("http://example.com")
	// ...

	req, err := http.NewRequest("GET", "http://example.com", nil)
	// ...
	req.Header.Add("If-None-Match", `W/"wyzzy"`)
	resp, err := client.Do(req)
	// ...

For control over proxies, TLS configuration, keep-alives,
compression, and other settings, create a [Transport]:

	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get("https://example.com")

Clients and Transports are safe for concurrent use by multiple
goroutines and for efficiency should only be created once and re-used.

# Servers

ListenAndServe starts an HTTP server with a given address and handler.
The handler is usually nil, which means to use [DefaultServeMux].
[Handle] and [HandleFunc] add handlers to [DefaultServeMux]:

	http.Handle("/foo", fooHandler)

	http.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	log.Fatal(http.ListenAndServe(":8080", nil))

More control over the server's behavior is available by creating a
custom Server:

	s := &http.Server{
		Addr:           ":8080",
		Handler:        myHandler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())

# HTTP/2

The http package has transparent support for the HTTP/2 protocol.

[Server] and [DefaultTransport] automatically enable HTTP/2 support
when using HTTPS. [Transport] does not enable HTTP/2 by default.
*/
```

### Exported type docs

```go
// /usr/local/go/src/net/http/server.go:64–91
// A Handler responds to an HTTP request.
//
// [Handler.ServeHTTP] should write reply headers and data to the [ResponseWriter]
// and then return. Returning signals that the request is finished; it
// is not valid to use the [ResponseWriter] or read from the
// [Request.Body] after or concurrently with the completion of the
// ServeHTTP call.
//
// Depending on the HTTP client software, HTTP protocol version, and
// any intermediaries between the client and the Go server, it may not
// be possible to read from the [Request.Body] after writing to the
// [ResponseWriter]. Cautious handlers should read the [Request.Body]
// first, and then reply.
//
// Except for reading the body, handlers should not modify the
// provided Request.
//
// If ServeHTTP panics, the server (the caller of ServeHTTP) assumes
// that the effect of the panic was isolated to the active request.
// It recovers the panic, logs a stack trace to the server error log,
// and either closes the network connection or sends an HTTP/2
// RST_STREAM, depending on the HTTP protocol. To abort a handler so
// the client sees an interrupted response but the server doesn't log
// an error, panic with the value [ErrAbortHandler].
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}
```

```go
// /usr/local/go/src/net/http/request.go:107–113
// A Request represents an HTTP request received by a server
// or to be sent by a client.
//
// The field semantics differ slightly between client and server
// usage. In addition to the notes on the fields below, see the
// documentation for [Request.Write] and [RoundTripper].
type Request struct {
```

```go
// /usr/local/go/src/net/http/server.go:163–178
// The Flusher interface is implemented by ResponseWriters that allow
// an HTTP handler to flush buffered data to the client.
//
// The default HTTP/1.x and HTTP/2 [ResponseWriter] implementations
// support [Flusher], but ResponseWriter wrappers may not. Handlers
// should always test for this ability at runtime.
//
// Note that even for ResponseWriters that support Flush,
// if the client is connected through an HTTP proxy,
// the buffered data may not reach the client until the response
// completes.
type Flusher interface {
```

```go
// /usr/local/go/src/net/http/server.go:179–188
// The Hijacker interface is implemented by ResponseWriters that allow
// an HTTP handler to take over the connection.
//
// The default [ResponseWriter] for HTTP/1.x connections supports
// Hijacker, but HTTP/2 connections intentionally do not.
// ResponseWriter wrappers may also not support Hijacker. Handlers
// should always test for this ability at runtime.
type Hijacker interface {
```

```go
// /usr/local/go/src/net/http/server.go:209–217
// The CloseNotifier interface is implemented by ResponseWriters which
// allow detecting when the underlying connection has gone away.
//
// This mechanism can be used to cancel long operations on the server
// if the client has disconnected before the response is ready.
//
// Deprecated: the CloseNotifier interface predates Go's context package.
// New code should use [Request.Context] instead.
```

### Exported function docs

```go
// /usr/local/go/src/net/http/request.go:341–352
// Context returns the request's context. To change the context, use
// [Request.Clone] or [Request.WithContext].
//
// The returned context is always non-nil; it defaults to the
// background context.
//
// For outgoing client requests, the context controls cancellation.
//
// For incoming server requests, the context is canceled when the
// client's connection closes, the request is canceled (with HTTP/2),
// or when the ServeHTTP method returns.
func (r *Request) Context() context.Context {
```

```go
// /usr/local/go/src/net/http/request.go:359–367
// WithContext returns a shallow copy of r with its context changed
// to ctx. The provided ctx must be non-nil.
//
// For outgoing client request, the context controls the entire
// lifetime of a request and its response: obtaining a connection,
// sending the request, and reading the response headers and body.
//
// To create a new request with a context, use [NewRequestWithContext].
// To make a deep copy of a request with a new context, use [Request.Clone].
func (r *Request) WithContext(ctx context.Context) *Request {
```

```go
// /usr/local/go/src/net/http/request.go:378–383
// Clone returns a deep copy of r with its context changed to ctx.
// The provided ctx must be non-nil.
//
// Clone only makes a shallow copy of the Body field.
//
// For an outgoing client request, the context controls the entire
// lifetime of a request and its response: obtaining a connection,
// sending the request, and reading the response headers and body.
func (r *Request) Clone(ctx context.Context) *Request {
```

```go
// /usr/local/go/src/net/http/request.go:415–416
// ProtoAtLeast reports whether the HTTP protocol used
// in the request is at least major.minor.
func (r *Request) ProtoAtLeast(major, minor int) bool {
```

```go
// /usr/local/go/src/net/http/request.go:422
// UserAgent returns the client's User-Agent, if sent in the request.
func (r *Request) UserAgent() string {
```

```go
// /usr/local/go/src/net/http/request.go:427
// Cookies parses and returns the HTTP cookies sent with the request.
func (r *Request) Cookies() []*Cookie {
```

```go
// /usr/local/go/src/net/http/request.go:458–463
// AddCookie adds a cookie to the request. Per RFC 6265 section 5.4,
// AddCookie does not attach more than one [Cookie] header field. That
// means all cookies, if any, are written into the same line,
// separated by semicolon.
// AddCookie only sanitizes c's name and value, and does not sanitize
// a Cookie header already present in the request.
func (r *Request) AddCookie(c *Cookie) {
```

```go
// /usr/local/go/src/net/http/request.go:473–479
// Referer returns the referring URL, if sent in the request.
//
// Referer is misspelled as in the request itself, a mistake from the
// earliest days of HTTP.  This value can also be fetched from the
// [Header] map as Header["Referer"]; the benefit of making it available
// as a method is that the compiler can diagnose programs that use the
// alternate (correct English) spelling req.Referrer() but cannot
// diagnose programs that use the Header map directly.
func (r *Request) Referer() string {
```

### Exported struct field docs

```go
// /usr/local/go/src/net/http/request.go:114–130
	// Method specifies the HTTP method (GET, POST, PUT, etc.).
	// For client requests, an empty string means GET.
	Method string

	// URL specifies either the URI being requested (for server
	// requests) or the URL to access (for client requests).
	//
	// For server requests, the URL is parsed from the URI
	// supplied on the Request-Line as stored in RequestURI.  For
	// most requests, fields other than Path and RawQuery will be
	// empty. (See RFC 7230, Section 5.3)
	//
	// For client requests, the URL's Host specifies the server to
	// connect to, while the Request's Host field optionally
	// specifies the Host header value to send in the HTTP
	// request.
	URL *url.URL
```

```go
// /usr/local/go/src/net/http/request.go:173–186
	// Body is the request's body.
	//
	// For client requests, a nil body means the request has no
	// body, such as a GET request. The HTTP Client's Transport
	// is responsible for calling the Close method.
	//
	// For server requests, the Request Body is always non-nil
	// but will return EOF immediately when no body is present.
	// The Server will close the request body. The ServeHTTP
	// Handler does not need to.
	//
	// Body must allow Read to be called concurrently with Close.
	// In particular, calling Close should unblock a Read waiting
	// for input.
	Body io.ReadCloser
```

### Unexported comment exemplars

```go
// /usr/local/go/src/net/http/server.go:253–256
// A conn represents the server side of an HTTP connection.
type conn struct {
```

```go
// /usr/local/go/src/net/http/server.go:343–352
// chunkWriter writes to a response's conn buffer, and is the writer
// wrapped by the response.w buffered writer.
//
// chunkWriter also is responsible for finalizing the Header, including
// conditionally setting the Content-Type and setting a Content-Length
// in cases where the handler's final output is smaller than the buffer
// size. It also conditionally adds chunk headers, when in chunking mode.
//
// See the comment above (*response).Write for the entire write flow.
type chunkWriter struct {
```

```go
// /usr/local/go/src/net/http/server.go:643–659
// connReader is the io.Reader wrapper used by *conn. It combines a
// selectively-activated io.LimitedReader (to bound request header
// read sizes) with support for selectively keeping an io.Reader.Read
// call blocked in a background goroutine to wait for activity and
// trigger a CloseNotifier channel.
// After a Handler has hijacked the conn and exited, connReader behaves like a
// proxy for the net.Conn and the aforementioned behavior is bypassed.
type connReader struct {
	rwc net.Conn // rwc is the underlying network connection.

	mu      sync.Mutex // guards following
```

```go
// /usr/local/go/src/net/http/server.go:316
// c.mu must be held.
```

```go
// /usr/local/go/src/net/http/server.go:339–341
// This should be >= 512 bytes for DetectContentType,
// but otherwise it's somewhat arbitrary.
```

### Inline comments inside function bodies

```go
// /usr/local/go/src/net/http/server.go:712–737
// We were past the end of the previous request's body already
// (since we wouldn't be in a background read otherwise), so
// this is a pipelined HTTP request. Prior to Go 1.11 we used to
// send on the CloseNotify channel and cancel the context here,
// but the behavior was documented as only "may", and we only
// did that because that's how CloseNotify accidentally behaved
// in very early Go releases prior to context support. Once we
// added context support, people used a Handler's
// Request.Context() and passed it along. Having that context
// cancel on pipelined HTTP requests caused problems.
// Fortunately, almost nothing uses HTTP/1.x pipelining.
// Unfortunately, apt-get does, or sometimes does.
// New Go 1.11 behavior: don't fire CloseNotify or cancel
// contexts on pipelined requests. Shouldn't affect people, but
// fixes cases like Issue 23921. This does mean that a client
// closing their TCP connection after sending a pipelined
// request won't cancel the context, but we'll catch that on any
// write failure (in checkConnErrorWriter.Write).
// If the server never writes, yes, there are still contrived
// server & client behaviors where this fails to ever cancel the
// context, but that's kinda why HTTP/1.x pipelining died
// anyway.
```

```go
// /usr/local/go/src/net/http/server.go:739–741
// Ignore this error. It's the expected error from
// another goroutine calling abortPendingRead.
```

```go
// /usr/local/go/src/net/http/server.go:2182–2191
// TODO(bradfitz): let ServeHTTP handlers handle
// requests with non-standard expectation[s]? Seems
// theoretical at best, and doesn't fit into the
// current ServeHTTP model anyway. We'd need to
// make the ResponseWriter an optional
// "ExpectReplier" interface or something.
//
// For now we'll just obey RFC 7231 5.1.1 which says
// "A server that receives an Expect field-value other
// than 100-continue MAY respond with a 417 (Expectation
// Failed) status code to indicate that the unexpected
// expectation cannot be met."
```

```go
// /usr/local/go/src/net/http/server.go:863–866
// Note: if this reader size is ever changed, update
// TestHandlerBodyClose's assumptions.
```

```go
// /usr/local/go/src/net/http/server.go:2207–2209
// Release the bufioWriter that writes to the chunk writer, it is not
// used after a connection has been hijacked.
```

### Error message strings

```go
// /usr/local/go/src/net/http/request.go
var ErrMissingFile = errors.New("http: no such file")
var ErrNoCookie = errors.New("http: named cookie not present")
errors.New("http: multipart handled by ParseMultipartForm")
errors.New("missing form body")
errors.New("http: POST too large")
errors.New("net/http: nil Context")
fmt.Errorf("net/http: invalid method %q", method)
var errMissingHost = errors.New("http: Request.Write on Request with no Host or URL set")

// /usr/local/go/src/net/http/server.go
ErrBodyNotAllowed = errors.New("http: request method or response status code does not allow body")
ErrHijacked       = errors.New("http: connection has been hijacked")
ErrContentLength  = errors.New("http: wrote more than the declared Content-Length")
var errTooLarge    = errors.New("http: request too large")
var ErrAbortHandler = errors.New("net/http: abort Handler")
var ErrServerClosed = errors.New("http: Server closed")
var ErrHandlerTimeout = errors.New("http: Handler timeout")
fmt.Errorf("parsing %q: %w", patstr, err)
var errRequestCanceledConn = errors.New("net/http: request canceled while waiting for connection") // TODO: unify?
```

### Hedge/TODO comments

TODO comments are frequent in net/http (30+ in non-bundle files alone).
Shape varies:

```go
// /usr/local/go/src/net/http/request.go:659
// TODO: validate r.Method too? At least it's less likely to

// /usr/local/go/src/net/http/server.go:1469
// TODO: return an error if WriteHeader gets a return parameter

// /usr/local/go/src/net/http/transport.go:2782
var errRequestCanceledConn = errors.New("net/http: request canceled while waiting for connection") // TODO: unify?
```

Named-author form is common: `// TODO(bradfitz): ...`, `// TODO(bcmills): ...`
Most TODOs in production code are real open questions, not cleanup stubs.

---

## encoding/json

### Package doc

```go
// /usr/local/go/src/encoding/json/encode.go:7–43
// Package json implements encoding and decoding of JSON as defined in RFC 7159.
// The mapping between JSON and Go values is described in the documentation for
// the Marshal and Unmarshal functions.
//
// See "JSON and Go" for an introduction to this package:
// https://golang.org/doc/articles/json_and_go.html
//
// # Security Considerations
//
// The JSON standard (RFC 7159) is lax in its definition of a number of parser
// behaviors. As such, many JSON parsers behave differently in various
// scenarios. These differences in parsers mean that systems that use multiple
// independent JSON parser implementations may parse the same JSON object in
// differing ways.
//
// Systems that rely on a JSON object being parsed consistently for security
// purposes should be careful to understand the behaviors of this parser, as
// well as how these behaviors may cause interoperability issues with other
// parser implementations.
//
// Due to the Go Backwards Compatibility promise (https://go.dev/doc/go1compat)
// there are a number of behaviors this package exhibits that may cause
// interopability issues, but cannot be changed. In particular the following
// parsing behaviors may cause issues:
//
//   - If a JSON object contains duplicate keys, keys are processed in the order
//     they are observed, meaning later values will replace or be merged into
//     prior values, depending on the field type ...
//   - When parsing a JSON object into a Go struct, keys are considered in a
//     case-insensitive fashion.
//   - When parsing a JSON object into a Go struct, unknown keys in the JSON
//     object are ignored ...
//   - Invalid UTF-8 bytes in JSON strings are replaced by the Unicode
//     replacement character.
//   - Large JSON number integers will lose precision when unmarshaled into
//     floating-point types.
```

### Exported type docs

```go
// /usr/local/go/src/encoding/json/decode.go:116–124
// Unmarshaler is the interface implemented by types
// that can unmarshal a JSON description of themselves.
// The input can be assumed to be a valid encoding of
// a JSON value. UnmarshalJSON must copy the JSON data
// if it wishes to retain the data after returning.
type Unmarshaler interface {
```

```go
// /usr/local/go/src/encoding/json/decode.go:125–127
// An UnmarshalTypeError describes a JSON value that was
// not appropriate for a value of a specific Go type.
type UnmarshalTypeError struct {
```

### Exported function docs

```go
// /usr/local/go/src/encoding/json/encode.go:64–230 (abbreviated)
// Marshal returns the JSON encoding of v.
//
// Marshal traverses the value v recursively.
// If an encountered value implements [Marshaler]
// and is not a nil pointer, Marshal calls [Marshaler.MarshalJSON]
// to produce JSON. If no [Marshaler.MarshalJSON] method is present but the
// value implements [encoding.TextMarshaler] instead, Marshal calls
// [encoding.TextMarshaler.MarshalText] and encodes the result as a JSON string.
// The nil pointer exception is not strictly necessary
// but mimics a similar, necessary exception in the behavior of
// [Unmarshaler.UnmarshalJSON].
//
// Otherwise, Marshal uses the following type-dependent default encodings:
//
// Boolean values encode as JSON booleans.
//
// Floating point, integer, and [Number] values encode as JSON numbers.
// NaN and +/-Inf values will return an [UnsupportedValueError].
//
// String values encode as JSON strings coerced to valid UTF-8,
// replacing invalid bytes with the Unicode replacement rune.
// So that the JSON will be safe to embed inside HTML <script> tags,
// the string is encoded using [HTMLEscape],
// which replaces "<", ">", "&", U+2028, and U+2029 are escaped
// to "<",">", "&", " ", and " ".
// This replacement can be disabled when using an [Encoder],
// by calling [Encoder.SetEscapeHTML](false).
// ...
// Channel, complex, and function values cannot be encoded in JSON.
// Attempting to encode such a value causes Marshal to return
// an [UnsupportedTypeError].
//
// JSON cannot represent cyclic data structures and Marshal does not
// handle them. Passing cyclic structures to Marshal will result in
// an error.
func Marshal(v any) ([]byte, error) {
```

```go
// /usr/local/go/src/encoding/json/encode.go:232–235
// MarshalIndent is like [Marshal] but applies [Indent] to format the output.
// Each JSON element in the output will begin on a new line beginning with prefix
// followed by one or more copies of indent according to the indentation nesting.
func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
```

```go
// /usr/local/go/src/encoding/json/decode.go:25–101
// Unmarshal parses the JSON-encoded data and stores the result
// in the value pointed to by v. If v is nil or not a pointer,
// Unmarshal returns an [InvalidUnmarshalError].
//
// Unmarshal uses the inverse of the encodings that
// [Marshal] uses, allocating maps, slices, and pointers as necessary,
// with the following additional rules:
// ...
// The JSON null value unmarshals into an interface, map, pointer, or slice
// by setting that Go value to nil. Because null is often used in JSON to mean
// "not present," unmarshaling a JSON null into any other Go type has no effect
// on the value and produces no error.
//
// When unmarshaling quoted strings, invalid UTF-8 or
// invalid UTF-16 surrogate pairs are not treated as an error.
// Instead, they are replaced by the Unicode replacement
// character U+FFFD.
func Unmarshal(data []byte, v any) error {
```

### Unexported comment exemplars

```go
// /usr/local/go/src/encoding/json/encode.go:293–298
// jsonError is an error wrapper type for internal use only.
// Panics with errors are wrapped in jsonError so that the top-level recover
// can distinguish intentional panics from this package.
type jsonError struct{ error }
```

```go
// /usr/local/go/src/encoding/json/encode.go:346–350
// error aborts the encoding by panicking with err wrapped in jsonError.
func (e *encodeState) error(err error) {
	panic(jsonError{err})
}
```

```go
// /usr/local/go/src/encoding/json/encode.go:371–372
type encOpts struct {
	// quoted causes primitive fields to be encoded inside JSON strings.
	quoted bool
	// escapeHTML causes '<', '>', and '&' to be escaped in JSON strings.
	escapeHTML bool
}
```

### Inline comments inside function bodies

```go
// /usr/local/go/src/encoding/json/encode.go:301–307
	// Keep track of what pointers we've seen in the current recursive call
	// path, to avoid cycles that could lead to a stack overflow. Only do
	// the relatively expensive map operations if ptrLevel is larger than
	// startDetectingCyclesAfter, so that we skip the work if we're within a
	// reasonable amount of nested pointers deep.
```

```go
// /usr/local/go/src/encoding/json/encode.go:393–398
	// To deal with recursive types, populate the map with an
	// indirect func before we build it. If the type is recursive,
	// the second lookup for the type will return the indirect func.
	//
	// This indirect func is only used for recursive types,
	// and briefly during racing calls to typeEncoder.
```

```go
// /usr/local/go/src/encoding/json/encode.go:422–425
	// If we have a non-pointer value whose type implements
	// Marshaler with a value receiver, then we're better off taking
	// the address of the value - otherwise we end up with an
	// allocation as we cast the value to an interface.
```

```go
// /usr/local/go/src/encoding/json/encode.go:577–581
	// Convert as if by ES6 number to string conversion.
	// This matches most other JSON generators.
	// See golang.org/issue/6384 and golang.org/issue/14135.
	// Like fmt %g, but the exponent cutoffs are different
	// and exponents themselves are not padded to two digits.
```

```go
// /usr/local/go/src/encoding/json/encode.go:637–638
	// This function implements the JSON numbers grammar.
	// See https://tools.ietf.org/html/rfc7159#section-6
```

```go
// /usr/local/go/src/encoding/json/encode.go:1264–1269
	// Delete all fields that are hidden by the Go rules for embedded fields,
	// except that fields with JSON tags are promoted.
	//
	// The fields are sorted in primary order of name, secondary order
	// of field index length. Loop over names; for each name, delete
	// hidden fields by choosing the one dominant field that survives.
```

### Error message strings

```go
// /usr/local/go/src/encoding/json/decode.go
fmt.Errorf("json: cannot set embedded pointer to unexported struct: %v", subv.Type().Elem())
fmt.Errorf("json: unknown field %q", key)
fmt.Errorf("json: invalid use of ,string struct tag, trying to unmarshal unquoted value into %v", subv.Type())
fmt.Errorf("json: invalid use of ,string struct tag, trying to unmarshal %q into %v", item, v.Type())
fmt.Errorf("json: invalid number literal, trying to unmarshal %q into Number", item)

// /usr/local/go/src/encoding/json/encode.go
fmt.Errorf("json: invalid number literal %q", numStr)
fmt.Errorf("json: encoding error for type %q: %q", v.Type().String(), err.Error())
```

### Hedge/TODO comments

TODOs are rare in non-test encode/decode files. No BUG or HACK markers found.

---

## io

### Package doc

```go
// /usr/local/go/src/io/io.go:5–12
// Package io provides basic interfaces to I/O primitives.
// Its primary job is to wrap existing implementations of such primitives,
// such as those in package os, into shared public interfaces that
// abstract the functionality, plus some other related primitives.
//
// Because these interfaces and primitives wrap lower-level operations with
// various implementations, unless otherwise informed clients should not
// assume they are safe for parallel execution.
package io
```

### Exported type docs

```go
// /usr/local/go/src/io/io.go:55–86
// Reader is the interface that wraps the basic Read method.
//
// Read reads up to len(p) bytes into p. It returns the number of bytes
// read (0 <= n <= len(p)) and any error encountered. Even if Read
// returns n < len(p), it may use all of p as scratch space during the call.
// If some data is available but not len(p) bytes, Read conventionally
// returns what is available instead of waiting for more.
//
// When Read encounters an error or end-of-file condition after
// successfully reading n > 0 bytes, it returns the number of
// bytes read. It may return the (non-nil) error from the same call
// or return the error (and n == 0) from a subsequent call.
// An instance of this general case is that a Reader returning
// a non-zero number of bytes at the end of the input stream may
// return either err == EOF or err == nil. The next Read should
// return 0, EOF.
//
// Callers should always process the n > 0 bytes returned before
// considering the error err. Doing so correctly handles I/O errors
// that happen after reading some bytes and also both of the
// allowed EOF behaviors.
//
// If len(p) == 0, Read should always return n == 0. It may return a
// non-nil error if some error condition is known, such as EOF.
//
// Implementations of Read are discouraged from returning a
// zero byte count with a nil error, except when len(p) == 0.
// Callers should treat a return of 0 and nil as indicating that
// nothing happened; in particular it does not indicate EOF.
//
// Implementations must not retain p.
type Reader interface {
	Read(p []byte) (n int, err error)
}
```

```go
// /usr/local/go/src/io/io.go:90–97
// Writer is the interface that wraps the basic Write method.
//
// Write writes len(p) bytes from p to the underlying data stream.
// It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// Write must return a non-nil error if it returns n < len(p).
// Write must not modify the slice data, even temporarily.
//
// Implementations must not retain p.
type Writer interface {
	Write(p []byte) (n int, err error)
}
```

```go
// /usr/local/go/src/io/io.go:100–103
// Closer is the interface that wraps the basic Close method.
//
// The behavior of Close after the first call is undefined.
// Specific implementations may document their own behavior.
type Closer interface {
	Close() error
}
```

```go
// /usr/local/go/src/io/io.go:353–367
// ReaderAt is the interface that wraps the basic ReadAt method.
//
// ReadAt reads len(p) bytes into p starting at offset off in the
// underlying input source. It returns the number of bytes
// read (0 <= n <= len(p)) and any error encountered.
//
// When ReadAt returns n < len(p), it returns a non-nil error
// explaining why more bytes were not returned. In this respect,
// ReadAt is stricter than Read.
//
// Even if ReadAt returns n < len(p), it may use all of p as scratch
// space during the call. If some data is available but not len(p) bytes,
// ReadAt blocks until either all the data is available or an error occurs.
// In this respect ReadAt is different from Read.
// ...
// Clients of ReadAt can execute parallel ReadAt calls on the
// same input source.
//
// Implementations must not retain p.
type ReaderAt interface {
```

```go
// /usr/local/go/src/io/io.go:445–451
// A LimitedReader reads from R but limits the amount of
// data returned to just N bytes. Each call to Read
// updates N to reflect the new amount remaining.
// Read returns EOF when N <= 0 or when the underlying R returns EOF.
type LimitedReader struct {
	R Reader // underlying reader
	N int64  // max bytes remaining
}
```

```go
// /usr/local/go/src/io/io.go:465–467
// SectionReader implements Read, Seek, and ReadAt on a section
// of an underlying [ReaderAt].
type SectionReader struct {
```

### Exported function docs

```go
// /usr/local/go/src/io/io.go:314–325
// ReadAtLeast reads from r into buf until it has read at least min bytes.
// It returns the number of bytes copied and an error if fewer bytes were read.
// The error is EOF only if no bytes were read.
// If an EOF happens after reading fewer than min bytes,
// ReadAtLeast returns [ErrUnexpectedEOF].
// If min is greater than the length of buf, ReadAtLeast returns [ErrShortBuffer].
// On return, n >= min if and only if err == nil.
// If r returns an error having read at least min bytes, the error is dropped.
func ReadAtLeast(r Reader, buf []byte, min int) (n int, err error) {
```

```go
// /usr/local/go/src/io/io.go:353–360
// ReadFull reads exactly len(buf) bytes from r into buf.
// It returns the number of bytes copied and an error if fewer bytes were read.
// The error is EOF only if no bytes were read.
// If an EOF happens after reading some but not all the bytes,
// ReadFull returns [ErrUnexpectedEOF].
// On return, n == len(buf) if and only if err == nil.
// If r returns an error having read at least len(buf) bytes, the error is dropped.
func ReadFull(r Reader, buf []byte) (n int, err error) {
```

```go
// /usr/local/go/src/io/io.go:375–385
// Copy copies from src to dst until either EOF is reached
// on src or an error occurs. It returns the number of bytes
// copied and the first error encountered while copying, if any.
//
// A successful Copy returns err == nil, not err == EOF.
// Because Copy is defined to read from src until EOF, it does
// not treat an EOF from Read as an error to be reported.
//
// If src implements [WriterTo],
// the copy is implemented by calling src.WriteTo(dst).
// Otherwise, if dst implements [ReaderFrom],
// the copy is implemented by calling dst.ReadFrom(src).
func Copy(dst Writer, src Reader) (written int64, err error) {
```

```go
// /usr/local/go/src/io/io.go:388–393
// CopyBuffer is identical to Copy except that it stages through the
// provided buffer (if one is required) rather than allocating a
// temporary one. If buf is nil, one is allocated; otherwise if it has
// zero length, CopyBuffer panics.
//
// If either src implements [WriterTo] or dst implements [ReaderFrom],
// buf will not be used to perform the copy.
func CopyBuffer(dst Writer, src Reader, buf []byte) (written int64, err error) {
```

```go
// /usr/local/go/src/io/io.go:440–442
// LimitReader returns a Reader that reads from r
// but stops with EOF after n bytes.
// The underlying implementation is a *LimitedReader.
func LimitReader(r Reader, n int64) Reader { return &LimitedReader{r, n} }
```

### Unexported comment exemplars

```go
// /usr/local/go/src/io/io.go:396–397
// copyBuffer is the actual implementation of Copy and CopyBuffer.
// if buf is nil, one is allocated.
func copyBuffer(dst Writer, src Reader, buf []byte) (written int64, err error) {
```

### Inline comments inside function bodies

```go
// /usr/local/go/src/io/io.go:399–401
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(WriterTo); ok {
```

```go
// /usr/local/go/src/io/io.go:403–404
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rf, ok := dst.(ReaderFrom); ok {
```

```go
// /usr/local/go/src/io/io.go:364–368
	if written < n && err == nil {
		// src stopped early; must have been EOF.
		err = EOF
	}
```

### Error message strings

```go
// /usr/local/go/src/io/io.go
var ErrShortWrite    = errors.New("short write")
var errInvalidWrite  = errors.New("invalid write result")
var ErrShortBuffer   = errors.New("short buffer")
var EOF              = errors.New("EOF")
var ErrUnexpectedEOF = errors.New("unexpected EOF")
var ErrNoProgress    = errors.New("multiple Read calls return no data or error")
var errWhence        = errors.New("Seek: invalid whence")
var errOffset        = errors.New("Seek: invalid offset")
```

### Hedge/TODO comments

No TODO/BUG/HACK markers found in io/io.go. The package is remarkably clean.

---

## sync

### Package doc

```go
// /usr/local/go/src/sync/mutex.go:5–10
// Package sync provides basic synchronization primitives such as mutual
// exclusion locks. Other than the [Once] and [WaitGroup] types, most are intended
// for use by low-level library routines. Higher-level synchronization is
// better done via channels and communication.
//
// Values containing the types defined in this package should not be copied.
package sync
```

### Exported type docs

```go
// /usr/local/go/src/sync/mutex.go:17–29
// A Mutex is a mutual exclusion lock.
// The zero value for a Mutex is an unlocked mutex.
//
// A Mutex must not be copied after first use.
//
// In the terminology of [the Go memory model],
// the n'th call to [Mutex.Unlock] "synchronizes before" the m'th call to [Mutex.Lock]
// for any n < m.
// A successful call to [Mutex.TryLock] is equivalent to a call to Lock.
// A failed call to TryLock does not establish any "synchronizes before"
// relation at all.
//
// [the Go memory model]: https://go.dev/ref/mem
type Mutex struct {
```

```go
// /usr/local/go/src/sync/rwmutex.go:16–37
// A RWMutex is a reader/writer mutual exclusion lock.
// The lock can be held by an arbitrary number of readers or a single writer.
// The zero value for a RWMutex is an unlocked mutex.
//
// A RWMutex must not be copied after first use.
//
// If any goroutine calls [RWMutex.Lock] while the lock is already held by
// one or more readers, concurrent calls to [RWMutex.RLock] will block until
// the writer has acquired (and released) the lock, to ensure that
// the lock eventually becomes available to the writer.
// Note that this prohibits recursive read-locking.
// A [RWMutex.RLock] cannot be upgraded into a [RWMutex.Lock],
// nor can a [RWMutex.Lock] be downgraded into a [RWMutex.RLock].
//
// In the terminology of [the Go memory model],
// the n'th call to [RWMutex.Unlock] "synchronizes before" the m'th call to Lock
// for any n < m, just as for [Mutex].
// ...
// [the Go memory model]: https://go.dev/ref/mem
type RWMutex struct {
```

```go
// /usr/local/go/src/sync/once.go:12–20
// Once is an object that will perform exactly one action.
//
// A Once must not be copied after first use.
//
// In the terminology of [the Go memory model],
// the return from f "synchronizes before"
// the return from any call of once.Do(f).
//
// [the Go memory model]: https://go.dev/ref/mem
type Once struct {
```

```go
// /usr/local/go/src/sync/waitgroup.go:17–42
// A WaitGroup is a counting semaphore typically used to wait
// for a group of goroutines or tasks to finish.
//
// Typically, a main goroutine will start tasks, each in a new
// goroutine, by calling [WaitGroup.Go] and then wait for all tasks to
// complete by calling [WaitGroup.Wait]. For example:
//
//	var wg sync.WaitGroup
//	wg.Go(task1)
//	wg.Go(task2)
//	wg.Wait()
//
// A WaitGroup may also be used for tracking tasks without using Go to
// start new goroutines by using [WaitGroup.Add] and [WaitGroup.Done].
// ...
// A WaitGroup must not be copied after first use.
type WaitGroup struct {
```

### Exported function docs

```go
// /usr/local/go/src/sync/mutex.go:42–44
// Lock locks m.
// If the lock is already in use, the calling goroutine
// blocks until the mutex is available.
func (m *Mutex) Lock() {
```

```go
// /usr/local/go/src/sync/mutex.go:49–53
// TryLock tries to lock m and reports whether it succeeded.
//
// Note that while correct uses of TryLock do exist, they are rare,
// and use of TryLock is often a sign of a deeper problem
// in a particular use of mutexes.
func (m *Mutex) TryLock() bool {
```

```go
// /usr/local/go/src/sync/mutex.go:58–63
// Unlock unlocks m.
// It is a run-time error if m is not locked on entry to Unlock.
//
// A locked [Mutex] is not associated with a particular goroutine.
// It is allowed for one goroutine to lock a Mutex and then
// arrange for another goroutine to unlock it.
func (m *Mutex) Unlock() {
```

```go
// /usr/local/go/src/sync/once.go:35–53
// Do calls the function f if and only if Do is being called for the
// first time for this instance of [Once]. In other words, given
//
//	var once Once
//
// if once.Do(f) is called multiple times, only the first call will invoke f,
// even if f has a different value in each invocation. A new instance of
// Once is required for each function to execute.
//
// Do is intended for initialization that must be run exactly once. Since f
// is niladic, it may be necessary to use a function literal to capture the
// arguments to a function to be invoked by Do:
//
//	config.once.Do(func() { config.init(filename) })
//
// Because no call to Do returns until the one call to f returns, if f causes
// Do to be called, it will deadlock.
//
// If f panics, Do considers it to have returned; future calls of Do return
// without calling f.
func (o *Once) Do(f func()) {
```

```go
// /usr/local/go/src/sync/waitgroup.go:54–67
// Add adds delta, which may be negative, to the [WaitGroup] task counter.
// If the counter becomes zero, all goroutines blocked on [WaitGroup.Wait] are released.
// If the counter goes negative, Add panics.
//
// Callers should prefer [WaitGroup.Go].
//
// Note that calls with a positive delta that occur when the counter is zero
// must happen before a Wait. Calls with a negative delta, or calls with a
// positive delta that start when the counter is greater than zero, may happen
// at any time.
// Typically this means the calls to Add should execute before the statement
// creating the goroutine or other event to be waited for.
// If a WaitGroup is reused to wait for several independent sets of events,
// new Add calls must happen after all previous Wait calls have returned.
// See the WaitGroup example.
func (wg *WaitGroup) Add(delta int) {
```

```go
// /usr/local/go/src/sync/waitgroup.go:148–153
// Done decrements the [WaitGroup] task counter by one.
// It is equivalent to Add(-1).
//
// Callers should prefer [WaitGroup.Go].
//
// In the terminology of [the Go memory model], a call to Done
// "synchronizes before" the return of any Wait call that it unblocks.
func (wg *WaitGroup) Done() {
```

```go
// /usr/local/go/src/sync/waitgroup.go:159
// Wait blocks until the [WaitGroup] task counter is zero.
func (wg *WaitGroup) Wait() {
```

### Unexported comment exemplars

```go
// /usr/local/go/src/sync/once.go:22–27
	// done indicates whether the action has been performed.
	// It is first in the struct because it is used in the hot path.
	// The hot path is inlined at every call site.
	// Placing done first allows more compact instructions on some architectures (amd64/386),
	// and fewer instructions (to calculate offset) on other architectures.
	done atomic.Bool
```

```go
// /usr/local/go/src/sync/waitgroup.go:46–49
	// Bits (high to low):
	//   bits[0:32]  counter
	//   bits[32]    flag: synctest bubble membership
	//   bits[33:64] wait count
	state atomic.Uint64
```

```go
// /usr/local/go/src/sync/rwmutex.go:49–61
// Happens-before relationships are indicated to the race detector via:
// - Unlock  -> Lock:  readerSem
// - Unlock  -> RLock: readerSem
// - RUnlock -> Lock:  writerSem
//
// The methods below temporarily disable handling of race synchronization
// events in order to provide the more precise model above to the race
// detector.
//
// For example, atomic.AddInt32 in RLock should not appear to provide
// acquire-release semantics, which would incorrectly synchronize racing
// readers, thus potentially missing races.
```

### Inline comments inside function bodies

```go
// /usr/local/go/src/sync/once.go:55–68
	// Note: Here is an incorrect implementation of Do:
	//
	//	if o.done.CompareAndSwap(false, true) {
	//		f()
	//	}
	//
	// Do guarantees that when it returns, f has finished.
	// This implementation would not implement that guarantee:
	// given two simultaneous calls, the winner of the cas would
	// call f, and the second would return immediately, without
	// waiting for the first's call to f to complete.
	// This is why the slow path falls back to a mutex, and why
	// the o.done.Store must be delayed until after f returns.

	if !o.done.Load() {
		// Outlined slow-path to allow inlining of the fast-path.
		o.doSlow(f)
	}
```

```go
// /usr/local/go/src/sync/waitgroup.go:103–110
	// This goroutine has set counter to 0 when waiters > 0.
	// Now there can't be concurrent mutations of state:
	// - Adds must not happen concurrently with Wait,
	// - Wait does not increment waiters if it sees counter == 0.
	// Still do a cheap sanity check to detect WaitGroup misuse.
```

```go
// /usr/local/go/src/sync/waitgroup.go:176–186
			defer func() {
				if x := recover(); x != nil {
					// f panicked, which will be fatal because
					// this is a new goroutine.
					//
					// Calling Done will unblock Wait in the main goroutine,
					// allowing it to race with the fatal panic and
					// possibly even exit the process (os.Exit(0))
					// before the panic completes.
					//
					// This is almost certainly undesirable,
					// so instead avoid calling Done and simply panic.
					panic(x)
				}
```

### Hedge/TODO comments

No TODO comments found in mutex.go, once.go, or waitgroup.go. These files are
considered stable and finished; hedge comments are absent.

---

## os/exec

### Package doc

```go
// /usr/local/go/src/os/exec/exec.go:5–90
// Package exec runs external commands. It wraps os.StartProcess to make it
// easier to remap stdin and stdout, connect I/O with pipes, and do other
// adjustments.
//
// Unlike the "system" library call from C and other languages, the
// os/exec package intentionally does not invoke the system shell and
// does not expand any glob patterns or handle other expansions,
// pipelines, or redirections typically done by shells. The package
// behaves more like C's "exec" family of functions. To expand glob
// patterns, either call the shell directly, taking care to escape any
// dangerous input, or use the [path/filepath] package's Glob function.
// To expand environment variables, use package os's ExpandEnv.
//
// Note that the examples in this package assume a Unix system.
// They may not run on Windows, and they do not run in the Go Playground
// used by go.dev and pkg.go.dev.
//
// # Executables in the current directory
//
// The functions [Command] and [LookPath] look for a program
// in the directories listed in the current path, following the
// conventions of the host operating system.
// Operating systems have for decades included the current
// directory in this search, sometimes implicitly and sometimes
// configured explicitly that way by default.
// Modern practice is that including the current directory
// is usually unexpected and often leads to security problems.
//
// To avoid those security problems, as of Go 1.19, this package will not resolve a program
// using an implicit or explicit path entry relative to the current directory.
// ...
// Before adding such overrides, make sure you understand the
// security implications of doing so.
// See https://go.dev/blog/path-security for more information.
package exec
```

### Exported type docs

```go
// /usr/local/go/src/os/exec/exec.go:110–112
// Error is returned by [LookPath] when it fails to classify a file as an
// executable.
type Error struct {
```

```go
// /usr/local/go/src/os/exec/exec.go:125–127
// ErrWaitDelay is returned by [Cmd.Wait] if the process exits with a
// successful status code but its output pipes are not closed before the
// command's WaitDelay expires.
var ErrWaitDelay = errors.New("exec: WaitDelay expired before I/O complete")
```

```go
// /usr/local/go/src/os/exec/exec.go:144–147
// Cmd represents an external command being prepared or run.
//
// A Cmd cannot be reused after calling its [Cmd.Start], [Cmd.Run],
// [Cmd.Output], or [Cmd.CombinedOutput] methods.
type Cmd struct {
```

### Exported struct field docs

```go
// /usr/local/go/src/os/exec/exec.go:149–154
	// Path is the path of the command to run.
	//
	// This is the only field that must be set to a non-zero
	// value. If Path is relative, it is evaluated relative
	// to Dir.
	Path string
```

```go
// /usr/local/go/src/os/exec/exec.go:162–171
	// Env specifies the environment of the process.
	// Each entry is of the form "key=value".
	// If Env is nil, the new process uses the current process's
	// environment.
	// If Env contains duplicate environment keys, only the last
	// value in the slice for each duplicate key is used.
	// As a special case on Windows, SYSTEMROOT is always added if
	// missing and not explicitly set to the empty string.
	//
	// See also the Dir field, which may set PWD in the environment.
	Env []string
```

```go
// /usr/local/go/src/os/exec/exec.go:174–192
	// Dir specifies the working directory of the command.
	// If Dir is the empty string, Run runs the command in the
	// calling process's current directory.
	//
	// On Unix systems, the value of Dir also determines the
	// child process's PWD environment variable if not otherwise
	// specified. A Unix process represents its working directory
	// not by name but as an implicit reference to a node in the
	// file tree. So, if the child process obtains its working
	// directory by calling a function such as C's getcwd, which
	// computes the canonical name by walking up the file tree, it
	// will not recover the original value of Dir if that value
	// was an alias involving symbolic links. However, if the
	// child process calls Go's [os.Getwd] or GNU C's
	// get_current_dir_name, and the value of PWD is an alias for
	// the current directory, those functions will return the
	// value of PWD, which matches the value of Dir.
	Dir string
```

### Unexported comment exemplars

```go
// /usr/local/go/src/os/exec/exec.go:130–131
// wrappedError wraps an error without relying on fmt.Errorf.
```

```go
// /usr/local/go/src/os/exec/exec.go:329–336
	// The stack saved when the Command was created, if GODEBUG contains
	// execwait=2. Used for debugging leaks.
	createdByStack []byte

	// For a security release long ago, we created x/sys/execabs,
	// which manipulated the unexported lookPathErr error field
	// in this struct. For Go 1.19 we exported the field as Err error,
	// above, but we have to keep lookPathErr around for use by
	// old programs building against new toolchains.
	// The String and Start methods look for an error in lookPathErr
	// in preference to Err, to preserve the errors that execabs sets.
	//
	// In general we don't guarantee misuse of reflect like this,
	// but the misuse of reflect was by us, the best of various bad
	// options to fix the security problem, and people depend on
	// those old copies of execabs continuing to work.
	// The result is that we have to leave this variable around for the
	// rest of time, a compatibility scar.
	//
	// See https://go.dev/blog/path-security
	// and https://go.dev/issue/43724 for more context.
	lookPathErr error
```

### Inline comments inside function bodies

```go
// /usr/local/go/src/os/exec/exec.go:641–645
	// Check for doubled Start calls before we defer failure cleanup. If the prior
	// call to Start succeeded, we don't want to spuriously close its pipes.
	// It is an error to call Start twice even if the first call did not create a process.
	if atomic.SwapInt32(&c.startCalled, 1) != 0 {
		return errors.New("exec: already started")
	}
```

```go
// /usr/local/go/src/os/exec/exec.go:679–695
		} else {
			// If *Cmd was made without using Command at all, or if Command was
			// called with a relative path, we had to wait until now to resolve
			// it in case c.Dir was changed.
			//
			// Unfortunately, we cannot write the result back to c.Path because programs
			// may assume that they can call Start concurrently with reading the path.
			// (It is safe and non-racy to do so on Unix platforms, and users might not
			// test with the race detector on all platforms;
			// see https://go.dev/issue/62596.)
			//
			// So we will pass the fully resolved path to os.StartProcess, but leave
			// c.Path as is: missing a bit of logging information seems less harmful
			// than triggering a surprising data race, and if the user really cares
			// about that bit of logging they can always use LookPath to resolve it.
```

### Error message strings

```go
// /usr/local/go/src/os/exec/exec.go
var ErrWaitDelay = errors.New("exec: WaitDelay expired before I/O complete")
var ErrDot = errors.New("cannot run executable found relative to current directory")
errors.New("exec: already started")
errors.New("exec: no command")
errors.New("exec: command with a non-nil Cancel was not created with CommandContext")
errors.New("exec: not started")
errors.New("exec: Wait was already called")
errors.New("exec: Stdout already set")
errors.New("exec: Stderr already set")
errors.New("exec: Stdin already set")
errors.New("exec: StdinPipe after process started")
errors.New("exec: StdoutPipe after process started")
errors.New("exec: StderrPipe after process started")
errors.New("exec: environment variable contains NUL")
```

### Hedge/TODO comments

```go
// /usr/local/go/src/os/exec/exec.go (no TODO in production Start/Wait paths)
```

No BUG or HACK comments found. A few minor inline notes are plain prose, not
tagged hedge markers.

---

## errors

### Package doc

```go
// /usr/local/go/src/errors/errors.go:5–60
// Package errors implements functions to manipulate errors.
//
// The [New] function creates errors whose only content is a text message.
//
// An error e wraps another error if e's type has one of the methods
//
//	Unwrap() error
//	Unwrap() []error
//
// If e.Unwrap() returns a non-nil error w or a slice containing w,
// then we say that e wraps w. ...
//
// An easy way to create wrapped errors is to call [fmt.Errorf] and apply
// the %w verb to the error argument:
//
//	wrapsErr := fmt.Errorf("... %w ...", ..., err, ...)
//
// Successive unwrapping of an error creates a tree. The [Is] and [As]
// functions inspect an error's tree by examining first the error
// itself followed by the tree of each of its children in turn
// (pre-order, depth-first traversal).
//
// See https://go.dev/blog/go1.13-errors for a deeper discussion of the
// philosophy of wrapping and when to wrap.
```

### Exported type/func docs

```go
// /usr/local/go/src/errors/errors.go:62–63
// New returns an error that formats as the given text.
// Each call to New returns a distinct error value even if the text is identical.
func New(text string) error {
```

```go
// /usr/local/go/src/errors/errors.go:77–89
// ErrUnsupported indicates that a requested operation cannot be performed,
// because it is unsupported. For example, a call to [os.Link] when using a
// file system that does not support hard links.
//
// Functions and methods should not return this error but should instead
// return an error including appropriate context that satisfies
//
//	errors.Is(err, errors.ErrUnsupported)
//
// either by directly wrapping ErrUnsupported or by implementing an [Is] method.
//
// Functions and methods should document the cases in which an error
// wrapping this will be returned.
var ErrUnsupported = New("unsupported operation")
```

```go
// /usr/local/go/src/errors/wrap.go:11–16
// Unwrap returns the result of calling the Unwrap method on err, if err's
// type contains an Unwrap method returning error.
// Otherwise, Unwrap returns nil.
//
// Unwrap only calls a method of the form "Unwrap() error".
// In particular Unwrap does not unwrap errors returned by [Join].
func Unwrap(err error) error {
```

```go
// /usr/local/go/src/errors/wrap.go:27–44
// Is reports whether any error in err's tree matches target.
// The target must be comparable.
//
// The tree consists of err itself, followed by the errors obtained by repeatedly
// calling its Unwrap() error or Unwrap() []error method. When err wraps multiple
// errors, Is examines err followed by a depth-first traversal of its children.
//
// An error is considered to match a target if it is equal to that target or if
// it implements a method Is(error) bool such that Is(target) returns true.
//
// An error type might provide an Is method so it can be treated as equivalent
// to an existing error. For example, if MyError defines
//
//	func (m MyError) Is(target error) bool { return target == fs.ErrExist }
//
// then Is(MyError{}, fs.ErrExist) returns true. See [syscall.Errno.Is] for
// an example in the standard library. An Is method should only shallowly
// compare err and the target and not call [Unwrap] on either.
func Is(err, target error) bool {
```

```go
// /usr/local/go/src/errors/wrap.go:81–100
// As finds the first error in err's tree that matches target, and if one is found, sets
// target to that error value and returns true. Otherwise, it returns false.
//
// For most uses, prefer [AsType]. As is equivalent to [AsType] but sets its target
// argument rather than returning the matching error and doesn't require its target
// argument to implement error.
// ...
// An error matches target if the error's concrete value is assignable to the value
// pointed to by target, or if the error has a method As(any) bool such that
// As(target) returns true. In the latter case, the As method is responsible for
// setting target.
//
// An error type might provide an As method so it can be treated as if it were a
// different error type.
//
// As panics if target is not a non-nil pointer to either a type that implements
// error, or to any interface type.
func As(err error, target any) bool {
```

### Unexported comment exemplars

```go
// /usr/local/go/src/errors/errors.go:68
// errorString is a trivial implementation of error.
type errorString struct {
```

### Error message strings

```go
// /usr/local/go/src/errors/errors.go
// (errors.New calls — these ARE the error strings)
errors.New("context canceled")         // context package
errors.New("context deadline exceeded") // context package (via deadlineExceededError.Error())
errors.New("unsupported operation")
```

### Hedge/TODO comments

No TODO, BUG, or HACK markers in the errors package.

---

## context

### Package doc

```go
// /usr/local/go/src/context/context.go:5–60
// Package context defines the Context type, which carries deadlines,
// cancellation signals, and other request-scoped values across API boundaries
// and between processes.
//
// Incoming requests to a server should create a [Context], and outgoing
// calls to servers should accept a Context. The chain of function
// calls between them must propagate the Context, optionally replacing
// it with a derived Context created using [WithCancel], [WithDeadline],
// [WithTimeout], or [WithValue].
//
// A Context may be canceled to indicate that work done on its behalf should stop.
// A Context with a deadline is canceled after the deadline passes.
// When a Context is canceled, all Contexts derived from it are also canceled.
//
// The [WithCancel], [WithDeadline], and [WithTimeout] functions take a
// Context (the parent) and return a derived Context (the child) and a
// [CancelFunc]. Calling the CancelFunc directly cancels the child and its
// children, removes the parent's reference to the child, and stops
// any associated timers. Failing to call the CancelFunc leaks the
// child and its children until the parent is canceled. The go vet tool
// checks that CancelFuncs are used on all control-flow paths.
//
// Programs that use Contexts should follow these rules to keep interfaces
// consistent across packages and enable static analysis tools to check context
// propagation:
//
// Do not store Contexts inside a struct type; instead, pass a Context
// explicitly to each function that needs it. ...
//
// Do not pass a nil [Context], even if a function permits it. Pass [context.TODO]
// if you are unsure about which Context to use.
//
// Use context Values only for request-scoped data that transits processes and
// APIs, not for passing optional parameters to functions.
//
// The same Context may be passed to functions running in different goroutines;
// Contexts are safe for simultaneous use by multiple goroutines.
//
// See https://go.dev/blog/context for example code for a server that uses
// Contexts.
package context
```

### Exported type docs

```go
// /usr/local/go/src/context/context.go:64–66
// A Context carries a deadline, a cancellation signal, and other values across
// API boundaries.
//
// Context's methods may be called by multiple goroutines simultaneously.
type Context interface {
```

```go
// /usr/local/go/src/context/context.go:228–232
// A CancelFunc tells an operation to abandon its work.
// A CancelFunc does not wait for the work to stop.
// A CancelFunc may be called by multiple goroutines simultaneously.
// After the first call, subsequent calls to a CancelFunc do nothing.
type CancelFunc func()
```

```go
// /usr/local/go/src/context/context.go:259–265
// A CancelCauseFunc behaves like a [CancelFunc] but additionally sets the cancellation cause.
// This cause can be retrieved by calling [Cause] on the canceled Context or on
// any of its derived Contexts.
//
// If the context has already been canceled, CancelCauseFunc does not set the cause.
// For example, if childContext is derived from parentContext:
//   - if parentContext is canceled with cause1 before childContext is canceled with cause2,
//     then Cause(parentContext) == Cause(childContext) == cause1
//   - if childContext is canceled with cause2 before parentContext is canceled with cause1,
//     then Cause(parentContext) == cause1 and Cause(childContext) == cause2
type CancelCauseFunc func(cause error)
```

### Exported function docs

```go
// /usr/local/go/src/context/context.go:211–215
// Background returns a non-nil, empty [Context]. It is never canceled, has no
// values, and has no deadline. It is typically used by the main function,
// initialization, and tests, and as the top-level Context for incoming
// requests.
func Background() Context {
```

```go
// /usr/local/go/src/context/context.go:219–222
// TODO returns a non-nil, empty [Context]. Code should use context.TODO when
// it's unclear which Context to use or it is not yet available (because the
// surrounding function has not yet been extended to accept a Context
// parameter).
func TODO() Context {
```

```go
// /usr/local/go/src/context/context.go:233–239
// WithCancel returns a derived context that points to the parent context
// but has a new Done channel. The returned context's Done channel is closed
// when the returned cancel function is called or when the parent context's
// Done channel is closed, whichever happens first.
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this [Context] complete.
func WithCancel(parent Context) (ctx Context, cancel CancelFunc) {
```

```go
// /usr/local/go/src/context/context.go:616–626
// WithDeadline returns a derived context that points to the parent context
// but has the deadline adjusted to be no later than d. If the parent's
// deadline is already earlier than d, WithDeadline(parent, d) is semantically
// equivalent to parent. The returned [Context.Done] channel is closed when
// the deadline expires, when the returned cancel function is called,
// or when the parent context's Done channel is closed, whichever happens first.
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this [Context] complete.
func WithDeadline(parent Context, d time.Time) (Context, CancelFunc) {
```

```go
// /usr/local/go/src/context/context.go:693–704
// WithTimeout returns WithDeadline(parent, time.Now().Add(timeout)).
//
// Canceling this context releases resources associated with it, so code should
// call cancel as soon as the operations running in this [Context] complete:
//
//	func slowOperationWithTimeout(ctx context.Context) (Result, error) {
//		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
//		defer cancel()  // releases resources if slowOperation completes before timeout elapses
//		return slowOperation(ctx)
//	}
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
```

```go
// /usr/local/go/src/context/context.go:714–728
// WithValue returns a derived context that points to the parent Context.
// In the derived context, the value associated with key is val.
//
// Use context Values only for request-scoped data that transits processes and
// APIs, not for passing optional parameters to functions.
//
// The provided key must be comparable and should not be of type
// string or any other built-in type to avoid collisions between
// packages using context. Users of WithValue should define their own
// types for keys. To avoid allocating when assigning to an
// interface{}, context keys often have concrete type
// struct{}. Alternatively, exported context key variables' static
// type should be a pointer or interface.
func WithValue(parent Context, key, val any) Context {
```

### Unexported comment exemplars

```go
// /usr/local/go/src/context/context.go:180
// An emptyCtx is never canceled, has no values, and has no deadline.
// It is the common base of backgroundCtx and todoCtx.
type emptyCtx struct{}
```

```go
// /usr/local/go/src/context/context.go:424–430
// A cancelCtx can be canceled. When canceled, it also cancels any children
// that implement canceler.
type cancelCtx struct {
	Context

	mu       sync.Mutex            // protects following fields
	done     atomic.Value          // of chan struct{}, created lazily, closed by first cancel call
	children map[canceler]struct{} // set to nil by the first cancel call
```

```go
// /usr/local/go/src/context/context.go:369–374
// parentCancelCtx returns the underlying *cancelCtx for parent.
// It does this by looking up parent.Value(&cancelCtxKey) to find
// the innermost enclosing *cancelCtx and then checking whether
// parent.Done() matches that *cancelCtx. (If not, the *cancelCtx
// has been wrapped in a custom implementation providing a
// different done channel, in which case we should not bypass it.)
func parentCancelCtx(parent Context) (*cancelCtx, bool) {
```

```go
// /usr/local/go/src/context/context.go:382–383
// removeChild removes a context from its parent.
func removeChild(parent Context, child canceler) {
```

```go
// /usr/local/go/src/context/context.go:388–389
// A canceler is a context type that can be canceled directly. The
// implementations are *cancelCtx and *timerCtx.
type canceler interface {
```

```go
// /usr/local/go/src/context/context.go:670–673
// A timerCtx carries a timer and a deadline. It embeds a cancelCtx to
// implement Done and Err. It implements cancel by stopping its timer then
// delegating to cancelCtx.cancel.
type timerCtx struct {
```

### Inline comments inside function bodies

```go
// /usr/local/go/src/context/context.go:476–483
	select {
	case <-done:
		// parent is already canceled
		child.cancel(false, parent.Err(), Cause(parent))
		return
	default:
	}

	if p, ok := parentCancelCtx(parent); ok {
		// parent is a *cancelCtx, or derives from one.
		p.mu.Lock()
		if err := p.err.Load(); err != nil {
			// parent has already been canceled
			child.cancel(false, err.(error), p.cause)
```

```go
// /usr/local/go/src/context/context.go:493–496
	if a, ok := parent.(afterFuncer); ok {
		// parent implements an AfterFunc method.
		c.mu.Lock()
```

```go
// /usr/local/go/src/context/context.go:461–463
func (c *cancelCtx) Err() error {
	// An atomic load is ~5x faster than a mutex, which can matter in tight loops.
	if err := c.err.Load(); err != nil {
		// Ensure the done channel has been closed before returning a non-nil error.
```

```go
// /usr/local/go/src/context/context.go:643
		c.cancel(true, DeadlineExceeded, cause) // deadline has already passed
```

```go
// /usr/local/go/src/context/context.go:662–664
	if removeFromParent {
		// Remove this timerCtx from its parent cancelCtx's children.
		removeChild(c.cancelCtx.Context, c)
	}
```

### Error message strings

```go
// /usr/local/go/src/context/context.go
var Canceled = errors.New("context canceled")
// deadlineExceededError.Error() returns:
"context deadline exceeded"
```

### Hedge/TODO comments

No TODO or BUG comments found in context.go. The package doc itself explains
design rationale inline rather than deferring to TODO markers.

---

## Patterns observed

1. **Package docs scale with domain complexity.** `io` gets four terse sentences.
   `net/http` gets a structured tutorial with code samples and H2 sections.
   `context` falls in between: dense prose rules, no code samples in the package doc
   itself but code in interface method docs. Length is proportional to how much
   the caller needs to know before using the package at all.

2. **Interface method docs carry the contract, not the interface name.**
   `Reader.Read` gets 200+ words because the contract is subtle. `Closer.Close`
   gets two sentences. `ByteWriter.WriteByte` gets nothing — the name is the doc.
   Verbosity follows genuine complexity, not reflexive thoroughness.

3. **"must not," "must," "should" are precise, not decorative.** Stdlib uses
   "must" for invariants the runtime will not enforce (e.g., "Write must not modify
   the slice data"), "should" for convention (e.g., "Callers should prefer"), and
   reserves "note that" for genuine surprises. The words are load-bearing.

4. **Inline body comments explain WHY at decision points, never WHAT.**
   `// Avoids an allocation and a copy.` follows a type assertion, not a Read call.
   `// Outlined slow-path to allow inlining of the fast-path.` names the compiler
   optimization driving the split. `// parent is already canceled` annotates a
   select branch where the non-obvious path needs a label. The code needs no
   gloss; only the intent behind the branch does.

5. **Error strings are short, lowercase, no period.** Sentinel vars: `"short write"`,
   `"EOF"`, `"context canceled"`. Package-prefixed errors consistently use
   `"pkg: noun"` shape: `"http: named cookie not present"`,
   `"exec: already started"`, `"json: unknown field %q"`. The prefix tells you
   where the error originated. No two adjacent error sites in the same function
   read identically — phrasing varies with context.

6. **Unexported type comments name the role, not the fields.** `cancelCtx` gets
   "can be canceled. When canceled, it also cancels any children that implement
   canceler" — the structural relationship, not a field inventory. `connReader`
   gets its architectural role: what it wraps, what it adds, how it behaves after
   hijack. The unexported comment earns its words by explaining something the
   struct definition cannot.

7. **TODO markers are persistent, often named, and tied to real constraints.**
   `// TODO(bradfitz): ...` annotates a deliberate non-implementation with a
   rationale. They are not cleanup reminders but documented design decisions left
   open. The pattern `// TODO: unify?` on an error var signals conscious duplication.
   BUG markers are nearly absent in these packages (one found in sync/atomic/doc.go
   for a 386 instruction limit). HACK/XXX are absent entirely.

8. **Struct field comments speak "for X case, Y" when fields have dual meaning.**
   `Request.Body`, `Request.URL`, `Request.Method` each repeat "For client
   requests... / For server requests..." because the type is used in both
   directions. This pattern — repeated "For X" leads within one field comment —
   is idiomatic for dual-use types and absent from single-use types.

9. **Once-wrong-implementation notes.** `sync.Once.Do` contains a multi-line
   `// Note: Here is an incorrect implementation` block explaining exactly why a
   simpler CAS approach would fail. This is the most elaborate inline comment in
   the surveyed packages — justified because the "obvious" impl is actively
   tempting and wrong.

10. **Function one-liners are genuinely terse.** `UserAgent`, `Cookies`,
    `ProtoAtLeast`, `Wait` — single-sentence docs with no blank lines. The pattern
    is: if the function name + signature tells you everything you need to know, the
    doc adds at most one sentence of context (return value semantics, or one
    behavioral edge). No padding.

11. **Security notes are a distinct register.** `os/exec` package doc dedicates
    a full `# Executables in the current directory` section to a security change,
    with runnable before/after examples and a `GODEBUG` escape hatch. The voice
    is direct and imperative: "make sure you understand the security implications."
    No hedging, no softening. Security content gets proportionally more space than
    its code surface area.

12. **"Deprecated:" is its own sentence, first in the comment block.** Both
    `CloseNotifier` and `ErrWriteAfterFlush` follow the pattern: state the
    deprecation, then give the migration path. The word "Deprecated:" is always
    the leading token — tooling (godoc, gopls) relies on this shape to render
    deprecation warnings.
