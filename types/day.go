package types

import (
	"fmt"
	"time"
	_ "time/tzdata" // the runtime image has no zoneinfo
)

// homeZone is the time zone of the Client's home. Days and clock times are
// always in this zone; it is not a setting.
var homeZone = mustLoadLocation("Europe/Stockholm")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// dayLayout is how a Day is written: in storage, in forms and by String.
const dayLayout = "2006-01-02"

// Day is a calendar date in the Client's home, beginning and ending at the
// Rollover (midnight at home). Daily Notes are written for a Day, and
// Acknowledgements belong to the Day they were made on.
//
// Days are comparable with ==. The zero Day is not a valid date.
type Day struct {
	year  int
	month time.Month
	day   int
}

// HomeTime returns t in the Client's home time zone.
func HomeTime(t time.Time) time.Time {
	return t.In(homeZone)
}

// DayOf returns the Day at the moment t in the Client's home.
func DayOf(t time.Time) Day {
	y, m, d := HomeTime(t).Date()
	return Day{year: y, month: m, day: d}
}

// ParseDay parses a Day written as YYYY-MM-DD and refuses anything else,
// including dates that don't exist, such as 2026-02-30.
func ParseDay(s string) (Day, error) {
	t, err := time.Parse(dayLayout, s)
	if err != nil {
		return Day{}, fmt.Errorf("invalid day %q: %w", s, err)
	}
	y, m, d := t.Date()
	return Day{year: y, month: m, day: d}, nil
}

// Next returns the following Day.
func (d Day) Next() Day {
	y, m, dd := d.midnight().AddDate(0, 0, 1).Date()
	return Day{year: y, month: m, day: dd}
}

// Weekday returns the day of the week of d.
func (d Day) Weekday() time.Weekday {
	return d.midnight().Weekday()
}

// Year, Month and DayOfMonth return the parts of d's date.
func (d Day) Year() int         { return d.year }
func (d Day) Month() time.Month { return d.month }
func (d Day) DayOfMonth() int   { return d.day }

// String returns d as YYYY-MM-DD.
func (d Day) String() string {
	return d.midnight().Format(dayLayout)
}

// midnight returns the start of d. It is in UTC, which has no DST, so date
// arithmetic on it never lands on the wrong day.
func (d Day) midnight() time.Time {
	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, time.UTC)
}
