package router_test

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

type webAppManifest struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Lang            string         `json:"lang"`
	Description     string         `json:"description"`
	Display         string         `json:"display"`
	Orientation     *string        `json:"orientation"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	ThemeColor      string         `json:"theme_color"`
	BackgroundColor string         `json:"background_color"`
	Icons           []manifestIcon `json:"icons"`
}

const (
	manifestLink       = `<link rel="manifest" href="/manifest.webmanifest" />`
	appleTouchIconLink = `<link rel="apple-touch-icon" href="/assets/img/apple-touch-icon.png" />`
)

func (a *pinApp) manifest() webAppManifest {
	t := a.t
	t.Helper()
	w := a.get("/manifest.webmanifest")
	if w.Code != http.StatusOK {
		t.Fatalf("GET manifest: status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/manifest+json") {
		t.Errorf("manifest Content-Type = %q, want application/manifest+json", ct)
	}
	var m webAppManifest
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("manifest is not JSON: %v", err)
	}
	return m
}

// assertPNG fetches path and fails unless it is a PNG of size×size.
func (a *pinApp) assertPNG(path string, size int) {
	t := a.t
	t.Helper()
	w := a.get(path)
	if w.Code != http.StatusOK {
		t.Errorf("GET %s: status = %d, want 200", path, w.Code)
		return
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Errorf("%s is not a PNG: %v", path, err)
		return
	}
	if cfg.Width != size || cfg.Height != size {
		t.Errorf("%s is %d×%d, want %d×%d", path, cfg.Width, cfg.Height, size, size)
	}
}

// The installed app is named Angel, opens standalone at the PIN keypad and
// follows the device's orientation.
func TestManifest_DescribesInstallableApp(t *testing.T) {
	m := newPinApp(t).manifest()

	if m.Name != "Angel" || m.ShortName != "Angel" {
		t.Errorf("name, short_name = %q, %q, want Angel", m.Name, m.ShortName)
	}
	if m.Lang != "sv" {
		t.Errorf("lang = %q, want sv", m.Lang)
	}
	if m.Description == "" {
		t.Error("description is empty")
	}
	if m.Display != "standalone" {
		t.Errorf("display = %q, want standalone", m.Display)
	}
	if m.Orientation != nil {
		t.Errorf("orientation = %q, want none", *m.Orientation)
	}
	if m.StartURL != "/" || m.Scope != "/" {
		t.Errorf("start_url, scope = %q, %q, want /", m.StartURL, m.Scope)
	}
	if m.ThemeColor != "#f8fafc" || m.BackgroundColor != "#f8fafc" {
		t.Errorf("theme_color, background_color = %q, %q, want #f8fafc", m.ThemeColor, m.BackgroundColor)
	}
}

// Every icon the manifest lists is a PNG of its stated size, and there are
// 192 and 512 icons plus a maskable 512.
func TestManifest_IconsAreServedAtTheirStatedSize(t *testing.T) {
	app := newPinApp(t)
	m := app.manifest()

	have := map[string]bool{}
	for _, icon := range m.Icons {
		if icon.Type != "image/png" {
			t.Errorf("%s: type = %q, want image/png", icon.Src, icon.Type)
		}
		w, h, ok := strings.Cut(icon.Sizes, "x")
		size, err := strconv.Atoi(w)
		if !ok || err != nil || w != h {
			t.Errorf("%s: sizes = %q, want NxN", icon.Src, icon.Sizes)
			continue
		}
		app.assertPNG(icon.Src, size)
		purpose := icon.Purpose
		if purpose == "" {
			purpose = "any"
		}
		for _, p := range strings.Fields(purpose) {
			have[p+" "+icon.Sizes] = true
		}
	}
	for _, want := range []string{"any 192x192", "any 512x512", "maskable 512x512"} {
		if !have[want] {
			t.Errorf("manifest has no %s icon", want)
		}
	}
}

func TestAppleTouchIcon_Is180(t *testing.T) {
	newPinApp(t).assertPNG("/assets/img/apple-touch-icon.png", 180)
}

// Every page shares the base head, so each can be added to the home screen.
func TestPages_LinkManifestAndAppleTouchIcon(t *testing.T) {
	app := newPinApp(t)

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
		for _, want := range []string{manifestLink, appleTouchIconLink} {
			if !strings.Contains(body, want) {
				t.Errorf("%s page missing %s", name, want)
			}
		}
	}
}
