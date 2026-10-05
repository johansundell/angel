package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"
	_ "time/tzdata" // the runtime image has no zoneinfo

	"github.com/gin-gonic/gin"
	"github.com/johansundell/angel/httperror"
)

// Daily Notes are keyed by the calendar date in the Client's home.
var localZone = mustLoadLocation("Europe/Stockholm")

const dateLayout = "2006-01-02"

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

var errNotesNotConfigured = errors.New("note storage is not configured")

// today returns the current local date.
func (h *Handler) today() time.Time {
	return h.now().In(localZone)
}

var (
	swedishWeekdays = [...]string{"söndag", "måndag", "tisdag", "onsdag", "torsdag", "fredag", "lördag"}
	swedishMonths   = [...]string{"januari", "februari", "mars", "april", "maj", "juni", "juli", "augusti", "september", "oktober", "november", "december"}
)

// swedishDate formats t like "måndag 5 oktober 2026".
func swedishDate(t time.Time) string {
	return fmt.Sprintf("%s %d %s %d", swedishWeekdays[t.Weekday()], t.Day(), swedishMonths[t.Month()-1], t.Year())
}

// CaregiverView shows today's Daily Note to a caregiver, or an affirmative
// empty state when there is none.
func (h *Handler) CaregiverView(c *gin.Context) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	today := h.today()
	note, ok, err := h.notes.GetDailyNote(c.Request.Context(), today.Format(dateLayout))
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	data := gin.H{"title": "Dagens anteckning", "date": swedishDate(today)}
	if ok {
		data["note"] = note
	}
	return h.render(c, http.StatusOK, "caregiver.html", data)
}
