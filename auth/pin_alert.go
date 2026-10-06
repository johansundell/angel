package auth

import (
	"sync"
	"time"

	"github.com/johansundell/angel/logging"
)

// PIN Alert defaults: DefaultAlertFailures checked wrong PINs, from any
// addresses, within DefaultAlertWindow trigger the PIN Alert (ADR-0005).
const (
	DefaultAlertFailures = 20
	DefaultAlertWindow   = 24 * time.Hour
)

// PINAlert tells the Client that many wrong PINs have been entered and the
// Caregiver PIN should be changed.
type PINAlert struct {
	// Count is the number of wrong PINs: the ones that triggered the alert
	// and every one since.
	Count int
	// First and Last are when the first and the latest of them were entered.
	First, Last time.Time
}

// pinAlerter counts checked wrong PINs from all addresses. Once max of them
// fall within window, the alert triggers and stays, in memory, until the
// service restarts; changing the Caregiver PIN requires a restart, so it
// clears exactly when the fix is applied.
type pinAlerter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	log    logging.Logger
	// recent holds the times of wrong PINs within window, oldest first,
	// until the alert triggers.
	recent []time.Time
	alert  *PINAlert
}

func newPINAlerter(max int, window time.Duration, l logging.Logger) *pinAlerter {
	return &pinAlerter{max: max, window: window, log: logging.OrStd(l)}
}

// record counts a wrong PIN entered at now.
func (p *pinAlerter) record(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.alert != nil {
		p.alert.Count++
		p.alert.Last = now
		return
	}
	keep := 0
	for keep < len(p.recent) && !now.Before(p.recent[keep].Add(p.window)) {
		keep++
	}
	p.recent = append(p.recent[keep:], now)
	if len(p.recent) < p.max {
		return
	}
	p.alert = &PINAlert{Count: len(p.recent), First: p.recent[0], Last: now}
	p.recent = nil
	p.log.Warningf("PIN Alert: %d wrong PINs since %s; change the Caregiver PIN", p.alert.Count, p.alert.First.Format(time.RFC3339))
}

// current returns the alert, if it has triggered.
func (p *pinAlerter) current() (PINAlert, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.alert == nil {
		return PINAlert{}, false
	}
	return *p.alert, true
}
