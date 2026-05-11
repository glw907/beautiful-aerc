package catkin

import (
	"bytes"
	"iter"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
)

const chromaStyle = "monokai"

// Styles is Catkin's render-time style table. The zero value is no-op
// so Render produces plain output. Catkin owns the struct so the package
// stays library-pure; consumers map their theme onto it at the boundary.
type Styles struct {
	Heading         [6]lipgloss.Style
	Quote           lipgloss.Style
	DeepQuote       lipgloss.Style
	Bold            lipgloss.Style
	Italic          lipgloss.Style
	BoldItalic      lipgloss.Style
	CodeInline      lipgloss.Style
	CodeBlock       lipgloss.Style
	Link            lipgloss.Style
	URL             lipgloss.Style
	ListMarker      lipgloss.Style
	TaskBox         lipgloss.Style
	MatchHighlight  lipgloss.Style
	Dim             lipgloss.Style
	Squiggle        lipgloss.Style
	TidyChange      lipgloss.Style
	Popover         lipgloss.Style
	PopoverSelected lipgloss.Style
}

type spanKind int

const (
	spanText spanKind = iota
	spanCode
	spanBoldItalic
	spanBold
	spanItalic
	spanLink
)

type span struct {
	kind     spanKind
	text     string
	linkText string
	linkURL  string
}

var (
	codeSpanRE    = regexp.MustCompile("^`[^`\n]+`")
	tripleStarRE  = regexp.MustCompile(`^\*\*\*[^*\n]+\*\*\*`)
	doubleStarRE  = regexp.MustCompile(`^\*\*[^*\n]+\*\*`)
	doubleUnderRE = regexp.MustCompile(`^__[^_\n]+__`)
	singleStarRE  = regexp.MustCompile(`^\*[^*\n]+\*`)
	singleUnderRE = regexp.MustCompile(`^_[^_\n]+_`)
	linkRE        = regexp.MustCompile(`^\[([^\]\n]+)\]\(([^)\n]+)\)`)
)

type spanYield struct {
	kind              spanKind
	text              string
	linkText, linkURL string // populated only for spanLink
}

// tokenize splits s into inline-styling spans. The split is lossless:
// concatenating span.text rebuilds s.
func tokenize(s string) []span {
	var out []span
	var plain strings.Builder
	flush := func() {
		if plain.Len() > 0 {
			out = append(out, span{kind: spanText, text: plain.String()})
			plain.Reset()
		}
	}
	for sp := range spans(s) {
		if sp.kind == spanText {
			plain.WriteString(sp.text)
			continue
		}
		flush()
		out = append(out, span{
			kind:     sp.kind,
			text:     sp.text,
			linkText: sp.linkText,
			linkURL:  sp.linkURL,
		})
	}
	flush()
	return out
}

// spans scans s for inline-styling spans. Untouched bytes yield one
// rune at a time as spanText.
func spans(s string) iter.Seq[spanYield] {
	type pat struct {
		re   *regexp.Regexp
		kind spanKind
	}
	pats := []pat{
		{codeSpanRE, spanCode},
		{tripleStarRE, spanBoldItalic},
		{doubleStarRE, spanBold},
		{doubleUnderRE, spanBold},
		{singleStarRE, spanItalic},
		{singleUnderRE, spanItalic},
	}
	return func(yield func(spanYield) bool) {
		i := 0
		for i < len(s) {
			rest := s[i:]
			matched := false
			for _, p := range pats {
				if m := p.re.FindString(rest); m != "" {
					if !yield(spanYield{kind: p.kind, text: m}) {
						return
					}
					i += len(m)
					matched = true
					break
				}
			}
			if matched {
				continue
			}
			if m := linkRE.FindStringSubmatch(rest); m != nil {
				if !yield(spanYield{kind: spanLink, text: m[0], linkText: m[1], linkURL: m[2]}) {
					return
				}
				i += len(m[0])
				continue
			}
			_, size := utf8.DecodeRuneInString(rest)
			if !yield(spanYield{kind: spanText, text: rest[:size]}) {
				return
			}
			i += size
		}
	}
}

func renderSpans(spans []span, st Styles) string {
	var b strings.Builder
	for _, sp := range spans {
		switch sp.kind {
		case spanText:
			b.WriteString(sp.text)
		case spanCode:
			b.WriteString(st.CodeInline.Render(sp.text))
		case spanBoldItalic:
			b.WriteString(st.BoldItalic.Render(sp.text))
		case spanBold:
			b.WriteString(st.Bold.Render(sp.text))
		case spanItalic:
			b.WriteString(st.Italic.Render(sp.text))
		case spanLink:
			b.WriteString(st.Link.Render("[" + sp.linkText + "]"))
			b.WriteString(st.URL.Render("(" + sp.linkURL + ")"))
		}
	}
	return b.String()
}

// chromaResolved caches the (lexer, style, formatter) triple for a
// given language tag, avoiding per-render map walks inside chroma.
type chromaResolved struct {
	lexer     chroma.Lexer
	style     *chroma.Style
	formatter chroma.Formatter
}

// chromaCache keys on the info string; the empty key marks unknown lang.
var chromaCache sync.Map

func resolveChroma(info string) chromaResolved {
	if v, ok := chromaCache.Load(info); ok {
		return v.(chromaResolved)
	}
	r := chromaResolved{lexer: lexers.Get(info)}
	if r.lexer != nil {
		r.style = chromastyles.Get(chromaStyle)
		if r.style == nil {
			r.style = chromastyles.Fallback
		}
		r.formatter = formatters.Get("terminal256")
		if r.formatter == nil {
			r.formatter = formatters.Fallback
		}
	}
	chromaCache.Store(info, r)
	return r
}

// highlightFence returns one styled string per source line in body.
// Unknown lexers and chroma errors fall back to st.CodeBlock per line.
func highlightFence(info, body string, st Styles) []string {
	rawLines := strings.Split(body, "\n")
	fallback := func() []string {
		out := make([]string, len(rawLines))
		for i, line := range rawLines {
			out[i] = st.CodeBlock.Render(line)
		}
		return out
	}

	r := resolveChroma(info)
	if r.lexer == nil {
		return fallback()
	}
	iter, err := r.lexer.Tokenise(nil, body)
	if err != nil {
		return fallback()
	}
	var buf bytes.Buffer
	if err := r.formatter.Format(&buf, r.style, iter); err != nil {
		return fallback()
	}
	out := strings.Split(buf.String(), "\n")
	if len(out) > len(rawLines) {
		out = out[:len(rawLines)]
	}
	for len(out) < len(rawLines) {
		out = append(out, st.CodeBlock.Render(""))
	}
	return out
}
