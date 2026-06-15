package user

import (
	"encoding/json"
	"net/http"

	"github.com/lutia-io/huma/pkg/apperror"
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
	mux.HandleFunc("GET /users", h.List)
	mux.HandleFunc("GET /users/{id}", h.Get)
	mux.HandleFunc("POST /users", h.Insert)
	mux.HandleFunc("PATCH /users/{id}", h.Update)
}

func (h *httpHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.Find(r.Context())
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, users)
}

func (h *httpHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := h.service.FindById(r.Context(), id)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, user)
}
func (h *httpHandler) Insert(w http.ResponseWriter, r *http.Request) {
	var req insertUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("user.http.Insert", "Invalid request body", err))
		return
	}
	id, err := h.service.Insert(r.Context(), req)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *httpHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("user.http.Update", "Invalid request body", err))
		return
	}
	err := h.service.Update(r.Context(), id, req)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusNoContent, nil)
}
