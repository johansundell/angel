package types

import "time"

// DailyNote is the Client's note for one Day.
type DailyNote struct {
	Date      Day // the Day this note is written for
	Text      string
	Important bool // the Important Flag (Viktigt)
	CreatedAt time.Time
	UpdatedAt time.Time
}
