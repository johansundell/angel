package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/johansundell/angel/backup"
	"github.com/johansundell/angel/types"
)

// runBackup is the backup subcommand: angel backup [-db file] [-dir dir].
// It prints the archive path and returns the process exit code.
func runBackup(args []string, s types.AppSettings, stdout, stderr io.Writer) int {
	opts, err := backupOptions(args, s, stderr)
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
	return 0
}

// backupOptions reads the backup flags. Flags win over SQLITE_PATH and
// BACKUP_DIR; without either directory, backups go to a backups folder beside
// the database.
func backupOptions(args []string, s types.AppSettings, output io.Writer) (backup.Options, error) {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(output)
	db := fs.String("db", s.SqlitePath, "database file to back up (SQLITE_PATH)")
	dir := fs.String("dir", s.BackupDir, "directory for the archives (BACKUP_DIR; default: backups beside the database)")
	if err := fs.Parse(args); err != nil {
		return backup.Options{}, err
	}
	if fs.NArg() > 0 {
		return backup.Options{}, fmt.Errorf("unexpected arguments: %q", fs.Args())
	}
	opts := backup.Options{DBPath: *db, Dir: *dir}
	if opts.Dir == "" {
		opts.Dir = filepath.Join(filepath.Dir(opts.DBPath), "backups")
	}
	return opts, nil
}
