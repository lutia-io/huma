package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/lutia-io/huma/pkg/requestid"
)

func testLogger(t *testing.T) (*Logger, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer

	return NewWithWriter(&buf), &buf
}

func TestNew(t *testing.T) {
	log, _ := testLogger(t)

	if log == nil {
		t.Fatal("New: expected non-nil logger")
	}
	if log.Slog() == nil {
		t.Fatal("Slog: expected non-nil inner logger")
	}
}

func TestInfo(t *testing.T) {
	log, buf := testLogger(t)

	log.Info("hello", KeyPath, "/")

	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) {
		t.Fatalf("log: got %q", out)
	}
	if !strings.Contains(out, `"path":"/"`) {
		t.Fatalf("expected path attr, got %q", out)
	}
}

func TestError(t *testing.T) {
	log, buf := testLogger(t)

	log.Error("failed", KeyError, "boom")

	out := buf.String()
	if !strings.Contains(out, `"level":"ERROR"`) {
		t.Fatalf("expected error level, got %q", out)
	}
	if !strings.Contains(out, `"msg":"failed"`) {
		t.Fatalf("log: got %q", out)
	}
}

func TestInfoContext_withoutRequestID(t *testing.T) {
	log, buf := testLogger(t)

	log.InfoContext(context.Background(), "started", KeyMethod, "GET")

	out := buf.String()
	if !strings.Contains(out, `"msg":"started"`) {
		t.Fatalf("log: got %q", out)
	}
	if strings.Contains(out, KeyRequestID) {
		t.Fatalf("unexpected request_id in log without context id, got %q", out)
	}
}

func TestInfoContext_withRequestID(t *testing.T) {
	log, buf := testLogger(t)

	ctx := requestid.WithContext(context.Background(), "req-123")
	log.InfoContext(ctx, "started", KeyMethod, "GET")

	out := buf.String()
	if !strings.Contains(out, `"request_id":"req-123"`) {
		t.Fatalf("expected request_id in log, got %q", out)
	}
}

func TestWarnContext_withRequestID(t *testing.T) {
	log, buf := testLogger(t)

	ctx := requestid.WithContext(context.Background(), "req-456")
	log.WarnContext(ctx, "slow", KeyDuration, "1s")

	out := buf.String()
	if !strings.Contains(out, `"level":"WARN"`) {
		t.Fatalf("expected warn level, got %q", out)
	}
	if !strings.Contains(out, `"request_id":"req-456"`) {
		t.Fatalf("expected request_id in log, got %q", out)
	}
}

func TestErrorContext_withRequestID(t *testing.T) {
	log, buf := testLogger(t)

	ctx := requestid.WithContext(context.Background(), "req-789")
	log.ErrorContext(ctx, "panic", KeyPanic, "boom")

	out := buf.String()
	if !strings.Contains(out, `"level":"ERROR"`) {
		t.Fatalf("expected error level, got %q", out)
	}
	if !strings.Contains(out, `"request_id":"req-789"`) {
		t.Fatalf("expected request_id in log, got %q", out)
	}
}

func TestWarnContext_withoutRequestID(t *testing.T) {
	log, buf := testLogger(t)

	log.WarnContext(context.Background(), "client error", KeyStatus, 404)

	out := buf.String()
	if !strings.Contains(out, `"level":"WARN"`) {
		t.Fatalf("expected warn level, got %q", out)
	}
	if strings.Contains(out, KeyRequestID) {
		t.Fatalf("unexpected request_id in log without context id, got %q", out)
	}
}

func TestErrorContext_withoutRequestID(t *testing.T) {
	log, buf := testLogger(t)

	log.ErrorContext(context.Background(), "server error", KeyStatus, 500)

	out := buf.String()
	if !strings.Contains(out, `"level":"ERROR"`) {
		t.Fatalf("expected error level, got %q", out)
	}
	if strings.Contains(out, KeyRequestID) {
		t.Fatalf("unexpected request_id in log without context id, got %q", out)
	}
}
