package render

import (
	"net/http"

	"github.com/lutia-io/huma/pkg/apperror"
)

func WriteInternalError(w http.ResponseWriter) {
	WriteJSON(w, http.StatusInternalServerError, apperror.PublicInternal())
}

func WriteError(w http.ResponseWriter, err error) {
	if err == nil {
		WriteInternalError(w)
		return
	}

	if e, ok := apperror.As(err); ok {
		WriteJSON(w, e.HTTPStatus(), e.Public())
		return
	}
	WriteInternalError(w)
}
