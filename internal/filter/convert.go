// SPDX-License-Identifier: MIT

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

// imageStripPlugin removes <img> tags from the output, emitting only the
// alt text (if any). Terminals cannot display images, and marketing emails
// embed dozens of product images whose URLs are pure noise. For image-links
// (<a><img></a>), the commonmark plugin handles the <a> — this plugin just
// ensures the <img> inside renders as alt text instead of ![alt](url).
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

// isSmallImage returns true if the image has explicit width or height
// attributes at or below 24px. Such images are decorative (icons, dots,
// progress indicators) and their alt text is noise.
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

// layoutTablePlugin flattens HTML tables that lack <th> elements (layout
// tables) into sequential paragraphs. Tables with <th> elements (data
// tables) pass through to the table plugin for pipe table rendering.
type layoutTablePlugin struct{}

func (p *layoutTablePlugin) Name() string { return "layout-table" }

// Init registers the layout table renderer at PriorityEarly so it intercepts
// <table> nodes before the standard table plugin (PriorityStandard).
func (p *layoutTablePlugin) Init(conv *converter.Converter) error {
	conv.Register.RendererFor("table", converter.TagTypeBlock,
		p.renderTable, converter.PriorityEarly)
	return nil
}

// hasTableHeader walks the table node's children looking for any <th> element.
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
		// Data table — let the table plugin handle it.
		return converter.RenderTryNext
	}

	// Layout table — render each cell's content as a paragraph.
	renderCells(ctx, w, n)
	return converter.RenderSuccess
}

// renderCells recursively finds <td> elements and renders their children
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

// newConverter creates an html-to-markdown converter with layout table
// detection and GFM table support for data tables.
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

// convertHTML converts an HTML string to markdown. Layout tables are
// flattened; data tables become pipe tables.
func convertHTML(input string) (string, error) {
	conv := newConverter()
	md, err := conv.ConvertString(input)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(md), nil
}
