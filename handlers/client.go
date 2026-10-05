package handlers

import (
	"errors"
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
	today, err := h.loadEditor(c, h.today(), todayEditor)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	advance, err := h.loadEditor(c, h.tomorrow(), advanceEditor)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	acks, err := h.loadAckFeed(c)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	return h.render(c, http.StatusOK, "client.html", gin.H{
		"title":   "Dagens anteckning",
		"today":   today,
		"advance": advance,
		"acks":    acks,
	})
}

// AckFeed renders only the dashboard's acknowledgement feed, which the
// dashboard polls so the Client sees new Acknowledgements without reloading the page
// and losing unsaved edits.
func (h *Handler) AckFeed(c *gin.Context) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	acks, err := h.loadAckFeed(c)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	return h.renderFragment(c, http.StatusOK, "client.html", "ack-feed", acks)
}

// ackFeedEntry is one line of the acknowledgement feed, such as
// "Maria kl 08:35".
type ackFeedEntry struct {
	Who string // the Caregiver's first name, or "Okänd ängel"
	At  string // local time, 15:04
}

// loadAckFeed returns today's Acknowledgements, newest first. Being keyed by
// today's date, the feed starts empty at each Rollover.
func (h *Handler) loadAckFeed(c *gin.Context) ([]ackFeedEntry, error) {
	acks, err := h.notes.ListAcknowledgements(c.Request.Context(), h.today().Format(dateLayout))
	if err != nil {
		return nil, err
	}
	feed := make([]ackFeedEntry, len(acks))
	for i, a := range acks {
		who := a.Name
		if who == "" {
			who = "Okänd ängel"
		}
		feed[len(acks)-1-i] = ackFeedEntry{Who: who, At: clockTime(a.CreatedAt)}
	}
	return feed, nil
}

// noteEditor is one note editor on the dashboard: the fixed wording of the
// form, and the day it edits with that day's note, if there is one.
type noteEditor struct {
	Action       string // the save URL; clearing posts to Action + "/clear"
	ID           string // the textarea's element ID
	TextLabel    string
	Rows         int
	ClearLabel   string
	ClearConfirm string
	MaxLen       int

	Date    string // dateLayout, posted back so stale forms are refused
	Day     string // swedishDate
	Note    *types.DailyNote
	SavedAt string
}

var (
	todayEditor = noteEditor{
		Action:       "/admin/note",
		ID:           "note-text",
		TextLabel:    "Instruktioner till vårdpersonalen",
		Rows:         10,
		ClearLabel:   "Rensa dagens anteckning",
		ClearConfirm: "Rensa dagens anteckning? Vårdpersonalen ser då att allt är som vanligt.",
	}
	advanceEditor = noteEditor{
		Action:       "/admin/advance",
		ID:           "advance-text",
		TextLabel:    "Instruktioner till vårdpersonalen i morgon",
		Rows:         6,
		ClearLabel:   "Rensa morgondagens anteckning",
		ClearConfirm: "Rensa morgondagens anteckning?",
	}
)

// loadEditor fills in editor e for day's note.
func (h *Handler) loadEditor(c *gin.Context, day time.Time, e noteEditor) (noteEditor, error) {
	e.MaxLen = maxDailyNoteLen
	e.Date = day.Format(dateLayout)
	e.Day = swedishDate(day)
	note, ok, err := h.notes.GetDailyNote(c.Request.Context(), e.Date)
	if err != nil {
		return e, err
	}
	if ok {
		e.Note = &note
		e.SavedAt = clockTime(note.UpdatedAt)
	}
	return e, nil
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

// errStaleEditor rejects a form loaded before a Rollover: its day is no
// longer the one the endpoint edits, so saving it would overwrite or delete
// the wrong day's note.
var errStaleEditor = errors.New("the page is out of date after midnight; reload it")

// checkEditorDate refuses the post unless the form's date field is day.
func checkEditorDate(c *gin.Context, day time.Time) error {
	if c.PostForm("date") != day.Format(dateLayout) {
		return httperror.ReturnWithHTTPStatus(errStaleEditor, http.StatusConflict)
	}
	return nil
}

// saveNoteFor stores the editor's note for day. Saving blank text clears
// the note, so caregivers see the empty state rather than an empty card.
func (h *Handler) saveNoteFor(c *gin.Context, day time.Time) error {
	if h.notes == nil {
		return httperror.ReturnWithHTTPStatus(errNotesNotConfigured, http.StatusInternalServerError)
	}
	if err := checkEditorDate(c, day); err != nil {
		return err
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
	if err := checkEditorDate(c, day); err != nil {
		return err
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
