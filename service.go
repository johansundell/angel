package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/johansundell/angel/auth"
	"github.com/johansundell/angel/handlers"
	"github.com/johansundell/angel/logging"
	"github.com/johansundell/angel/logqueue"
	"github.com/johansundell/angel/router"
	"github.com/johansundell/angel/store"
	"github.com/johansundell/angel/types"
	"github.com/kardianos/service"
)

var logger service.Logger

type program struct {
	exit     chan struct{}
	done     chan struct{} // closed when run returns; runErr holds its result
	runErr   error
	stopOnce sync.Once
	// failed receives the error when the HTTP server stops on its own after a
	// successful start, so main can exit and let the service manager restart us.
	failed chan error
}

func newProgram() *program {
	return &program{failed: make(chan error, 1)}
}

// injectable constructors so tests can mock storage initialization and listening
var newSQLiteStore = func(path string) (store.Store, error) {
	s, err := store.NewSQLite(path)
	if err != nil {
		return nil, err
	}
	return s, nil
}
var newMySQLStore = func(cfg types.MySQLSettings) (store.Store, error) {
	s, err := store.NewMySQL(cfg)
	if err != nil {
		return nil, err
	}
	return s, nil
}
var newFileMakerStore = func(ctx context.Context, cfg types.FileMakerSettings) (store.Store, error) {
	s, err := store.NewFileMaker(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return s, nil
}
var netListen = net.Listen

// routesFor picks the routes to serve, so tests can add their own.
var routesFor = router.GetRoutes

func (p *program) Start(s service.Service) error {
	loadSettings()
	if err := settings.Validate(); err != nil {
		logError("invalid configuration: %v", err)
		return err
	}
	if service.Interactive() {
		logInfo("Running in terminal.")
	} else {
		logInfo("Running under service manager.")
	}
	return p.startWorker()
}

// startWorker runs the service worker in the background. Start should not
// block while the service is serving requests, but it must report
// initialization failures to the service manager.
func (p *program) startWorker() error {
	p.exit = make(chan struct{})
	p.done = make(chan struct{})
	startup := make(chan error, 1)
	go func() {
		p.runErr = p.run(startup)
		close(p.done)
	}()
	return <-startup
}

func (p *program) run(startup chan<- error) error {
	logInfo("I'm running %v, with version %v.", service.Platform(), Version)
	if settings.PIN.LegacySessionTimeout {
		logWarning("SESSION_TIMEOUT is deprecated; rename it to CAREGIVER_SESSION_TIMEOUT.")
	}

	st, pingTimeout, err := openStore()
	if err != nil {
		logError("failed to initialize %s storage: %v", settings.Storage, err)
		startup <- err
		return err
	}
	defer st.Close()

	pingCtx, cancelPing := context.WithTimeout(context.Background(), pingTimeout)
	err = st.Ping(pingCtx)
	cancelPing()
	if err != nil {
		logError("database ping failed: %v", err)
		startup <- err
		return err
	}
	ensureAuthToken()

	// Request logs are written in the background. The deferred Close runs after
	// the HTTP server has shut down, drains the queue for up to 5 more seconds
	// and runs before the store is closed.
	logQueue := logqueue.New(st, appLogger(), settings.Debug)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logQueue.Close(ctx)
	}()

	ensureSessionSecret()
	authn, err := auth.New(auth.Config{
		CaregiverPIN: settings.PIN.CaregiverPIN,
		MasterPIN:    settings.PIN.MasterPIN,
		Secret:       []byte(settings.PIN.SessionSecret),
		SecureCookie: settings.PIN.SecureCookie,

		CaregiverSessionTTL: settings.PIN.CaregiverSessionTTL,
		ClientSessionTTL:    settings.PIN.ClientSessionTTL,
	})
	if err != nil {
		logError("failed to set up PIN authentication: %v", err)
		startup <- err
		return err
	}

	notes, closeNotes, err := openNotes(st)
	if err != nil {
		logError("failed to open note storage: %v", err)
		startup <- err
		return err
	}
	defer closeNotes()

	handler, err := handlers.NewHandler(st, settings.UseFileSystem, embeddedTemplates, nameOfService, Version,
		handlers.WithAuth(authn), handlers.WithNotes(notes))
	if err != nil {
		logError("failed to create handlers: %v", err)
		startup <- err
		return err
	}

	routerEngine, err := router.NewRouter(router.Config{
		Handler:  handler,
		LogSink:  logQueue,
		Settings: settings,
		Assets:   embeddedAssets,
		Version:  Version,
		Logger:   appLogger(),
		Routes:   routesFor(handler),
	})
	if err != nil {
		logError("failed to create router: %v", err)
		startup <- err
		return err
	}
	timeoutHandler := http.TimeoutHandler(routerEngine, settings.Timeout, "Timeout")
	var rootHandler http.Handler = timeoutHandler
	if Version != "" {
		rootHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Version", Version)
			timeoutHandler.ServeHTTP(w, r)
		})
	}

	srv := &http.Server{
		Handler: rootHandler,
		Addr:    settings.Port,
	}
	listener, err := netListen("tcp", settings.Port)
	if err != nil {
		logError("failed to listen on %s: %v", settings.Port, err)
		startup <- err
		return err
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(listener) }()
	startup <- nil

	select {
	case err := <-serveErr:
		logError("HTTP server stopped: %v", err)
		select {
		case p.failed <- err:
		default:
		}
		return err
	case <-p.exit:
	}

	// The shutdown deadline is shorter than TIMEOUT (default 15s), so a slow
	// request can still be cut off when the service stops.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logError("HTTP server shutdown failed: %v", err)
		return err
	}
	return nil
}

func wrapStore[T store.Store](s T, err error) (store.Store, error) {
	if err != nil {
		return nil, err
	}
	return s, nil
}

// storeOpeners opens each STORAGE backend. Keep it in step with
// types.StorageBackends (TestStoreOpenersMatchStorageBackends).
var storeOpeners = map[string]func() (store.Store, time.Duration, error){
	types.StorageSQLite: func() (store.Store, time.Duration, error) {
		s, err := newSQLiteStore(settings.SqlitePath)
		return s, 5 * time.Second, err
	},
	types.StorageMySQL: func() (store.Store, time.Duration, error) {
		s, err := newMySQLStore(settings.MySQL)
		return s, 5 * time.Second, err
	},
	types.StorageFileMaker: func() (store.Store, time.Duration, error) {
		fm := settings.FileMaker
		if fm.InsecureSkipVerify {
			logWarning("FMS_INSECURE_SKIP_VERIFY is set: the FileMaker Server certificate is NOT verified, so credentials can be intercepted. Use FMS_CA_FILE instead.")
		}
		ctx, cancel := context.WithTimeout(context.Background(), fm.Timeout)
		defer cancel()
		s, err := newFileMakerStore(ctx, fm)
		return s, fm.Timeout, err
	},
}

// openStore opens the storage backend chosen with STORAGE.
func openStore() (store.Store, time.Duration, error) {
	open, ok := storeOpeners[settings.Storage]
	if !ok {
		return nil, 0, fmt.Errorf("unsupported STORAGE %q", settings.Storage)
	}
	return open()
}

// openNotes returns where Daily Notes are kept. They always live in SQLite
// (ADR-0003): the main store when STORAGE=sqlite, otherwise a separate SQLite
// file at SQLITE_PATH. The returned func closes what openNotes opened.
func openNotes(st store.Store) (store.NoteStore, func() error, error) {
	if ns, ok := st.(store.NoteStore); ok {
		return ns, func() error { return nil }, nil
	}
	s, err := store.NewSQLite(settings.SqlitePath)
	if err != nil {
		return nil, nil, err
	}
	return s, s.Close, nil
}

// Stop waits for graceful shutdown to finish: once it returns,
// kardianos/service returns from Run and the process exits.
func (p *program) Stop(s service.Service) error {
	logInfo("I'm Stopping!")
	if p.done == nil {
		return nil
	}
	p.stopOnce.Do(func() { close(p.exit) })
	<-p.done
	return p.runErr
}

// ensureAuthToken generates a temporary random token when AUTH_TOKEN is not
// configured, so protected routes stay locked down. The token is logged
// because it is the only way to call those routes during this run.
func ensureAuthToken() {
	if settings.AuthToken != "" {
		return
	}
	settings.AuthToken = rand.Text()
	logWarning("AUTH_TOKEN is not set; using temporary token for this run: %s", settings.AuthToken)
}

// ensureSessionSecret generates a random signing secret when SESSION_SECRET
// is not configured. Sessions then end whenever the service restarts, which
// only means caregivers enter the PIN again. Unlike the AUTH_TOKEN, the secret
// is never logged.
func ensureSessionSecret() {
	if settings.PIN.SessionSecret != "" {
		return
	}
	settings.PIN.SessionSecret = rand.Text() + rand.Text()
	logInfo("SESSION_SECRET is not set; sessions will not survive a restart.")
}

// serviceLogger adapts the kardianos/service logger, whose methods return an
// error, to logging.Logger.
type serviceLogger struct {
	l service.Logger
}

func (s serviceLogger) Infof(format string, v ...interface{})    { s.l.Infof(format, v...) }
func (s serviceLogger) Warningf(format string, v ...interface{}) { s.l.Warningf(format, v...) }
func (s serviceLogger) Errorf(format string, v ...interface{})   { s.l.Errorf(format, v...) }

// appLogger returns the service manager's logger, or the standard logger
// before one is set up (tests, early startup).
func appLogger() logging.Logger {
	if logger == nil {
		return logging.Std{}
	}
	return serviceLogger{l: logger}
}

func logInfo(format string, v ...interface{}) {
	appLogger().Infof(format, v...)
}

func logWarning(format string, v ...interface{}) {
	appLogger().Warningf(format, v...)
}

func logError(format string, v ...interface{}) {
	appLogger().Errorf(format, v...)
}
