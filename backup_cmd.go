package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/johansundell/angel/backup"
	"github.com/johansundell/angel/types"
)

// runBackup is the backup subcommand:
// angel backup [-db file] [-dir dir] [-retention-days n] [-cron-snippet].
// It prints the archive path, then prunes archives past the retention and
// reports them on stderr. With -cron-snippet it prints a crontab line for a
// daily backup instead. It returns the process exit code.
func runBackup(args []string, s types.AppSettings, stdout, stderr io.Writer) int {
	a, err := backupOptions(args, s, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "backup:", err)
		return 2
	}
	if a.cronSnippet {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(stderr, "backup:", err)
			return 1
		}
		fmt.Fprintln(stdout, cronSnippet(exe, a.forward))
		return 0
	}
	opts := a.opts
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
	keep, err := retentionPeriod(a.retention)
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

// backupArgs is what the backup flags ask for.
type backupArgs struct {
	opts      backup.Options
	retention string // days, unchecked (see retentionPeriod)
	// cronSnippet asks for a crontab line instead of a backup; forward holds
	// the other flags given, for that line.
	cronSnippet bool
	forward     []string
}

// backupOptions reads the backup flags. Flags win over SQLITE_PATH,
// BACKUP_DIR and RETENTION_DAYS; without either directory, backups go to a
// backups folder beside the database.
func backupOptions(args []string, s types.AppSettings, output io.Writer) (backupArgs, error) {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(output)
	db := fs.String("db", s.SqlitePath, "database file to back up (SQLITE_PATH)")
	dir := fs.String("dir", s.BackupDir, "directory for the archives (BACKUP_DIR; default: backups beside the database)")
	// A string, so a malformed value reaches retentionPeriod after the backup
	// instead of failing the flag parsing before it.
	days := fs.String("retention-days", strconv.Itoa(s.BackupRetentionDays), "delete archives older than this many days, always keeping the newest (RETENTION_DAYS)")
	cron := fs.Bool("cron-snippet", false, "print a crontab line that runs this backup daily at 03:00, with the other flags given, and exit")
	if err := fs.Parse(args); err != nil {
		return backupArgs{}, err
	}
	if fs.NArg() > 0 {
		return backupArgs{}, fmt.Errorf("unexpected arguments: %q", fs.Args())
	}
	a := backupArgs{
		opts:        backup.Options{DBPath: *db, Dir: *dir},
		retention:   *days,
		cronSnippet: *cron,
	}
	if a.opts.Dir == "" {
		a.opts.Dir = filepath.Join(filepath.Dir(a.opts.DBPath), "backups")
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name != "cron-snippet" {
			a.forward = append(a.forward, "-"+f.Name, f.Value.String())
		}
	})
	return a, nil
}

// cronSnippet returns a crontab line that runs exe's backup daily at 03:00
// with args, sending its output to the system log (journalctl -t
// angel-backup). The service finds its .env beside exe, so cron's working
// folder doesn't matter.
func cronSnippet(exe string, args []string) string {
	words := []string{cronQuote(exe), "backup"}
	for _, a := range args {
		words = append(words, cronQuote(a))
	}
	return "0 3 * * * " + strings.Join(words, " ") + " 2>&1 | logger -t angel-backup"
}

// cronQuote quotes s for the shell when needed, and escapes %, which cron
// would turn into a newline.
func cronQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./=:,+@") == "" {
		return s
	}
	s = "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	return strings.ReplaceAll(s, "%", `\%`)
}

// maxRetentionDays keeps the retention well inside time.Duration, which
// overflows past about 106,751 days.
const maxRetentionDays = 36500

// retentionPeriod turns a retention in days into a period, refusing anything
// but a whole number from 1 to maxRetentionDays. An invalid RETENTION_DAYS
// loads as 0.
func retentionPeriod(value string) (time.Duration, error) {
	days, err := strconv.Atoi(value)
	if err != nil || days < 1 || days > maxRetentionDays {
		return 0, fmt.Errorf("retention must be 1 to %d days (-retention-days or RETENTION_DAYS), got %q", maxRetentionDays, value)
	}
	return time.Duration(days) * 24 * time.Hour, nil
}
