package file

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/render"
)

const maxMultipartMemory = 32 << 20 // 32 MiB

type httpHandler struct {
	service *Service
}

func newHTTPHandler(service *Service, mux *http.ServeMux) *httpHandler {
	handler := &httpHandler{
		service: service,
	}
	handler.Register(mux)
	return handler
}

func (h *httpHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /file", h.List)
	mux.HandleFunc("POST /file", h.Insert)
	mux.HandleFunc("GET /file/{id}", h.Download)
	mux.HandleFunc("GET /file/{id}/metadata", h.Metadata)
	mux.HandleFunc("DELETE /file/{id}", h.Delete)
}

func (h *httpHandler) List(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	params, err := parseListParams(r)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	result, err := h.service.List(r.Context(), p, params)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, result)
}

func (h *httpHandler) Insert(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	if err := principal.RequireOrganizationUser(p, p.NetworkID, p.OrganizationID); err != nil {
		render.WriteError(w, err)
		return
	}
	if p.NetworkID == "" || p.OrganizationID == "" {
		render.WriteError(w, apperror.NewUnauthorizedError("Organization user token missing network or organization", nil))
		return
	}
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid multipart form", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		render.WriteError(w, apperror.NewBadRequestError("File is required", err))
		return
	}
	defer file.Close()

	filename := strings.TrimSpace(r.FormValue("filename"))
	if filename == "" {
		filename = filepath.Base(header.Filename)
	}

	contentType := strings.TrimSpace(r.FormValue("contentType"))
	if contentType == "" {
		contentType = header.Header.Get("Content-Type")
	}
	if contentType == "" || contentType == "application/octet-stream" {
		if detected := mime.TypeByExtension(filepath.Ext(filename)); detected != "" {
			contentType = detected
		}
	}

	id, err := h.service.Create(r.Context(), CreateParams{
		Filename:           filename,
		ContentType:        contentType,
		OrganizationID:     p.OrganizationID,
		OrganizationUserID: p.ID,
		NetworkID:          p.NetworkID,
		IdempotencyKey:     r.FormValue("idempotencyKey"),
		Content:            file,
	})
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *httpHandler) Metadata(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	meta, err := h.service.Get(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, meta)
}

func (h *httpHandler) Download(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	id := r.PathValue("id")
	if _, err := h.service.Get(r.Context(), p, id); err != nil {
		render.WriteError(w, err)
		return
	}
	content, found, err := h.service.OpenContent(r.Context(), id)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	if !found {
		render.WriteError(w, apperror.NewNotFoundError("File not found", nil))
		return
	}
	defer content.Reader.Close()

	w.Header().Set("Content-Type", content.File.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, content.File.Filename))
	if content.File.SizeBytes > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", content.File.SizeBytes))
	}
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, content.Reader); err != nil {
		// Headers already written; nothing useful to return to the client.
		return
	}
}

func (h *httpHandler) Delete(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	id := r.PathValue("id")
	if _, err := h.service.Get(r.Context(), p, id); err != nil {
		render.WriteError(w, err)
		return
	}
	found, err := h.service.SoftDelete(r.Context(), id)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	if !found {
		render.WriteError(w, apperror.NewNotFoundError("File not found", nil))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
