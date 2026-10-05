package router_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/johansundell/angel/types"
)

func TestAdvanceNote_DashboardShowsTomorrowsEditor(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: "2026-10-05", Text: "Dagens anteckning"})
	app.saveNote(types.DailyNote{Date: "2026-10-06", Text: "Morgondagens <anteckning>", Important: true})

	w := app.get("/admin", app.clientSession())

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		"Morgondagens anteckning", "tisdag 6 oktober",
		`action="/admin/advance"`, `action="/admin/advance/clear"`,
		"Morgondagens &lt;anteckning&gt;",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard missing %q", want)
		}
	}
	// Today's editor is still there, with today's note.
	if !strings.Contains(body, "Dagens anteckning") || !strings.Contains(body, `action="/admin/note"`) {
		t.Error("today's editor missing")
	}
}

func TestAdvanceNote_EmptyEditorWithoutAdvanceNote(t *testing.T) {
	app := newPinApp(t)

	body := app.get("/admin", app.clientSession()).Body.String()

	if !strings.Contains(body, `action="/admin/advance"`) {
		t.Error("advance editor missing")
	}
	if strings.Contains(body, `action="/admin/advance/clear"`) {
		t.Error("clear offered without an advance note")
	}
}

func TestAdvanceNote_SaveStoresUnderTomorrow(t *testing.T) {
	app := newPinApp(t)
	client := app.clientSession()

	w := app.postForm("/admin/advance", url.Values{"text": {" Handla mjölk.\r\nRing doktorn. "}, "important": {"on"}}, client)

	assertRedirect(t, w, "/admin")
	n, ok := app.note("2026-10-06")
	if !ok {
		t.Fatal("advance note not saved under tomorrow's date")
	}
	if n.Text != "Handla mjölk.\nRing doktorn." || !n.Important {
		t.Errorf("saved %+v", n)
	}
	if _, ok := app.note("2026-10-05"); ok {
		t.Error("advance note saved under today's date")
	}

	// Editing replaces it and can drop the Important Flag.
	assertRedirect(t, app.postForm("/admin/advance", url.Values{"text": {"Bara mjölk."}}, client), "/admin")
	if n, _ := app.note("2026-10-06"); n.Text != "Bara mjölk." || n.Important {
		t.Errorf("after edit saved %+v", n)
	}
}

func TestAdvanceNote_SavesForStockholmTomorrow(t *testing.T) {
	app := newPinApp(t)
	// 22:30 UTC is already 00:30 on Tuesday in Stockholm, so tomorrow is
	// Wednesday.
	app.clock.now = time.Date(2026, 10, 5, 22, 30, 0, 0, time.UTC)

	app.postForm("/admin/advance", url.Values{"text": {"Onsdagens anteckning"}}, app.clientSession())

	if _, ok := app.note("2026-10-07"); !ok {
		t.Error("advance note not saved for the local tomorrow")
	}
	if _, ok := app.note("2026-10-06"); ok {
		t.Error("advance note saved for the UTC tomorrow")
	}
}

func TestAdvanceNote_HiddenFromCaregiversUntilRollover(t *testing.T) {
	app := newPinApp(t)
	// 23:50 on Monday in Stockholm (CEST, UTC+2).
	app.clock.now = time.Date(2026, 10, 5, 21, 50, 0, 0, time.UTC)
	app.saveNote(types.DailyNote{Date: "2026-10-05", Text: "Måndagens anteckning"})
	assertRedirect(t, app.postForm("/admin/advance", url.Values{"text": {"Tisdagens anteckning"}, "important": {"on"}}, app.clientSession()), "/admin")

	body := app.get("/note", app.caregiverSession()).Body.String()
	if strings.Contains(body, "Tisdagens anteckning") {
		t.Error("caregiver saw tomorrow's advance note before rollover")
	}
	if !strings.Contains(body, "Måndagens anteckning") {
		t.Error("caregiver does not see today's note before rollover")
	}

	// One minute before midnight it is still hidden.
	app.clock.Advance(9 * time.Minute)
	if body := app.get("/note", app.caregiverSession()).Body.String(); strings.Contains(body, "Tisdagens anteckning") {
		t.Error("advance note shown at 23:59")
	}

	// At midnight it becomes the active Daily Note, with no publish step.
	app.clock.Advance(time.Minute)
	body = app.get("/note", app.caregiverSession()).Body.String()
	for _, want := range []string{"Tisdagens anteckning", `class="note note-important"`, "tisdag 6 oktober"} {
		if !strings.Contains(body, want) {
			t.Errorf("caregiver view after rollover missing %q", want)
		}
	}
	if strings.Contains(body, "Måndagens anteckning") {
		t.Error("yesterday's note still shown after rollover")
	}
}

func TestAdvanceNote_BecomesTodaysNoteInDashboardAfterRollover(t *testing.T) {
	app := newPinApp(t)
	app.postForm("/admin/advance", url.Values{"text": {"Tisdagens anteckning"}}, app.clientSession())

	// 00:05 on Tuesday in Stockholm.
	app.clock.now = time.Date(2026, 10, 5, 22, 5, 0, 0, time.UTC)
	body := app.get("/admin", app.clientSession()).Body.String()

	today, tomorrow, found := strings.Cut(body, "Morgondagens anteckning")
	if !found {
		t.Fatal("advance editor missing")
	}
	if !strings.Contains(today, "Tisdagens anteckning") {
		t.Error("rolled-over note not in today's editor")
	}
	if strings.Contains(tomorrow, "Tisdagens anteckning") {
		t.Error("rolled-over note still in tomorrow's editor")
	}
	if !strings.Contains(tomorrow, "onsdag 7 oktober") {
		t.Error("tomorrow's editor not for Wednesday")
	}
}

func TestAdvanceNote_ClearRemovesOnlyTomorrow(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: "2026-10-05", Text: "Dagens anteckning"})
	app.saveNote(types.DailyNote{Date: "2026-10-06", Text: "Morgondagens anteckning"})

	assertRedirect(t, app.postForm("/admin/advance/clear", nil, app.clientSession()), "/admin")

	if _, ok := app.note("2026-10-06"); ok {
		t.Error("advance note still stored after clearing")
	}
	if _, ok := app.note("2026-10-05"); !ok {
		t.Error("clearing tomorrow removed today's note")
	}
}

func TestAdvanceNote_SavingBlankTextClears(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: "2026-10-06", Text: "Morgondagens anteckning"})

	assertRedirect(t, app.postForm("/admin/advance", url.Values{"text": {" \r\n "}}, app.clientSession()), "/admin")

	if _, ok := app.note("2026-10-06"); ok {
		t.Error("blank advance note stored instead of clearing")
	}
}

func TestAdvanceNote_RejectsOverlongNote(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: "2026-10-06", Text: "Befintlig anteckning"})

	w := app.postForm("/admin/advance", url.Values{"text": {strings.Repeat("å", 5001)}}, app.clientSession())

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if n, _ := app.note("2026-10-06"); n.Text != "Befintlig anteckning" {
		t.Error("overlong note replaced the existing one")
	}
}

func TestAdvanceNote_RequiresClientSession(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: "2026-10-06", Text: "Privat anteckning"})

	for name, cookies := range map[string][]*http.Cookie{
		"no session": nil,
		"caregiver":  {app.caregiverSession()},
	} {
		t.Run(name, func(t *testing.T) {
			assertRedirect(t, app.postForm("/admin/advance", url.Values{"text": {"Kapad"}}, cookies...), "/")
			assertRedirect(t, app.postForm("/admin/advance/clear", nil, cookies...), "/")
			if n, ok := app.note("2026-10-06"); !ok || n.Text != "Privat anteckning" {
				t.Errorf("advance note modified: %+v, %v", n, ok)
			}
		})
	}
}
