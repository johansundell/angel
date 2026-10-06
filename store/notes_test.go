package store

import (
	"context"
	"testing"

	"github.com/johansundell/angel/types"
)

func mustDay(t *testing.T, s string) types.Day {
	t.Helper()
	d, err := types.ParseDay(s)
	if err != nil {
		t.Fatalf("ParseDay(%q): %v", s, err)
	}
	return d
}

func TestNoteStore_CRUD(t *testing.T) {
	s := newTestSQLite(t)
	ctx := context.Background()
	today := mustDay(t, "2026-10-06")

	// Empty store returns ok = false.
	_, ok, err := s.GetDailyNote(ctx, today)
	if err != nil {
		t.Fatalf("GetDailyNote: %v", err)
	}
	if ok {
		t.Error("expected no note initially")
	}

	// Save a note for today.
	note := types.DailyNote{
		Date:      today,
		Text:      "Ta medicin kl 09:00",
		Important: true,
	}
	if err := s.SaveDailyNote(ctx, note); err != nil {
		t.Fatalf("SaveDailyNote: %v", err)
	}

	// Retrieve today's note.
	got, ok, err := s.GetDailyNote(ctx, today)
	if err != nil {
		t.Fatalf("GetDailyNote: %v", err)
	}
	if !ok {
		t.Fatal("expected note to exist")
	}
	if got.Date != today || got.Text != note.Text || !got.Important {
		t.Errorf("got %+v, want %+v", got, note)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("expected timestamps to be set, got created=%v updated=%v", got.CreatedAt, got.UpdatedAt)
	}

	// Update note for today.
	note.Text = "Ta medicin kl 10:00"
	note.Important = false
	if err := s.SaveDailyNote(ctx, note); err != nil {
		t.Fatalf("SaveDailyNote update: %v", err)
	}

	got, ok, err = s.GetDailyNote(ctx, today)
	if err != nil || !ok {
		t.Fatalf("GetDailyNote after update: ok=%v err=%v", ok, err)
	}
	if got.Text != "Ta medicin kl 10:00" || got.Important {
		t.Errorf("got %+v after update", got)
	}

	// Another day does not return today's note.
	tomorrow := today.Next()
	_, ok, err = s.GetDailyNote(ctx, tomorrow)
	if err != nil {
		t.Fatalf("GetDailyNote tomorrow: %v", err)
	}
	if ok {
		t.Error("tomorrow should not have a note")
	}

	// Delete today's note.
	if err := s.DeleteDailyNote(ctx, today); err != nil {
		t.Fatalf("DeleteDailyNote: %v", err)
	}
	_, ok, err = s.GetDailyNote(ctx, today)
	if err != nil {
		t.Fatalf("GetDailyNote after delete: %v", err)
	}
	if ok {
		t.Error("note should be deleted")
	}
}
