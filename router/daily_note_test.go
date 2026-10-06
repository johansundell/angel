package router_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/johansundell/angel/types"
)

// caregiverSession logs in with the Caregiver PIN and returns the session cookie.
func (a *pinApp) caregiverSession() *http.Cookie {
	a.t.Helper()
	c := sessionCookie(a.t, a.submitPIN(testCaregiverPIN, "10.0.0.1:1111"))
	if c == nil {
		a.t.Fatal("expected a session cookie")
	}
	return c
}

func day(s string) types.Day {
	d, err := types.ParseDay(s)
	if err != nil {
		panic(err)
	}
	return d
}

func (a *pinApp) saveNote(n types.DailyNote) {
	a.t.Helper()
	if err := a.store.SaveDailyNote(context.Background(), n); err != nil {
		a.t.Fatalf("SaveDailyNote: %v", err)
	}
}

func TestDailyNote_CaregiverSeesTodaysNote(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Ge medicin klockan 10.\nVattna blommorna."})

	w := app.get("/note", app.caregiverSession())

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"Ge medicin klockan 10.", "Vattna blommorna.", "måndag 5 oktober", `name="viewport"`} {
		if !strings.Contains(body, want) {
			t.Errorf("note view missing %q", want)
		}
	}
	if strings.Contains(body, "Allt är som vanligt") {
		t.Error("empty state shown although a note exists")
	}
	if strings.Contains(body, `class="note note-important"`) {
		t.Error("note without the Important Flag rendered as important")
	}
}

func TestDailyNote_ImportantNoteIsHighlighted(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Ring sjuksköterskan om febern stiger.", Important: true})

	w := app.get("/note", app.caregiverSession())

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{`class="note note-important"`, `role="alert"`, "Viktigt", "Ring sjuksköterskan om febern stiger."} {
		if !strings.Contains(body, want) {
			t.Errorf("important note view missing %q", want)
		}
	}
}

func TestDailyNote_EmptyStateWhenNoNoteToday(t *testing.T) {
	app := newPinApp(t)
	// Notes for other days must not show up today.
	app.saveNote(types.DailyNote{Date: day("2026-10-04"), Text: "Gårdagens anteckning"})
	app.saveNote(types.DailyNote{Date: day("2026-10-06"), Text: "Morgondagens anteckning"})

	w := app.get("/note", app.caregiverSession())

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Inga särskilda instruktioner idag. Allt är som vanligt!") {
		t.Error("missing affirmative empty state")
	}
	for _, unwanted := range []string{"Gårdagens anteckning", "Morgondagens anteckning"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("note for another day shown: %q", unwanted)
		}
	}
}

func TestDailyNote_UsesStockholmCalendarDate(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Måndagens anteckning"})
	app.saveNote(types.DailyNote{Date: day("2026-10-06"), Text: "Tisdagens anteckning"})

	// 22:30 UTC is already 00:30 on Tuesday in Stockholm (CEST, UTC+2).
	app.clock.now = time.Date(2026, 10, 5, 22, 30, 0, 0, time.UTC)
	body := app.get("/note", app.caregiverSession()).Body.String()

	if !strings.Contains(body, "Tisdagens anteckning") || !strings.Contains(body, "tisdag 6 oktober") {
		t.Errorf("expected Tuesday's note after local midnight, got %q", body)
	}
	if strings.Contains(body, "Måndagens anteckning") {
		t.Error("yesterday's note shown after local midnight")
	}
}

func TestDailyNote_RequiresCaregiverSession(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "Privat anteckning"})

	w := app.get("/note")
	assertRedirect(t, w, "/")
	if strings.Contains(w.Body.String(), "Privat anteckning") {
		t.Error("note leaked without a session")
	}

	c := app.caregiverSession()
	app.clock.Advance(testCaregiverSessionTTL + time.Second)
	w = app.get("/note", c)
	assertRedirect(t, w, "/")
	if strings.Contains(w.Body.String(), "Privat anteckning") {
		t.Error("note leaked with an expired session")
	}
}

func TestDailyNote_TextIsEscaped(t *testing.T) {
	app := newPinApp(t)
	app.saveNote(types.DailyNote{Date: day("2026-10-05"), Text: "<script>alert(1)</script>"})

	body := app.get("/note", app.caregiverSession()).Body.String()

	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("note text rendered unescaped")
	}
}
