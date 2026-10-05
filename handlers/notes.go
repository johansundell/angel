package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // the runtime image has no zoneinfo

	"github.com/gin-gonic/gin"
	"github.com/johansundell/angel/httperror"
	"github.com/johansundell/angel/types"
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

// ackQueryParam carries the ID of the caregiver's own Acknowledgement from
// AcknowledgeNote back to CaregiverView.
const ackQueryParam = "kvitterat"

// maxCaregiverNameLen caps the stored first name, in characters.
const maxCaregiverNameLen = 40

// CaregiverView shows today's Daily Note to a caregiver, or an affirmative
// empty state when there is none, followed by the Kvittera form. After
// acknowledging, it also confirms who acknowledged and when.
func (h *Handler) CaregiverView(c *gin.Context) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	ctx := c.Request.Context()
	today := h.today()
	date := today.Format(dateLayout)
	note, ok, err := h.notes.GetDailyNote(ctx, date)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	data := gin.H{"title": "Dagens anteckning", "date": swedishDate(today)}
	if ok {
		data["note"] = note
	}
	if id, err := strconv.ParseInt(c.Query(ackQueryParam), 10, 64); err == nil {
		// Only today's acknowledgements are confirmed, so a stale link does
		// not claim a new day's note has been read.
		acks, err := h.notes.ListAcknowledgements(ctx, date)
		if err != nil {
			return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
		}
		if i := slices.IndexFunc(acks, func(a types.Acknowledgement) bool { return a.ID == id }); i >= 0 {
			data["confirmation"] = ackConfirmation(acks[i])
		}
	}
	return h.render(c, http.StatusOK, "caregiver.html", data)
}

// AcknowledgeNote records that a caregiver has read today's Daily Note, then
// redirects to the note view, which confirms it.
func (h *Handler) AcknowledgeNote(c *gin.Context) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	now := h.now()
	ack := types.Acknowledgement{
		Date:      now.In(localZone).Format(dateLayout),
		Name:      caregiverName(c.PostForm("name")),
		CreatedAt: now,
	}
	id, err := h.notes.AddAcknowledgement(c.Request.Context(), ack)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	// Redirect after POST, so reloading the confirmation does not
	// acknowledge again.
	c.Redirect(http.StatusSeeOther, "/note?"+ackQueryParam+"="+strconv.FormatInt(id, 10))
	return nil
}

// caregiverName trims the optional first name and caps its length.
func caregiverName(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxCaregiverNameLen {
		s = strings.TrimSpace(string(r[:maxCaregiverNameLen]))
	}
	return s
}

// ackConfirmation is the text shown after acknowledging, such as
// "Kvitterat av Maria kl 08:35".
func ackConfirmation(a types.Acknowledgement) string {
	at := a.CreatedAt.In(localZone).Format("15:04")
	if a.Name == "" {
		return "Kvitterat kl " + at
	}
	return "Kvitterat av " + a.Name + " kl " + at
}
