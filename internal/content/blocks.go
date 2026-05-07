package content

// Block represents a semantic unit of email content. The unexported
// marker method seals the interface to the concrete types in this
// package. Discrimination is by Go type switch on the value.
type Block interface {
	isBlock()
}

// Span represents an inline styled segment within a block. Sealed
// the same way as Block.
type Span interface {
	isSpan()
}

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

func (Paragraph) isBlock()        {}
func (Heading) isBlock()          {}
func (Blockquote) isBlock()       {}
func (QuoteAttribution) isBlock() {}
func (Signature) isBlock()        {}
func (Rule) isBlock()             {}
func (CodeBlock) isBlock()        {}
func (Table) isBlock()            {}
func (ListItem) isBlock()         {}

type Text struct{ Content string }
type Bold struct{ Content string }
type Italic struct{ Content string }
type Code struct{ Content string }
type Link struct {
	Text string
	URL  string
}

func (Text) isSpan()   {}
func (Bold) isSpan()   {}
func (Italic) isSpan() {}
func (Code) isSpan()   {}
func (Link) isSpan()   {}

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
