package user

import (
	"encoding/json"
	"net/http"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/render"
)

type httpHandler struct {
	service *service
}

func newHTTPHandler(service *service, mux *http.ServeMux) *httpHandler {
	handler := &httpHandler{
		service: service,
	}
	handler.Register(mux)
	return handler
}

func (h *httpHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /user", h.Insert)
	mux.HandleFunc("GET /user/{id}", h.Get)
	mux.HandleFunc("PATCH /user/{id}", h.Patch)
	mux.HandleFunc("POST /user/{id}/password", h.UpdatePassword)
}

func (h *httpHandler) Insert(w http.ResponseWriter, r *http.Request) {
	var req insertUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	id, err := h.service.Insert(r.Context(), req)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *httpHandler) Get(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	u, err := h.service.Get(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, u)
}

func (h *httpHandler) Patch(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	existing, err := h.service.Get(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	var req patchUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	if err := h.service.Patch(r.Context(), existing, req); err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, map[string]string{"id": existing.ID})
}

func (h *httpHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	existing, err := h.service.Get(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	var req updatePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	if err := h.service.UpdatePassword(r.Context(), existing, req); err != nil {
		render.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
