package router_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/johansundell/angel/router"
	"github.com/johansundell/angel/types"
)

// proxyWarnings returns the logged warnings about forwarding headers.
func proxyWarnings(tl *testLogger) []string {
	var got []string
	for _, e := range tl.entries {
		if e.level == "WARN" && strings.Contains(e.message, "TRUSTED_PROXIES") {
			got = append(got, e.message)
		}
	}
	return got
}

func newProxyTestRouter(t *testing.T, trusted []string) (*gin.Engine, *testLogger) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tl := &testLogger{}
	r, err := router.NewRouter(router.Config{
		Handler:  mustNewHandler(t, nopStore{}, false, fstest.MapFS{}, "test", "dev"),
		LogSink:  &recordingSink{},
		Settings: types.AppSettings{AuthToken: "secret-token", TrustedProxies: trusted},
		Assets:   fstest.MapFS{},
		Logger:   tl,
		Routes:   testRoutes(),
	})
	if err != nil {
		t.Fatalf("NewRouter failed: %v", err)
	}
	return r, tl
}

func proxyRequest(r *gin.Engine, remoteAddr string, headers map[string]string) {
	req := httptest.NewRequest(http.MethodGet, "/hello/x", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(httptest.NewRecorder(), req)
}

func TestUntrustedForwardingHeaderWarnsOnce(t *testing.T) {
	for _, header := range []string{"X-Forwarded-For", "CF-Connecting-IP"} {
		t.Run(header, func(t *testing.T) {
			r, tl := newProxyTestRouter(t, nil)

			proxyRequest(r, "172.18.0.1:5000", map[string]string{header: "203.0.113.7"})
			proxyRequest(r, "172.18.0.1:5000", map[string]string{header: "203.0.113.8"})
			proxyRequest(r, "10.0.0.9:5000", map[string]string{header: "203.0.113.9"})

			got := proxyWarnings(tl)
			if len(got) != 1 {
				t.Fatalf("got %d warnings, want 1: %q", len(got), got)
			}
			for _, want := range []string{"172.18.0.1", "TRUSTED_PROXIES", "examples/reverse-proxy/"} {
				if !strings.Contains(got[0], want) {
					t.Errorf("warning %q does not mention %q", got[0], want)
				}
			}
		})
	}
}

func TestTrustedOrDirectRequestsDoNotWarn(t *testing.T) {
	r, tl := newProxyTestRouter(t, []string{"127.0.0.1", "10.1.0.0/16"})

	proxyRequest(r, "127.0.0.1:5000", map[string]string{"X-Forwarded-For": "203.0.113.7"})
	proxyRequest(r, "10.1.2.3:5000", map[string]string{"CF-Connecting-IP": "203.0.113.7"})
	proxyRequest(r, "198.51.100.4:5000", nil)

	if got := proxyWarnings(tl); len(got) != 0 {
		t.Fatalf("got warnings %q, want none", got)
	}
}
