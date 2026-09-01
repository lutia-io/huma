package render

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lutia-io/huma/pkg/apperror"
)

func TestWriteInternalError(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteInternalError(rec)

	assertErrorResponse(t, rec, http.StatusInternalServerError, "internal", "Internal error")
}

func TestWriteError_nil(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, nil)

	assertErrorResponse(t, rec, http.StatusInternalServerError, "internal", "Internal error")
}

func TestWriteError_badRequest(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, apperror.NewBadRequestError("invalid input", nil))

	assertErrorResponse(t, rec, http.StatusBadRequest, "bad_request", "invalid input")
}

func TestWriteError_unauthorized(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, apperror.NewUnauthorizedError("nope", nil))

	assertErrorResponse(t, rec, http.StatusUnauthorized, "unauthorized", "nope")
}

func TestWriteError_forbidden(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, apperror.NewForbiddenError("denied", nil))

	assertErrorResponse(t, rec, http.StatusForbidden, "forbidden", "denied")
}

func TestWriteError_conflict(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, apperror.NewConflictError("already exists", nil))

	assertErrorResponse(t, rec, http.StatusConflict, "conflict", "already exists")
}

func TestWriteError_notFound(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, apperror.NewNotFoundError("missing", nil))

	assertErrorResponse(t, rec, http.StatusNotFound, "not_found", "missing")
}

func TestWriteError_wrapped(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, fmt.Errorf("store: %w", apperror.NewNotFoundError("missing", errors.New("no rows"))))

	assertErrorResponse(t, rec, http.StatusNotFound, "not_found", "missing")
}

func TestWriteError_internalVariant(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, apperror.NewInternalError("db down", errors.New("pq: connection refused")))

	assertErrorResponse(t, rec, http.StatusInternalServerError, "internal", "Internal error")
}

func TestWriteError_unknownVariant(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, &apperror.Error{Variant: "other", Msg: "ignored"})

	assertErrorResponse(t, rec, http.StatusInternalServerError, "internal", "Internal error")
}

func TestWriteError_nonAppError(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, errors.New("plain error"))

	assertErrorResponse(t, rec, http.StatusInternalServerError, "internal", "Internal error")
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode, wantMsg string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("status: got %d want %d", rec.Code, wantStatus)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: got %q want application/json", ct)
	}
	wantBody := `{"code":"` + wantCode + `","message":"` + wantMsg + `"}`
	if got := strings.TrimSpace(rec.Body.String()); got != wantBody {
		t.Fatalf("body: got %q want %q", got, wantBody)
	}
}
