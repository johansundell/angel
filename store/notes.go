package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/johansundell/angel/types"
)

// NoteStore keeps Daily Notes, one per calendar date, and the Caregivers'
// Acknowledgements of them.
type NoteStore interface {
	// GetDailyNote returns the note for date (YYYY-MM-DD); ok is false when
	// there is none.
	GetDailyNote(ctx context.Context, date string) (note types.DailyNote, ok bool, err error)
	// SaveDailyNote creates or replaces the note for n.Date. CreatedAt and
	// UpdatedAt are set by the store.
	SaveDailyNote(ctx context.Context, n types.DailyNote) error
	// DeleteDailyNote removes the note for date (YYYY-MM-DD), if any.
	DeleteDailyNote(ctx context.Context, date string) error
	// AddAcknowledgement records a and returns its ID. The caller sets Date
	// and CreatedAt.
	AddAcknowledgement(ctx context.Context, a types.Acknowledgement) (id int64, err error)
	// ListAcknowledgements returns the acknowledgements for date
	// (YYYY-MM-DD), oldest first.
	ListAcknowledgements(ctx context.Context, date string) ([]types.Acknowledgement, error)
}

func (s *SQLiteStore) GetDailyNote(ctx context.Context, date string) (types.DailyNote, bool, error) {
	n := types.DailyNote{Date: date}
	var created, updated string
	err := s.db.QueryRowContext(ctx, `SELECT text, important, created_at, updated_at FROM daily_notes WHERE date = ?`, date).
		Scan(&n.Text, &n.Important, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return types.DailyNote{}, false, nil
	}
	if err != nil {
		return types.DailyNote{}, false, err
	}
	if n.CreatedAt, err = time.Parse(sqliteTimeLayout, created); err != nil {
		return types.DailyNote{}, false, err
	}
	if n.UpdatedAt, err = time.Parse(sqliteTimeLayout, updated); err != nil {
		return types.DailyNote{}, false, err
	}
	return n, true, nil
}

func (s *SQLiteStore) SaveDailyNote(ctx context.Context, n types.DailyNote) error {
	now := s.timeArg(time.Now())
	_, err := s.db.ExecContext(ctx, `INSERT INTO daily_notes (date, text, important, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET text = excluded.text, important = excluded.important, updated_at = excluded.updated_at`,
		n.Date, n.Text, n.Important, now, now)
	return err
}

func (s *SQLiteStore) DeleteDailyNote(ctx context.Context, date string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM daily_notes WHERE date = ?`, date)
	return err
}
