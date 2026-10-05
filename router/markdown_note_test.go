package router_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/johansundell/angel/types"
)

func TestMarkdownNote_RenderedForCaregivers(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: "2026-10-05", Text: "## Medicin\n\n**Viktigt:** ge *två* tabletter.\n\n- Morgon\n- Kväll\n\n[Apoteket](https://example.com/apotek)"})

	body := app.get("/note", app.caregiverSession()).Body.String()

	for _, want := range []string{
		"<h2>Medicin</h2>",
		"<strong>Viktigt:</strong>",
		"<em>två</em>",
		"<li>Morgon</li>",
		`<a href="https://example.com/apotek">Apoteket</a>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("note view missing %q", want)
		}
	}
	if strings.Contains(body, "**Viktigt:**") {
		t.Error("Markdown source shown instead of HTML")
	}
}

func TestMarkdownNote_SingleLineBreaksAreKept(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: "2026-10-05", Text: "Ge medicin klockan 10.\nVattna blommorna."})

	body := app.get("/note", app.caregiverSession()).Body.String()

	if !strings.Contains(body, "Ge medicin klockan 10.<br") {
		t.Errorf("single line break not kept, got %q", body)
	}
}

func TestMarkdownNote_RawHTMLAndScriptLinksAreNotRendered(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: "2026-10-05", Text: "<b onclick=\"alert(1)\">Hej</b>\n\n<script>alert(2)</script>\n\n[Klicka](javascript:alert(3))"})

	body := app.get("/note", app.caregiverSession()).Body.String()

	for _, unwanted := range []string{"<script>alert(2)", "<b onclick", "javascript:alert(3)"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("note view contains %q", unwanted)
		}
	}
}

func TestMarkdownNote_ImportantFlagWrapsRenderedNote(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: "2026-10-05", Text: "Ring **sjuksköterskan**.", Important: true})

	body := app.get("/note", app.caregiverSession()).Body.String()

	assertInOrder(t, body, `class="note note-important"`, "Viktigt", "<strong>sjuksköterskan</strong>")
}

func TestMarkdownNote_EditorShowsSource(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: "2026-10-05", Text: "## Medicin\n**Två** tabletter."})
	app.saveNote(types.DailyNote{Date: "2026-10-06", Text: "- Handla *mjölk*"})

	body := app.get("/admin", app.clientSession()).Body.String()

	for _, want := range []string{"## Medicin\n**Två** tabletter.", "- Handla *mjölk*"} {
		if !strings.Contains(body, want) {
			t.Errorf("editor missing Markdown source %q", want)
		}
	}
	if strings.Contains(body, "<strong>Två</strong>") || strings.Contains(body, "<h2>Medicin</h2>") {
		t.Error("editor shows rendered HTML instead of the source")
	}
	for _, id := range []string{`id="note-text-hint"`, `id="advance-text-hint"`} {
		assertInOrder(t, body, id, "Du kan använda Markdown")
	}
}

func TestMarkdownNote_SavedFromEditorAndRendered(t *testing.T) {
	app := newPinApp(t)
	client := app.clientSession()
	src := "## Medicin\r\n**Två** tabletter.\r\n- Morgon\r\n- Kväll"

	assertRedirect(t, app.postForm("/admin/note", url.Values{"date": {"2026-10-05"}, "text": {src}}, client), "/admin")

	n, _ := app.note("2026-10-05")
	if want := "## Medicin\n**Två** tabletter.\n- Morgon\n- Kväll"; n.Text != want {
		t.Errorf("saved %q, want the Markdown source %q", n.Text, want)
	}
	if body := app.get("/admin", client).Body.String(); !strings.Contains(body, "## Medicin\n**Två** tabletter.") {
		t.Errorf("editor does not show the saved source: %q", body)
	}
	body := app.get("/note", app.caregiverSession()).Body.String()
	assertInOrder(t, body, "<h2>Medicin</h2>", "<strong>Två</strong> tabletter.", "<li>Morgon</li>", "<li>Kväll</li>")
}
