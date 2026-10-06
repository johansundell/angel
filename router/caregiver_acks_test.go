package router_test

import (
	"strings"
	"testing"
	"time"

	"github.com/johansundell/angel/handlers"
)

const emptyCaregiverAcks = "Ingen har kvitterat idag än."

func TestCaregiverAcks_ListsTimesNewestFirstAboveForm(t *testing.T) {
	app := newPinApp(t)
	// Stockholm is UTC+2 in October.
	app.acknowledgeAt("Maria", time.Date(2026, 10, 5, 6, 35, 0, 0, time.UTC))
	app.acknowledgeAt("", time.Date(2026, 10, 5, 10, 5, 0, 0, time.UTC))
	app.acknowledgeAt("Ahmed", time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC))

	body := app.get("/note", app.caregiverSession()).Body.String()

	assertInOrder(t, body, "Kvitteringar idag",
		"Kvitterat kl 17:00", "Kvitterat kl 12:05", "Kvitterat kl 08:35",
		`action="/note/ack"`)
	if strings.Contains(body, emptyCaregiverAcks) {
		t.Error("empty state shown alongside acknowledgements")
	}
}

func TestCaregiverAcks_NeverShowsNames(t *testing.T) {
	app := newPinApp(t)
	app.acknowledgeAt("Maria", time.Date(2026, 10, 5, 6, 35, 0, 0, time.UTC))

	body := app.get("/note", app.caregiverSession()).Body.String()

	if strings.Contains(body, "Maria") {
		t.Errorf("caregiver view shows another caregiver's name: %q", body)
	}
}

func TestCaregiverAcks_EmptyState(t *testing.T) {
	app := newPinApp(t)

	body := app.get("/note", app.caregiverSession()).Body.String()

	assertInOrder(t, body, "Kvitteringar idag", emptyCaregiverAcks, `action="/note/ack"`)
}

func TestCaregiverAcks_OwnAcknowledgementListedWithConfirmation(t *testing.T) {
	app := newPinApp(t)
	app.clock.now = time.Date(2026, 10, 5, 6, 35, 0, 0, time.UTC)
	session := app.caregiverSession()

	body := app.follow(app.acknowledge("Maria", session), session).Body.String()

	// The one-time confirmation keeps the name; the list doesn't.
	if !strings.Contains(body, "✓ Kvitterat av Maria kl 08:35") {
		t.Errorf("missing confirmation in %q", body)
	}
	assertInOrder(t, body, "Kvitteringar idag", "Kvitterat kl 08:35")
	if strings.Contains(body, emptyCaregiverAcks) {
		t.Error("empty state shown after acknowledging")
	}
}

func TestCaregiverAcks_StartsFreshAfterRollover(t *testing.T) {
	app := newPinApp(t)
	// 21:55 UTC is 23:55 on Monday in Stockholm.
	app.acknowledgeAt("", time.Date(2026, 10, 5, 21, 55, 0, 0, time.UTC))
	// 22:05 UTC is 00:05 on Tuesday.
	app.clock.now = time.Date(2026, 10, 5, 22, 5, 0, 0, time.UTC)

	body := app.get("/note", app.caregiverSession()).Body.String()

	if strings.Contains(body, "Kvitterat kl 23:55") {
		t.Error("yesterday's acknowledgement in today's list")
	}
	if !strings.Contains(body, emptyCaregiverAcks) {
		t.Errorf("missing empty state after rollover, got %q", body)
	}
}

func TestCaregiverAcks_NoPolling(t *testing.T) {
	app := newPinApp(t)

	body := app.get("/note", app.caregiverSession()).Body.String()

	if strings.Contains(body, "<script") {
		t.Error("caregiver view has a script; the list refreshes on reload only")
	}
}

func TestCaregiverAcks_DashboardSaysCaregiversSeeTimes(t *testing.T) {
	app := newPinApp(t)

	body := app.get("/admin", app.clientSession()).Body.String()

	assertInOrder(t, body, "Kvitteringar idag", "Vårdpersonalen ser tiderna.")
}

func TestCaregiverAcks_SharedNamesShowNameAndTime(t *testing.T) {
	app := newPinApp(t, handlers.WithSharedCaregiverNames(true))
	app.acknowledgeAt("Maria", time.Date(2026, 10, 5, 6, 35, 0, 0, time.UTC))
	app.acknowledgeAt("", time.Date(2026, 10, 5, 10, 5, 0, 0, time.UTC))
	app.acknowledgeAt("Ahmed", time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC))

	body := app.get("/note", app.caregiverSession()).Body.String()

	// Anonymous entries keep the times-only wording.
	assertInOrder(t, body, "Kvitteringar idag",
		"Ahmed kl 17:00", "Kvitterat kl 12:05", "Maria kl 08:35",
		`action="/note/ack"`)
}

func TestCaregiverAcks_SharedNamesAreEscaped(t *testing.T) {
	app := newPinApp(t, handlers.WithSharedCaregiverNames(true))
	app.acknowledgeAt("<b>Eva</b>", time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC))

	body := app.get("/note", app.caregiverSession()).Body.String()

	if !strings.Contains(body, "&lt;b&gt;Eva&lt;/b&gt; kl 10:00") || strings.Contains(body, "<b>Eva") {
		t.Errorf("name not escaped: %q", body)
	}
}

func TestCaregiverAcks_NamesOffByDefault(t *testing.T) {
	app := newPinApp(t, handlers.WithSharedCaregiverNames(false))
	app.acknowledgeAt("Maria", time.Date(2026, 10, 5, 6, 35, 0, 0, time.UTC))

	body := app.get("/note", app.caregiverSession()).Body.String()

	if strings.Contains(body, "Maria") || !strings.Contains(body, "Kvitterat kl 08:35") {
		t.Errorf("names off: want only the time, got %q", body)
	}
}

func TestCaregiverAcks_DashboardSaysCaregiversSeeNames(t *testing.T) {
	app := newPinApp(t, handlers.WithSharedCaregiverNames(true))

	body := app.get("/admin", app.clientSession()).Body.String()

	assertInOrder(t, body, "Kvitteringar idag", "Vårdpersonalen ser namn och tider.")
	if strings.Contains(body, "Vårdpersonalen ser tiderna.") {
		t.Error("dashboard says caregivers see only the times")
	}
}
