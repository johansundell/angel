package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/johansundell/angel/auth"
	"github.com/johansundell/angel/httperror"
)

// homeFor is where each role lands after entering its PIN.
var homeFor = map[auth.Role]string{
	auth.RoleCaregiver: "/note",
	auth.RoleClient:    "/admin",
}

const (
	msgInvalidPIN  = "Fel PIN-kod. Försök igen."
	msgRateLimited = "För många försök. Vänta en stund och försök igen."
)

var errAuthNotConfigured = errors.New("PIN authentication is not configured")

// Entry shows the PIN entry screen, or sends a Caregiver or Client with a valid
// session straight to their view.
func (h *Handler) Entry(c *gin.Context) error {
	if h.auth == nil {
		return httperror.ReturnWithHTTPStatus(errAuthNotConfigured, http.StatusInternalServerError)
	}
	if role, ok := h.auth.Session(c.Request); ok {
		c.Redirect(http.StatusSeeOther, homeFor[role])
		return nil
	}
	return h.renderEntry(c, http.StatusOK, "")
}

// SubmitPIN checks the PIN posted from the entry form. A correct PIN starts a
// session and redirects to that role's view; a wrong one shows the entry screen
// again with an error.
func (h *Handler) SubmitPIN(c *gin.Context) error {
	if h.auth == nil {
		return httperror.ReturnWithHTTPStatus(errAuthNotConfigured, http.StatusInternalServerError)
	}
	role, err := h.auth.Login(c.ClientIP(), c.PostForm("pin"))
	switch {
	case errors.Is(err, auth.ErrRateLimited):
		return h.renderEntry(c, http.StatusTooManyRequests, msgRateLimited)
	case errors.Is(err, auth.ErrInvalidPIN):
		return h.renderEntry(c, http.StatusUnauthorized, msgInvalidPIN)
	case err != nil:
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	h.auth.StartSession(c.Writer, role)
	c.Redirect(http.StatusSeeOther, homeFor[role])
	return nil
}

// Logout ends the session on this device and returns to the entry screen,
// so a shared phone can be handed over or used with the other PIN. It needs
// no session: logging out twice, or after expiry, does no harm. A pending
// acknowledgement confirmation is dropped too, so it is not shown to the
// next caregiver on the phone.
func (h *Handler) Logout(c *gin.Context) error {
	if h.auth == nil {
		return httperror.ReturnWithHTTPStatus(errAuthNotConfigured, http.StatusInternalServerError)
	}
	h.auth.EndSession(c.Writer)
	h.setAckCookie(c, "", -1)
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusSeeOther, "/")
	return nil
}

func (h *Handler) renderEntry(c *gin.Context, status int, errMsg string) error {
	c.Header("Cache-Control", "no-store")
	return h.render(c, status, "entry.html", gin.H{"title": "Ange PIN-kod", "error": errMsg})
}

func (h *Handler) render(c *gin.Context, status int, tmplFile string, data gin.H) error {
	return h.execute(c, status, true, tmplFile, "base", data)
}

// renderFragment writes only the named template from tmplFile, without the
// page around it, for a script to swap into an open page.
func (h *Handler) renderFragment(c *gin.Context, status int, tmplFile, name string, data any) error {
	return h.execute(c, status, false, tmplFile, name, data)
}

func (h *Handler) execute(c *gin.Context, status int, withBase bool, tmplFile, name string, data any) error {
	tmpl, err := h.getTemplate(withBase, tmplFile)
	if err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	if err := tmpl.ExecuteTemplate(c.Writer, name, data); err != nil {
		return httperror.ReturnWithHTTPStatus(err, http.StatusInternalServerError)
	}
	return nil
}
