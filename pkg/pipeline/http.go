package pipeline

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
	mux.HandleFunc("GET /pipeline-definition", h.List)
	mux.HandleFunc("GET /pipeline-definition/{id}", h.Get)
	mux.HandleFunc("POST /pipeline-definition", h.Insert)
	mux.HandleFunc("PATCH /pipeline-definition/{id}", h.Patch)

	mux.HandleFunc("POST /pipeline", h.InsertPipeline)
	mux.HandleFunc("GET /pipeline", h.ListPipelines)
	mux.HandleFunc("GET /pipeline/{id}", h.GetPipeline)
	mux.HandleFunc("GET /pipeline/{id}/node", h.ListPipelineNodes)
	mux.HandleFunc("GET /pipeline-node/{id}", h.GetPipelineNode)
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
	pipeline, err := h.service.Get(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, pipeline)
}

func (h *httpHandler) Insert(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	var req insertPipelineDefinitionRequest
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
	req.Internal = false

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
	if err := principal.RequireUser(p, existing.NetworkID); err != nil {
		render.WriteError(w, err)
		return
	}
	var req patchPipelineDefinitionRequest
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

func (h *httpHandler) InsertPipeline(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	if err := principal.RequireOrganizationUser(p, p.NetworkID, p.OrganizationID); err != nil {
		render.WriteError(w, err)
		return
	}
	var req insertPipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	id, err := h.service.Enqueue(r.Context(), EnqueueRequest{
		PipelineDefinitionID: req.PipelineDefinitionID,
		NetworkID:            p.NetworkID,
		OrganizationID:       p.OrganizationID,
		OrganizationUserID:   p.ID,
		Input:                req.Input,
		DedupeKey:            req.DedupeKey,
	})
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (h *httpHandler) ListPipelines(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	params, err := parseRunListParams(r)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	result, err := h.service.ListPipelines(r.Context(), p, params)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, result)
}

func (h *httpHandler) GetPipeline(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	run, err := h.service.GetPipeline(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, run)
}

func (h *httpHandler) ListPipelineNodes(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	nodes, err := h.service.ListPipelineNodes(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, nodes)
}

func (h *httpHandler) GetPipelineNode(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	n, err := h.service.GetPipelineNode(r.Context(), p, r.PathValue("id"))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, n)
}
