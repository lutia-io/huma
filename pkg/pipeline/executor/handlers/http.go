package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/pipeline/executor"
	"github.com/lutia-io/huma/pkg/resolver"
)

type HTTP struct {
	client *http.Client
}

func NewHTTP(client *http.Client) *HTTP {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTP{client: client}
}

func (h *HTTP) Type() node.Type {
	return node.TypeHTTP
}

func (h *HTTP) Execute(ctx context.Context, execCtx executor.ExecutionContext, n pipeline.SnapshotNode) (json.RawMessage, error) {
	raw, err := json.Marshal(n.Definition)
	if err != nil {
		return nil, err
	}
	def, err := node.ParseDefinition(node.TypeHTTP, raw)
	if err != nil {
		return nil, err
	}
	httpDef, ok := def.(node.HTTPContext)
	if !ok {
		return nil, fmt.Errorf("invalid HTTP definition type %T", def)
	}

	payload := map[string]any{
		"url":     httpDef.URL,
		"headers": headersToAny(httpDef.Headers),
	}
	if len(httpDef.Body) > 0 {
		var body any
		if err := json.Unmarshal(httpDef.Body, &body); err != nil {
			return nil, fmt.Errorf("invalid HTTP body: %w", err)
		}
		payload["body"] = body
	}

	resolved, err := resolver.ResolveInput(payload, execCtx.Input)
	if err != nil {
		return nil, fmt.Errorf("resolving HTTP definition: %w", err)
	}

	urlValue, _ := resolved["url"].(string)
	if urlValue == "" {
		return nil, fmt.Errorf("HTTP url resolved empty")
	}

	var bodyReader io.Reader
	if body, ok := resolved["body"]; ok && body != nil {
		switch v := body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		default:
			encoded, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewReader(encoded)
		}
	}

	req, err := http.NewRequestWithContext(ctx, httpDef.Method, urlValue, bodyReader)
	if err != nil {
		return nil, err
	}
	if headers, ok := resolved["headers"].(map[string]any); ok {
		for key, value := range headers {
			if s, ok := value.(string); ok && s != "" {
				req.Header.Set(key, s)
			}
		}
	}
	if httpDef.Method != http.MethodGet && httpDef.Method != http.MethodHead {
		if req.Header.Get("Idempotency-Key") == "" {
			req.Header.Set("Idempotency-Key", execCtx.IdempotencyKey)
		}
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	out := map[string]any{
		"status":  resp.StatusCode,
		"headers": flattenHeaders(resp.Header),
		"body":    decodeBody(respBody, resp.Header.Get("Content-Type")),
	}
	return json.Marshal(out)
}

func headersToAny(headers map[string]string) map[string]any {
	out := make(map[string]any, len(headers))
	for k, v := range headers {
		out[k] = v
	}
	return out
}

func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vals := range h {
		if len(vals) > 0 {
			out[k] = vals[0]
		}
	}
	return out
}

func decodeBody(body []byte, contentType string) any {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	if strings.Contains(contentType, "json") || json.Valid(trimmed) {
		var v any
		if err := json.Unmarshal(trimmed, &v); err == nil {
			return v
		}
	}
	return string(body)
}
