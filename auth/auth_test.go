package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_RejectsUnsafeConfig(t *testing.T) {
	valid := func() Config {
		return Config{CaregiverPIN: "1234", MasterPIN: "987654", Secret: []byte(strings.Repeat("s", 32)), CaregiverSessionTTL: 20 * time.Minute, ClientSessionTTL: 8 * time.Hour}
	}
	tests := []struct {
		name   string
		modify func(*Config)
	}{
		{"missing caregiver PIN", func(c *Config) { c.CaregiverPIN = "" }},
		{"missing master PIN", func(c *Config) { c.MasterPIN = "" }},
		{"same PINs", func(c *Config) { c.MasterPIN = c.CaregiverPIN }},
		{"short secret", func(c *Config) { c.Secret = []byte("short") }},
		{"no caregiver TTL", func(c *Config) { c.CaregiverSessionTTL = 0 }},
		{"no client TTL", func(c *Config) { c.ClientSessionTTL = 0 }},
	}
	if _, err := New(valid()); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid()
			tc.modify(&cfg)
			if _, err := New(cfg); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestStartSession_LifetimePerRole(t *testing.T) {
	now := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	a, err := New(Config{
		CaregiverPIN: "1234", MasterPIN: "987654", Secret: []byte(strings.Repeat("s", 32)),
		CaregiverSessionTTL: 20 * time.Minute, ClientSessionTTL: 8 * time.Hour,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	for role, ttl := range map[Role]time.Duration{RoleCaregiver: 20 * time.Minute, RoleClient: 8 * time.Hour} {
		t.Run(string(role), func(t *testing.T) {
			now = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
			w := httptest.NewRecorder()
			a.StartSession(w, role)
			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("got %d cookies, want 1", len(cookies))
			}
			c := cookies[0]
			if got := time.Duration(c.MaxAge) * time.Second; got != ttl {
				t.Errorf("MaxAge = %v, want %v", got, ttl)
			}
			if !c.Expires.Equal(now.Add(ttl)) {
				t.Errorf("Expires = %v, want %v", c.Expires, now.Add(ttl))
			}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})

			now = now.Add(ttl - time.Second)
			if got, ok := a.Session(req); !ok || got != role {
				t.Fatalf("just before expiry: Session = %q, %v; want %q, true", got, ok, role)
			}
			now = now.Add(time.Second)
			if _, ok := a.Session(req); ok {
				t.Fatal("session still valid at expiry")
			}
		})
	}
}

func TestLimiter_PrunesFinishedWindows(t *testing.T) {
	l := newLimiter(2, time.Minute)
	now := time.Unix(0, 0)
	for i := 0; i < 100; i++ {
		l.reserve(string(rune('a'+i)), now)
	}
	l.reserve("x", now.Add(time.Minute))
	if len(l.failures) != 1 {
		t.Fatalf("expected finished windows to be pruned, %d left", len(l.failures))
	}
}

func TestLogin_ConcurrentFailuresCannotExceedLimit(t *testing.T) {
	a, err := New(Config{CaregiverPIN: "1234", MasterPIN: "987654", Secret: []byte(strings.Repeat("s", 32)), CaregiverSessionTTL: 20 * time.Minute, ClientSessionTTL: 8 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	const attempts = 200
	var checked atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := a.Login("203.0.113.7", "0000"); errors.Is(err, ErrInvalidPIN) {
				checked.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := checked.Load(); got != DefaultMaxFailures {
		t.Fatalf("%d PINs were checked in a burst, want exactly %d", got, DefaultMaxFailures)
	}
}
