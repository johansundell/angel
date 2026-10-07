package router_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const serviceWorkerRegistration = `navigator.serviceWorker.register("/sw.js")`

// serviceWorker fetches the service worker and checks it is served as
// JavaScript that the browser revalidates on every update check.
func (a *pinApp) serviceWorker() string {
	t := a.t
	t.Helper()
	w := a.get("/sw.js")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /sw.js: status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("service worker Content-Type = %q, want text/javascript", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("service worker Cache-Control = %q, want no-cache", cc)
	}
	return w.Body.String()
}

// precache reads the list of URLs the service worker stores at install.
func precache(t *testing.T, sw string) []string {
	t.Helper()
	m := regexp.MustCompile(`const PRECACHE = (\[[^\]]*\]);`).FindStringSubmatch(sw)
	if m == nil {
		t.Fatal("service worker has no PRECACHE list")
	}
	var urls []string
	if err := json.Unmarshal([]byte(m[1]), &urls); err != nil {
		t.Fatalf("PRECACHE is not a JSON list of strings: %v", err)
	}
	return urls
}

// offlineURL reads the URL the service worker answers failed navigations with.
func offlineURL(t *testing.T, sw string) string {
	t.Helper()
	m := regexp.MustCompile(`const OFFLINE = ("[^"]*");`).FindStringSubmatch(sw)
	if m == nil {
		t.Fatal("service worker has no OFFLINE page")
	}
	u, err := strconv.Unquote(m[1])
	if err != nil {
		t.Fatalf("OFFLINE is not a string: %v", err)
	}
	return u
}

// Each release changes the service worker, so browsers install the new one
// and it replaces the old release's cache.
func TestServiceWorker_StampedWithBuildVersion(t *testing.T) {
	app := newPinApp(t)
	sw := app.serviceWorker()
	version := app.get("/").Header().Get("X-Version")
	if version == "" {
		t.Fatal("no X-Version header")
	}
	if !strings.Contains(sw, strconv.Quote(version)) {
		t.Errorf("service worker does not contain build version %q", version)
	}
	for _, want := range []string{"skipWaiting()", "clients.claim()", "caches.delete("} {
		if !strings.Contains(sw, want) {
			t.Errorf("service worker missing %s", want)
		}
	}
}

// Only the app shell is stored on the device; a Daily Note never is
// (ADR-0007).
func TestServiceWorker_PrecachesOnlyAppShell(t *testing.T) {
	app := newPinApp(t)
	sw := app.serviceWorker()
	urls := precache(t, sw)
	if len(urls) == 0 {
		t.Fatal("PRECACHE is empty")
	}

	shell := regexp.MustCompile(`^/(assets/(css/[\w-]+\.css|img/[\w-]+\.(png|svg)|[\w-]+\.html)|manifest\.webmanifest)$`)
	for _, u := range urls {
		if !shell.MatchString(u) {
			t.Errorf("PRECACHE holds %q, which is not an app-shell file", u)
		}
		// Shell files load before any PIN is entered.
		if w := app.get(u); w.Code != http.StatusOK {
			t.Errorf("GET %s without a session: status = %d, want 200", u, w.Code)
		}
	}
	for _, private := range []string{`"/note`, `"/admin`} {
		if strings.Contains(sw, private) {
			t.Errorf("service worker names %s…, a page behind a PIN", private)
		}
	}
	for _, want := range []string{"/assets/css/main.css", "/manifest.webmanifest", offlineURL(t, sw)} {
		if !slices.Contains(urls, want) {
			t.Errorf("PRECACHE does not hold %s", want)
		}
	}
}

// Offline, the app says so in Swedish, reveals nothing about the Client and
// lets them try again.
func TestOfflinePage_ServedWithoutSession(t *testing.T) {
	app := newPinApp(t)
	page := offlineURL(t, app.serviceWorker())

	w := app.get(page)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200", page, w.Code)
	}
	body := w.Body.String()
	light, dark := app.pageBackgrounds()
	for _, want := range []string{
		fmt.Sprintf(`<meta name="theme-color" content="%s" media="(prefers-color-scheme: light)" />`, light),
		fmt.Sprintf(`<meta name="theme-color" content="%s" media="(prefers-color-scheme: dark)" />`, dark),
		"Ingen anslutning. Anteckningen kan inte visas utan internet.",
		"Försök igen</button>",
		`<html lang="sv">`,
		`<link rel="stylesheet" href="/assets/css/main.css" />`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("offline page missing %s", want)
		}
	}
}

func TestPages_RegisterServiceWorker(t *testing.T) {
	app := newPinApp(t)

	for name, body := range app.everyPage() {
		if !strings.Contains(body, serviceWorkerRegistration) {
			t.Errorf("%s page does not register the service worker", name)
		}
	}
}
