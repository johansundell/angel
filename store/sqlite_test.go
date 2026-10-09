package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/johansundell/angel/store"
	"github.com/johansundell/angel/types"
)

func TestFileURI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"angel.db", "file:angel.db"},
		{"/var/data/angel.db", "file:/var/data/angel.db"},
		{"path/with?question.db", "file:path/with%3fquestion.db"},
		{"path/with#hash.db", "file:path/with%23hash.db"},
		{"path/with%percent.db", "file:path/with%25percent.db"},
		{"/a?b#c%d/test.db", "file:/a%3fb%23c%25d/test.db"},
	}

	for _, tt := range tests {
		got := store.FileURI(tt.input)
		if got != tt.want {
			t.Errorf("FileURI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNewSQLitePathsWithURICharacters(t *testing.T) {
	temp := t.TempDir()
	// Directory containing ?, # and % characters:
	base := filepath.Join(temp, "a?b#c%d")
	if err := os.MkdirAll(base, 0o750); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(base, "angel.db")
	st, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite with URI chars failed: %v", err)
	}
	defer st.Close()

	day, err := types.ParseDay("2026-10-09")
	if err != nil {
		t.Fatal(err)
	}

	// Verify we can perform operations on the database:
	note := types.DailyNote{
		Date: day,
		Text: "Testing URI escaping",
	}
	if err := st.SaveDailyNote(context.Background(), note); err != nil {
		t.Fatalf("SaveDailyNote failed: %v", err)
	}

	readNote, ok, err := st.GetDailyNote(context.Background(), day)
	if err != nil {
		t.Fatalf("GetDailyNote failed: %v", err)
	}
	if !ok {
		t.Fatal("expected note to exist")
	}
	if readNote.Text != note.Text {
		t.Errorf("got text %q, want %q", readNote.Text, note.Text)
	}

	// Verify the database file was created at the exact path, not truncated:
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("expected database at %s, got error: %v", dbPath, err)
	}

	// Verify no truncated file (like "a") was created in temp:
	truncated := filepath.Join(temp, "a")
	if _, err := os.Stat(truncated); !os.IsNotExist(err) {
		t.Errorf("truncated file %s should not exist", truncated)
	}
}
