package record

import (
	"encoding/json"
	"net/http"

	"github.com/lutia-io/huma/pkg/apperror"
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
	mux.HandleFunc("POST /record", h.Insert)
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
	var req insertRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	id, err := h.service.Create(r.Context(), CreateParams{
		SchemaID:           req.SchemaID,
		OrganizationID:     p.OrganizationID,
		OrganizationUserID: p.ID,
		NetworkID:          p.NetworkID,
		Data:               req.Data,
	})
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}
