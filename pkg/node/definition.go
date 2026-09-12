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
	TypeBulk       Type = "BULK"
)

const (
	FileOpRead  = "READ"
	FileOpWrite = "WRITE"

	RecordOpList   = "LIST"
	RecordOpGet    = "GET"
	RecordOpCreate = "CREATE"
	RecordOpUpdate = "UPDATE"
	RecordOpUpsert = "UPSERT"

	BulkOpCreate = "CREATE"
	BulkOpUpsert = "UPSERT"
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

// BulkRecord writes many records of one schema from a list.
type BulkRecord struct {
	SchemaID string         `json:"schemaId"`
	From     string         `json:"from"`
	As       string         `json:"as,omitempty"`
	RecordID string         `json:"recordId,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

// BulkContext inserts or upserts multiple records for one or more record types.
type BulkContext struct {
	Operation string       `json:"operation,omitempty"`
	Records   []BulkRecord `json:"records"`
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
		as, err := normalizeListAlias(ctx.As)
		if err != nil {
			return nil, fmt.Errorf("invalid LIST_MAPPER definition: %w", err)
		}
		ctx.As = as
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
	case TypeBulk:
		var ctx BulkContext
		if err := json.Unmarshal(data, &ctx); err != nil {
			return nil, fmt.Errorf("invalid BULK definition: %w", err)
		}
		ctx.Operation = strings.ToUpper(strings.TrimSpace(ctx.Operation))
		if ctx.Operation == "" {
			ctx.Operation = BulkOpCreate
		}
		switch ctx.Operation {
		case BulkOpCreate, BulkOpUpsert:
		default:
			return nil, fmt.Errorf("invalid BULK definition: operation must be CREATE or UPSERT")
		}
		if len(ctx.Records) == 0 {
			return nil, fmt.Errorf("invalid BULK definition: at least one record type is required")
		}
		for i := range ctx.Records {
			item := &ctx.Records[i]
			item.SchemaID = strings.TrimSpace(item.SchemaID)
			if item.SchemaID == "" {
				return nil, fmt.Errorf("invalid BULK definition: records[%d].schemaId is required", i)
			}
			item.From = strings.TrimSpace(item.From)
			if item.From == "" {
				return nil, fmt.Errorf("invalid BULK definition: records[%d].from is required", i)
			}
			as, err := normalizeListAlias(item.As)
			if err != nil {
				return nil, fmt.Errorf("invalid BULK definition: records[%d]: %w", i, err)
			}
			item.As = as
			item.RecordID = strings.TrimSpace(item.RecordID)
			if item.Data == nil {
				item.Data = map[string]any{}
			}
		}
		return ctx, nil
	default:
		return nil, fmt.Errorf("unknown node type %q", t)
	}
}

func normalizeListAlias(as string) (string, error) {
	as = strings.TrimSpace(as)
	if as == "" {
		as = "item"
	}
	if !listMapperAliasPattern.MatchString(as) {
		return "", fmt.Errorf("as must be an identifier")
	}
	switch as {
	case "Record", "Context", "Input":
		return "", fmt.Errorf("as %q is reserved", as)
	}
	return as, nil
}
