package logger

// Standard attribute keys shared across HTTP middleware and application code.
const (
	KeyRequestID  = "request_id"
	KeyMethod     = "method"
	KeyPath       = "path"
	KeyQuery      = "query"
	KeyRemoteAddr = "remote_addr"
	KeyStatus     = "status"
	KeyBytes      = "bytes"
	KeyDuration   = "duration"
	KeyError      = "error"
	KeyCount      = "count"
	KeyEmail      = "email"
	KeyID         = "id"
	KeyPanic      = "panic"
	KeyStack      = "stack"
	KeyName       = "name"
	KeySlug       = "slug"
	KeyUserID     = "user_id"
	KeyPort       = "port"
)
