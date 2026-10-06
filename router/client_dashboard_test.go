package router_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/johansundell/angel/types"
)

// clientSession logs in with the Master PIN and returns the session cookie.
func (a *pinApp) clientSession() *http.Cookie {
	a.t.Helper()
	c := sessionCookie(a.t, a.submitPIN(testMasterPIN, "10.0.0.1:1111"))
	if c == nil {
		a.t.Fatal("expected a session cookie")
	}
	return c
}

// postForm posts form to path with the given cookies.
func (a *pinApp) postForm(path string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return a.do(req)
}

func (a *pinApp) note(d types.Day) (types.DailyNote, bool) {
	a.t.Helper()
	n, ok, err := a.store.GetDailyNote(context.Background(), d)
	if err != nil {
		a.t.Fatalf("GetDailyNote: %v", err)
	}
	return n, ok
}

func TestClientDashboard_ShowsEditorWithTodaysNote(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Ge medicin <klockan> 10.", Important: true})

	w := app.get("/admin", app.clientSession())

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`action="/admin/note"`, `method="post"`, "<textarea", `name="text"`,
		"Ge medicin &lt;klockan&gt; 10.", // escaped inside the textarea
		`name="important"`, "Viktigt", "checked",
		`action="/admin/note/clear"`, "Rensa",
		"måndag 5 oktober",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard missing %q", want)
		}
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestClientDashboard_EmptyEditorWithoutNote(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-04"), Text: "Gårdagens anteckning", Important: true})

	body := app.get("/admin", app.clientSession()).Body.String()

	if strings.Contains(body, "Gårdagens anteckning") {
		t.Error("yesterday's note shown in today's editor")
	}
	if strings.Contains(body, "checked") {
		t.Error("Important Flag checked without a note")
	}
	if strings.Contains(body, `action="/admin/note/clear"`) {
		t.Error("clear offered without a note")
	}
}

func TestClientDashboard_SaveUpdatesCaregiverView(t *testing.T) {
	app := newPinApp(t)
	client := app.clientSession()

	w := app.postForm("/admin/note", url.Values{"date": {"2026-10-05"}, "text": {"  Ge medicin klockan 10.\r\nVattna blommorna.  "}, "important": {"on"}}, client)

	assertRedirect(t, w, "/admin")
	n, ok := app.note(day("2026-10-05"))
	if !ok {
		t.Fatal("note not saved")
	}
	if n.Text != "Ge medicin klockan 10.\nVattna blommorna." || !n.Important {
		t.Errorf("saved %+v", n)
	}

	body := app.get("/note", app.caregiverSession()).Body.String()
	for _, want := range []string{"Ge medicin klockan 10.", "Vattna blommorna.", `class="note note-important"`} {
		if !strings.Contains(body, want) {
			t.Errorf("caregiver view missing %q", want)
		}
	}

	// Editing again replaces the text and can drop the Important Flag.
	assertRedirect(t, app.postForm("/admin/note", url.Values{"date": {"2026-10-05"}, "text": {"Allt som vanligt, men handla mjölk."}}, client), "/admin")
	n, _ = app.note(day("2026-10-05"))
	if n.Text != "Allt som vanligt, men handla mjölk." || n.Important {
		t.Errorf("after edit saved %+v", n)
	}
	if body := app.get("/admin", client).Body.String(); !strings.Contains(body, "handla mjölk") || strings.Contains(body, "checked") {
		t.Errorf("dashboard does not reflect the edit: %q", body)
	}
}

func TestClientDashboard_SavesForStockholmDate(t *testing.T) {
	app := newPinApp(t)
	// 22:30 UTC is already 00:30 on Tuesday in Stockholm (CEST, UTC+2).
	app.clock.now = time.Date(2026, 10, 5, 22, 30, 0, 0, time.UTC)

	app.postForm("/admin/note", url.Values{"date": {"2026-10-06"}, "text": {"Tisdagens anteckning"}}, app.clientSession())

	if _, ok := app.note(day("2026-10-06")); !ok {
		t.Error("note not saved for the local date")
	}
	if _, ok := app.note(day("2026-10-05")); ok {
		t.Error("note saved for the UTC date")
	}
}

func TestClientDashboard_ClearRestoresEmptyState(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Ring sjuksköterskan.", Important: true})
	app.saveNote(types.DailyNote{Date: day("2026-10-06"), Text: "Morgondagens anteckning"})

	assertRedirect(t, app.postForm("/admin/note/clear", url.Values{"date": {"2026-10-05"}}, app.clientSession()), "/admin")

	if _, ok := app.note(day("2026-10-05")); ok {
		t.Error("today's note still stored after clearing")
	}
	if _, ok := app.note(day("2026-10-06")); !ok {
		t.Error("clearing today removed tomorrow's note")
	}
	body := app.get("/note", app.caregiverSession()).Body.String()
	if !strings.Contains(body, "Inga särskilda instruktioner idag. Allt är som vanligt!") || strings.Contains(body, "Ring sjuksköterskan") {
		t.Errorf("caregiver view not back to the empty state: %q", body)
	}
}

func TestClientDashboard_SavingBlankTextClears(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Ring sjuksköterskan."})

	assertRedirect(t, app.postForm("/admin/note", url.Values{"date": {"2026-10-05"}, "text": {" \r\n "}, "important": {"on"}}, app.clientSession()), "/admin")

	if _, ok := app.note(day("2026-10-05")); ok {
		t.Error("blank note stored instead of clearing")
	}
}

func TestClientDashboard_RejectsOverlongNote(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Befintlig anteckning"})

	w := app.postForm("/admin/note", url.Values{"date": {"2026-10-05"}, "text": {strings.Repeat("å", 5001)}}, app.clientSession())

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if n, _ := app.note(day("2026-10-05")); n.Text != "Befintlig anteckning" {
		t.Error("overlong note replaced the existing one")
	}
}

func TestClientDashboard_RequiresClientSession(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Privat anteckning"})
	expired := app.clientSession()
	app.clock.Advance(testClientSessionTTL + time.Second)
	// Taken after the clock moved, so it is a live caregiver session.
	caregiver := app.caregiverSession()

	for name, cookies := range map[string][]*http.Cookie{
		"no session":     nil,
		"caregiver":      {caregiver},
		"expired client": {expired},
	} {
		t.Run(name, func(t *testing.T) {
			w := app.get("/admin", cookies...)
			assertRedirect(t, w, "/")
			if strings.Contains(w.Body.String(), "Privat anteckning") {
				t.Error("note leaked")
			}
			assertRedirect(t, app.postForm("/admin/note", url.Values{"text": {"Kapad"}}, cookies...), "/")
			assertRedirect(t, app.postForm("/admin/note/clear", nil, cookies...), "/")
			if n, ok := app.note(day("2026-10-05")); !ok || n.Text != "Privat anteckning" {
				t.Errorf("note modified: %+v, %v", n, ok)
			}
		})
	}
}

func TestClientDashboard_HasSessionExpiredGuard(t *testing.T) {
	app := newPinApp(t)

	body := app.get("/admin", app.clientSession()).Body.String()

	// The banner is in the page from the start, hidden, so the script only
	// has to reveal it; it tells the Client to copy unsaved text first.
	for _, want := range []string{
		`id="session-expired"`, `role="alert"`, "hidden",
		"Sessionen har gått ut – kopiera din text innan du loggar in igen",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard missing %q", want)
		}
	}
}
