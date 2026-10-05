package handlers

import (
	"errors"
	"html/template"
	"io/fs"
	"os"

	"github.com/johansundell/angel/auth"
	"github.com/johansundell/angel/store"
	"github.com/johansundell/angel/utils"
)

type Handler struct {
	store            store.Store
	templates        fs.FS // holds tmpl/*.html
	nameOfService    string
	versionOfService string
	auth             *auth.Authenticator // nil: PIN entry and role-protected views fail closed
}

// Option configures optional Handler dependencies.
type Option func(*Handler)

// WithAuth enables PIN entry and the caregiver and client views.
func WithAuth(a *auth.Authenticator) Option {
	return func(h *Handler) { h.auth = a }
}

// NewHandler creates the handlers. With useFileSystem, templates are read
// from the tmpl folder next to the binary on every request (edit without
// rebuilding) and embedded is ignored; otherwise the embedded filesystem is
// required.
func NewHandler(s store.Store, useFileSystem bool, embedded fs.FS, name, version string, opts ...Option) (*Handler, error) {
	templates := embedded
	if useFileSystem {
		templates = os.DirFS(utils.GetBinaryBasePath())
	} else if embedded == nil {
		return nil, errors.New("embedded templates filesystem is nil")
	}
	h := &Handler{
		store:            s,
		templates:        templates,
		nameOfService:    name,
		versionOfService: version,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h, nil
}

func (h *Handler) getTemplate(withBase bool, tmplFile ...string) (*template.Template, error) {
	files := make([]string, 0, len(tmplFile)+1)
	for _, t := range tmplFile {
		files = append(files, "tmpl/"+t)
	}
	if withBase {
		files = append(files, "tmpl/base.html")
	}
	return template.ParseFS(h.templates, files...)
}
