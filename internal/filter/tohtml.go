package filter

import (
	"bytes"
	"fmt"
	"io"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// ToHTML converts markdown from r to a standalone HTML document written to w.
// Replaces pandoc in aerc's multipart-converters.
func ToHTML(r io.Reader, w io.Writer) error {
	src, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.Table),
	)

	var body bytes.Buffer
	if err := md.Convert(src, &body); err != nil {
		return fmt.Errorf("converting markdown: %w", err)
	}

	const head = `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body>
`
	const tail = `</body>
</html>
`
	if _, err := fmt.Fprint(w, head+body.String()+tail); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}
