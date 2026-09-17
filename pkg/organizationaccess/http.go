package organizationaccess

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
	handler := &httpHandler{service: service}
	handler.Register(mux)
	return handler
}

func (h *httpHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /organization-group", h.ListGroups)
	mux.HandleFunc("POST /organization-group", h.InsertGroup)
	mux.HandleFunc("GET /organization-group/{id}", h.GetGroup)
	mux.HandleFunc("PATCH /organization-group/{id}", h.PatchGroup)
	mux.HandleFunc("DELETE /organization-group/{id}", h.DeleteGroup)
	mux.HandleFunc("POST /organization-group/{id}/member", h.AddMember)
	mux.HandleFunc("DELETE /organization-group/{id}/member/{organizationUserId}", h.RemoveMember)
	mux.HandleFunc("POST /organization-group/{id}/permission", h.AssignPermission)
	mux.HandleFunc("DELETE /organization-group/{id}/permission/{permissionId}", h.UnassignPermission)
	mux.HandleFunc("GET /organization-permission", h.ListPermissions)
	mux.HandleFunc("POST /organization-permission", h.InsertPermission)
	mux.HandleFunc("GET /organization-permission/{id}", h.GetPermission)
	mux.HandleFunc("PATCH /organization-permission/{id}", h.PatchPermission)
	mux.HandleFunc("DELETE /organization-permission/{id}", h.DeletePermission)
}

func (h *httpHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	networkID, ok := network.ResolveID(r, p, r.URL.Query().Get("networkId"))
	if !ok {
		render.WriteError(w, apperror.NewBadRequestError("Network ID is required", nil))
		return
	}
	organizationID, ok := organization.ResolveID(r, p, r.URL.Query().Get("organizationId"))
	if !ok {
		render.WriteError(w, apperror.NewBadRequestError("Organization ID is required", nil))
		return
	}
	items, err := h.service.ListGroups(r.Context(), p, networkID, organizationID)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, items)
}

func (h *httpHandler) GetGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	g, err := h.service.GetGroup(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, g)
}

func (h *httpHandler) InsertGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	var req insertGroupRequest
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
	id, err := h.service.InsertGroup(r.Context(), p, req)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *httpHandler) PatchGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	existing, err := h.service.GetGroup(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	var req patchGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	if err := h.service.PatchGroup(r.Context(), p, existing, req); err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, map[string]string{"id": existing.ID})
}

func (h *httpHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	existing, err := h.service.GetGroup(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	if err := h.service.DeleteGroup(r.Context(), p, existing); err != nil {
		render.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *httpHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	existing, err := h.service.GetGroup(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	var req addMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	if err := h.service.AddMember(r.Context(), p, existing, req.OrganizationUserID); err != nil {
		render.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *httpHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	existing, err := h.service.GetGroup(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	if err := h.service.RemoveMember(r.Context(), p, existing, r.PathValue("organizationUserId")); err != nil {
		render.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *httpHandler) AssignPermission(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	existing, err := h.service.GetGroup(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	var req assignPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	if err := h.service.AssignPermission(r.Context(), p, existing, req.PermissionID); err != nil {
		render.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *httpHandler) UnassignPermission(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	existing, err := h.service.GetGroup(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	if err := h.service.UnassignPermission(r.Context(), p, existing, r.PathValue("permissionId")); err != nil {
		render.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *httpHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	networkID, ok := network.ResolveID(r, p, r.URL.Query().Get("networkId"))
	if !ok {
		render.WriteError(w, apperror.NewBadRequestError("Network ID is required", nil))
		return
	}
	organizationID, ok := organization.ResolveID(r, p, r.URL.Query().Get("organizationId"))
	if !ok {
		render.WriteError(w, apperror.NewBadRequestError("Organization ID is required", nil))
		return
	}
	items, err := h.service.ListPermissions(r.Context(), p, networkID, organizationID)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, items)
}

func (h *httpHandler) GetPermission(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	perm, err := h.service.GetPermission(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, perm)
}

func (h *httpHandler) InsertPermission(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	var req insertPermissionRequest
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
	id, err := h.service.InsertPermission(r.Context(), p, req)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *httpHandler) PatchPermission(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	existing, err := h.service.GetPermission(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	var req patchPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	if err := h.service.PatchPermission(r.Context(), p, existing, req); err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, map[string]string{"id": existing.ID})
}

func (h *httpHandler) DeletePermission(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	existing, err := h.service.GetPermission(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	if err := h.service.DeletePermission(r.Context(), p, existing); err != nil {
		render.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
