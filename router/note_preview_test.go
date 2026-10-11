package router_test

import (
	"strings"
	"testing"

	"github.com/johansundell/angel/types"
)

const (
	todayCaption   = "Så här ser vårdpersonalen lappen:"
	advanceCaption = "Så här visas lappen från midnatt:"
	emptyStateText = "Inga särskilda instruktioner idag. Allt är som vanligt!"
)

// section returns the part of body from the element with id onwards, up to
// the next one of the given ids, so assertions stay within one section.
func section(t *testing.T, body, from, to string) string {
	t.Helper()
	i := strings.Index(body, from)
	if i < 0 {
		t.Fatalf("missing %q in %q", from, body)
	}
	s := body[i:]
	if to != "" {
		if j := strings.Index(s, to); j >= 0 {
			s = s[:j]
		}
	}
	return s
}

func TestNotePreview_TodayRendersNoteAboveEditor(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "## Medicin\n\nGe **två** tabletter."})

	body := app.get("/admin", app.clientSession()).Body.String()
	today := section(t, body, `id="today-heading"`, `id="ack-feed-heading"`)

	assertInOrder(t, today, todayCaption, `class="note"`, `class="note-text note-markdown"`,
		"<h2>Medicin</h2>", "<strong>två</strong>", `<form method="post" action="/admin/note"`)
	if strings.Contains(today, "note-important") {
		t.Error("plain note shown as important")
	}
}

func TestNotePreview_TodayImportantNote(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Ring *sjuksköterskan*.", Important: true})

	body := app.get("/admin", app.clientSession()).Body.String()
	today := section(t, body, `id="today-heading"`, `id="ack-feed-heading"`)

	assertInOrder(t, today, todayCaption, `class="note note-important"`, `class="note-flag">Viktigt</p>`,
		"<em>sjuksköterskan</em>", `action="/admin/note"`)
}

func TestNotePreview_TodayEmptyState(t *testing.T) {
	app := newPinApp(t)

	body := app.get("/admin", app.clientSession()).Body.String()
	today := section(t, body, `id="today-heading"`, `id="ack-feed-heading"`)

	assertInOrder(t, today, todayCaption, `class="note note-empty"`, emptyStateText, `action="/admin/note"`)
}

func TestNotePreview_AdvanceNoteRendered(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-06"), Text: "- Handla *mjölk*", Important: true})

	body := app.get("/admin", app.clientSession()).Body.String()
	advance := section(t, body, `id="advance-heading"`, "</main>")

	assertInOrder(t, advance, advanceCaption, `class="note note-important"`, "Viktigt",
		"<li>Handla <em>mjölk</em></li>", `action="/admin/advance"`)
	// Today has no note, so only today shows the empty state.
	if strings.Contains(advance, "note-empty") {
		t.Error("advance section shows the empty state")
	}
}

func TestNotePreview_AdvanceOmittedWithoutNote(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Dagens anteckning"})

	body := app.get("/admin", app.clientSession()).Body.String()
	advance := section(t, body, `id="advance-heading"`, "</main>")

	for _, unwanted := range []string{advanceCaption, `class="note"`, `class="note note-`, emptyStateText} {
		if strings.Contains(advance, unwanted) {
			t.Errorf("advance section contains %q without an advance note", unwanted)
		}
	}
}

func TestNotePreview_NotPolled(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Dagens anteckning"})

	body := app.get("/admin/acks", app.clientSession()).Body.String()

	if strings.Contains(body, "Dagens anteckning") || strings.Contains(body, `class="note"`) {
		t.Errorf("live feed includes the note preview: %q", body)
	}
}
