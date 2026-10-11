package handlers

import (
	"bytes"
	"html/template"

	"github.com/johansundell/angel/types"
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

// noteCard is what the note-card template shows: a Daily Note rendered for
// caregivers, or the empty state when Note is nil.
type noteCard struct {
	Note    *types.DailyNote
	HTML    template.HTML
	Preview bool // shown on the Client Dashboard rather than to caregivers
}

// newNoteCard renders note, which may be nil, for the note-card template.
func newNoteCard(note *types.DailyNote) (noteCard, error) {
	card := noteCard{Note: note}
	if note == nil {
		return card, nil
	}
	rendered, err := renderMarkdown(note.Text)
	if err != nil {
		return card, err
	}
	card.HTML = rendered
	return card, nil
}
