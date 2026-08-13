package schema

import (
	"encoding/json"
	"net/http"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/network"
	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/render"
)

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
	mux.HandleFunc("POST /schema", h.Insert)
}

func (h *httpHandler) Insert(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	var req insertSchemaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	networkID, ok := network.ResolveID(r, p, req.NetworkID)
	if !ok {
		render.WriteError(w, apperror.NewBadRequestError("Network ID is required", nil))
		return
	}
	req.NetworkID = networkID
	req.UserID = p.ID
	if err := principal.RequireUser(p, req.NetworkID); err != nil {
		render.WriteError(w, err)
		return
	}
	// Internal schemas are not allowed to be created via the API
	req.Internal = false

	id, err := h.service.Insert(r.Context(), req)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}
