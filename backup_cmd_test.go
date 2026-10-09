package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johansundell/angel/store"
	"github.com/johansundell/angel/types"
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
			opts, _, err := backupOptions(tt.args, types.AppSettings{SqlitePath: tt.sqlitePath, BackupDir: tt.envDir, BackupRetentionDays: 7}, io.Discard)
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
		if _, _, err := backupOptions(args, types.AppSettings{SqlitePath: "/app/data/angel.db", BackupRetentionDays: 7}, io.Discard); err == nil {
			t.Errorf("backupOptions(%q) should fail", args)
		}
	}
}

func TestBackupOptionsRetention(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		envDays int
		want    int
	}{
		{"RETENTION_DAYS sets it", nil, 14, 14},
		{"the flag wins", []string{"-retention-days", "3"}, 14, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, days, err := backupOptions(tt.args, types.AppSettings{SqlitePath: "/app/data/angel.db", BackupRetentionDays: tt.envDays}, io.Discard)
			if err != nil {
				t.Fatalf("backupOptions: %v", err)
			}
			if days != tt.want {
				t.Errorf("retention = %d days, want %d", days, tt.want)
			}
		})
	}
}

func TestRetentionPeriod(t *testing.T) {
	if keep, err := retentionPeriod(7); err != nil || keep != 7*24*time.Hour {
		t.Errorf("retentionPeriod(7) = %v, %v", keep, err)
	}
	// 0 is also what an invalid RETENTION_DAYS loads as. Huge values would
	// overflow time.Duration and wrap to a small or negative period.
	for _, days := range []int{0, -3, 36501, 200000} {
		if _, err := retentionPeriod(days); err == nil || !strings.Contains(err.Error(), "RETENTION_DAYS") {
			t.Errorf("retentionPeriod(%d) err = %v, want one naming RETENTION_DAYS", days, err)
		}
	}
}

// TestRunBackupBadRetentionStillBacksUp checks that a bad retention setting
// skips pruning and fails the run, but never costs the backup itself.
func TestRunBackupBadRetentionStillBacksUp(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "angel.db")
	s, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backups := filepath.Join(dir, "backups")
	if err := os.Mkdir(backups, 0o750); err != nil {
		t.Fatal(err)
	}
	old := "angel_" + time.Now().UTC().AddDate(0, 0, -30).Format("20060102T150405Z") + ".db.gz"
	if err := os.WriteFile(filepath.Join(backups, old), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	if code := runBackup(nil, types.AppSettings{SqlitePath: dbPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("runBackup exit %d with RETENTION_DAYS invalid, want 1", code)
	}
	if archive := strings.TrimSpace(stdout.String()); archive == "" {
		t.Error("the backup should still be stored and printed")
	} else if _, err := os.Stat(archive); err != nil {
		t.Errorf("archive missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backups, old)); err != nil {
		t.Errorf("nothing should be pruned with a bad retention: %v", err)
	}
	if !strings.Contains(stderr.String(), "RETENTION_DAYS") {
		t.Errorf("stderr should name RETENTION_DAYS, got %q", stderr.String())
	}
}

func TestLoadSettings_RetentionDays(t *testing.T) {
	defer unsetEnv("RETENTION_DAYS")()
	for _, tt := range []struct {
		env  string
		want int
	}{{"", 7}, {"14", 14}, {"week", 0}} {
		if tt.env == "" {
			os.Unsetenv("RETENTION_DAYS")
		} else {
			os.Setenv("RETENTION_DAYS", tt.env)
		}
		loadSettings(filepath.Join(t.TempDir(), "missing.env"))
		if settings.BackupRetentionDays != tt.want {
			t.Errorf("RETENTION_DAYS=%q: BackupRetentionDays = %d, want %d", tt.env, settings.BackupRetentionDays, tt.want)
		}
	}
}

// TestRunBackupPrunes checks that a backup removes archives past the
// retention and reports them, keeping the recent ones.
func TestRunBackupPrunes(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "angel.db")
	s, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backups := filepath.Join(dir, "backups")
	if err := os.Mkdir(backups, 0o750); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	old := "angel_" + now.AddDate(0, 0, -10).Format("20060102T150405Z") + ".db.gz"
	recent := "angel_" + now.AddDate(0, 0, -2).Format("20060102T150405Z") + ".db.gz"
	for _, name := range []string{old, recent} {
		if err := os.WriteFile(filepath.Join(backups, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr strings.Builder
	if code := runBackup(nil, types.AppSettings{SqlitePath: dbPath, BackupRetentionDays: 7}, &stdout, &stderr); code != 0 {
		t.Fatalf("runBackup exit %d, stderr: %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(backups, old)); !os.IsNotExist(err) {
		t.Errorf("%s is past the retention and should be gone", old)
	}
	if _, err := os.Stat(filepath.Join(backups, recent)); err != nil {
		t.Errorf("%s is within the retention and should stay: %v", recent, err)
	}
	if !strings.Contains(stderr.String(), old) {
		t.Errorf("stderr should report the removed archive, got %q", stderr.String())
	}
}

// TestRunBackupCommand runs the backup subcommand end to end against a live
// database.
func TestRunBackupCommand(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "angel.db")
	s, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backups := filepath.Join(dir, "out")

	var stdout, stderr strings.Builder
	if code := runBackup(nil, types.AppSettings{SqlitePath: dbPath, BackupDir: backups, BackupRetentionDays: 7}, &stdout, &stderr); code != 0 {
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
	missing := filepath.Join(t.TempDir(), "missing.db")
	var stdout, stderr strings.Builder
	if code := runBackup(nil, types.AppSettings{SqlitePath: missing, BackupRetentionDays: 7}, &stdout, &stderr); code != 1 {
		t.Fatalf("runBackup exit %d for a missing database, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Error("runBackup should say why it failed")
	}
}

func TestLoadSettings_BackupDir(t *testing.T) {
	defer unsetEnv("BACKUP_DIR")()
	os.Setenv("BACKUP_DIR", "/srv/backups")
	loadSettings(filepath.Join(t.TempDir(), "missing.env"))
	if settings.BackupDir != "/srv/backups" {
		t.Errorf("BackupDir = %q, want /srv/backups", settings.BackupDir)
	}
}
