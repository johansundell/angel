package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/johansundell/angel/backup"
)

// runBackup is the backup subcommand: angel backup [-db file] [-dir dir].
// It prints the archive path and returns the process exit code.
func runBackup(args []string, stdout, stderr io.Writer) int {
	// Reads .env like the service does, so SQLITE_PATH and BACKUP_DIR match it.
	loadSettings()
	opts, err := backupOptions(args, settings.SqlitePath, os.Getenv("BACKUP_DIR"), stderr)
	if err != nil {
		if err == flag.ErrHelp {
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

// backupOptions reads the backup flags. Flags win over sqlitePath and
// backupDir; without either, backups go to a backups folder beside the
// database.
func backupOptions(args []string, sqlitePath, backupDir string, output io.Writer) (backup.Options, error) {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(output)
	db := fs.String("db", sqlitePath, "database file to back up (SQLITE_PATH)")
	dir := fs.String("dir", backupDir, "directory for the archives (BACKUP_DIR; default: backups beside the database)")
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
