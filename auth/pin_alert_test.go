package auth

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type warnRecorder struct{ warnings []string }

func (r *warnRecorder) Infof(string, ...interface{}) {}
func (r *warnRecorder) Warningf(format string, v ...interface{}) {
	r.warnings = append(r.warnings, fmt.Sprintf(format, v...))
}
func (r *warnRecorder) Errorf(string, ...interface{}) {}

// alertApp is an Authenticator on a settable clock with a recorded log.
type alertApp struct {
	t   *testing.T
	a   *Authenticator
	now time.Time
	// step is how far the clock moves after each wrong PIN.
	step time.Duration
	log  *warnRecorder
}

func newAlertApp(t *testing.T) *alertApp {
	t.Helper()
	app := &alertApp{t: t, now: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC), step: time.Minute, log: &warnRecorder{}}
	a, err := New(Config{
		CaregiverPIN: "1234", MasterPIN: "987654", Secret: []byte(strings.Repeat("s", 32)),
		CaregiverSessionTTL: 20 * time.Minute, ClientSessionTTL: 8 * time.Hour,
		Now:    func() time.Time { return app.now },
		Logger: app.log,
	})
	if err != nil {
		t.Fatal(err)
	}
	app.a = a
	return app
}

// wrongPINs enters n wrong PINs, each from its own address so the rate
// limiter never blocks them, step apart.
func (app *alertApp) wrongPINs(n int) {
	app.t.Helper()
	for i := 0; i < n; i++ {
		addr := fmt.Sprintf("198.51.100.%d:%d", i%250, app.now.UnixNano())
		if _, err := app.a.Login(addr, "0000"); !errors.Is(err, ErrInvalidPIN) {
			app.t.Fatalf("Login = %v, want ErrInvalidPIN", err)
		}
		app.now = app.now.Add(app.step)
	}
}

func TestPINAlert_TriggersOnTwentiethWrongPINWithin24h(t *testing.T) {
	app := newAlertApp(t)
	start := app.now

	app.wrongPINs(AlertFailures - 1)
	if _, ok := app.a.PINAlert(); ok {
		t.Fatal("alert after 19 wrong PINs")
	}
	app.wrongPINs(1)
	alert, ok := app.a.PINAlert()
	if !ok {
		t.Fatal("no alert after 20 wrong PINs")
	}
	if alert.Count != AlertFailures {
		t.Errorf("Count = %d, want %d", alert.Count, AlertFailures)
	}
	if !alert.First.Equal(start) {
		t.Errorf("First = %v, want %v", alert.First, start)
	}
	if want := start.Add(19 * time.Minute); !alert.Last.Equal(want) {
		t.Errorf("Last = %v, want %v", alert.Last, want)
	}
}

func TestPINAlert_IgnoresWrongPINsOlderThan24h(t *testing.T) {
	app := newAlertApp(t)

	app.wrongPINs(AlertFailures - 1)
	app.now = app.now.Add(AlertWindow)
	app.wrongPINs(1)
	if _, ok := app.a.PINAlert(); ok {
		t.Fatal("wrong PINs older than 24h counted")
	}
	// The 19 old ones have expired, so 18 more (19 in the window) still
	// do not trigger, and one more does.
	app.wrongPINs(AlertFailures - 2)
	if _, ok := app.a.PINAlert(); ok {
		t.Fatal("alert after 19 wrong PINs within 24h")
	}
	app.wrongPINs(1)
	if _, ok := app.a.PINAlert(); !ok {
		t.Fatal("no alert after 20 wrong PINs within 24h")
	}
}

func TestPINAlert_CountsOnlyTheRollingWindow(t *testing.T) {
	app := newAlertApp(t)
	start := app.now

	app.wrongPINs(10) // minutes 0 to 9
	// Those from minutes 0 to 5 are now older than 24h; 4 remain.
	app.now = start.Add(AlertWindow + 5*time.Minute + 30*time.Second)
	app.step = 0
	app.wrongPINs(AlertFailures - 5)
	if _, ok := app.a.PINAlert(); ok {
		t.Fatal("alert after 19 wrong PINs within 24h")
	}
	app.wrongPINs(1)
	alert, ok := app.a.PINAlert()
	if !ok {
		t.Fatal("no alert after 20 wrong PINs within 24h")
	}
	if want := start.Add(6 * time.Minute); alert.Count != AlertFailures || !alert.First.Equal(want) {
		t.Errorf("alert = %+v, want Count %d and First %v", alert, AlertFailures, want)
	}
}

func TestPINAlert_StaysAndKeepsCountingRegardlessOfWindow(t *testing.T) {
	app := newAlertApp(t)
	app.wrongPINs(AlertFailures)
	first, _ := app.a.PINAlert()

	app.now = app.now.Add(48 * time.Hour)
	if _, ok := app.a.PINAlert(); !ok {
		t.Fatal("alert cleared after 24h")
	}
	// Slow guessing, one a day, would never trigger the alert, but once it
	// has, each guess is counted and moves Last on.
	app.step = AlertWindow
	for i := 1; i <= 5; i++ {
		app.wrongPINs(1)
		alert, _ := app.a.PINAlert()
		if alert.Count != AlertFailures+i {
			t.Fatalf("after %d more: Count = %d, want %d", i, alert.Count, AlertFailures+i)
		}
		if want := app.now.Add(-AlertWindow); !alert.First.Equal(first.First) || !alert.Last.Equal(want) {
			t.Fatalf("after %d more: alert = %+v, want First %v and Last %v", i, alert, first.First, want)
		}
	}
	if len(app.log.warnings) != 1 {
		t.Errorf("got %d warnings, want 1: %q", len(app.log.warnings), app.log.warnings)
	}
}

func TestPINAlert_RateLimitedAttemptsDoNotCount(t *testing.T) {
	app := newAlertApp(t)
	limited := 0
	for i := 0; i < 100; i++ {
		if _, err := app.a.Login("203.0.113.7", "0000"); errors.Is(err, ErrRateLimited) {
			limited++
		}
	}
	if limited != 100-DefaultMaxFailures {
		t.Fatalf("%d attempts rate limited, want %d", limited, 100-DefaultMaxFailures)
	}
	if _, ok := app.a.PINAlert(); ok {
		t.Fatal("rate-limited attempts triggered the alert")
	}
}

func TestPINAlert_CorrectPINsDoNotCount(t *testing.T) {
	app := newAlertApp(t)
	for i := 0; i < 2*AlertFailures; i++ {
		if _, err := app.a.Login(fmt.Sprintf("198.51.100.%d", i), "1234"); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := app.a.PINAlert(); ok {
		t.Fatal("correct PINs triggered the alert")
	}
}

func TestPINAlert_LogsOnceWhenTriggered(t *testing.T) {
	app := newAlertApp(t)

	app.wrongPINs(AlertFailures - 1)
	if len(app.log.warnings) != 0 {
		t.Fatalf("warned before the alert: %q", app.log.warnings)
	}
	app.wrongPINs(AlertFailures)
	if len(app.log.warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %q", len(app.log.warnings), app.log.warnings)
	}
	if !strings.Contains(app.log.warnings[0], "PIN Alert") {
		t.Errorf("warning = %q", app.log.warnings[0])
	}
}

func TestPINAlerter_KeepsOnlyRecentFailures(t *testing.T) {
	p := newPINAlerter(3, time.Hour, nil)
	now := time.Unix(0, 0)
	for i := 0; i < 100; i++ {
		p.record(now)
		now = now.Add(time.Hour)
	}
	if len(p.recent) > 2 {
		t.Fatalf("%d failures kept, want at most 2", len(p.recent))
	}
}
