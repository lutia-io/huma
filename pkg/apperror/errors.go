package apperror

type ErrorVariant string

const (
	ErrorVariantBadRequest   ErrorVariant = "bad_request"
	ErrorVariantUnauthorized ErrorVariant = "unauthorized"
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
	if e.Msg == "" {
		return "application error"
	}
	return e.Msg
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewBadRequestError(msg string, err error) error {
	return &Error{Variant: ErrorVariantBadRequest, Msg: msg, Err: err}
}

func NewUnauthorizedError(msg string, err error) error {
	return &Error{Variant: ErrorVariantUnauthorized, Msg: msg, Err: err}
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
