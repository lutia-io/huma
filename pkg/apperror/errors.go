package apperror

import (
	"errors"
	"net/http"
)

type ErrorVariant string

const (
	ErrorVariantBadRequest   ErrorVariant = "bad_request"
	ErrorVariantUnauthorized ErrorVariant = "unauthorized"
	ErrorVariantForbidden    ErrorVariant = "forbidden"
	ErrorVariantConflict     ErrorVariant = "conflict"
	ErrorVariantNotFound     ErrorVariant = "not_found"
	ErrorVariantInternal     ErrorVariant = "internal"
)

type Error struct {
	Variant ErrorVariant
	Msg     string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch {
	case e.Msg == "" && e.Err == nil:
		return "application error"
	case e.Err == nil:
		return e.Msg
	case e.Msg == "":
		return e.Err.Error()
	default:
		return e.Msg + ": " + e.Err.Error()
	}
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *Error) HTTPStatus() int {
	if e == nil {
		return http.StatusInternalServerError
	}
	switch e.Variant {
	case ErrorVariantBadRequest:
		return http.StatusBadRequest
	case ErrorVariantUnauthorized:
		return http.StatusUnauthorized
	case ErrorVariantForbidden:
		return http.StatusForbidden
	case ErrorVariantConflict:
		return http.StatusConflict
	case ErrorVariantNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// Public is the JSON body returned to API clients. Internal details stay off this type.
type Public struct {
	Code    ErrorVariant `json:"code"`
	Message string       `json:"message"`
}

func PublicInternal() Public {
	return Public{Code: ErrorVariantInternal, Message: "Internal error"}
}

func (e *Error) Public() Public {
	if e == nil || e.HTTPStatus() == http.StatusInternalServerError {
		return PublicInternal()
	}
	return Public{Code: e.Variant, Message: e.Msg}
}

func As(err error) (*Error, bool) {
	return errors.AsType[*Error](err)
}

func IsVariant(err error, v ErrorVariant) bool {
	e, ok := As(err)
	return ok && e.Variant == v
}

func IsBadRequest(err error) bool   { return IsVariant(err, ErrorVariantBadRequest) }
func IsUnauthorized(err error) bool { return IsVariant(err, ErrorVariantUnauthorized) }
func IsForbidden(err error) bool    { return IsVariant(err, ErrorVariantForbidden) }
func IsConflict(err error) bool     { return IsVariant(err, ErrorVariantConflict) }
func IsNotFound(err error) bool     { return IsVariant(err, ErrorVariantNotFound) }
func IsInternal(err error) bool     { return IsVariant(err, ErrorVariantInternal) }

func NewBadRequestError(msg string, err error) error {
	return &Error{Variant: ErrorVariantBadRequest, Msg: msg, Err: err}
}

func NewUnauthorizedError(msg string, err error) error {
	return &Error{Variant: ErrorVariantUnauthorized, Msg: msg, Err: err}
}

func NewForbiddenError(msg string, err error) error {
	return &Error{Variant: ErrorVariantForbidden, Msg: msg, Err: err}
}

func NewConflictError(msg string, err error) error {
	return &Error{Variant: ErrorVariantConflict, Msg: msg, Err: err}
}

func NewNotFoundError(msg string, err error) error {
	return &Error{Variant: ErrorVariantNotFound, Msg: msg, Err: err}
}

func NewInternalError(msg string, err error) error {
	return &Error{Variant: ErrorVariantInternal, Msg: msg, Err: err}
}
