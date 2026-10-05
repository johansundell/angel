package router_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// logout posts the Logga ut form with the given cookies.
func (a *pinApp) logout(cookies ...*http.Cookie) *httptest.ResponseRecorder {
	a.t.Helper()
	return a.postForm("/logout", nil, cookies...)
}

// assertSessionCleared fails unless w tells the browser to delete the
// session cookie, with the attributes it was set with.
func assertSessionCleared(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	c := sessionCookie(t, w)
	if c == nil {
		t.Fatal("expected the session cookie to be cleared")
	}
	if c.MaxAge >= 0 || c.Value != "" {
		t.Errorf("cookie not deleted: MaxAge = %d, Value = %q", c.MaxAge, c.Value)
	}
	if c.Path != "/" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cleared cookie attributes differ from the session cookie: %+v", c)
	}
}

func TestLogout_EndsSessionForEachRole(t *testing.T) {
	app := newPinApp(t)

	for name, session := range map[string]func() *http.Cookie{
		"caregiver": app.caregiverSession,
		"client":    app.clientSession,
	} {
		t.Run(name, func(t *testing.T) {
			w := app.logout(session())

			assertRedirect(t, w, "/")
			assertSessionCleared(t, w)
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
		})
	}
}

func TestLogout_AllowsLoggingInAgainAsOtherRole(t *testing.T) {
	app := newPinApp(t)
	caregiver := app.caregiverSession()
	// With a session, the entry screen sends the caregiver straight back.
	assertRedirect(t, app.get("/", caregiver), "/note")

	assertRedirect(t, app.logout(caregiver), "/")

	// The browser has dropped the cookie: the entry screen shows again,
	// and the Master PIN opens the dashboard on the same phone.
	assertEntryPage(t, app.get("/"))
	assertRedirect(t, app.submitPIN(testMasterPIN, "10.0.0.1:1111"), "/admin")
}

func TestLogout_WithoutSessionIsHarmless(t *testing.T) {
	app := newPinApp(t)

	w := app.logout()

	assertRedirect(t, w, "/")
	assertSessionCleared(t, w)
}

func TestLogout_RequiresPost(t *testing.T) {
	app := newPinApp(t)

	// A GET, such as a link prefetch, must not log anyone out.
	w := app.get("/logout", app.clientSession())

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
	if sessionCookie(t, w) != nil {
		t.Error("GET /logout touched the session cookie")
	}
}

func TestLogout_ButtonOnCaregiverAndClientViews(t *testing.T) {
	app := newPinApp(t)

	for path, session := range map[string]func() *http.Cookie{
		"/note":  app.caregiverSession,
		"/admin": app.clientSession,
	} {
		t.Run(path, func(t *testing.T) {
			body := app.get(path, session()).Body.String()
			assertInOrder(t, body, `<form method="post" action="/logout"`, "Logga ut", "</form>")
		})
	}

	if body := app.get("/").Body.String(); strings.Contains(body, `action="/logout"`) {
		t.Error("entry screen offers logout without a session")
	}
}
