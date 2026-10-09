package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/johansundell/angel/backup"
	"github.com/johansundell/angel/types"
)

// runBackup is the backup subcommand:
// angel backup [-db file] [-dir dir] [-retention-days n].
// It prints the archive path, then prunes archives past the retention and
// reports them on stderr. It returns the process exit code.
func runBackup(args []string, s types.AppSettings, stdout, stderr io.Writer) int {
	opts, days, err := backupOptions(args, s, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "backup:", err)
		return 2
	}
	// One clock for the archive name and the pruning.
	now := time.Now()
	opts.Now = func() time.Time { return now }
	archive, err := backup.Run(context.Background(), opts)
	if err != nil {
		fmt.Fprintln(stderr, "backup failed:", err)
		return 1
	}
	fmt.Fprintln(stdout, archive)

	// Only after a backup succeeded, so a failing job never prunes. A bad
	// retention is checked here, not with the flags: a typo must cost the
	// pruning, never the backup.
	keep, err := retentionPeriod(days)
	if err != nil {
		fmt.Fprintln(stderr, "backup stored, but not pruning old archives:", err)
		return 1
	}
	removed, err := backup.Prune(opts.Dir, keep, now)
	for _, p := range removed {
		fmt.Fprintln(stderr, "removed", p)
	}
	if err != nil {
		fmt.Fprintln(stderr, "backup stored, but pruning old archives failed:", err)
		return 1
	}
	return 0
}

// backupOptions reads the backup flags and returns the backup options and how
// many days to keep archives, unchecked (see retentionPeriod). Flags win over SQLITE_PATH, BACKUP_DIR and
// RETENTION_DAYS; without either directory, backups go to a backups folder
// beside the database.
func backupOptions(args []string, s types.AppSettings, output io.Writer) (backup.Options, int, error) {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(output)
	db := fs.String("db", s.SqlitePath, "database file to back up (SQLITE_PATH)")
	dir := fs.String("dir", s.BackupDir, "directory for the archives (BACKUP_DIR; default: backups beside the database)")
	days := fs.Int("retention-days", s.BackupRetentionDays, "delete archives older than this many days, always keeping the newest (RETENTION_DAYS)")
	if err := fs.Parse(args); err != nil {
		return backup.Options{}, 0, err
	}
	if fs.NArg() > 0 {
		return backup.Options{}, 0, fmt.Errorf("unexpected arguments: %q", fs.Args())
	}
	opts := backup.Options{DBPath: *db, Dir: *dir}
	if opts.Dir == "" {
		opts.Dir = filepath.Join(filepath.Dir(opts.DBPath), "backups")
	}
	return opts, *days, nil
}

// maxRetentionDays keeps the retention well inside time.Duration, which
// overflows past about 106,751 days.
const maxRetentionDays = 36500

// retentionPeriod turns a retention in days into a period, refusing values
// outside 1 to maxRetentionDays. An invalid RETENTION_DAYS loads as 0.
func retentionPeriod(days int) (time.Duration, error) {
	if days < 1 || days > maxRetentionDays {
		return 0, fmt.Errorf("retention must be 1 to %d days (-retention-days or RETENTION_DAYS), got %d", maxRetentionDays, days)
	}
	return time.Duration(days) * 24 * time.Hour, nil
}
