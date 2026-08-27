package organizationuser

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
	mux.HandleFunc("GET /organization-user", h.List)
	mux.HandleFunc("GET /organization-user/{id}", h.Get)
	mux.HandleFunc("POST /organization-user", h.Insert)
	mux.HandleFunc("PATCH /organization-user/{id}", h.Patch)
	mux.HandleFunc("POST /organization-user/{id}/password", h.UpdatePassword)
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

func (h *httpHandler) Insert(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	var req insertOrganizationUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	networkID, ok := network.ResolveID(r, p, req.NetworkID)
	if !ok {
		render.WriteError(w, apperror.NewBadRequestError("Network ID is required", nil))
		return
	}
	organizationID, ok := organization.ResolveID(r, p, req.OrganizationID)
	if !ok {
		render.WriteError(w, apperror.NewBadRequestError("Organization ID is required", nil))
		return
	}
	req.NetworkID = networkID
	req.OrganizationID = organizationID
	// Platform users invite org users into a network they administer.
	if err := principal.RequireUser(p, req.NetworkID); err != nil {
		render.WriteError(w, err)
		return
	}
	id, err := h.service.Insert(r.Context(), req)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
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
	if p.Type != principal.TypeOrganizationUser || p.ID != existing.ID {
		if err := principal.RequireUser(p, existing.NetworkID); err != nil {
			render.WriteError(w, err)
			return
		}
	}
	var req patchOrganizationUserRequest
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
	if err := principal.RequireOrganizationUser(p, "", ""); err != nil {
		render.WriteError(w, err)
		return
	}
	id := r.PathValue("id")
	if id != p.ID {
		render.WriteError(w, apperror.NewNotFoundError("Organization user not found", nil))
		return
	}
	existing, err := h.service.Get(r.Context(), p, id)
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
