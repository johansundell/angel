package auth

import (
	"sync"
	"time"
)

// limiter counts failed PIN attempts per client address in a fixed window
// that starts at the first failure. An address with max failures is blocked
// until its window ends.
type limiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	failures map[string]*failureWindow
}

type failureWindow struct {
	start time.Time
	count int
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, failures: make(map[string]*failureWindow)}
}

func (l *limiter) allow(addr string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pruneLocked(now)
	f, ok := l.failures[addr]
	return !ok || f.count < l.max
}

func (l *limiter) fail(addr string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.failures[addr]
	if !ok || l.expired(f, now) {
		l.failures[addr] = &failureWindow{start: now, count: 1}
		return
	}
	f.count++
}

func (l *limiter) reset(addr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, addr)
}

// pruneLocked drops finished windows so the map stays bounded by the number
// of addresses that failed recently.
func (l *limiter) pruneLocked(now time.Time) {
	for addr, f := range l.failures {
		if l.expired(f, now) {
			delete(l.failures, addr)
		}
	}
}

func (l *limiter) expired(f *failureWindow, now time.Time) bool {
	return !now.Before(f.start.Add(l.window))
}
