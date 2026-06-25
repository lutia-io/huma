package render

import (
	"errors"
	"net/http"

	"github.com/lutia-io/huma/pkg/apperror"
)

func WriteInternalError(w http.ResponseWriter) {
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"message": "Internal error"})
}

func WriteError(w http.ResponseWriter, err error) {
	if err == nil {
		WriteInternalError(w)
		return
	}

	if e, ok := errors.AsType[*apperror.Error](err); ok {
		switch e.Variant {
		case apperror.ErrorVariantBadRequest:
			WriteJSON(w, http.StatusBadRequest, map[string]string{"message": e.Msg})
			return
		case apperror.ErrorVariantConflict:
			WriteJSON(w, http.StatusConflict, map[string]string{"message": e.Msg})
			return
		case apperror.ErrorVariantNotFound:
			WriteJSON(w, http.StatusNotFound, map[string]string{"message": e.Msg})
			return
		default:
			WriteInternalError(w)
			return
		}
	}
	WriteInternalError(w)
}
