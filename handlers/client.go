package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/johansundell/angel/httperror"
	"github.com/johansundell/angel/types"
)

// maxDailyNoteLen caps a Daily Note, in characters.
const maxDailyNoteLen = 5000

// ClientDashboard is the Client's authoring page: editors for today's Daily
// Note and tomorrow's Advance Note, each with the Important Flag and a way to
// clear it. An Advance Note is simply the Daily Note stored under tomorrow's
// date, so at the Rollover it becomes today's note without any publish step.
func (h *Handler) ClientDashboard(c *gin.Context) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	today := h.today()
	tomorrow := h.tomorrow()
	todayNote, err := h.editorData(c, today)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	advanceNote, err := h.editorData(c, tomorrow)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	return h.render(c, http.StatusOK, "client.html", gin.H{
		"title":           "Dagens anteckning",
		"today":           todayNote,
		"advance":         advanceNote,
		"maxDailyNoteLen": maxDailyNoteLen,
	})
}

// editorData is what an editor on the dashboard needs for the note of day:
// its date, and the note with the time it was saved, if there is one.
func (h *Handler) editorData(c *gin.Context, day time.Time) (gin.H, error) {
	note, ok, err := h.notes.GetDailyNote(c.Request.Context(), day.Format(dateLayout))
	if err != nil {
		return nil, err
	}
	data := gin.H{"date": swedishDate(day)}
	if ok {
		data["note"] = note
		data["savedAt"] = note.UpdatedAt.In(localZone).Format("15:04")
	}
	return data, nil
}

// SaveNote creates or replaces today's Daily Note from the editor, then
// redirects back to the dashboard.
func (h *Handler) SaveNote(c *gin.Context) error {
	return h.saveNoteFor(c, h.today())
}

// SaveAdvanceNote creates or replaces tomorrow's Advance Note, then
// redirects back to the dashboard.
func (h *Handler) SaveAdvanceNote(c *gin.Context) error {
	return h.saveNoteFor(c, h.tomorrow())
}

// ClearNote removes today's Daily Note, restoring the caregivers' empty
// state, then redirects back to the dashboard.
func (h *Handler) ClearNote(c *gin.Context) error {
	return h.clearNoteFor(c, h.today())
}

// ClearAdvanceNote removes tomorrow's Advance Note, then redirects back to
// the dashboard.
func (h *Handler) ClearAdvanceNote(c *gin.Context) error {
	return h.clearNoteFor(c, h.tomorrow())
}

// saveNoteFor stores the editor's note for day. Saving blank text clears
// the note, so caregivers see the empty state rather than an empty card.
func (h *Handler) saveNoteFor(c *gin.Context, day time.Time) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	text := noteText(c.PostForm("text"))
	if n := utf8.RuneCountInString(text); n > maxDailyNoteLen {
		return httperror.ReturnWithHTTPStatus(fmt.Errorf("note is %d characters, at most %d allowed", n, maxDailyNoteLen), http.StatusBadRequest)
	}
	date := day.Format(dateLayout)
	var err error
	if text == "" {
		err = h.notes.DeleteDailyNote(c.Request.Context(), date)
	} else {
		err = h.notes.SaveDailyNote(c.Request.Context(), types.DailyNote{
			Date:      date,
			Text:      text,
			Important: c.PostForm("important") != "",
		})
	}
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	// Redirect after POST, so reloading the dashboard does not resubmit.
	c.Redirect(http.StatusSeeOther, "/admin")
	return nil
}

// clearNoteFor removes the note for day, then redirects back to the
// dashboard.
func (h *Handler) clearNoteFor(c *gin.Context, day time.Time) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	if err := h.notes.DeleteDailyNote(c.Request.Context(), day.Format(dateLayout)); err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	c.Redirect(http.StatusSeeOther, "/admin")
	return nil
}

// noteText normalizes browser line endings (CRLF) and trims surrounding
// whitespace.
func noteText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}
