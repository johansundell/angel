package types

import "time"

// Acknowledgement records that a Caregiver has read the Daily Note for Date.
type Acknowledgement struct {
	ID        int64
	Date      Day    // the Day the acknowledgement was made on
	Name      string // the Caregiver's first name; empty when anonymous
	CreatedAt time.Time
}
