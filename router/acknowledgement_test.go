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

// acknowledge posts the Kvittera form with the given caregiver name.
func (a *pinApp) acknowledge(name string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	a.t.Helper()
	form := url.Values{"name": {name}}
	req := httptest.NewRequest(http.MethodPost, "/note/ack", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return a.do(req)
}

// follow GETs the redirect target of w as a browser would: with the given
// cookies plus any cookies w set.
func (a *pinApp) follow(w *httptest.ResponseRecorder, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	a.t.Helper()
	assertRedirect(a.t, w, "/note")
	return a.get("/note", append(cookies, w.Result().Cookies()...)...)
}

// ackCookie returns the one-time confirmation cookie set by w.
func ackCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == "angel_ack" {
			return c
		}
	}
	t.Fatal("expected an angel_ack cookie")
	return nil
}

func (a *pinApp) acknowledgements(date string) []types.Acknowledgement {
	a.t.Helper()
	acks, err := a.store.ListAcknowledgements(context.Background(), date)
	if err != nil {
		a.t.Fatalf("ListAcknowledgements: %v", err)
	}
	return acks
}

func TestAcknowledgement_NoteViewHasKvitteraForm(t *testing.T) {
	app := newPinApp(t)

	body := app.get("/note", app.caregiverSession()).Body.String()

	for _, want := range []string{`action="/note/ack"`, `method="post"`, `name="name"`, "Kvittera", "Förnamn"} {
		if !strings.Contains(body, want) {
			t.Errorf("note view missing %q", want)
		}
	}
	if strings.Contains(body, "Kvitterat") {
		t.Error("confirmation shown before acknowledging")
	}
}

func TestAcknowledgement_RecordsAndConfirmsWithName(t *testing.T) {
	app := newPinApp(t)
	// 06:35 UTC is 08:35 in Stockholm (CEST, UTC+2).
	app.clock.now = time.Date(2026, 10, 5, 6, 35, 12, 0, time.UTC)
	c := app.caregiverSession()

	w := app.follow(app.acknowledge("  Maria  ", c), c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "Kvitterat av Maria kl 08:35") {
		t.Errorf("missing confirmation, got %q", body)
	}
	acks := app.acknowledgements("2026-10-05")
	if len(acks) != 1 {
		t.Fatalf("got %d acknowledgements, want 1", len(acks))
	}
	if acks[0].Name != "Maria" {
		t.Errorf("Name = %q, want %q", acks[0].Name, "Maria")
	}
	if !acks[0].CreatedAt.Equal(app.clock.now) {
		t.Errorf("CreatedAt = %v, want %v", acks[0].CreatedAt, app.clock.now)
	}
}

func TestAcknowledgement_AnonymousWhenNameBlank(t *testing.T) {
	app := newPinApp(t)
	app.clock.now = time.Date(2026, 10, 5, 12, 5, 0, 0, time.UTC)
	c := app.caregiverSession()

	body := app.follow(app.acknowledge("   ", c), c).Body.String()

	if !strings.Contains(body, "Kvitterat kl 14:05") {
		t.Errorf("missing anonymous confirmation, got %q", body)
	}
	acks := app.acknowledgements("2026-10-05")
	if len(acks) != 1 || acks[0].Name != "" {
		t.Fatalf("acknowledgements = %+v, want one anonymous", acks)
	}
}

func TestAcknowledgement_UsesStockholmCalendarDate(t *testing.T) {
	app := newPinApp(t)
	// 22:30 UTC on Monday is 00:30 on Tuesday in Stockholm.
	app.clock.now = time.Date(2026, 10, 5, 22, 30, 0, 0, time.UTC)
	c := app.caregiverSession()

	app.acknowledge("Nattpersonal", c)

	if acks := app.acknowledgements("2026-10-06"); len(acks) != 1 {
		t.Errorf("got %d acknowledgements on 2026-10-06, want 1", len(acks))
	}
	if acks := app.acknowledgements("2026-10-05"); len(acks) != 0 {
		t.Errorf("got %d acknowledgements on 2026-10-05, want 0", len(acks))
	}
}

func TestAcknowledgement_MultipleVisitsSameDay(t *testing.T) {
	app := newPinApp(t)
	app.clock.now = time.Date(2026, 10, 5, 6, 0, 0, 0, time.UTC)
	morning := app.caregiverSession()
	app.acknowledge("Maria", morning)

	app.clock.now = time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
	evening := app.caregiverSession()
	body := app.follow(app.acknowledge("Ahmed", evening), evening).Body.String()

	if !strings.Contains(body, "Kvitterat av Ahmed kl 17:00") {
		t.Errorf("missing second confirmation, got %q", body)
	}
	if strings.Contains(body, "Kvitterat av Maria") {
		t.Error("second caregiver shown the first caregiver's confirmation")
	}
	acks := app.acknowledgements("2026-10-05")
	if len(acks) != 2 || acks[0].Name != "Maria" || acks[1].Name != "Ahmed" {
		t.Fatalf("acknowledgements = %+v, want Maria then Ahmed", acks)
	}
}

func TestAcknowledgement_NameIsEscapedAndCapped(t *testing.T) {
	app := newPinApp(t)
	c := app.caregiverSession()

	body := app.follow(app.acknowledge("<b>"+strings.Repeat("å", 100), c), c).Body.String()

	if strings.Contains(body, "<b>") {
		t.Error("caregiver name rendered unescaped")
	}
	acks := app.acknowledgements("2026-10-05")
	if len(acks) != 1 {
		t.Fatalf("got %d acknowledgements, want 1", len(acks))
	}
	if n := len([]rune(acks[0].Name)); n != 40 {
		t.Errorf("stored name has %d characters, want 40", n)
	}
}

func TestAcknowledgement_ConfirmationIsShownOnce(t *testing.T) {
	app := newPinApp(t)
	c := app.caregiverSession()

	w := app.acknowledge("Maria", c)
	flash := ackCookie(t, w)
	if !flash.HttpOnly || !flash.Secure || flash.Path != "/note" {
		t.Errorf("angel_ack cookie = %+v, want HttpOnly, Secure, Path=/note", flash)
	}
	first := app.get("/note", c, flash)
	if !strings.Contains(first.Body.String(), "Kvitterat av Maria") {
		t.Fatal("missing confirmation right after acknowledging")
	}
	cleared := ackCookie(t, first)
	if cleared.MaxAge >= 0 {
		t.Errorf("confirmation cookie not cleared: MaxAge = %d", cleared.MaxAge)
	}

	// A caregiver picking up the same phone later must not be told the note
	// is already acknowledged.
	if strings.Contains(app.get("/note", c).Body.String(), "Kvitterat") {
		t.Error("confirmation shown again on the next visit")
	}
	for _, path := range []string{"/note?kvitterat=1", "/note?ack=1"} {
		if strings.Contains(app.get(path, c).Body.String(), "Kvitterat") {
			t.Errorf("GET %s: confirmation shown without the cookie", path)
		}
	}
}

func TestAcknowledgement_ConfirmationIsOnlyForTodaysAcknowledgement(t *testing.T) {
	app := newPinApp(t)
	app.clock.now = time.Date(2026, 10, 5, 6, 0, 0, 0, time.UTC)
	c := app.caregiverSession()
	flash := ackCookie(t, app.acknowledge("Maria", c))

	// The next morning a leftover cookie must not claim today is acknowledged.
	app.clock.now = time.Date(2026, 10, 6, 6, 0, 0, 0, time.UTC)
	c = app.caregiverSession()
	for _, value := range []string{flash.Value, "999", "abc"} {
		w := app.get("/note", c, &http.Cookie{Name: "angel_ack", Value: value})
		if w.Code != http.StatusOK {
			t.Fatalf("angel_ack=%s: status = %d, want 200", value, w.Code)
		}
		if strings.Contains(w.Body.String(), "Kvitterat") {
			t.Errorf("angel_ack=%s: confirmation shown for an acknowledgement that is not today's", value)
		}
	}
}

func TestAcknowledgement_RequiresCaregiverSession(t *testing.T) {
	app := newPinApp(t)

	assertRedirect(t, app.acknowledge("Inkräktare"), "/")

	client := sessionCookie(t, app.submitPIN(testMasterPIN, "10.0.0.2:2222"))
	if client == nil {
		t.Fatal("expected a client session cookie")
	}
	assertRedirect(t, app.acknowledge("Klienten", client), "/")

	c := app.caregiverSession()
	app.clock.Advance(testSessionTTL + time.Second)
	assertRedirect(t, app.acknowledge("Utgången", c), "/")

	if acks := app.acknowledgements("2026-10-05"); len(acks) != 0 {
		t.Errorf("got %d acknowledgements without a caregiver session, want 0", len(acks))
	}
}
