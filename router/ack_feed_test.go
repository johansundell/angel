package router_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

const emptyFeed = "Inga kvitteringar registrerade idag än."

// acknowledgeAt records an acknowledgement by name at the given UTC time.
func (a *pinApp) acknowledgeAt(name string, at time.Time) {
	a.t.Helper()
	a.clock.now = at
	a.acknowledge(name, a.caregiverSession())
}

// assertInOrder fails unless each of want appears in body, in that order.
func assertInOrder(t *testing.T, body string, want ...string) {
	t.Helper()
	pos := 0
	for _, w := range want {
		i := strings.Index(body[pos:], w)
		if i < 0 {
			t.Fatalf("missing %q (in order %q) in %q", w, want, body)
		}
		pos += i + len(w)
	}
}

func TestAckFeed_DashboardListsTodaysAcknowledgementsNewestFirst(t *testing.T) {
	app := newPinApp(t)
	// Stockholm is UTC+2 in October.
	app.acknowledgeAt("Maria", time.Date(2026, 10, 5, 6, 35, 0, 0, time.UTC))
	app.acknowledgeAt("", time.Date(2026, 10, 5, 10, 5, 0, 0, time.UTC))
	app.acknowledgeAt("Ahmed", time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC))

	w := app.get("/admin", app.clientSession())

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	assertInOrder(t, body, "Kvitteringar idag", "Ahmed kl 17:00", "Okänd ängel kl 12:05", "Maria kl 08:35")
	if strings.Contains(body, emptyFeed) {
		t.Error("empty state shown alongside acknowledgements")
	}
}

func TestAckFeed_EmptyState(t *testing.T) {
	app := newPinApp(t)

	body := app.get("/admin", app.clientSession()).Body.String()

	if !strings.Contains(body, emptyFeed) {
		t.Errorf("missing empty state, got %q", body)
	}
}

func TestAckFeed_StartsFreshAfterRollover(t *testing.T) {
	app := newPinApp(t)
	// 21:55 UTC is 23:55 on Monday in Stockholm.
	app.acknowledgeAt("Elin", time.Date(2026, 10, 5, 21, 55, 0, 0, time.UTC))
	// 22:05 UTC is 00:05 on Tuesday.
	app.clock.now = time.Date(2026, 10, 5, 22, 5, 0, 0, time.UTC)

	body := app.get("/admin", app.clientSession()).Body.String()

	if strings.Contains(body, "Elin") {
		t.Error("yesterday's acknowledgement in today's feed")
	}
	if !strings.Contains(body, emptyFeed) {
		t.Errorf("missing empty state after rollover, got %q", body)
	}

	app.acknowledgeAt("Omar", time.Date(2026, 10, 5, 22, 10, 0, 0, time.UTC))
	body = app.get("/admin", app.clientSession()).Body.String()
	if !strings.Contains(body, "Omar kl 00:10") || strings.Contains(body, "Elin") {
		t.Errorf("feed after rollover = %q", body)
	}
}

func TestAckFeed_NameIsEscaped(t *testing.T) {
	app := newPinApp(t)
	app.acknowledgeAt("<b>Eva</b>", time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC))

	body := app.get("/admin", app.clientSession()).Body.String()

	if !strings.Contains(body, "&lt;b&gt;Eva&lt;/b&gt; kl 10:00") || strings.Contains(body, "<b>Eva") {
		t.Errorf("name not escaped: %q", body)
	}
}

func TestAckFeed_LiveFragment(t *testing.T) {
	app := newPinApp(t)
	client := app.clientSession()

	w := app.get("/admin/acks", client)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), emptyFeed) {
		t.Errorf("fragment missing empty state: %q", w.Body.String())
	}

	app.acknowledgeAt("Maria", time.Date(2026, 10, 5, 6, 35, 0, 0, time.UTC))
	w = app.get("/admin/acks", client)
	body := w.Body.String()
	if !strings.Contains(body, "Maria kl 08:35") || strings.Contains(body, emptyFeed) {
		t.Errorf("fragment = %q", body)
	}
	// Only the feed, not a whole page with the editors.
	if strings.Contains(body, "<html") || strings.Contains(body, "<textarea") {
		t.Errorf("fragment is a whole page: %q", body)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestAckFeed_RequiresClientSession(t *testing.T) {
	app := newPinApp(t)
	app.acknowledgeAt("Maria", time.Date(2026, 10, 5, 6, 35, 0, 0, time.UTC))

	for name, cookies := range map[string][]*http.Cookie{
		"no session": nil,
		"caregiver":  {app.caregiverSession()},
	} {
		t.Run(name, func(t *testing.T) {
			w := app.get("/admin/acks", cookies...)
			assertRedirect(t, w, "/")
			if strings.Contains(w.Body.String(), "Maria") {
				t.Error("acknowledgement leaked")
			}
		})
	}
}
