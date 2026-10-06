package store

import (
	"context"
	"time"

	"github.com/johansundell/angel/types"
)

func (s *SQLiteStore) AddAcknowledgement(ctx context.Context, a types.Acknowledgement) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO acknowledgements (date, name, created_at) VALUES (?, ?, ?)`,
		a.Date.String(), a.Name, s.timeArg(a.CreatedAt))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) ListAcknowledgements(ctx context.Context, day types.Day) ([]types.Acknowledgement, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, created_at FROM acknowledgements WHERE date = ? ORDER BY created_at, id`, day.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var acks []types.Acknowledgement
	for rows.Next() {
		a := types.Acknowledgement{Date: day}
		var created string
		if err := rows.Scan(&a.ID, &a.Name, &created); err != nil {
			return nil, err
		}
		if a.CreatedAt, err = time.Parse(sqliteTimeLayout, created); err != nil {
			return nil, err
		}
		acks = append(acks, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return acks, nil
}
