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
	got, err := ParseDefinition(TypeFile, []byte(`{"operation":"write","filename":"notice.txt","contentType":"text/plain","content":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := got.(FileContext)
	if !ok || ctx.Operation != FileOpWrite || ctx.Filename != "notice.txt" || ctx.ContentType != "text/plain" || ctx.Content != "hello" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseDefinition_listMapper(t *testing.T) {
	got, err := ParseDefinition(TypeListMapper, []byte(`{"from":"{{ .Input.0.records }}","as":"investor","mapping":{"id":"{{ .investor.id }}"}}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := got.(ListMapperContext)
	if !ok || ctx.From == "" || ctx.As != "investor" || ctx.Mapping["id"] != "{{ .investor.id }}" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseDefinition_listMapperDefaultsAs(t *testing.T) {
	got, err := ParseDefinition(TypeListMapper, []byte(`{"from":"{{ .Input.0.records }}"}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := got.(ListMapperContext)
	if !ok || ctx.As != "item" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseDefinition_listMapperRejectsReservedAs(t *testing.T) {
	if _, err := ParseDefinition(TypeListMapper, []byte(`{"from":"{{ .Input.0 }}","as":"Input"}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseDefinition_record(t *testing.T) {
	got, err := ParseDefinition(TypeRecord, []byte(`{"operation":"list","schemaId":"{{ .Input.schemaId }}","filters":[{"field":"fundId","op":"eq","value":"{{ .Input.fundId }}"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := got.(RecordContext)
	if !ok || ctx.Operation != RecordOpList || len(ctx.Filters) != 1 {
		t.Fatalf("got %#v", got)
	}
}

func TestParseDefinition_bulk(t *testing.T) {
	got, err := ParseDefinition(TypeBulk, []byte(`{
		"records": [
			{"schemaId":"{{ .Input.investorSchemaId }}","from":"{{ .Input.0.investors }}","as":"investor","data":{"name":"{{ .investor.name }}"}},
			{"schemaId":"schema-prop","from":"{{ .Input.0.properties }}"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := got.(BulkContext)
	if !ok || len(ctx.Records) != 2 {
		t.Fatalf("got %#v", got)
	}
	if ctx.Records[0].As != "investor" || ctx.Records[0].Data["name"] != "{{ .investor.name }}" {
		t.Fatalf("got first %#v", ctx.Records[0])
	}
	if ctx.Records[1].As != "item" || ctx.Records[1].SchemaID != "schema-prop" {
		t.Fatalf("got second %#v", ctx.Records[1])
	}
	if ctx.Operation != BulkOpCreate {
		t.Fatalf("operation=%s", ctx.Operation)
	}
}

func TestParseDefinition_bulkUpsert(t *testing.T) {
	got, err := ParseDefinition(TypeBulk, []byte(`{
		"operation":"upsert",
		"records": [
			{"schemaId":"schema-inv","from":"{{ .Input.0.items }}","recordId":"{{ .item.id }}"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := got.(BulkContext)
	if !ok || ctx.Operation != BulkOpUpsert || ctx.Records[0].RecordID != "{{ .item.id }}" {
		t.Fatalf("got %#v", got)
	}
}

func TestParseDefinition_bulkRequiresRecords(t *testing.T) {
	if _, err := ParseDefinition(TypeBulk, []byte(`{"records":[]}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseDefinition_unknownType(t *testing.T) {
	if _, err := ParseDefinition(Type("SQL"), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error")
	}
}
