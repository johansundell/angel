package router_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/johansundell/angel/auth"
)

const pinAlertText = "Många felaktiga PIN-försök – byt Caregiver PIN"

// wrongPINs enters n wrong PINs, each from its own address so none is rate
// limited, one minute apart.
func (a *pinApp) wrongPINs(n int) {
	a.t.Helper()
	for i := 0; i < n; i++ {
		if w := a.submitPIN("0000", fmt.Sprintf("198.51.100.%d:1111", i)); w.Code != http.StatusUnauthorized {
			a.t.Fatalf("wrong PIN status = %d, want 401", w.Code)
		}
		a.clock.Advance(time.Minute)
	}
}

func TestPINAlert_ShownToClientOnDashboardAndPolledFeed(t *testing.T) {
	app := newPinApp(t)
	// Log in first: the Master PIN is checked too, but is not wrong.
	client := app.clientSession()
	caregiver := app.caregiverSession()

	// 07:00 UTC is 09:00 in Stockholm.
	app.clock.now = time.Date(2026, 10, 5, 7, 0, 0, 0, time.UTC)
	app.wrongPINs(auth.AlertFailures - 1)
	for _, path := range []string{"/admin", "/admin/acks"} {
		if body := app.get(path, client).Body.String(); strings.Contains(body, pinAlertText) {
			t.Errorf("%s shows the PIN Alert after 19 wrong PINs", path)
		}
	}

	app.wrongPINs(1)

	for _, path := range []string{"/admin", "/admin/acks"} {
		t.Run(path, func(t *testing.T) {
			w := app.get(path, client)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			assertInOrder(t, w.Body.String(), pinAlertText, "20 felaktiga", "5 oktober kl 09:00", "5 oktober kl 09:19")
		})
	}
	if body := app.get("/admin", client).Body.String(); strings.Index(body, pinAlertText) > strings.Index(body, "Dagens anteckning</h1>") {
		t.Error("PIN Alert is not at the top of the dashboard")
	}

	w := app.get("/note", caregiver)
	if w.Code != http.StatusOK {
		t.Fatalf("caregiver view status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); strings.Contains(body, "PIN-försök") {
		t.Error("Caregiver view shows the PIN Alert")
	}
}

func TestPINAlert_KeepsCountingOnPolledFeedAfterTriggering(t *testing.T) {
	app := newPinApp(t)

	// 07:00 UTC is 09:00 in Stockholm.
	app.clock.now = time.Date(2026, 10, 5, 7, 0, 0, 0, time.UTC)
	app.wrongPINs(auth.AlertFailures)
	// The next day, well outside the window, guessing goes on.
	app.clock.now = time.Date(2026, 10, 6, 13, 0, 0, 0, time.UTC)
	app.wrongPINs(3)
	client := app.clientSession()

	for _, path := range []string{"/admin", "/admin/acks"} {
		t.Run(path, func(t *testing.T) {
			body := app.get(path, client).Body.String()
			assertInOrder(t, body, pinAlertText, `class="pin-alert-body"`, "23 felaktiga", "5 oktober kl 09:00", "6 oktober kl 15:02")
		})
	}
}

func TestPINAlert_AnnouncedOnceByTitleNotOnEveryCount(t *testing.T) {
	app := newPinApp(t)
	client := app.clientSession()
	app.wrongPINs(auth.AlertFailures)

	body := app.get("/admin", client).Body.String()
	// The container stays on the page across polls, so it must not be a live
	// region: only the title, which appears once, is the alert.
	if !strings.Contains(body, `<div id="pin-alert">`) {
		t.Error(`#pin-alert is missing or still has a role`)
	}
	if want := `<p class="pin-alert-title" role="alert">` + pinAlertText; !strings.Contains(body, want) {
		t.Errorf("dashboard missing %q", want)
	}
	if n := strings.Count(app.get("/admin/acks", client).Body.String(), `role="alert"`); n != 1 {
		t.Errorf("polled feed has %d role=\"alert\", want 1 (the title)", n)
	}
	// While the alert shows, polling updates only its body, leaving the
	// title, and so the announcement, alone.
	if !strings.Contains(body, `.pin-alert-body`) {
		t.Error("dashboard script does not update the PIN Alert body on its own")
	}
}
