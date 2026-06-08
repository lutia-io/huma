package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewLogger_statusAndBytes(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hello"))
	})

	h := NewLogger(logger, inner)
	req := httptest.NewRequest(http.MethodGet, "/things?q=1", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusTeapot)
	}
	out := buf.String()
	if !strings.Contains(out, `status=418`) {
		t.Fatalf("log missing status: %q", out)
	}
	if !strings.Contains(out, `bytes=5`) {
		t.Fatalf("log missing byte count: %q", out)
	}
	if !strings.Contains(out, `method=GET`) || !strings.Contains(out, `path=/things`) {
		t.Fatalf("log missing method/path: %q", out)
	}
	if !strings.Contains(out, "q=1") {
		t.Fatalf("log missing query: %q", out)
	}
}

func TestNewLogger_implicitOK(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "x")
	})

	h := NewLogger(logger, inner)
	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if !strings.Contains(buf.String(), `status=200`) {
		t.Fatalf("expected implicit 200 in log: %q", buf.String())
	}
}

// TestNewLogger_noWrite covers the branch where the handler returns without
// writing anything: the recorder's status stays 0 and the logger defaults it
// to 200.
func TestNewLogger_noWrite(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	h := NewLogger(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if !strings.Contains(buf.String(), `status=200`) {
		t.Fatalf("expected defaulted 200 in log: %q", buf.String())
	}
	if !strings.Contains(buf.String(), `bytes=0`) {
		t.Fatalf("expected zero byte count: %q", buf.String())
	}
}

// TestNewLogger_levels exercises the level selection for 4xx (warn) and 5xx
// (error) responses.
func TestNewLogger_levels(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantLevel  string
		wantStatus string
	}{
		{"server error", http.StatusInternalServerError, "level=ERROR", "status=500"},
		{"client error", http.StatusBadRequest, "level=WARN", "status=400"},
		{"ok", http.StatusOK, "level=INFO", "status=200"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
			h := NewLogger(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			out := buf.String()
			// Grab only the completion line for the level assertion.
			if !strings.Contains(out, "http request completed") {
				t.Fatalf("missing completion log: %q", out)
			}
			completed := out[strings.Index(out, "http request completed"):]
			completedLine := completed
			if i := strings.IndexByte(completed, '\n'); i >= 0 {
				completedLine = completed[:i]
			}
			// Level appears before the message; assert it is present anywhere in output.
			if !strings.Contains(out, tt.wantLevel) {
				t.Fatalf("want level %q in %q", tt.wantLevel, out)
			}
			if !strings.Contains(completedLine, tt.wantStatus) {
				t.Fatalf("want %q in completion line %q", tt.wantStatus, completedLine)
			}
		})
	}
}

// TestNewLogger_requestID ensures the request ID from context is logged.
func TestNewLogger_requestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// NewRequestID populates the context that NewLogger reads from.
	h := NewRequestID(NewLogger(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "abc-123")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if !strings.Contains(buf.String(), "request_id=abc-123") {
		t.Fatalf("expected request_id in log: %q", buf.String())
	}
}

// TestNewLogger_doubleWriteHeader verifies only the first status is recorded.
func TestNewLogger_doubleWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	h := NewLogger(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(buf.String(), "status=201") {
		t.Fatalf("expected first status recorded: %q", buf.String())
	}
}

// TestNewLogger_unwrap verifies the recorder exposes the underlying writer via
// http.ResponseController (which relies on Unwrap).
func TestNewLogger_unwrap(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var flushErr error
	h := NewLogger(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "stream")
		flushErr = http.NewResponseController(w).Flush()
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if flushErr != nil {
		t.Fatalf("flush via Unwrap failed: %v", flushErr)
	}
	if !rec.Flushed {
		t.Fatalf("expected underlying recorder to be flushed")
	}
}
