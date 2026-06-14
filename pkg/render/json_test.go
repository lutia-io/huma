package render

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type errResponseWriter struct {
	header http.Header
	status int
}

func (w *errResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *errResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (w *errResponseWriter) WriteHeader(status int) {
	w.status = status
}

func TestWriteJSON_encodesBody(t *testing.T) {
	rec := httptest.NewRecorder()

	if err := WriteJSON(rec, http.StatusCreated, map[string]string{"ok": "yes"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusCreated)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: got %q want application/json", ct)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"ok":"yes"}` {
		t.Fatalf("body: got %q want %q", got, `{"ok":"yes"}`)
	}
}

func TestWriteJSON_nilBody(t *testing.T) {
	rec := httptest.NewRecorder()

	if err := WriteJSON(rec, http.StatusNoContent, nil); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusNoContent)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: got %q want application/json", ct)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body: got %q want empty", rec.Body.String())
	}
}

func TestWriteJSON_encodeError(t *testing.T) {
	w := &errResponseWriter{}

	if err := WriteJSON(w, http.StatusOK, map[string]string{"ok": "yes"}); err == nil {
		t.Fatal("WriteJSON: expected encode error, got nil")
	}
	if w.status != http.StatusOK {
		t.Fatalf("status: got %d want %d", w.status, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: got %q want application/json", ct)
	}
}
