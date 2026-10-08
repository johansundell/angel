package main

import (
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johansundell/angel/types"
)

// freeAddr returns a loopback address with a port that is free right now.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}

// testPINSettings is a valid keypad configuration for service tests.
var testPINSettings = types.PINSettings{CaregiverPIN: "1234", MasterPIN: "987654", CaregiverSessionTTL: 20 * time.Minute, ClientSessionTTL: 8 * time.Hour}

func useTestSettings(t *testing.T, port string) {
	t.Helper()
	originalSettings := settings
	settings = types.AppSettings{
		Port:       port,
		Timeout:    15 * time.Second,
		SqlitePath: filepath.Join(t.TempDir(), "test_stop.db"),
		PIN:        testPINSettings,
	}
	t.Cleanup(func() { settings = originalSettings })
}

func TestStop_WaitsForShutdown(t *testing.T) {
	addr := freeAddr(t)
	useTestSettings(t, addr)

	p := newProgram()
	if err := p.startWorker(); err != nil {
		t.Fatalf("expected startup to succeed, got %v", err)
	}

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("expected the server to answer before Stop, got %v", err)
	}
	resp.Body.Close()

	if err := p.Stop(nil); err != nil {
		t.Fatalf("expected Stop to succeed, got %v", err)
	}

	// Stop must not return before the server has shut down.
	if conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("expected the port to refuse connections once Stop returned")
	}

	// A second Stop must not panic on closing p.exit again.
	if err := p.Stop(nil); err != nil {
		t.Errorf("expected a second Stop to return nil, got %v", err)
	}
}

type failingListener struct{}

func (failingListener) Accept() (net.Conn, error) { return nil, errors.New("accept failed") }
func (failingListener) Close() error              { return nil }
func (failingListener) Addr() net.Addr            { return &net.TCPAddr{} }

func TestRun_ServeFailureIsReported(t *testing.T) {
	useTestSettings(t, "127.0.0.1:0")

	originalListen := netListen
	netListen = func(string, string) (net.Listener, error) { return failingListener{}, nil }
	defer func() { netListen = originalListen }()

	p := newProgram()
	if err := p.startWorker(); err != nil {
		t.Fatalf("expected startup to succeed, got %v", err)
	}

	select {
	case err := <-p.failed:
		if !strings.Contains(err.Error(), "accept failed") {
			t.Errorf("expected the serve error to be reported, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("expected the serve failure to be reported on p.failed")
	}

	// run has already returned, so Stop must return its error without blocking.
	if err := p.Stop(nil); err == nil || !strings.Contains(err.Error(), "accept failed") {
		t.Errorf("expected Stop to return the serve error, got %v", err)
	}
}
