// Package filter cleans inbound email bodies (HTML or plain text)
// into normalized markdown for the content renderer, and converts
// outbound markdown to HTML for multipart/alternative send.
package filter

import (
	"strconv"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
)

// imageStripPlugin replaces <img> with its alt text. Terminals can't
// display images, and marketing emails embed dozens of product
// images whose URLs are pure noise. For image-links (<a><img></a>),
// the commonmark plugin renders the <a> while this one keeps the
// inner <img> from emitting ![alt](url).
type imageStripPlugin struct{}

func (p *imageStripPlugin) Name() string { return "image-strip" }

func (p *imageStripPlugin) Init(conv *converter.Converter) error {
	conv.Register.RendererFor("img", converter.TagTypeInline,
		p.renderImg, converter.PriorityEarly)
	return nil
}

func (p *imageStripPlugin) renderImg(_ converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if isSmallImage(n) {
		return converter.RenderSuccess
	}
	for _, attr := range n.Attr {
		if attr.Key == "alt" && strings.TrimSpace(attr.Val) != "" {
			w.WriteString(strings.TrimSpace(attr.Val))
			return converter.RenderSuccess
		}
	}
	return converter.RenderSuccess
}

// isSmallImage returns true on explicit width/height attributes ≤24px.
// At that size the alt text is noise (icons, dots, progress beads).
func isSmallImage(n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key == "width" || attr.Key == "height" {
			px, err := strconv.Atoi(strings.TrimSuffix(attr.Val, "px"))
			if err == nil && px <= 24 {
				return true
			}
		}
	}
	return false
}

// layoutTablePlugin flattens <th>-less layout tables into sequential
// paragraphs. Tables with <th> rows fall through to the table plugin
// for pipe-table rendering.
type layoutTablePlugin struct{}

func (p *layoutTablePlugin) Name() string { return "layout-table" }

// Init registers at PriorityEarly so layout tables intercept <table>
// before the standard table plugin (PriorityStandard).
func (p *layoutTablePlugin) Init(conv *converter.Converter) error {
	conv.Register.RendererFor("table", converter.TagTypeBlock,
		p.renderTable, converter.PriorityEarly)
	return nil
}

func hasTableHeader(n *html.Node) bool {
	if n.Type == html.ElementNode && n.Data == "th" {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if hasTableHeader(c) {
			return true
		}
	}
	return false
}

func (p *layoutTablePlugin) renderTable(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if hasTableHeader(n) {
		return converter.RenderTryNext
	}
	renderCells(ctx, w, n)
	return converter.RenderSuccess
}

// renderCells walks the tree and renders <td> children as paragraphs
// separated by blank lines.
func renderCells(ctx converter.Context, w converter.Writer, n *html.Node) {
	if n.Type == html.ElementNode && n.Data == "td" {
		w.WriteString("\n\n")
		ctx.RenderChildNodes(ctx, w, n)
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderCells(ctx, w, c)
	}
}

// newConverter wires html-to-markdown with layout-table detection and
// GFM pipe tables for data tables.
func newConverter() *converter.Converter {
	return converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
			&layoutTablePlugin{},
			&imageStripPlugin{},
		),
	)
}

// convertHTML converts HTML to markdown. Layout tables flatten and
// data tables become pipe tables.
func convertHTML(input string) (string, error) {
	conv := newConverter()
	md, err := conv.ConvertString(input)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(md), nil
}
