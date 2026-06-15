package apperror

import (
	"fmt"
)

type ErrorVariant string

const (
	ErrorVariantBadRequest ErrorVariant = "bad_request"
	ErrorVariantConflict   ErrorVariant = "conflict"
	ErrorVariantNotFound   ErrorVariant = "not_found"
	ErrorVariantInternal   ErrorVariant = "internal"
)

type Error struct {
	Variant ErrorVariant
	Op      string
	Msg     string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Op == "" && e.Msg == "" {
		return "user error"
	}
	switch {
	case e.Op != "" && e.Msg != "":
		return fmt.Sprintf("%s: %s", e.Op, e.Msg)
	case e.Op != "":
		return e.Op
	default:
		return e.Msg
	}
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewBadRequestError(op, msg string, err error) error {
	return &Error{Variant: ErrorVariantBadRequest, Op: op, Msg: msg, Err: err}
}

func NewConflictError(op, msg string, err error) error {
	return &Error{Variant: ErrorVariantConflict, Op: op, Msg: msg, Err: err}
}

func NewNotFoundError(op, msg string, err error) error {
	return &Error{Variant: ErrorVariantNotFound, Op: op, Msg: msg, Err: err}
}

func NewInternalError(op, msg string, err error) error {
	return &Error{Variant: ErrorVariantInternal, Op: op, Msg: msg, Err: err}
}
