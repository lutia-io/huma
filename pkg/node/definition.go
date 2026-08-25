package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Type string

const (
	TypeNoop   Type = "NOOP"
	TypeHTTP   Type = "HTTP"
	TypeMapper Type = "MAPPER"
	TypeFile   Type = "FILE"
)

type NoopContext struct {
	Message string `json:"message"`
}

type HTTPContext struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

type MapperContext struct {
	Mapping map[string]any `json:"mapping"`
}

type FileContext struct {
	Operation string `json:"operation"`
	FileID    string `json:"fileId,omitempty"`
	Filename  string `json:"filename,omitempty"`
}

func ParseDefinition(t Type, data json.RawMessage) (any, error) {
	if len(data) == 0 {
		data = []byte("{}")
	}
	switch t {
	case TypeNoop:
		var ctx NoopContext
		if err := json.Unmarshal(data, &ctx); err != nil {
			return nil, fmt.Errorf("invalid NOOP definition: %w", err)
		}
		return ctx, nil
	case TypeHTTP:
		var ctx HTTPContext
		if err := json.Unmarshal(data, &ctx); err != nil {
			return nil, fmt.Errorf("invalid HTTP definition: %w", err)
		}
		ctx.Method = strings.ToUpper(strings.TrimSpace(ctx.Method))
		ctx.URL = strings.TrimSpace(ctx.URL)
		if ctx.Method == "" {
			return nil, fmt.Errorf("invalid HTTP definition: method is required")
		}
		switch ctx.Method {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead:
		default:
			return nil, fmt.Errorf("invalid HTTP definition: unsupported method %q", ctx.Method)
		}
		if ctx.URL == "" {
			return nil, fmt.Errorf("invalid HTTP definition: url is required")
		}
		if len(ctx.Body) > 0 && !json.Valid(ctx.Body) {
			return nil, fmt.Errorf("invalid HTTP definition: body must be valid JSON")
		}
		return ctx, nil
	case TypeMapper:
		var ctx MapperContext
		if err := json.Unmarshal(data, &ctx); err != nil {
			return nil, fmt.Errorf("invalid MAPPER definition: %w", err)
		}
		if ctx.Mapping == nil {
			ctx.Mapping = map[string]any{}
		}
		return ctx, nil
	case TypeFile:
		var ctx FileContext
		if err := json.Unmarshal(data, &ctx); err != nil {
			return nil, fmt.Errorf("invalid FILE definition: %w", err)
		}
		ctx.Operation = strings.ToUpper(strings.TrimSpace(ctx.Operation))
		if ctx.Operation != "READ" && ctx.Operation != "WRITE" {
			return nil, fmt.Errorf("invalid FILE definition: operation must be READ or WRITE")
		}
		return ctx, nil
	default:
		return nil, fmt.Errorf("unknown node type %q", t)
	}
}
