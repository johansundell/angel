// Package auth turns keypad PINs into signed session cookies.
//
// The Caregiver PIN opens a caregiver session and the Master PIN a client
// session. Sessions are stateless: the cookie carries the role and expiry,
// signed with HMAC-SHA256, so nothing is stored server side and a restart
// with a new secret logs everyone out. Failed PIN attempts are rate limited
// per client address.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Role is what a session is allowed to see.
type Role string

const (
	RoleCaregiver Role = "caregiver"
	RoleClient    Role = "client"
)

// CookieName is the name of the session cookie.
const CookieName = "angel_session"

// Rate limiting defaults: after DefaultMaxFailures wrong PINs from one
// address within DefaultFailureWindow, that address is blocked until the
// window has passed.
const (
	DefaultMaxFailures   = 5
	DefaultFailureWindow = 15 * time.Minute
)

var (
	// ErrInvalidPIN means the PIN matched neither configured PIN.
	ErrInvalidPIN = errors.New("invalid PIN")
	// ErrRateLimited means the address has too many recent failures; the
	// PIN was not checked.
	ErrRateLimited = errors.New("too many failed PIN attempts")
)

// Config configures an Authenticator.
type Config struct {
	CaregiverPIN string
	MasterPIN    string
	// Secret signs session cookies; at least 32 bytes.
	Secret     []byte
	SessionTTL time.Duration
	// SecureCookie marks the cookie Secure (HTTPS only).
	SecureCookie bool
	// MaxFailures and FailureWindow default to DefaultMaxFailures and
	// DefaultFailureWindow when zero.
	MaxFailures   int
	FailureWindow time.Duration
	// Now defaults to time.Now.
	Now func() time.Time
}

// Authenticator checks PINs and issues and verifies session cookies. It is
// safe for concurrent use.
type Authenticator struct {
	caregiverPIN []byte
	masterPIN    []byte
	secret       []byte
	ttl          time.Duration
	secure       bool
	now          func() time.Time
	limiter      *limiter
}

// New validates cfg and returns an Authenticator.
func New(cfg Config) (*Authenticator, error) {
	if cfg.CaregiverPIN == "" || cfg.MasterPIN == "" {
		return nil, errors.New("caregiver PIN and master PIN must be set")
	}
	if cfg.CaregiverPIN == cfg.MasterPIN {
		return nil, errors.New("caregiver PIN and master PIN must differ")
	}
	if len(cfg.Secret) < 32 {
		return nil, errors.New("session secret must be at least 32 bytes")
	}
	if cfg.SessionTTL <= 0 {
		return nil, errors.New("session TTL must be positive")
	}
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = DefaultMaxFailures
	}
	if cfg.FailureWindow <= 0 {
		cfg.FailureWindow = DefaultFailureWindow
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Authenticator{
		caregiverPIN: []byte(cfg.CaregiverPIN),
		masterPIN:    []byte(cfg.MasterPIN),
		secret:       append([]byte(nil), cfg.Secret...),
		ttl:          cfg.SessionTTL,
		secure:       cfg.SecureCookie,
		now:          cfg.Now,
		limiter:      newLimiter(cfg.MaxFailures, cfg.FailureWindow),
	}, nil
}

// Login checks pin for a request from clientAddr. It returns ErrRateLimited
// without checking the PIN when clientAddr is blocked, and ErrInvalidPIN
// when the PIN is wrong; that attempt counts towards the block.
func (a *Authenticator) Login(clientAddr, pin string) (Role, error) {
	if !a.limiter.reserve(clientAddr, a.now()) {
		return "", ErrRateLimited
	}
	// Compare against both PINs every time so timing does not reveal which
	// one came closer.
	isCaregiver := subtle.ConstantTimeCompare([]byte(pin), a.caregiverPIN) == 1
	isClient := subtle.ConstantTimeCompare([]byte(pin), a.masterPIN) == 1
	switch {
	case isCaregiver:
		a.limiter.reset(clientAddr)
		return RoleCaregiver, nil
	case isClient:
		a.limiter.reset(clientAddr)
		return RoleClient, nil
	}
	return "", ErrInvalidPIN
}

// StartSession sets a session cookie for role on w.
func (a *Authenticator) StartSession(w http.ResponseWriter, role Role) {
	expires := a.now().Add(a.ttl)
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    a.sign(role, expires),
		Path:     "/",
		MaxAge:   int(a.ttl / time.Second),
		Expires:  expires,
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Session returns the role of a valid, unexpired session cookie on r.
func (a *Authenticator) Session(r *http.Request) (Role, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return "", false
	}
	return a.verify(c.Value)
}

// SecureCookies reports whether cookies should be marked Secure (HTTPS only).
func (a *Authenticator) SecureCookies() bool {
	return a.secure
}

// sign encodes "role|unix-expiry" and its HMAC as base64url, joined by ".".
func (a *Authenticator) sign(role Role, expires time.Time) string {
	payload := []byte(string(role) + "|" + strconv.FormatInt(expires.Unix(), 10))
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(a.mac(payload))
}

func (a *Authenticator) verify(value string) (Role, bool) {
	encPayload, encSig, ok := strings.Cut(value, ".")
	if !ok {
		return "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(encPayload)
	if err != nil {
		return "", false
	}
	sig, err := base64.RawURLEncoding.DecodeString(encSig)
	if err != nil || !hmac.Equal(sig, a.mac(payload)) {
		return "", false
	}
	roleStr, expStr, ok := strings.Cut(string(payload), "|")
	if !ok {
		return "", false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || !a.now().Before(time.Unix(exp, 0)) {
		return "", false
	}
	switch role := Role(roleStr); role {
	case RoleCaregiver, RoleClient:
		return role, true
	}
	return "", false
}

func (a *Authenticator) mac(payload []byte) []byte {
	m := hmac.New(sha256.New, a.secret)
	m.Write(payload)
	return m.Sum(nil)
}
