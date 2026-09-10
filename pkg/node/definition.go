package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type Type string

const (
	TypeNoop       Type = "NOOP"
	TypeHTTP       Type = "HTTP"
	TypeMapper     Type = "MAPPER"
	TypeListMapper Type = "LIST_MAPPER"
	TypeFile       Type = "FILE"
	TypeRecord     Type = "RECORD"
)

const (
	FileOpRead  = "READ"
	FileOpWrite = "WRITE"

	RecordOpList   = "LIST"
	RecordOpGet    = "GET"
	RecordOpCreate = "CREATE"
	RecordOpUpdate = "UPDATE"
	RecordOpUpsert = "UPSERT"
)

var listMapperAliasPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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

type ListMapperContext struct {
	From    string         `json:"from"`
	As      string         `json:"as,omitempty"`
	Mapping map[string]any `json:"mapping"`
}

type FileContext struct {
	Operation   string `json:"operation"`
	FileID      string `json:"fileId,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Content     string `json:"content,omitempty"`
}

type RecordFilter struct {
	Field string `json:"field"`
	Op    string `json:"op,omitempty"`
	Value string `json:"value,omitempty"`
}

type RecordContext struct {
	Operation string         `json:"operation"`
	SchemaID  string         `json:"schemaId,omitempty"`
	RecordID  string         `json:"recordId,omitempty"`
	Filters   []RecordFilter `json:"filters,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
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
	case TypeListMapper:
		var ctx ListMapperContext
		if err := json.Unmarshal(data, &ctx); err != nil {
			return nil, fmt.Errorf("invalid LIST_MAPPER definition: %w", err)
		}
		ctx.From = strings.TrimSpace(ctx.From)
		if ctx.From == "" {
			return nil, fmt.Errorf("invalid LIST_MAPPER definition: from is required")
		}
		ctx.As = strings.TrimSpace(ctx.As)
		if ctx.As == "" {
			ctx.As = "item"
		}
		if !listMapperAliasPattern.MatchString(ctx.As) {
			return nil, fmt.Errorf("invalid LIST_MAPPER definition: as must be an identifier")
		}
		switch ctx.As {
		case "Record", "Context", "Input":
			return nil, fmt.Errorf("invalid LIST_MAPPER definition: as %q is reserved", ctx.As)
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
		if ctx.Operation != FileOpRead && ctx.Operation != FileOpWrite {
			return nil, fmt.Errorf("invalid FILE definition: operation must be READ or WRITE")
		}
		return ctx, nil
	case TypeRecord:
		var ctx RecordContext
		if err := json.Unmarshal(data, &ctx); err != nil {
			return nil, fmt.Errorf("invalid RECORD definition: %w", err)
		}
		ctx.Operation = strings.ToUpper(strings.TrimSpace(ctx.Operation))
		switch ctx.Operation {
		case RecordOpList, RecordOpGet, RecordOpCreate, RecordOpUpdate, RecordOpUpsert:
		default:
			return nil, fmt.Errorf("invalid RECORD definition: operation must be LIST, GET, CREATE, UPDATE, or UPSERT")
		}
		if ctx.Data == nil {
			ctx.Data = map[string]any{}
		}
		return ctx, nil
	default:
		return nil, fmt.Errorf("unknown node type %q", t)
	}
}
