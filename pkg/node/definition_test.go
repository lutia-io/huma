package node

import (
	"encoding/json"
	"testing"
)

func TestParseDefinition_noop(t *testing.T) {
	got, err := ParseDefinition(TypeNoop, []byte(`{"message":"ok"}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := got.(NoopContext)
	if !ok || ctx.Message != "ok" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseDefinition_http(t *testing.T) {
	got, err := ParseDefinition(TypeHTTP, []byte(`{"method":"get","url":"https://example.com","headers":{"Accept":"application/json"},"body":{"q":"{{ .Input.orgId }}"}}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := got.(HTTPContext)
	if !ok {
		t.Fatalf("got %T", got)
	}
	if ctx.Method != "GET" || ctx.URL != "https://example.com" {
		t.Fatalf("got method=%s url=%s", ctx.Method, ctx.URL)
	}
	if ctx.Headers["Accept"] != "application/json" {
		t.Fatalf("got headers=%v", ctx.Headers)
	}
}

func TestParseDefinition_httpRejectsMissingURL(t *testing.T) {
	if _, err := ParseDefinition(TypeHTTP, []byte(`{"method":"POST"}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseDefinition_mapper(t *testing.T) {
	got, err := ParseDefinition(TypeMapper, []byte(`{"mapping":{"name":"{{ .Input.0.body.name }}"}}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := got.(MapperContext)
	if !ok || ctx.Mapping["name"] != "{{ .Input.0.body.name }}" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseDefinition_file(t *testing.T) {
	got, err := ParseDefinition(TypeFile, []byte(`{"operation":"read","fileId":"{{ .Input.fileId }}"}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := got.(FileContext)
	if !ok || ctx.Operation != "READ" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseDefinition_unknownType(t *testing.T) {
	if _, err := ParseDefinition(Type("SQL"), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error")
	}
}
