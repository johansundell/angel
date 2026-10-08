package main

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/johansundell/angel/store"
	"github.com/johansundell/angel/types"
)

func TestStart_InvalidTimeoutReturnsError(t *testing.T) {
	settings := types.AppSettings{Port: ":8080", Timeout: 0}
	if err := settings.Validate(); err == nil {
		t.Fatal("expected validation to fail with an invalid timeout")
	}
}

func TestStart_DatabaseInitializationErrorReturnsError(t *testing.T) {
	originalConstructor := newSQLiteStore
	newSQLiteStore = func(string) (store.Store, error) {
		return nil, errors.New("database unavailable")
	}
	defer func() { newSQLiteStore = originalConstructor }()

	originalSettings := settings
	settings = types.AppSettings{Port: ":8080", Timeout: 15 * time.Second}
	defer func() { settings = originalSettings }()

	p := &program{}
	if err := p.run(make(chan error, 1)); err == nil {
		t.Fatal("expected run to return the database initialization error")
	}
}

func TestRun_WarnsAboutDeprecatedSessionTimeout(t *testing.T) {
	originalConstructor := newSQLiteStore
	newSQLiteStore = func(string) (store.Store, error) {
		return nil, errors.New("database unavailable")
	}
	defer func() { newSQLiteStore = originalConstructor }()
	originalSettings := settings
	defer func() { settings = originalSettings }()
	var out bytes.Buffer
	defer log.SetOutput(log.Writer())
	log.SetOutput(&out)

	for _, legacy := range []bool{false, true} {
		out.Reset()
		settings = types.AppSettings{Port: ":8080", Timeout: 15 * time.Second}
		settings.PIN.LegacySessionTimeout = legacy
		(&program{}).run(make(chan error, 1))
		if got := strings.Contains(out.String(), "SESSION_TIMEOUT is deprecated"); got != legacy {
			t.Errorf("legacy=%v: deprecation notice logged = %v; log:\n%s", legacy, got, out.String())
		}
	}
}

func TestStart_UsesConfiguredSqlitePath(t *testing.T) {
	var capturedPath string
	originalConstructor := newSQLiteStore
	newSQLiteStore = func(path string) (store.Store, error) {
		capturedPath = path
		return nil, errors.New("stop here")
	}
	defer func() { newSQLiteStore = originalConstructor }()

	originalSettings := settings
	settings = types.AppSettings{Port: ":8080", Timeout: 15 * time.Second, SqlitePath: "/custom/data/my.db"}
	defer func() { settings = originalSettings }()

	p := &program{}
	_ = p.run(make(chan error, 1))

	if capturedPath != "/custom/data/my.db" {
		t.Errorf("expected sqlite path /custom/data/my.db, got %q", capturedPath)
	}
}
