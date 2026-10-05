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

// tomorrow returns the local date after today. AddDate keeps the calendar
// day right across DST changes.
func (h *Handler) tomorrow() time.Time {
	return h.today().AddDate(0, 0, 1)
}

var (
	swedishWeekdays = [...]string{"söndag", "måndag", "tisdag", "onsdag", "torsdag", "fredag", "lördag"}
	swedishMonths   = [...]string{"januari", "februari", "mars", "april", "maj", "juni", "juli", "augusti", "september", "oktober", "november", "december"}
)

// clockTime formats t as the local time of day, like "08:35".
func clockTime(t time.Time) string {
	return t.In(localZone).Format("15:04")
}

// swedishDate formats t like "måndag 5 oktober 2026".
func swedishDate(t time.Time) string {
	return fmt.Sprintf("%s %d %s %d", swedishWeekdays[t.Weekday()], t.Day(), swedishMonths[t.Month()-1], t.Year())
}

// ackCookieName is a one-time cookie carrying the ID of the caregiver's own
// Acknowledgement from AcknowledgeNote to CaregiverView. Unlike a link, it is
// cleared once shown, so the next caregiver on a shared phone is not told the
// note is already acknowledged.
const ackCookieName = "angel_ack"

// ackCookieTTL only needs to outlast the redirect.
const ackCookieTTL = time.Minute

// maxCaregiverNameLen caps the stored first name, in characters.
const maxCaregiverNameLen = 40

// CaregiverView shows today's Daily Note to a caregiver, or an affirmative
// empty state when there is none, followed by the Kvittera form. After
// acknowledging, it also confirms who acknowledged and when.
func (h *Handler) CaregiverView(c *gin.Context) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	today := h.today()
	date := today.Format(dateLayout)
	note, ok, err := h.notes.GetDailyNote(c.Request.Context(), date)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	data := gin.H{"title": "Dagens anteckning", "date": swedishDate(today), "maxNameLen": maxCaregiverNameLen}
	if ok {
		html, err := renderMarkdown(note.Text)
		if err != nil {
			return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
		}
		data["note"] = note
		data["noteHTML"] = html
	}
	confirmation, err := h.takeAckConfirmation(c, date)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	if confirmation != "" {
		data["confirmation"] = confirmation
	}
	return h.render(c, http.StatusOK, "caregiver.html", data)
}

// AcknowledgeNote records that a caregiver has read today's Daily Note, then
// redirects to the note view, which confirms it.
func (h *Handler) AcknowledgeNote(c *gin.Context) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	today := h.today()
	ack := types.Acknowledgement{
		Date:      today.Format(dateLayout),
		Name:      caregiverName(c.PostForm("name")),
		CreatedAt: today,
	}
	id, err := h.notes.AddAcknowledgement(c.Request.Context(), ack)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	h.setAckCookie(c, strconv.FormatInt(id, 10), int(ackCookieTTL/time.Second))
	// Redirect after POST, so reloading the confirmation does not
	// acknowledge again.
	c.Redirect(http.StatusSeeOther, "/note")
	return nil
}

// setAckCookie sets the one-time confirmation cookie; a negative maxAge
// clears it.
func (h *Handler) setAckCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     ackCookieName,
		Value:    value,
		Path:     "/note",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.auth == nil || h.auth.SecureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

// takeAckConfirmation consumes the one-time cookie set by AcknowledgeNote
// and returns its confirmation text, or "" when there is nothing to confirm.
// Only today's acknowledgements are confirmed, so a leftover cookie does not
// claim a new day's note has been read.
func (h *Handler) takeAckConfirmation(c *gin.Context, date string) (string, error) {
	v, err := c.Cookie(ackCookieName)
	if err != nil {
		return "", nil
	}
	h.setAckCookie(c, "", -1)
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return "", nil
	}
	acks, err := h.notes.ListAcknowledgements(c.Request.Context(), date)
	if err != nil {
		return "", err
	}
	if i := slices.IndexFunc(acks, func(a types.Acknowledgement) bool { return a.ID == id }); i >= 0 {
		return ackConfirmation(acks[i]), nil
	}
	return "", nil
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
	at := clockTime(a.CreatedAt)
	if a.Name == "" {
		return "Kvitterat kl " + at
	}
	return "Kvitterat av " + a.Name + " kl " + at
}
