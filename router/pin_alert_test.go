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
	app.wrongPINs(auth.DefaultAlertFailures - 1)
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
