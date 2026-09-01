package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestError_Error(t *testing.T) {
	cause := errors.New("no rows")
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{name: "nil", err: nil, want: "<nil>"},
		{name: "empty", err: &Error{}, want: "application error"},
		{name: "msg only", err: &Error{Msg: "Network not found"}, want: "Network not found"},
		{name: "cause only", err: &Error{Err: cause}, want: "no rows"},
		{name: "msg and cause", err: &Error{Msg: "Network not found", Err: cause}, want: "Network not found: no rows"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("no rows")
	err := NewNotFoundError("Network not found", cause)
	if !errors.Is(err, cause) {
		t.Fatal("expected wrapped cause")
	}
	var e *Error
	if e.Unwrap() != nil {
		t.Fatal("nil receiver Unwrap should be nil")
	}
}

func TestError_HTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil receiver", err: (*Error)(nil), want: http.StatusInternalServerError},
		{name: "bad request", err: NewBadRequestError("bad", nil), want: http.StatusBadRequest},
		{name: "unauthorized", err: NewUnauthorizedError("nope", nil), want: http.StatusUnauthorized},
		{name: "forbidden", err: NewForbiddenError("denied", nil), want: http.StatusForbidden},
		{name: "conflict", err: NewConflictError("exists", nil), want: http.StatusConflict},
		{name: "not found", err: NewNotFoundError("missing", nil), want: http.StatusNotFound},
		{name: "internal", err: NewInternalError("db down", nil), want: http.StatusInternalServerError},
		{name: "unknown", err: &Error{Variant: "other", Msg: "ignored"}, want: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := As(tt.err)
			if got := e.HTTPStatus(); got != tt.want {
				t.Fatalf("HTTPStatus() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestError_Public(t *testing.T) {
	cause := errors.New("pq: connection refused")
	tests := []struct {
		name string
		err  *Error
		want Public
	}{
		{name: "nil", err: nil, want: PublicInternal()},
		{name: "not found", err: &Error{Variant: ErrorVariantNotFound, Msg: "Network not found", Err: cause}, want: Public{Code: ErrorVariantNotFound, Message: "Network not found"}},
		{name: "internal hides msg", err: &Error{Variant: ErrorVariantInternal, Msg: "db down", Err: cause}, want: PublicInternal()},
		{name: "unknown variant", err: &Error{Variant: "other", Msg: "ignored"}, want: PublicInternal()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Public()
			if got != tt.want {
				t.Fatalf("Public() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIsHelpers(t *testing.T) {
	notFound := NewNotFoundError("missing", errors.New("no rows"))
	wrapped := fmt.Errorf("store: %w", notFound)

	if !IsNotFound(notFound) || !IsNotFound(wrapped) {
		t.Fatal("expected IsNotFound to unwrap")
	}
	if IsConflict(notFound) || IsNotFound(errors.New("plain")) || IsNotFound(nil) {
		t.Fatal("IsNotFound matched the wrong error")
	}
	if !IsConflict(NewConflictError("exists", nil)) {
		t.Fatal("expected IsConflict")
	}
	if !IsBadRequest(NewBadRequestError("bad", nil)) {
		t.Fatal("expected IsBadRequest")
	}
	if !IsUnauthorized(NewUnauthorizedError("nope", nil)) {
		t.Fatal("expected IsUnauthorized")
	}
	if !IsForbidden(NewForbiddenError("denied", nil)) {
		t.Fatal("expected IsForbidden")
	}
	if !IsInternal(NewInternalError("db down", nil)) {
		t.Fatal("expected IsInternal")
	}
	if _, ok := As(nil); ok {
		t.Fatal("As should fail for nil")
	}
	if _, ok := As(errors.New("plain")); ok {
		t.Fatal("As should fail for a plain error")
	}
}
