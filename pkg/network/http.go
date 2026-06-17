package network

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
	mux.HandleFunc("GET /networks", h.List)
	mux.HandleFunc("GET /networks/{id}", h.Get)
	mux.HandleFunc("POST /networks", h.Insert)
	mux.HandleFunc("PATCH /networks/{id}", h.Update)
	mux.HandleFunc("DELETE /networks/{id}", h.Delete)
}

func (h *httpHandler) List(w http.ResponseWriter, r *http.Request) {
	networks, err := h.service.Find(r.Context())
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, networks)
}

func (h *httpHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	network, err := h.service.FindByID(r.Context(), id)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, network)
}
func (h *httpHandler) Insert(w http.ResponseWriter, r *http.Request) {
	var req insertNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("network.http.Insert", "Invalid request body", err))
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
	var req updateNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("network.http.Update", "Invalid request body", err))
		return
	}
	err := h.service.UpdateByID(r.Context(), id, req)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *httpHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := h.service.DeleteByID(r.Context(), id)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusNoContent, nil)
}
