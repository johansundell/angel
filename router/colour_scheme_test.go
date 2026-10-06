package router_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// pageBackgrounds reads --bg from the served stylesheet's light :root block
// and from its prefers-color-scheme: dark block.
func (a *pinApp) pageBackgrounds() (light, dark string) {
	t := a.t
	t.Helper()
	w := a.get("/assets/css/main.css")
	if w.Code != http.StatusOK {
		t.Fatalf("GET stylesheet: status = %d", w.Code)
	}
	bg := regexp.MustCompile(`--bg:\s*([^;]+);`)
	s := w.Body.String()
	m := bg.FindStringSubmatch(s)
	if m == nil {
		t.Fatal("stylesheet has no --bg token")
	}
	light = strings.TrimSpace(m[1])
	i := strings.Index(s, "@media (prefers-color-scheme: dark)")
	if i < 0 {
		t.Fatal("stylesheet has no prefers-color-scheme: dark block")
	}
	m = bg.FindStringSubmatch(s[i:])
	if m == nil {
		t.Fatal("dark block does not redefine --bg")
	}
	return light, strings.TrimSpace(m[1])
}

// Every page declares both colour schemes and carries a theme-color per
// scheme matching that scheme's --bg, so the phone's address bar blends in.
// /health paints its own dark panel over --bg but shares the base template.
func TestPages_FollowDeviceColourScheme(t *testing.T) {
	app := newPinApp(t)
	light, dark := app.pageBackgrounds()

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthReq.Header.Set("Accept", "text/html")

	pages := map[string]*httptest.ResponseRecorder{
		"entry":     app.get("/"),
		"caregiver": app.get("/note", app.caregiverSession()),
		"client":    app.get("/admin", app.clientSession()),
		"health":    app.do(healthReq),
	}
	for name, w := range pages {
		if w.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", name, w.Code)
			continue
		}
		body := w.Body.String()
		for _, want := range []string{
			`<meta name="color-scheme" content="light dark" />`,
			fmt.Sprintf(`<meta name="theme-color" content="%s" media="(prefers-color-scheme: light)" />`, light),
			fmt.Sprintf(`<meta name="theme-color" content="%s" media="(prefers-color-scheme: dark)" />`, dark),
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s page missing %s", name, want)
			}
		}
	}
}
