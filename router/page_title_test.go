package router_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every page's tab title starts with "Angel – " so Angel stands out among
// other tabs and apps.
func TestPages_TitlePrefixedWithAngel(t *testing.T) {
	app := newPinApp(t)
	want := map[string]string{
		"entry":     "<title>Angel – Ange PIN-kod</title>",
		"caregiver": "<title>Angel – Dagens anteckning</title>",
		"client":    "<title>Angel – Dagens anteckning</title>",
		"health":    "<title>Angel – Health Check</title>",
	}

	bodies := app.everyPage()
	for name, title := range want {
		body, ok := bodies[name]
		if !ok {
			continue // everyPage already reported the failure
		}
		if !strings.Contains(body, title) {
			t.Errorf("%s page missing %s", name, title)
		}
	}
}

// The prefix lives in the page head, so the health check's JSON keeps its
// plain title.
func TestHealthJSON_TitleUnprefixed(t *testing.T) {
	app := newPinApp(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	w := app.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"title":"Health Check"`) {
		t.Errorf("health JSON = %s, want \"title\":\"Health Check\"", w.Body.String())
	}
}
