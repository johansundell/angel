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
			a, err := backupOptions(tt.args, types.AppSettings{SqlitePath: tt.sqlitePath, BackupDir: tt.envDir, BackupRetentionDays: 7}, io.Discard)
			if err != nil {
				t.Fatalf("backupOptions: %v", err)
			}
			opts := a.opts
			if opts.DBPath != tt.wantDB || opts.Dir != tt.wantDir {
				t.Errorf("got db=%q dir=%q, want db=%q dir=%q", opts.DBPath, opts.Dir, tt.wantDB, tt.wantDir)
			}
		})
	}
}

func TestBackupOptionsRejectsBadArgs(t *testing.T) {
	for _, args := range [][]string{{"-nope"}, {"extra"}} {
		if _, err := backupOptions(args, types.AppSettings{SqlitePath: "/app/data/angel.db", BackupRetentionDays: 7}, io.Discard); err == nil {
			t.Errorf("backupOptions(%q) should fail", args)
		}
	}
}

func TestBackupOptionsRetention(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		envDays int
		want    string
	}{
		{"RETENTION_DAYS sets it", nil, 14, "14"},
		{"the flag wins", []string{"-retention-days", "3"}, 14, "3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := backupOptions(tt.args, types.AppSettings{SqlitePath: "/app/data/angel.db", BackupRetentionDays: tt.envDays}, io.Discard)
			if err != nil {
				t.Fatalf("backupOptions: %v", err)
			}
			if a.retention != tt.want {
				t.Errorf("retention = %q days, want %q", a.retention, tt.want)
			}
		})
	}
}

func TestRetentionPeriod(t *testing.T) {
	if keep, err := retentionPeriod("7"); err != nil || keep != 7*24*time.Hour {
		t.Errorf("retentionPeriod(7) = %v, %v", keep, err)
	}
	// 0 is also what an invalid RETENTION_DAYS loads as. Huge values would
	// overflow time.Duration and wrap to a small or negative period.
	for _, days := range []string{"0", "-3", "36501", "200000", "week", "7.5", ""} {
		if _, err := retentionPeriod(days); err == nil || !strings.Contains(err.Error(), "RETENTION_DAYS") {
			t.Errorf("retentionPeriod(%q) err = %v, want one naming RETENTION_DAYS", days, err)
		}
	}
}

// TestRunBackupBadRetentionStillBacksUp checks that a bad retention setting
// skips pruning and fails the run, but never costs the backup itself.
func TestRunBackupBadRetentionStillBacksUp(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{"invalid RETENTION_DAYS", nil},
		{"malformed flag", []string{"-retention-days", "week"}},
		{"zero flag", []string{"-retention-days", "0"}},
	} {
		t.Run(tt.name, func(t *testing.T) { testBadRetentionStillBacksUp(t, tt.args) })
	}
}

func testBadRetentionStillBacksUp(t *testing.T, args []string) {
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
	// BackupRetentionDays 0 is what an invalid RETENTION_DAYS loads as; with
	// a retention flag, the flag decides.
	if code := runBackup(args, types.AppSettings{SqlitePath: dbPath}, &stdout, &stderr); code != 1 {
		t.Fatalf("runBackup exit %d with a bad retention, want 1; stderr: %s", code, stderr.String())
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

func TestCronSnippet(t *testing.T) {
	tests := []struct {
		name string
		exe  string
		args []string
		want string
	}{
		{
			name: "plain",
			exe:  "/opt/angel/angel",
			want: "0 3 * * * /opt/angel/angel backup 2>&1 | logger -t angel-backup",
		},
		{
			name: "flags are passed on, quoted for the shell",
			exe:  "/opt/angel/angel",
			args: []string{"-dir", "/srv/my backups", "-retention-days", "14"},
			want: "0 3 * * * /opt/angel/angel backup -dir '/srv/my backups' -retention-days 14 2>&1 | logger -t angel-backup",
		},
		{
			// cron turns an unescaped % into a newline.
			name: "percent and quote",
			exe:  "/opt/my angel/angel",
			args: []string{"-dir", "/srv/50%/it's"},
			want: `0 3 * * * '/opt/my angel/angel' backup -dir '/srv/50\%/it'\''s' 2>&1 | logger -t angel-backup`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cronSnippet(tt.exe, tt.args); got != tt.want {
				t.Errorf("cronSnippet:\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

// TestRunBackupCronSnippet checks that -cron-snippet (also --cron-snippet)
// prints the crontab line with the other flags, and takes no backup.
func TestRunBackupCronSnippet(t *testing.T) {
	dir := t.TempDir()
	for _, flagName := range []string{"-cron-snippet", "--cron-snippet"} {
		var stdout, stderr strings.Builder
		args := []string{"-dir", dir, flagName, "-retention-days", "14"}
		if code := runBackup(args, types.AppSettings{SqlitePath: filepath.Join(dir, "missing.db"), BackupRetentionDays: 7}, &stdout, &stderr); code != 0 {
			t.Fatalf("%s: exit %d, stderr: %s", flagName, code, stderr.String())
		}
		line := strings.TrimSpace(stdout.String())
		if !strings.HasPrefix(line, "0 3 * * * /") || !strings.Contains(line, " backup -dir "+dir+" -retention-days 14 ") {
			t.Errorf("%s printed %q", flagName, line)
		}
		if strings.Contains(line, "cron-snippet") {
			t.Errorf("%s: the crontab line must not print the snippet again: %q", flagName, line)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("%s took a backup: %v", flagName, entries)
		}
	}
}
