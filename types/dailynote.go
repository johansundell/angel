package types

import "time"

// DailyNote is the Client's note for one calendar date.
type DailyNote struct {
	Date      string // YYYY-MM-DD in local (Europe/Stockholm) time
	Text      string
	Important bool // the Important Flag (Viktigt)
	CreatedAt time.Time
	UpdatedAt time.Time
}
