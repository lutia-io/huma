package render

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lutia-io/huma/pkg/apperror"
)

func WriteInternalError(w http.ResponseWriter) {
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal error"})
}

func WriteError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, err error) {
	if err == nil {
		if logger != nil {
			logger.ErrorContext(ctx, "Handler returned nil error")
		}
		WriteInternalError(w)
		return
	}

	if e, ok := errors.AsType[*apperror.Error](err); ok {
		switch e.Variant {
		case apperror.ErrorVariantBadRequest:
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": e.Msg})
			return
		case apperror.ErrorVariantConflict:
			WriteJSON(w, http.StatusConflict, map[string]string{"error": e.Msg})
			return
		case apperror.ErrorVariantNotFound:
			WriteJSON(w, http.StatusNotFound, map[string]string{"error": e.Msg})
			return
		default:
			if logger != nil {
				logger.ErrorContext(ctx, "Internal application error", "op", e.Op, "msg", e.Msg, "error", e.Err)
			}
			WriteInternalError(w)
			return
		}
	}

	if logger != nil {
		logger.ErrorContext(ctx, "Unhandled error", "error", err)
	}
	WriteInternalError(w)
}
