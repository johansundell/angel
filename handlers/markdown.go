package handlers

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

// noteRenderer renders Daily Notes. Raw HTML and javascript: links are left
// out (goldmark's safe default), so even a note written with a leaked Master
// PIN cannot run scripts on caregivers' phones. Hard wraps keep each line of
// a note on its own line, as notes written before Markdown expect.
var noteRenderer = goldmark.New(goldmark.WithRendererOptions(html.WithHardWraps()))

// renderMarkdown converts a note's Markdown source to HTML for caregivers.
func renderMarkdown(src string) (template.HTML, error) {
	var buf bytes.Buffer
	if err := noteRenderer.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
