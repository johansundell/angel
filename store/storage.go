package store

import (
	"context"

	"github.com/johansundell/angel/types"
)

// Store is the unified storage contract for Angel. The caller must Close it.
type Store interface {
	Ping(ctx context.Context) error
	Close() error

	// Note operations (Daily Notes and Acknowledgements):
	GetDailyNote(ctx context.Context, day types.Day) (note types.DailyNote, ok bool, err error)
	SaveDailyNote(ctx context.Context, n types.DailyNote) error
	DeleteDailyNote(ctx context.Context, day types.Day) error
	AddAcknowledgement(ctx context.Context, a types.Acknowledgement) (id int64, err error)
	ListAcknowledgements(ctx context.Context, day types.Day) ([]types.Acknowledgement, error)
}

// NoteStore is an alias for Store.
type NoteStore = Store
