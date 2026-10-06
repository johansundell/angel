package types

import (
	"testing"
	"time"
)

func mustParseDay(t *testing.T, s string) Day {
	t.Helper()
	d, err := ParseDay(s)
	if err != nil {
		t.Fatalf("ParseDay(%q): %v", s, err)
	}
	return d
}

func TestDayOf(t *testing.T) {
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"summer, 22:30 UTC is 00:30 the next day at home", time.Date(2026, 7, 1, 22, 30, 0, 0, time.UTC), "2026-07-02"},
		{"summer, 21:59 UTC is 23:59 the same day at home", time.Date(2026, 7, 1, 21, 59, 0, 0, time.UTC), "2026-07-01"},
		{"winter, 22:30 UTC is 23:30 the same day at home", time.Date(2026, 1, 15, 22, 30, 0, 0, time.UTC), "2026-01-15"},
		{"winter, 23:00 UTC is midnight, the next day at home", time.Date(2026, 1, 15, 23, 0, 0, 0, time.UTC), "2026-01-16"},
		{"the zone of the input doesn't matter", time.Date(2026, 1, 15, 18, 30, 0, 0, time.FixedZone("EST", -5*3600)), "2026-01-16"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DayOf(tt.at).String(); got != tt.want {
				t.Errorf("DayOf(%v) = %s, want %s", tt.at, got, tt.want)
			}
		})
	}
}

func TestDayNext(t *testing.T) {
	tests := []struct {
		day, want string
	}{
		{"2026-10-05", "2026-10-06"},
		{"2026-10-24", "2026-10-25"}, // the night DST ends
		{"2026-10-25", "2026-10-26"}, // a 25-hour day at home
		{"2026-03-29", "2026-03-30"}, // a 23-hour day at home
		{"2026-09-30", "2026-10-01"},
		{"2026-02-28", "2026-03-01"},
		{"2028-02-28", "2028-02-29"},
		{"2026-12-31", "2027-01-01"},
	}
	for _, tt := range tests {
		if got := mustParseDay(t, tt.day).Next().String(); got != tt.want {
			t.Errorf("%s.Next() = %s, want %s", tt.day, got, tt.want)
		}
	}
}

func TestDayOf_NextMatchesTheRollover(t *testing.T) {
	// One minute before and after midnight at home, on the days DST starts
	// and ends: the Day after the Rollover is the Next of the Day before it.
	for _, midnight := range []time.Time{
		time.Date(2026, 3, 29, 0, 0, 0, 0, homeZone),
		time.Date(2026, 3, 30, 0, 0, 0, 0, homeZone),
		time.Date(2026, 10, 25, 0, 0, 0, 0, homeZone),
		time.Date(2026, 10, 26, 0, 0, 0, 0, homeZone),
	} {
		before, after := DayOf(midnight.Add(-time.Minute)), DayOf(midnight.Add(time.Minute))
		if before.Next() != after {
			t.Errorf("around %v: %s.Next() = %s, want %s", midnight, before, before.Next(), after)
		}
	}
}

func TestParseDay(t *testing.T) {
	for _, s := range []string{"2026-10-05", "2028-02-29", "0001-01-01"} {
		if got := mustParseDay(t, s).String(); got != s {
			t.Errorf("ParseDay(%q).String() = %q", s, got)
		}
	}
	for _, s := range []string{"", "2026-13-01", "2026-02-30", "2026-02-29", "20261005", "2026-10-5", "2026-10-05 ", "2026-10-05T00:00:00Z", "today"} {
		if d, err := ParseDay(s); err == nil {
			t.Errorf("ParseDay(%q) = %s, want an error", s, d)
		}
	}
}

func TestDayIsComparable(t *testing.T) {
	a := mustParseDay(t, "2026-10-05")
	if a != DayOf(time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)) {
		t.Error("the same date parsed and from a time should be equal")
	}
	if a == a.Next() {
		t.Error("different dates should not be equal")
	}
}

func TestDayParts(t *testing.T) {
	d := mustParseDay(t, "2026-10-05")
	if d.Year() != 2026 || d.Month() != time.October || d.DayOfMonth() != 5 || d.Weekday() != time.Monday {
		t.Errorf("parts of %s = %d, %v, %d, %v", d, d.Year(), d.Month(), d.DayOfMonth(), d.Weekday())
	}
}

func TestHomeTime(t *testing.T) {
	at := time.Date(2026, 10, 5, 6, 35, 0, 0, time.UTC)
	if got := HomeTime(at).Format("15:04"); got != "08:35" {
		t.Errorf("HomeTime(%v) = %s, want 08:35 (CEST)", at, got)
	}
	if !HomeTime(at).Equal(at) {
		t.Error("HomeTime must not change the instant")
	}
}
