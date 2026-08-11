package record

import (
	"encoding/json"
	"net/http"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/network"
	"github.com/lutia-io/huma/pkg/organization"
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
	var req insertRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	if nc, ok := network.Resolve(r, req.NetworkID); ok {
		req.NetworkID = nc.NetworkID
	}
	if oc, ok := organization.Resolve(r, req.OrganizationID); ok {
		req.OrganizationID = oc.OrganizationID
	}
	if err := principal.RequireOrganizationUser(p, req.NetworkID, req.OrganizationID); err != nil {
		render.WriteError(w, err)
		return
	}
	req.OrganizationUserID = p.ID
	id, err := h.service.Create(r.Context(), CreateParams{
		SchemaID:           req.SchemaID,
		OrganizationID:     req.OrganizationID,
		OrganizationUserID: req.OrganizationUserID,
		NetworkID:          req.NetworkID,
		Data:               req.Data,
	})
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}
