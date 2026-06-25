package render

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lutia-io/huma/pkg/apperror"
)

func TestWriteInternalError(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteInternalError(rec)

	assertErrorResponse(t, rec, http.StatusInternalServerError, "Internal error")
}

func TestWriteError_nil(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, nil)

	assertErrorResponse(t, rec, http.StatusInternalServerError, "Internal error")
}

func TestWriteError_badRequest(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, apperror.NewBadRequestError("op", "invalid input", nil))

	assertErrorResponse(t, rec, http.StatusBadRequest, "invalid input")
}

func TestWriteError_conflict(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, apperror.NewConflictError("op", "already exists", nil))

	assertErrorResponse(t, rec, http.StatusConflict, "already exists")
}

func TestWriteError_notFound(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, apperror.NewNotFoundError("op", "missing", nil))

	assertErrorResponse(t, rec, http.StatusNotFound, "missing")
}

func TestWriteError_internalVariant(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, apperror.NewInternalError("op", "db down", nil))

	assertErrorResponse(t, rec, http.StatusInternalServerError, "Internal error")
}

func TestWriteError_unknownVariant(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, &apperror.Error{Variant: "other", Msg: "ignored"})

	assertErrorResponse(t, rec, http.StatusInternalServerError, "Internal error")
}

func TestWriteError_nonAppError(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, errors.New("plain error"))

	assertErrorResponse(t, rec, http.StatusInternalServerError, "Internal error")
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMsg string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("status: got %d want %d", rec.Code, wantStatus)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: got %q want application/json", ct)
	}
	wantBody := `{"message":"` + wantMsg + `"}`
	if got := strings.TrimSpace(rec.Body.String()); got != wantBody {
		t.Fatalf("body: got %q want %q", got, wantBody)
	}
}
