package catkin

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

const chromaStyle = "monokai"

// Styles is Catkin's render-time style table. The zero value is
// no-op styles — Render produces output identical to plain mode.
//
// Catkin defines its own struct rather than borrowing a host
// theme type so the package stays library-pure: a downstream
// consumer (e.g., poplar's compose) maps its theme onto this
// struct at the boundary.
type Styles struct {
	Heading    [6]lipgloss.Style
	Quote      lipgloss.Style
	DeepQuote  lipgloss.Style
	Bold       lipgloss.Style
	Italic     lipgloss.Style
	BoldItalic lipgloss.Style
	CodeInline lipgloss.Style
	CodeBlock  lipgloss.Style
	Link       lipgloss.Style
	URL        lipgloss.Style
	ListMarker lipgloss.Style
	TaskBox    lipgloss.Style
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

// tokenize splits s into spans for inline-styling. The split is
// lossless: concatenating span.text rebuilds s.
func tokenize(s string) []span {
	var out []span
	var plain strings.Builder
	flush := func() {
		if plain.Len() > 0 {
			out = append(out, span{kind: spanText, text: plain.String()})
			plain.Reset()
		}
	}
	for i := 0; i < len(s); {
		rest := s[i:]
		if m := codeSpanRE.FindString(rest); m != "" {
			flush()
			out = append(out, span{kind: spanCode, text: m})
			i += len(m)
			continue
		}
		if m := tripleStarRE.FindString(rest); m != "" {
			flush()
			out = append(out, span{kind: spanBoldItalic, text: m})
			i += len(m)
			continue
		}
		if m := doubleStarRE.FindString(rest); m != "" {
			flush()
			out = append(out, span{kind: spanBold, text: m})
			i += len(m)
			continue
		}
		if m := doubleUnderRE.FindString(rest); m != "" {
			flush()
			out = append(out, span{kind: spanBold, text: m})
			i += len(m)
			continue
		}
		if m := singleStarRE.FindString(rest); m != "" {
			flush()
			out = append(out, span{kind: spanItalic, text: m})
			i += len(m)
			continue
		}
		if m := singleUnderRE.FindString(rest); m != "" {
			flush()
			out = append(out, span{kind: spanItalic, text: m})
			i += len(m)
			continue
		}
		if m := linkRE.FindStringSubmatch(rest); m != nil {
			flush()
			out = append(out, span{
				kind:     spanLink,
				text:     m[0],
				linkText: m[1],
				linkURL:  m[2],
			})
			i += len(m[0])
			continue
		}
		_, size := utf8.DecodeRuneInString(rest)
		plain.WriteString(rest[:size])
		i += size
	}
	flush()
	return out
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

// chromaResolved caches the (lexer, style, formatter) triple
// for a given language tag. Registry lookups are pure and stable
// for the process lifetime, so memoising avoids per-render map
// walks inside chroma.
type chromaResolved struct {
	lexer     chroma.Lexer
	style     *chroma.Style
	formatter chroma.Formatter
}

var chromaCache sync.Map // map[string]chromaResolved; "" key = unknown lang

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

// highlightFence returns one styled string per source line in
// body. On unknown lexer or any chroma error it applies
// st.CodeBlock to each line as a fallback.
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
