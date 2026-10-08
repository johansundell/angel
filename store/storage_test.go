package store

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestSQLite(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestClose(t *testing.T) {
	s, err := NewSQLite(filepath.Join(t.TempDir(), "close.db"))
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := s.Ping(context.Background()); err == nil {
		t.Error("Expected Ping to fail after Close")
	}
}

func TestNewSQLite_Pragmas(t *testing.T) {
	s := newTestSQLite(t)

	var journalMode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("Expected journal_mode wal, got %q", journalMode)
	}

	var busyTimeout int
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("Failed to query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("Expected busy_timeout 5000, got %d", busyTimeout)
	}
}
