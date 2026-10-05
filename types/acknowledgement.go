package types

import "time"

// Acknowledgement records that a Caregiver has read the Daily Note for Date.
type Acknowledgement struct {
	ID        int64
	Date      string // YYYY-MM-DD in local (Europe/Stockholm) time
	Name      string // the Caregiver's first name; empty when anonymous
	CreatedAt time.Time
}
