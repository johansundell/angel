package backup

import (
	"compress/gzip"
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/johansundell/angel/store"
	"github.com/johansundell/angel/types"
)

var fixedNow = func() time.Time { return time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC) }

// newLiveStore returns a running store (WAL mode, connection held open) with
// one Daily Note, and the path of its database file.
func newLiveStore(t *testing.T) (*store.SQLiteStore, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "angel.db")
	s, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	day, _ := types.ParseDay("2026-10-09")
	if err := s.SaveDailyNote(context.Background(), types.DailyNote{Date: day, Text: "Ta medicin kl 09:00", Important: true}); err != nil {
		t.Fatalf("SaveDailyNote: %v", err)
	}
	return s, dbPath
}

// restore decompresses archive into a fresh database file and opens it.
func restore(t *testing.T, archive string) *sql.DB {
	t.Helper()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("archive is not gzip: %v", err)
	}
	restored := filepath.Join(t.TempDir(), "restored.db")
	out, err := os.Create(restored)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, zr); err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", store.FileURI(restored)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunWritesCompressedTimestampedSnapshot(t *testing.T) {
	_, dbPath := newLiveStore(t)
	dir := filepath.Join(t.TempDir(), "backups")

	archive, err := Run(context.Background(), Options{DBPath: dbPath, Dir: dir, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := filepath.Join(dir, "angel_20261009T030000Z.db.gz")
	if archive != want {
		t.Errorf("archive = %q, want %q", archive, want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("backup dir should hold only the archive, has %v", names)
	}

	db := restore(t, archive)
	var text string
	var important bool
	if err := db.QueryRow(`SELECT text, important FROM daily_notes WHERE date = '2026-10-09'`).Scan(&text, &important); err != nil {
		t.Fatalf("read restored note: %v", err)
	}
	if text != "Ta medicin kl 09:00" || !important {
		t.Errorf("restored note = %q important=%v", text, important)
	}
	var check string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&check); err != nil || check != "ok" {
		t.Errorf("restored integrity_check = %q, %v", check, err)
	}
}

// TestRunDuringWrites backs up while the service keeps writing. Neither side
// may fail, and the snapshot holds every write that finished before it began.
func TestRunDuringWrites(t *testing.T) {
	s, dbPath := newLiveStore(t)
	ctx := context.Background()
	day, _ := types.ParseDay("2026-10-09")

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var writes atomic.Int64
	var writeErr error
	writerDone := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(writerDone)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := s.AddAcknowledgement(ctx, types.Acknowledgement{Date: day, Name: "Anna", CreatedAt: time.Now()}); err != nil {
				writeErr = err
				return
			}
			writes.Add(1)
		}
	}()
	// Let the writer get going, so the backup starts mid-stream.
	deadline := time.After(10 * time.Second)
	for writes.Load() < 20 {
		select {
		case <-writerDone:
			t.Fatalf("writer stopped before the backup began: %v", writeErr)
		case <-deadline:
			t.Fatalf("only %d writes in 10s before the backup began", writes.Load())
		case <-time.After(time.Millisecond):
		}
	}

	before := writes.Load()
	archive, err := Run(ctx, Options{DBPath: dbPath, Dir: t.TempDir(), Now: fixedNow})
	during := writes.Load() - before
	close(stop)
	wg.Wait()
	if err != nil {
		t.Fatalf("Run during writes: %v", err)
	}
	if writeErr != nil {
		t.Fatalf("service write failed during backup: %v", writeErr)
	}
	if during == 0 {
		t.Error("no writes landed while the backup ran; it must not block the service")
	}

	db := restore(t, archive)
	var acks int64
	if err := db.QueryRow(`SELECT count(*) FROM acknowledgements`).Scan(&acks); err != nil {
		t.Fatal(err)
	}
	if acks < before {
		t.Errorf("snapshot holds %d acknowledgements, but %d were written before it began", acks, before)
	}
	var notes int
	if err := db.QueryRow(`SELECT count(*) FROM daily_notes`).Scan(&notes); err != nil || notes != 1 {
		t.Errorf("restored notes = %d, %v; want 1", notes, err)
	}
}

func TestRunMissingDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "missing.db")

	if _, err := Run(context.Background(), Options{DBPath: dbPath, Dir: filepath.Join(dir, "backups"), Now: fixedNow}); err == nil {
		t.Fatal("Run should fail when the database doesn't exist")
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Errorf("Run must not create an empty database at %s", dbPath)
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "backups")); len(entries) != 0 {
		t.Errorf("failed Run left files behind: %v", entries)
	}
}

func TestRunRefusesToOverwrite(t *testing.T) {
	_, dbPath := newLiveStore(t)
	dir := t.TempDir()
	opts := Options{DBPath: dbPath, Dir: dir, Now: fixedNow}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if _, err := Run(context.Background(), opts); err == nil {
		t.Error("second Run with the same timestamp should fail, not overwrite the archive")
	}
}

func TestRunRejectsCorruptSnapshot(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "garbage.db")
	if err := os.WriteFile(dbPath, []byte("this is not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	backups := filepath.Join(dir, "backups")
	if _, err := Run(context.Background(), Options{DBPath: dbPath, Dir: backups, Now: fixedNow}); err == nil {
		t.Fatal("Run should fail on a file that isn't a SQLite database")
	}
	if entries, _ := os.ReadDir(backups); len(entries) != 0 {
		t.Errorf("failed Run left files behind: %v", entries)
	}
}

// TestRunSameSecond starts two backups with the same timestamp at once: one
// stores the archive, the other fails without touching it.
func TestRunSameSecond(t *testing.T) {
	_, dbPath := newLiveStore(t)
	dir := t.TempDir()
	opts := Options{DBPath: dbPath, Dir: dir, Now: fixedNow}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = Run(context.Background(), opts)
		}()
	}
	wg.Wait()

	if (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("want exactly one run to succeed, got errors %v and %v", errs[0], errs[1])
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("backup dir should hold only the archive, has %v (%v)", entries, err)
	}
	restore(t, filepath.Join(dir, entries[0].Name()))
}

// TestRunPathsWithURICharacters backs up from and into folders whose names
// hold ? and #, which SQLite URIs would otherwise treat as query and fragment.
func TestRunPathsWithURICharacters(t *testing.T) {
	base := filepath.Join(t.TempDir(), "a?b#c")
	if err := os.MkdirAll(base, 0o750); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(base, "angel.db")
	st, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	day, err := types.ParseDay("2026-10-09")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveDailyNote(context.Background(), types.DailyNote{Date: day, Text: "frukost"}); err != nil {
		t.Fatalf("save note: %v", err)
	}

	archive, err := Run(context.Background(), Options{DBPath: dbPath, Dir: filepath.Join(base, "back?ups#"), Now: fixedNow})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var text string
	if err := restore(t, archive).QueryRow(`SELECT text FROM daily_notes`).Scan(&text); err != nil || text != "frukost" {
		t.Errorf("restored note = %q, %v; want frukost", text, err)
	}
}
