package user

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/render"
)

type httpHandler struct {
	logger  *slog.Logger
	service *service
}

func newHTTPHandler(logger *slog.Logger, service *service, mux *http.ServeMux) *httpHandler {
	handler := &httpHandler{
		logger:  logger,
		service: service,
	}
	handler.Register(mux)
	return handler
}

func (h *httpHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /users", h.List)
	mux.HandleFunc("POST /users", h.Create)
}

func (h *httpHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.Find(r.Context())
	if err != nil {
		render.WriteError(r.Context(), h.logger, w, err)
		return
	}
	render.WriteJSON(w, http.StatusOK, users)
}

func (h *httpHandler) Create(w http.ResponseWriter, r *http.Request) {
	var user user
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		render.WriteError(r.Context(), h.logger, w,
			apperror.NewBadRequestError("user.http.Create", "Invalid request body", err))
		return
	}
	id, err := h.service.Insert(r.Context(), &user)
	if err != nil {
		render.WriteError(r.Context(), h.logger, w, err)
		return
	}
	render.WriteJSON(w, http.StatusCreated, map[string]string{"id": id})
}
