package router_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/johansundell/angel/auth"
	"github.com/johansundell/angel/handlers"
	"github.com/johansundell/angel/router"
	"github.com/johansundell/angel/store"
	"github.com/johansundell/angel/types"
)

const (
	testCaregiverPIN        = "1234"
	testMasterPIN           = "987654"
	testCaregiverSessionTTL = 20 * time.Minute
	testClientSessionTTL    = 8 * time.Hour
)

// fakeClock is a settable time source shared by the authenticator under test.
type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time          { return f.now }
func (f *fakeClock) Advance(d time.Duration) { f.now = f.now.Add(d) }

// pinApp is the app wired the way service.go wires it, backed by a temporary
// SQLite database and the repository's real templates and assets.
type pinApp struct {
	t     *testing.T
	r     *gin.Engine
	clock *fakeClock
	store *store.SQLiteStore
}

// newPinApp builds the app; opts are added to the handler's, for settings
// such as handlers.WithShareCaregiverNames.
func newPinApp(t *testing.T, opts ...handlers.Option) *pinApp {
	t.Helper()
	gin.SetMode(gin.TestMode)

	s, err := store.NewSQLite(filepath.Join(t.TempDir(), "angel.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	clock := &fakeClock{now: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)}
	authn, err := auth.New(auth.Config{
		CaregiverPIN:        testCaregiverPIN,
		MasterPIN:           testMasterPIN,
		Secret:              []byte("test-secret-test-secret-test-secret"),
		CaregiverSessionTTL: testCaregiverSessionTTL,
		ClientSessionTTL:    testClientSessionTTL,
		SecureCookie:        true,
		Now:                 clock.Now,
	})
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}

	repoRoot := os.DirFS("..")
	base := []handlers.Option{handlers.WithAuth(authn), handlers.WithNotes(s), handlers.WithClock(clock.Now)}
	h, err := handlers.NewHandler(s, false, repoRoot, "angel", "dev", append(base, opts...)...)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	r, err := router.NewRouter(router.Config{
		Handler:  h,
		Auth:     authn,
		LogSink:  &recordingSink{},
		Settings: types.AppSettings{AuthToken: "secret-token"},
		Assets:   repoRoot,
		Version:  "dev",
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return &pinApp{t: t, r: r, clock: clock, store: s}
}

func (a *pinApp) do(req *http.Request) *httptest.ResponseRecorder {
	a.t.Helper()
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)
	return w
}

func (a *pinApp) get(path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return a.do(req)
}

// submitPIN posts the entry form from the given client address.
func (a *pinApp) submitPIN(pin, remoteAddr string) *httptest.ResponseRecorder {
	a.t.Helper()
	form := url.Values{"pin": {pin}}
	req := httptest.NewRequest(http.MethodPost, "/pin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = remoteAddr
	return a.do(req)
}

func sessionCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	return nil
}

func assertRedirect(t *testing.T, w *httptest.ResponseRecorder, location string) {
	t.Helper()
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d (body %q)", w.Code, http.StatusSeeOther, w.Body.String())
	}
	if got := w.Header().Get("Location"); got != location {
		t.Fatalf("Location = %q, want %q", got, location)
	}
}

func assertEntryPage(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	body := w.Body.String()
	for _, want := range []string{"Ange PIN-kod", `action="/pin"`, `name="pin"`, `inputmode="numeric"`, "Logga in", `lang="sv"`, `name="viewport"`} {
		if !strings.Contains(body, want) {
			t.Errorf("entry page missing %q", want)
		}
	}
	for _, unwanted := range []string{"keypad", "data-digit", "Radera"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("entry page should not contain on-screen keypad artifact %q", unwanted)
		}
	}
}

func TestEntry_ShowsEntryPageWithoutSession(t *testing.T) {
	app := newPinApp(t)

	w := app.get("/")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	assertEntryPage(t, w)
}

func TestEntry_InvalidPINShowsErrorAndGrantsNothing(t *testing.T) {
	app := newPinApp(t)

	w := app.submitPIN("0000", "10.0.0.1:1111")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Fel PIN-kod") {
		t.Errorf("expected Swedish error message, got %q", w.Body.String())
	}
	assertEntryPage(t, w)
	if c := sessionCookie(t, w); c != nil && c.Value != "" {
		t.Errorf("invalid PIN must not set a session cookie, got %q", c.Value)
	}
}

func TestEntry_CaregiverPINOpensCaregiverView(t *testing.T) {
	app := newPinApp(t)

	w := app.submitPIN(testCaregiverPIN, "10.0.0.1:1111")

	assertRedirect(t, w, "/note")
	c := sessionCookie(t, w)
	if c == nil {
		t.Fatal("expected a session cookie")
	}
	if !c.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if !c.Secure {
		t.Error("session cookie must be Secure when configured")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if got := time.Duration(c.MaxAge) * time.Second; got < 15*time.Minute || got > 30*time.Minute {
		t.Errorf("cookie MaxAge = %v, want between 15 and 30 minutes", got)
	}

	view := app.get("/note", c)
	if view.Code != http.StatusOK {
		t.Fatalf("GET /note status = %d, want 200", view.Code)
	}

	// Coming back to the entry URL during the visit goes straight to the view.
	assertRedirect(t, app.get("/", c), "/note")

	// A caregiver session does not open the client dashboard.
	assertRedirect(t, app.get("/admin", c), "/")
}

func TestEntry_MasterPINOpensClientDashboard(t *testing.T) {
	app := newPinApp(t)

	w := app.submitPIN(testMasterPIN, "10.0.0.1:1111")

	assertRedirect(t, w, "/admin")
	c := sessionCookie(t, w)
	if c == nil {
		t.Fatal("expected a session cookie")
	}

	view := app.get("/admin", c)
	if view.Code != http.StatusOK {
		t.Fatalf("GET /admin status = %d, want 200", view.Code)
	}
	assertRedirect(t, app.get("/", c), "/admin")
	assertRedirect(t, app.get("/note", c), "/")
}

func TestEntry_ProtectedViewsRequireSession(t *testing.T) {
	app := newPinApp(t)

	assertRedirect(t, app.get("/note"), "/")
	assertRedirect(t, app.get("/admin"), "/")
}

func TestEntry_CaregiverSessionExpires(t *testing.T) {
	app := newPinApp(t)
	c := sessionCookie(t, app.submitPIN(testCaregiverPIN, "10.0.0.1:1111"))
	if c == nil {
		t.Fatal("expected a session cookie")
	}

	app.clock.Advance(testCaregiverSessionTTL - time.Minute)
	if w := app.get("/note", c); w.Code != http.StatusOK {
		t.Fatalf("session should still be valid, got status %d", w.Code)
	}

	// The browser would drop the cookie by MaxAge; the server must refuse it
	// too, in case it is replayed.
	app.clock.Advance(2 * time.Minute)
	assertRedirect(t, app.get("/note", c), "/")

	w := app.get("/", c)
	if w.Code != http.StatusOK {
		t.Fatalf("GET / after expiry status = %d, want 200", w.Code)
	}
	assertEntryPage(t, w)
}

func TestEntry_ClientSessionOutlivesCaregiverSession(t *testing.T) {
	app := newPinApp(t)
	w := app.submitPIN(testMasterPIN, "10.0.0.1:1111")
	c := sessionCookie(t, w)
	if c == nil {
		t.Fatal("expected a session cookie")
	}
	if got := time.Duration(c.MaxAge) * time.Second; got != testClientSessionTTL {
		t.Errorf("cookie MaxAge = %v, want %v", got, testClientSessionTTL)
	}

	app.clock.Advance(testClientSessionTTL - time.Minute)
	if w := app.get("/admin", c); w.Code != http.StatusOK {
		t.Fatalf("client session should still be valid, got status %d", w.Code)
	}

	app.clock.Advance(2 * time.Minute)
	assertRedirect(t, app.get("/admin", c), "/")
}

func TestEntry_TamperedCookieIsRejected(t *testing.T) {
	app := newPinApp(t)
	c := sessionCookie(t, app.submitPIN(testCaregiverPIN, "10.0.0.1:1111"))
	if c == nil {
		t.Fatal("expected a session cookie")
	}

	payload, sig, ok := strings.Cut(c.Value, ".")
	if !ok {
		t.Fatalf("unexpected cookie format %q", c.Value)
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	// Same expiry, upgraded role, original signature.
	forgedPayload := base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(string(raw), "caregiver", "client", 1)))
	if forgedPayload == payload {
		t.Fatalf("payload %q does not carry the role", raw)
	}
	for _, value := range []string{
		forgedPayload + "." + sig,
		payload + ".x" + sig,
		"client|99999999999",
		"",
	} {
		forged := &http.Cookie{Name: auth.CookieName, Value: value}
		assertRedirect(t, app.get("/admin", forged), "/")
		assertRedirect(t, app.get("/note", &http.Cookie{Name: auth.CookieName, Value: value + "garbage"}), "/")
	}
}

func TestEntry_RateLimitsRepeatedFailures(t *testing.T) {
	app := newPinApp(t)
	const attacker = "203.0.113.7:4000"

	for i := 0; i < auth.DefaultMaxFailures; i++ {
		if w := app.submitPIN("0000", attacker); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, w.Code)
		}
	}

	// Once blocked, even the right PIN is refused without being checked.
	w := app.submitPIN(testCaregiverPIN, attacker)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked attempt: status = %d, want 429", w.Code)
	}
	if !strings.Contains(w.Body.String(), "För många försök") {
		t.Errorf("expected Swedish rate-limit message, got %q", w.Body.String())
	}
	if c := sessionCookie(t, w); c != nil && c.Value != "" {
		t.Error("blocked attempt must not set a session cookie")
	}

	// Another device is not affected.
	assertRedirect(t, app.submitPIN(testCaregiverPIN, "198.51.100.2:5000"), "/note")

	// The block lifts after the window.
	app.clock.Advance(auth.DefaultFailureWindow + time.Second)
	assertRedirect(t, app.submitPIN(testCaregiverPIN, attacker), "/note")
}

func TestEntry_ForwardedForIsIgnoredWithoutTrustedProxies(t *testing.T) {
	app := newPinApp(t)
	const attacker = "203.0.113.7:4000"

	for i := 0; i <= auth.DefaultMaxFailures; i++ {
		form := url.Values{"pin": {"0000"}}
		req := httptest.NewRequest(http.MethodPost, "/pin", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Forwarded-For", "192.0.2."+string(rune('1'+i)))
		req.RemoteAddr = attacker
		app.do(req)
	}

	if w := app.submitPIN(testCaregiverPIN, attacker); w.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed X-Forwarded-For bypassed the rate limit: status = %d", w.Code)
	}
}

func TestHealthCheckAtHealthzPath(t *testing.T) {
	app := newPinApp(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Accept", "application/json")
	w := app.do(req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"dbStatus":"OK"`) {
		t.Errorf("unexpected health body %q", w.Body.String())
	}
	if w := app.get("/health"); w.Code != http.StatusNotFound {
		t.Errorf("GET /health status = %d, want 404 now that it is /healthz", w.Code)
	}
}

func TestNewRouter_RoleRoutesRequirePINAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, err := handlers.NewHandler(nopStore{}, false, fstest.MapFS{}, "test", "dev")
	if err != nil {
		t.Fatal(err)
	}

	_, err = router.NewRouter(router.Config{
		Handler:  h,
		LogSink:  &recordingSink{},
		Settings: types.AppSettings{AuthToken: "secret-token"},
		Assets:   fstest.MapFS{},
	})
	if err == nil || !strings.Contains(err.Error(), "PIN authentication must be configured") {
		t.Fatalf("expected NewRouter to fail closed without PIN auth, got %v", err)
	}
}

func TestNewRouter_RejectsInvalidTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, err := router.NewRouter(router.Config{
		Handler:  mustNewHandler(t, nopStore{}, false, fstest.MapFS{}, "test", "dev"),
		Auth:     mustNewAuth(t),
		LogSink:  &recordingSink{},
		Settings: types.AppSettings{AuthToken: "secret-token", TrustedProxies: []string{"not-an-ip"}},
		Assets:   fstest.MapFS{},
	})
	if err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
		t.Fatalf("expected TRUSTED_PROXIES error, got %v", err)
	}
}

func TestEntry_TrustedProxyForwardsClientAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, err := router.NewRouter(router.Config{
		Handler:  mustNewHandler(t, nopStore{}, false, os.DirFS(".."), "test", "dev"),
		Auth:     mustNewAuth(t),
		LogSink:  &recordingSink{},
		Settings: types.AppSettings{AuthToken: "secret-token", TrustedProxies: []string{"127.0.0.1"}},
		Assets:   os.DirFS(".."),
	})
	if err != nil {
		t.Fatal(err)
	}
	submit := func(pin, forwardedFor string) int {
		form := url.Values{"pin": {pin}}
		req := httptest.NewRequest(http.MethodPost, "/pin", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Forwarded-For", forwardedFor)
		req.RemoteAddr = "127.0.0.1:9000" // e.g. cloudflared on the same host
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	for i := 0; i < auth.DefaultMaxFailures; i++ {
		submit("0000", "203.0.113.7")
	}
	if code := submit(testCaregiverPIN, "203.0.113.7"); code != http.StatusTooManyRequests {
		t.Fatalf("forwarded attacker should be blocked, got %d", code)
	}
	// Other caregivers behind the same proxy are not locked out.
	if code := submit(testCaregiverPIN, "198.51.100.2"); code != http.StatusSeeOther {
		t.Fatalf("other forwarded client should get in, got %d", code)
	}
}
