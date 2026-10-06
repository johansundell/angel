package store

import (
	"context"
	"testing"
	"time"

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

func TestNoteStore_AcknowledgementsByDay(t *testing.T) {
	s := newTestSQLite(t)
	ctx := context.Background()
	monday := mustDay(t, "2026-10-05")
	tuesday := monday.Next()
	at := time.Date(2026, 10, 5, 6, 35, 0, 0, time.UTC)

	first, err := s.AddAcknowledgement(ctx, types.Acknowledgement{Date: monday, Name: "Maria", CreatedAt: at})
	if err != nil {
		t.Fatalf("AddAcknowledgement: %v", err)
	}
	second, err := s.AddAcknowledgement(ctx, types.Acknowledgement{Date: monday, CreatedAt: at.Add(time.Hour)})
	if err != nil {
		t.Fatalf("AddAcknowledgement: %v", err)
	}
	if _, err := s.AddAcknowledgement(ctx, types.Acknowledgement{Date: tuesday, Name: "Ahmed", CreatedAt: at.Add(24 * time.Hour)}); err != nil {
		t.Fatalf("AddAcknowledgement: %v", err)
	}

	acks, err := s.ListAcknowledgements(ctx, monday)
	if err != nil {
		t.Fatalf("ListAcknowledgements: %v", err)
	}
	if len(acks) != 2 {
		t.Fatalf("got %d acknowledgements for monday, want 2: %+v", len(acks), acks)
	}
	if acks[0].ID != first || acks[0].Date != monday || acks[0].Name != "Maria" || !acks[0].CreatedAt.Equal(at) {
		t.Errorf("first acknowledgement = %+v", acks[0])
	}
	if acks[1].ID != second || acks[1].Date != monday || acks[1].Name != "" {
		t.Errorf("second acknowledgement = %+v", acks[1])
	}

	acks, err = s.ListAcknowledgements(ctx, tuesday.Next())
	if err != nil {
		t.Fatalf("ListAcknowledgements: %v", err)
	}
	if len(acks) != 0 {
		t.Errorf("got %+v for a day without acknowledgements", acks)
	}
}
