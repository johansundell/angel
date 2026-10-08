package router

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/johansundell/angel/auth"
	"github.com/johansundell/angel/handlers"
	"github.com/johansundell/angel/httperror"
	"github.com/johansundell/angel/logging"
	"github.com/johansundell/angel/types"
	"github.com/johansundell/angel/utils"
)

// HandlerFuncWithError defines a handler function that returns an error
type HandlerFuncWithError func(*gin.Context) error

// Route defines the configuration for a single HTTP endpoint
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc HandlerFuncWithError
	// Role, when set, requires a PIN session for that role; other visitors
	// are redirected to the entry screen.
	Role auth.Role
}

// Routes is a collection of Route definitions
type Routes []Route

// Logger is the leveled logger middleware reports to.
type Logger = logging.Logger

// Config contains the dependencies and settings required to construct a router
type Config struct {
	Handler  *handlers.Handler
	Auth     *auth.Authenticator // Required when any route has a Role
	Settings types.AppSettings
	Assets   fs.FS
	Version  string
	Logger   Logger // Optional: defaults to standard logger when nil
	Routes   Routes // Optional: defaults to GetRoutes(Handler) when nil
}

// NewRouter creates a new web handler with middleware and registered routes
func NewRouter(cfg Config) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)

	if cfg.Handler == nil {
		return nil, errors.New("handler must be provided")
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// ClientIP keys the PIN rate limiter, so forwarding headers are only
	// believed from configured proxies; otherwise anyone could pick their
	// own address with X-Forwarded-For.
	if err := router.SetTrustedProxies(cfg.Settings.TrustedProxies); err != nil {
		return nil, fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}
	l := logging.OrStd(cfg.Logger)
	warnUntrusted, err := untrustedProxyWarning(cfg.Settings.TrustedProxies, l)
	if err != nil {
		return nil, fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}
	router.Use(warnUntrusted)

	if cfg.Version != "" {
		router.Use(func(c *gin.Context) {
			c.Header("X-Version", cfg.Version)
			c.Next()
		})
	}

	router.NoRoute(func(c *gin.Context) {
		c.String(http.StatusNotFound, http.StatusText(http.StatusNotFound))
	})
	router.HandleMethodNotAllowed = true
	router.NoMethod(func(c *gin.Context) {
		c.String(http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
	})

	routes := cfg.Routes
	if routes == nil {
		routes = GetRoutes(cfg.Handler)
	}

	debug := cfg.Settings.Debug
	if debug {
		router.Use(AccessLog(l))
	}

	for _, route := range routes {
		if route.Role != "" && cfg.Auth == nil {
			return nil, fmt.Errorf("PIN authentication must be configured for route %q", route.Name)
		}

		fn := route.HandlerFunc

		if route.Role != "" {
			fn = RequireRole(cfg.Auth, route.Role)(fn)
		}

		router.Handle(route.Method, route.Pattern, WrapHandler(fn))
		if debug {
			l.Infof("route %s %s (%s)", route.Method, route.Pattern, route.Name)
		}
	}

	// Static files
	fsys, err := getStaticFiles(cfg.Assets, cfg.Settings.UseFileSystem)
	if err != nil {
		return nil, err
	}
	router.StaticFS("/assets", fsys)

	// Browsers look for the web app manifest by the link in the page head;
	// it sits at the root so its scope can cover the whole site.
	router.GET("/manifest.webmanifest", func(c *gin.Context) {
		c.Header("Content-Type", "application/manifest+json")
		c.FileFromFS("manifest.webmanifest", fsys)
	})

	sw, err := serviceWorker(cfg.Version)
	if err != nil {
		return nil, err
	}
	router.GET("/sw.js", sw)

	return router, nil
}

// AccessLog logs one line per request (method, path, status, duration and
// client IP) through l. NewRouter adds it when DEBUG=true. The query string
// is left out so tokens in URLs don't end up in the log.
func AccessLog(l Logger) gin.HandlerFunc {
	l = logging.OrStd(l)
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		l.Infof("%s %s %d %v %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), time.Since(start).Round(time.Microsecond), c.ClientIP())
	}
}

// untrustedProxyWarning logs a warning, once, the first time a request
// carries a forwarding header from a peer outside trusted. That usually means
// the service runs behind a proxy without TRUSTED_PROXIES, so every visitor
// shares the proxy's address and one PIN rate limit. An empty trusted list is
// not warned about on its own: it is correct when clients connect directly.
func untrustedProxyWarning(trusted []string, l Logger) (gin.HandlerFunc, error) {
	l = logging.OrStd(l)
	nets := make([]*net.IPNet, 0, len(trusted))
	for _, p := range trusted {
		if !strings.Contains(p, "/") {
			// A bare address trusts only itself, as in gin.
			if ip := net.ParseIP(p); ip != nil && ip.To4() != nil {
				p += "/32"
			} else {
				p += "/128"
			}
		}
		_, n, err := net.ParseCIDR(p)
		if err != nil {
			return nil, err
		}
		nets = append(nets, n)
	}
	var warned atomic.Bool
	return func(c *gin.Context) {
		if warned.Load() {
			return
		}
		if c.GetHeader("X-Forwarded-For") == "" && c.GetHeader("CF-Connecting-IP") == "" {
			return
		}
		peer := net.ParseIP(c.RemoteIP())
		if peer == nil {
			return // not a TCP peer, e.g. a Unix socket, which gin always trusts
		}
		for _, n := range nets {
			if n.Contains(peer) {
				return
			}
		}
		if warned.CompareAndSwap(false, true) {
			l.Warningf("forwarding header from %s, which is not in TRUSTED_PROXIES: every visitor gets that address and shares one PIN rate limit; add the proxy's address to TRUSTED_PROXIES (see examples/reverse-proxy/)", c.RemoteIP())
		}
	}, nil
}

// RequireRole lets the request through only with a valid session for role;
// anyone else is sent back to the entry screen.
func RequireRole(a *auth.Authenticator, role auth.Role) func(HandlerFuncWithError) HandlerFuncWithError {
	return func(inner HandlerFuncWithError) HandlerFuncWithError {
		return func(c *gin.Context) error {
			if a == nil {
				return httperror.ReturnWithHTTPStatus(errors.New("PIN authentication is not configured"), http.StatusInternalServerError)
			}
			if got, ok := a.Session(c.Request); !ok || got != role {
				c.Redirect(http.StatusSeeOther, "/")
				return nil
			}
			// Private household notes must not linger in shared caches or
			// the back/forward cache after the session ends.
			c.Header("Cache-Control", "no-store")
			return inner(c)
		}
	}
}

func getStaticFiles(assets fs.FS, useLocal bool) (http.FileSystem, error) {
	if useLocal {
		assetDir := filepath.Join(utils.GetBinaryBasePath(), "assets")
		return http.FS(os.DirFS(assetDir)), nil
	}

	if assets == nil {
		return nil, errors.New("embedded assets filesystem is nil")
	}

	fsys, err := fs.Sub(assets, "assets")
	if err != nil {
		return nil, err
	}
	return http.FS(fsys), nil
}

// WrapHandler wraps a HandlerFuncWithError into a Gin HandlerFunc
func WrapHandler(inner HandlerFuncWithError) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := inner(c); err != nil {
			c.String(httperror.HTTPStatus(err), httperror.StatusText(err))
		}
	}
}
