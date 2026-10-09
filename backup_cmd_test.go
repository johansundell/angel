package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/johansundell/angel/store"
)

func TestBackupOptions(t *testing.T) {
	tests := []struct {
		name               string
		args               []string
		sqlitePath, envDir string
		wantDB, wantDir    string
	}{
		{
			name:       "defaults put backups beside the database",
			sqlitePath: "/app/data/angel.db",
			wantDB:     "/app/data/angel.db",
			wantDir:    "/app/data/backups",
		},
		{
			name:       "BACKUP_DIR sets the directory",
			sqlitePath: "/app/data/angel.db",
			envDir:     "/srv/backups",
			wantDB:     "/app/data/angel.db",
			wantDir:    "/srv/backups",
		},
		{
			name:       "flags win over the environment",
			args:       []string{"-db", "/tmp/other.db", "-dir", "/tmp/out"},
			sqlitePath: "/app/data/angel.db",
			envDir:     "/srv/backups",
			wantDB:     "/tmp/other.db",
			wantDir:    "/tmp/out",
		},
		{
			name:       "-db alone moves the default directory with it",
			args:       []string{"-db", "/tmp/other.db"},
			sqlitePath: "/app/data/angel.db",
			wantDB:     "/tmp/other.db",
			wantDir:    "/tmp/backups",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := backupOptions(tt.args, tt.sqlitePath, tt.envDir, io.Discard)
			if err != nil {
				t.Fatalf("backupOptions: %v", err)
			}
			if opts.DBPath != tt.wantDB || opts.Dir != tt.wantDir {
				t.Errorf("got db=%q dir=%q, want db=%q dir=%q", opts.DBPath, opts.Dir, tt.wantDB, tt.wantDir)
			}
		})
	}
}

func TestBackupOptionsRejectsBadArgs(t *testing.T) {
	for _, args := range [][]string{{"-nope"}, {"extra"}} {
		if _, err := backupOptions(args, "/app/data/angel.db", "", io.Discard); err == nil {
			t.Errorf("backupOptions(%q) should fail", args)
		}
	}
}

// TestRunBackupCommand runs the backup subcommand end to end against a live
// database named by SQLITE_PATH and BACKUP_DIR.
func TestRunBackupCommand(t *testing.T) {
	defer unsetEnv("SQLITE_PATH", "BACKUP_DIR")()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "angel.db")
	s, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	backups := filepath.Join(dir, "out")
	os.Setenv("SQLITE_PATH", dbPath)
	os.Setenv("BACKUP_DIR", backups)

	var stdout, stderr strings.Builder
	if code := runBackup(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("runBackup exit %d, stderr: %s", code, stderr.String())
	}
	entries, err := os.ReadDir(backups)
	if err != nil || len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "angel_") || !strings.HasSuffix(entries[0].Name(), ".db.gz") {
		t.Fatalf("expected one angel_<timestamp>.db.gz in %s, got %v (%v)", backups, entries, err)
	}
	if got := strings.TrimSpace(stdout.String()); got != filepath.Join(backups, entries[0].Name()) {
		t.Errorf("stdout = %q, want the archive path", got)
	}
}

func TestRunBackupCommandFails(t *testing.T) {
	defer unsetEnv("SQLITE_PATH", "BACKUP_DIR")()
	dir := t.TempDir()
	var stdout, stderr strings.Builder
	code := runBackup([]string{"-db", filepath.Join(dir, "missing.db")}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("runBackup should fail for a missing database")
	}
	if stderr.Len() == 0 {
		t.Error("runBackup should say why it failed")
	}
}
