package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/johansundell/angel/httperror"
	"github.com/johansundell/angel/types"
)

// maxNoteLen caps a Daily Note, in characters.
const maxNoteLen = 5000

// ClientDashboard is the Client's authoring page: an editor for today's
// Daily Note with the Important Flag, and a way to clear it.
func (h *Handler) ClientDashboard(c *gin.Context) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	today := h.today()
	note, ok, err := h.notes.GetDailyNote(c.Request.Context(), today.Format(dateLayout))
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	data := gin.H{"title": "Översikt", "date": swedishDate(today), "maxNoteLen": maxNoteLen}
	if ok {
		data["note"] = note
		data["savedAt"] = note.UpdatedAt.In(localZone).Format("15:04")
	}
	return h.render(c, http.StatusOK, "client.html", data)
}

// SaveNote creates or replaces today's Daily Note from the editor, then
// redirects back to the dashboard. Saving blank text clears the note, so
// caregivers see the empty state rather than an empty card.
func (h *Handler) SaveNote(c *gin.Context) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	text := noteText(c.PostForm("text"))
	if n := utf8.RuneCountInString(text); n > maxNoteLen {
		return httperror.ReturnWithHTTPStatus(fmt.Errorf("note is %d characters, at most %d allowed", n, maxNoteLen), http.StatusBadRequest)
	}
	date := h.today().Format(dateLayout)
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

// ClearNote removes today's Daily Note, restoring the caregivers' empty
// state, then redirects back to the dashboard.
func (h *Handler) ClearNote(c *gin.Context) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	if err := h.notes.DeleteDailyNote(c.Request.Context(), h.today().Format(dateLayout)); err != nil {
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
