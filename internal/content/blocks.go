// SPDX-License-Identifier: MIT

package content

// Block represents a semantic unit of email content.
type Block interface {
	blockType() blockKind
}

type blockKind int

const (
	kindParagraph blockKind = iota
	kindHeading
	kindBlockquote
	kindQuoteAttribution
	kindSignature
	kindRule
	kindCodeBlock
	kindTable
	kindListItem
)

// Span represents an inline styled segment within a block.
type Span interface {
	spanType() spanKind
}

type spanKind int

const (
	kindText spanKind = iota
	kindBold
	kindItalic
	kindCode
	kindLink
)

type Paragraph struct{ Spans []Span }
type Heading struct {
	Spans []Span
	Level int
}
type Blockquote struct {
	Blocks []Block
	Level  int
}
type QuoteAttribution struct{ Spans []Span }
type Signature struct{ Lines [][]Span }
type Rule struct{}
type CodeBlock struct {
	Text string
	Lang string
}
type Table struct {
	Headers [][]Span
	Rows    [][][]Span
}
type ListItem struct {
	Spans   []Span
	Ordered bool
	Index   int
}

func (Paragraph) blockType() blockKind        { return kindParagraph }
func (Heading) blockType() blockKind          { return kindHeading }
func (Blockquote) blockType() blockKind       { return kindBlockquote }
func (QuoteAttribution) blockType() blockKind { return kindQuoteAttribution }
func (Signature) blockType() blockKind        { return kindSignature }
func (Rule) blockType() blockKind             { return kindRule }
func (CodeBlock) blockType() blockKind        { return kindCodeBlock }
func (Table) blockType() blockKind            { return kindTable }
func (ListItem) blockType() blockKind         { return kindListItem }

type Text struct{ Content string }
type Bold struct{ Content string }
type Italic struct{ Content string }
type Code struct{ Content string }
type Link struct {
	Text string
	URL  string
}

func (Text) spanType() spanKind   { return kindText }
func (Bold) spanType() spanKind   { return kindBold }
func (Italic) spanType() spanKind { return kindItalic }
func (Code) spanType() spanKind   { return kindCode }
func (Link) spanType() spanKind   { return kindLink }

// Address is a parsed email address with optional display name.
type Address struct {
	Name  string
	Email string
}

// ParsedHeaders holds the structured header fields from an email.
type ParsedHeaders struct {
	From    []Address
	To      []Address
	Cc      []Address
	Bcc     []Address
	Date    string
	Subject string
}
