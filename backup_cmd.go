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
	opts, keep, err := backupOptions(args, s, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "backup:", err)
		return 2
	}
	archive, err := backup.Run(context.Background(), opts)
	if err != nil {
		fmt.Fprintln(stderr, "backup failed:", err)
		return 1
	}
	fmt.Fprintln(stdout, archive)

	// Only after a backup succeeded, so a failing job never prunes.
	removed, err := backup.Prune(opts.Dir, keep, time.Now())
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
// long to keep archives. Flags win over SQLITE_PATH, BACKUP_DIR and
// RETENTION_DAYS; without either directory, backups go to a backups folder
// beside the database.
func backupOptions(args []string, s types.AppSettings, output io.Writer) (backup.Options, time.Duration, error) {
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
	if *days < 1 {
		return backup.Options{}, 0, fmt.Errorf("retention must be a whole number of days, at least 1 (-retention-days or RETENTION_DAYS); got %d", *days)
	}
	opts := backup.Options{DBPath: *db, Dir: *dir}
	if opts.Dir == "" {
		opts.Dir = filepath.Join(filepath.Dir(opts.DBPath), "backups")
	}
	return opts, time.Duration(*days) * 24 * time.Hour, nil
}
