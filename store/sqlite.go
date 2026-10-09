package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// uriEscaper escapes characters that end or escape the path in an SQLite
// URI filename (RFC 3986 / SQLite URI specification).
var uriEscaper = strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23")

// FileURI turns a file path into an SQLite URI filename ("file:<escaped_path>"),
// so a '?', '#' or '%' in the path cannot be misparsed as a URI query string,
// fragment or escape sequence.
func FileURI(path string) string {
	return "file:" + uriEscaper.Replace(path)
}

// sqliteTimeLayout is a fixed-width UTC layout, so created_at values compare
// and sort correctly as strings. The driver's default (RFC3339Nano) varies in
// length and does not.
const sqliteTimeLayout = "2006-01-02T15:04:05.000000Z07:00"

// SQLiteStore keeps Daily Notes and Acknowledgements in one SQLite file.
type SQLiteStore struct {
	db *sql.DB
}

var _ Store = (*SQLiteStore)(nil)

// NewSQLite opens (or creates) the SQLite database at file and returns a store
// that owns the connection.
func NewSQLite(file string) (*SQLiteStore, error) {
	db, err := openSQLite(file)
	if err != nil {
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) timeArg(t time.Time) string {
	return t.UTC().Format(sqliteTimeLayout)
}

func openSQLite(file string) (*sql.DB, error) {
	// SQLite-specific pragmas for better concurrency and durability, set in the
	// DSN so the driver applies them to every connection it opens.
	// WAL mode and a busy timeout reduce SQLITE_BUSY errors under contention.
	db, err := sql.Open("sqlite3", FileURI(file)+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}

	// Connection pool: limit to a single writer connection for file-backed SQLite
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0 * time.Second)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS daily_notes (
		date TEXT PRIMARY KEY,
		text TEXT NOT NULL,
		important INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS acknowledgements (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS acknowledgements_date ON acknowledgements (date, created_at)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
