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
	Secret []byte
	// CaregiverSessionTTL and ClientSessionTTL are how long a session of
	// each role lasts. Sessions have a fixed expiry and are never renewed.
	CaregiverSessionTTL time.Duration
	ClientSessionTTL    time.Duration
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
	caregiverTTL time.Duration
	clientTTL    time.Duration
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
	if cfg.CaregiverSessionTTL <= 0 || cfg.ClientSessionTTL <= 0 {
		return nil, errors.New("session TTLs must be positive")
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
		caregiverTTL: cfg.CaregiverSessionTTL,
		clientTTL:    cfg.ClientSessionTTL,
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

// StartSession sets a session cookie for role on w, lasting that role's
// session lifetime.
func (a *Authenticator) StartSession(w http.ResponseWriter, role Role) {
	// Anything but a Client session gets the shorter Caregiver lifetime.
	ttl := a.caregiverTTL
	if role == RoleClient {
		ttl = a.clientTTL
	}
	expires := a.now().Add(ttl)
	c := a.cookie(a.sign(role, expires), int(ttl/time.Second))
	c.Expires = expires
	http.SetCookie(w, c)
}

// EndSession tells the browser on w to delete the session cookie. Sessions
// are stateless, so a copy of the cookie kept elsewhere stays valid until it
// expires; this ends the session on the device that logs out.
func (a *Authenticator) EndSession(w http.ResponseWriter) {
	http.SetCookie(w, a.cookie("", -1))
}

// cookie builds the session cookie. Start and end share it because a browser
// only deletes a cookie whose name, path and domain match the one it holds.
func (a *Authenticator) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	}
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
