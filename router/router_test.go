package router_test

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/johansundell/angel/auth"
	"github.com/johansundell/angel/handlers"
	"github.com/johansundell/angel/router"
	"github.com/johansundell/angel/store"
	"github.com/johansundell/angel/types"
)

func TestGetRoutes(t *testing.T) {
	mockStore := nopStore{}
	h := mustNewHandler(t, mockStore, false, fstest.MapFS{}, "test", "dev")

	routes := router.GetRoutes(h)
	if len(routes) == 0 {
		t.Fatal("Expected GetRoutes to return route definitions, got empty")
	}

	expectedRoutes := map[string]struct {
		method  string
		pattern string
		role    auth.Role
	}{
		"Entry":            {method: "GET", pattern: "/"},
		"SubmitPIN":        {method: "POST", pattern: "/pin"},
		"Logout":           {method: "POST", pattern: "/logout"},
		"CaregiverView":    {method: "GET", pattern: "/note", role: auth.RoleCaregiver},
		"AcknowledgeNote":  {method: "POST", pattern: "/note/ack", role: auth.RoleCaregiver},
		"ClientDashboard":  {method: "GET", pattern: "/admin", role: auth.RoleClient},
		"AckFeed":          {method: "GET", pattern: "/admin/acks", role: auth.RoleClient},
		"SaveNote":         {method: "POST", pattern: "/admin/note", role: auth.RoleClient},
		"ClearNote":        {method: "POST", pattern: "/admin/note/clear", role: auth.RoleClient},
		"SaveAdvanceNote":  {method: "POST", pattern: "/admin/advance", role: auth.RoleClient},
		"ClearAdvanceNote": {method: "POST", pattern: "/admin/advance/clear", role: auth.RoleClient},
		"HealthCheck":      {method: "GET", pattern: "/healthz"},
	}

	if len(routes) != len(expectedRoutes) {
		t.Errorf("Expected %d routes, got %d", len(expectedRoutes), len(routes))
	}

	for _, route := range routes {
		expected, exists := expectedRoutes[route.Name]
		if !exists {
			t.Errorf("Unexpected route %q", route.Name)
			continue
		}
		if route.Method != expected.method {
			t.Errorf("Route %q: expected method %q, got %q", route.Name, expected.method, route.Method)
		}
		if route.Pattern != expected.pattern {
			t.Errorf("Route %q: expected pattern %q, got %q", route.Name, expected.pattern, route.Pattern)
		}
		if route.Role != expected.role {
			t.Errorf("Route %q: expected Role=%q, got %q", route.Name, expected.role, route.Role)
		}
		if route.HandlerFunc == nil {
			t.Errorf("Route %q: HandlerFunc should not be nil", route.Name)
		}
	}
}

func TestNewRouter_RequiresHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, err := router.NewRouter(router.Config{
		Version: "v1.0.0",
	})
	if err == nil {
		t.Fatalf("Expected NewRouter to fail when Handler is not provided, got nil")
	}
	expected := "handler must be provided"
	if err.Error() != expected {
		t.Errorf("Expected error %q, got %q", expected, err.Error())
	}
}

func TestNewRouter_EmbeddedModeNilAssetsReturnsError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockStore := nopStore{}
	h := mustNewHandler(t, mockStore, false, fstest.MapFS{}, "test", "dev")

	_, err := router.NewRouter(router.Config{
		Handler:  h,
		Auth:     mustNewAuth(t),
		Assets:   nil,
		Settings: types.AppSettings{UseFileSystem: false},
		Version:  "v1.0.0",
	})
	if err == nil {
		t.Fatalf("Expected NewRouter to fail when Assets is nil in embedded mode, got nil")
	}
	expected := "embedded assets filesystem is nil"
	if err.Error() != expected {
		t.Errorf("Expected error %q, got %q", expected, err.Error())
	}
}

func TestNewRouter_FileSystemModeNilAssetsSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockStore := nopStore{}
	h := mustNewHandler(t, mockStore, false, fstest.MapFS{}, "test", "dev")

	r, err := router.NewRouter(router.Config{
		Handler:  h,
		Auth:     mustNewAuth(t),
		Assets:   nil,
		Settings: types.AppSettings{UseFileSystem: true},
		Version:  "v1.0.0",
	})
	if err != nil {
		t.Fatalf("Expected NewRouter to succeed in filesystem mode without embedded assets, got: %v", err)
	}
	if r == nil {
		t.Fatal("Expected non-nil router")
	}
}

type testLogEntry struct {
	level   string
	message string
}

type testLogger struct {
	entries  []testLogEntry
	messages []string
}

func (tl *testLogger) Infof(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	tl.entries = append(tl.entries, testLogEntry{level: "INFO", message: msg})
	tl.messages = append(tl.messages, msg)
}

func (tl *testLogger) Warningf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	tl.entries = append(tl.entries, testLogEntry{level: "WARN", message: msg})
	tl.messages = append(tl.messages, msg)
}

func (tl *testLogger) Errorf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	tl.entries = append(tl.entries, testLogEntry{level: "ERROR", message: msg})
	tl.messages = append(tl.messages, msg)
}

// testRoutes returns routes for testing router features.
func testRoutes() router.Routes {
	ok := func(c *gin.Context) error {
		c.Status(http.StatusOK)
		return nil
	}
	return router.Routes{
		{Name: "Hello", Method: "GET", Pattern: "/hello/:name", HandlerFunc: ok},
	}
}

// nopStore satisfies store.Store for tests that never touch storage.
type nopStore struct{}

func (nopStore) Ping(context.Context) error { return nil }
func (nopStore) Close() error               { return nil }
func (nopStore) GetDailyNote(context.Context, types.Day) (types.DailyNote, bool, error) {
	return types.DailyNote{}, false, nil
}
func (nopStore) SaveDailyNote(context.Context, types.DailyNote) error { return nil }
func (nopStore) DeleteDailyNote(context.Context, types.Day) error     { return nil }
func (nopStore) AddAcknowledgement(context.Context, types.Acknowledgement) (int64, error) {
	return 0, nil
}
func (nopStore) ListAcknowledgements(context.Context, types.Day) ([]types.Acknowledgement, error) {
	return nil, nil
}

// mustNewAuth returns an Authenticator with the test PINs. Sessions are
// signed with a fixed secret, so separate instances accept each other's
// cookies.
func mustNewAuth(t *testing.T) *auth.Authenticator {
	t.Helper()
	authn, err := auth.New(auth.Config{
		CaregiverPIN:        testCaregiverPIN,
		MasterPIN:           testMasterPIN,
		Secret:              []byte("test-secret-test-secret-test-secret"),
		CaregiverSessionTTL: testCaregiverSessionTTL,
		ClientSessionTTL:    testClientSessionTTL,
	})
	if err != nil {
		t.Fatalf("auth.New failed: %v", err)
	}
	return authn
}

func mustNewHandler(t *testing.T, s store.Store, useFileSystem bool, embedded fs.FS, name, version string) *handlers.Handler {
	t.Helper()
	h, err := handlers.NewHandler(s, useFileSystem, embedded, name, version, handlers.WithAuth(mustNewAuth(t)))
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}
	return h
}

func TestNewRouter_Debug(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newRouter := func(debug bool) (*gin.Engine, *testLogger) {
		t.Helper()
		tl := &testLogger{}
		r, err := router.NewRouter(router.Config{
			Handler:  mustNewHandler(t, nopStore{}, false, fstest.MapFS{}, "test", "dev"),
			Settings: types.AppSettings{Debug: debug},
			Assets:   fstest.MapFS{},
			Logger:   tl,
			Routes:   testRoutes(),
		})
		if err != nil {
			t.Fatalf("NewRouter failed: %v", err)
		}
		return r, tl
	}

	t.Run("on", func(t *testing.T) {
		r, tl := newRouter(true)

		var routes int
		for _, e := range tl.entries {
			if strings.HasPrefix(e.message, "route ") {
				routes++
			}
		}
		if want := len(testRoutes()); routes != want {
			t.Errorf("Expected %d route lines, got %d: %v", want, routes, tl.messages)
		}
		found := false
		for _, m := range tl.messages {
			if strings.Contains(m, "route GET /hello/:name (Hello)") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected a route line for Hello, got %v", tl.messages)
		}

		before := len(tl.entries)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/hello/world?token=abc", nil))
		access := tl.entries[before:]
		if len(access) != 1 || access[0].level != "INFO" || !strings.HasPrefix(access[0].message, "GET /hello/world 200 ") {
			t.Fatalf("Expected one access log line for GET /hello/world 200, got %v", access)
		}
		if strings.Contains(access[0].message, "token=abc") {
			t.Errorf("Access log must not include the query string, got %q", access[0].message)
		}
	})

	t.Run("off", func(t *testing.T) {
		r, tl := newRouter(false)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/hello/world", nil))
		if len(tl.entries) != 0 {
			t.Errorf("Expected no debug logging, got %v", tl.messages)
		}
	})
}
