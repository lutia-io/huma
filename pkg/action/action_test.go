package action

import (
	"encoding/json"
	"testing"
)

func TestUnmarshalUpdateRecord_keepsSchemaID(t *testing.T) {
	raw := []byte(`{
		"type": "UPDATE_RECORD",
		"context": {
			"schemaId": "11111111-1111-1111-1111-111111111111",
			"recordId": "{{ .Record.id }}",
			"data": { "status": "confirmed" }
		}
	}`)

	var act Action
	if err := json.Unmarshal(raw, &act); err != nil {
		t.Fatal(err)
	}
	ctx, ok := act.Context.(UpdateRecordContext)
	if !ok {
		t.Fatalf("context type %T", act.Context)
	}
	if ctx.SchemaID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("schemaId=%q", ctx.SchemaID)
	}
	if ctx.RecordID != "{{ .Record.id }}" {
		t.Fatalf("recordId=%q", ctx.RecordID)
	}

	out, err := json.Marshal(act)
	if err != nil {
		t.Fatal(err)
	}
	var round Action
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatal(err)
	}
	got, ok := round.Context.(UpdateRecordContext)
	if !ok {
		t.Fatalf("round-trip context type %T", round.Context)
	}
	if got.SchemaID != ctx.SchemaID {
		t.Fatalf("round-trip schemaId=%q", got.SchemaID)
	}
}

func TestUnmarshalUpdateRecord_schemaIDOptional(t *testing.T) {
	raw := []byte(`{
		"type": "UPDATE_RECORD",
		"context": {
			"recordId": "rec-1",
			"data": { "status": "open" }
		}
	}`)

	var act Action
	if err := json.Unmarshal(raw, &act); err != nil {
		t.Fatal(err)
	}
	ctx, ok := act.Context.(UpdateRecordContext)
	if !ok {
		t.Fatalf("context type %T", act.Context)
	}
	if ctx.SchemaID != "" {
		t.Fatalf("schemaId=%q", ctx.SchemaID)
	}
	if ctx.RecordID != "rec-1" {
		t.Fatalf("recordId=%q", ctx.RecordID)
	}
}
