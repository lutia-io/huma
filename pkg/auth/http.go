package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

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
	mux.HandleFunc("POST /auth/login/user", h.LoginUser)
	mux.HandleFunc("POST /auth/login/organization-user", h.LoginOrganizationUser)
	mux.HandleFunc("POST /auth/refresh", h.Refresh)
	mux.HandleFunc("POST /auth/logout", h.Logout)
	mux.HandleFunc("GET /auth/me", h.Me)
}

func (h *httpHandler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req loginUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	pair, err := h.service.LoginUser(r.Context(), req, clientIP(r))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, pair)
}

func (h *httpHandler) LoginOrganizationUser(w http.ResponseWriter, r *http.Request) {
	var req loginOrganizationUserRequest
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
	pair, err := h.service.LoginOrganizationUser(r.Context(), req, clientIP(r))
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, pair)
}

func (h *httpHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	pair, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, pair)
}

func (h *httpHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.WriteError(w, apperror.NewBadRequestError("Invalid request body", err))
		return
	}
	if err := h.service.Logout(r.Context(), req.RefreshToken); err != nil {
		render.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *httpHandler) Me(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.FromContext(r.Context())
	if !ok {
		render.WriteError(w, apperror.NewUnauthorizedError("Authentication required", nil))
		return
	}
	me, err := h.service.Me(r.Context(), p)
	if err != nil {
		render.WriteError(w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, me)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func bearerToken(r *http.Request) (string, bool) {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if authz == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authz, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}
