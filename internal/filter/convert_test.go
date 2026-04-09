package filter

import (
	"strings"
	"testing"
)

func TestConvertHTML(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string // substring that must appear
	}{
		{
			name: "simple paragraph",
			html: "<p>Hello world</p>",
			want: "Hello world",
		},
		{
			name: "bold text",
			html: "<p><strong>Important</strong></p>",
			want: "**Important**",
		},
		{
			name: "link preserved",
			html: `<p><a href="https://example.com">Click here</a></p>`,
			want: "https://example.com",
		},
		{
			name: "heading",
			html: "<h2>Section Title</h2>",
			want: "## Section Title",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertHTML(tt.html)
			if err != nil {
				t.Fatalf("convertHTML: %v", err)
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("output missing %q\ngot: %s", tt.want, got)
			}
		})
	}
}

func TestConvertHTMLImageStripping(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		want    string // substring that must appear
		notWant string // substring that must NOT appear
	}{
		{
			name:    "standalone image stripped",
			html:    `<p>Text</p><img src="https://cdn.example.com/img.jpg" alt="Product">`,
			want:    "Product",
			notWant: "cdn.example.com",
		},
		{
			name:    "image with no alt stripped completely",
			html:    `<p>Hello</p><img src="https://cdn.example.com/logo.png">`,
			want:    "Hello",
			notWant: "cdn.example.com",
		},
		{
			name:    "image-link renders as link with alt text",
			html:    `<a href="https://example.com"><img src="https://cdn.example.com/hero.jpg" alt="Shop Now"></a>`,
			want:    "Shop Now",
			notWant: "cdn.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertHTML(tt.html)
			if err != nil {
				t.Fatalf("convertHTML: %v", err)
			}
			if !strings.Contains(got, tt.want) {
				t.Errorf("output missing %q\ngot: %s", tt.want, got)
			}
			if tt.notWant != "" && strings.Contains(got, tt.notWant) {
				t.Errorf("output should not contain %q\ngot: %s", tt.notWant, got)
			}
		})
	}
}

func TestConvertHTMLLayoutTable(t *testing.T) {
	html := `<table><tr><td>Cell 1</td><td>Cell 2</td></tr></table>`
	got, err := convertHTML(html)
	if err != nil {
		t.Fatalf("convertHTML: %v", err)
	}
	if strings.Contains(got, "|") {
		t.Errorf("layout table should be flattened, got pipe table:\n%s", got)
	}
	if !strings.Contains(got, "Cell 1") || !strings.Contains(got, "Cell 2") {
		t.Errorf("cell content should be preserved:\n%s", got)
	}
}

func TestConvertHTMLDataTable(t *testing.T) {
	html := `<table>
        <thead><tr><th>Name</th><th>Age</th></tr></thead>
        <tbody><tr><td>Alice</td><td>30</td></tr></tbody>
    </table>`
	got, err := convertHTML(html)
	if err != nil {
		t.Fatalf("convertHTML: %v", err)
	}
	if !strings.Contains(got, "|") {
		t.Errorf("data table should be a pipe table:\n%s", got)
	}
	if !strings.Contains(got, "Name") || !strings.Contains(got, "Alice") {
		t.Errorf("table content should be preserved:\n%s", got)
	}
}
