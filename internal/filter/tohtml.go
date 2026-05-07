package filter

import (
	"bytes"
	"fmt"
	"io"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

const htmlSkeletonHead = `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body>
`

const htmlSkeletonTail = `</body>
</html>
`

// MarkdownToHTML renders src to a standalone HTML document with the
// poplar default extensions (Linkify + Table). Use MarkdownBody when
// embedding the result inside a larger MIME message.
func MarkdownToHTML(src []byte) (string, error) {
	body, err := MarkdownBody(src)
	if err != nil {
		return "", err
	}
	return htmlSkeletonHead + body + htmlSkeletonTail, nil
}

// MarkdownBody renders src to inner HTML, with no document skeleton.
// Used by compose.AssembleMIME for the text/html part of a
// multipart/alternative.
func MarkdownBody(src []byte) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Linkify, extension.Table),
	)
	var body bytes.Buffer
	if err := md.Convert(src, &body); err != nil {
		return "", err
	}
	return body.String(), nil
}

// ToHTML reads markdown from r and writes the rendered HTML to w.
func ToHTML(r io.Reader, w io.Writer) error {
	src, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %v", err)
	}
	out, err := MarkdownToHTML(src)
	if err != nil {
		return fmt.Errorf("markdown -> html: %v", err)
	}
	if _, err := fmt.Fprint(w, out); err != nil {
		return fmt.Errorf("write stdout: %v", err)
	}
	return nil
}
