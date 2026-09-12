package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/pipeline/executor"
	"github.com/lutia-io/huma/pkg/record"
)

func TestBulkExecute(t *testing.T) {
	fake := &fakeRecords{id: "rec"}
	h := NewBulk(fake, staticSystemUsers{id: "sys-1"})
	result, err := h.Execute(context.Background(), executor.ExecutionContext{
		NetworkID:      "net-1",
		OrganizationID: "org-1",
		IdempotencyKey: "pipe:2:0",
		Input: map[string]any{
			"investorSchemaId": "schema-inv",
			"0": map[string]any{
				"investors": []any{
					map[string]any{"name": "Ada"},
					map[string]any{"name": "Grace"},
				},
				"properties": []any{
					map[string]any{"address": "1 Main"},
				},
			},
		},
	}, pipeline.SnapshotNode{
		Type: node.TypeBulk,
		Definition: node.BulkContext{
			Records: []node.BulkRecord{
				{
					SchemaID: "{{ .Input.investorSchemaId }}",
					From:     "{{ .Input.0.investors }}",
					As:       "investor",
					Data:     map[string]any{"name": "{{ .investor.name }}"},
				},
				{
					SchemaID: "schema-prop",
					From:     "{{ .Input.0.properties }}",
					As:       "item",
					Data:     map[string]any{"address": "{{ .item.address }}"},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var out struct {
		Total   int `json:"total"`
		Created []struct {
			SchemaID string   `json:"schemaId"`
			IDs      []string `json:"ids"`
			Total    int      `json:"total"`
		} `json:"created"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 3 || len(out.Created) != 2 {
		t.Fatalf("got %#v", out)
	}
	if out.Created[0].SchemaID != "schema-inv" || out.Created[0].Total != 2 {
		t.Fatalf("got first %#v", out.Created[0])
	}
	if out.Created[1].SchemaID != "schema-prop" || out.Created[1].Total != 1 {
		t.Fatalf("got second %#v", out.Created[1])
	}
	if len(fake.created) != 3 {
		t.Fatalf("created %d", len(fake.created))
	}
	if fake.created[0].SchemaID != "schema-inv" || fake.created[0].OrganizationUserID != "sys-1" {
		t.Fatalf("first create %#v", fake.created[0])
	}
	if fake.created[0].IdempotencyKey != "pipe:2:0:0:0" || fake.created[2].IdempotencyKey != "pipe:2:0:1:0" {
		t.Fatalf("idempotency %#v %#v", fake.created[0].IdempotencyKey, fake.created[2].IdempotencyKey)
	}

	var first map[string]any
	if err := json.Unmarshal(fake.created[0].Data, &first); err != nil {
		t.Fatal(err)
	}
	if first["name"] != "Ada" {
		t.Fatalf("first data %#v", first)
	}
	var last map[string]any
	if err := json.Unmarshal(fake.created[2].Data, &last); err != nil {
		t.Fatal(err)
	}
	if last["address"] != "1 Main" {
		t.Fatalf("last data %#v", last)
	}
}

func TestBulkExecute_upsert(t *testing.T) {
	existing := &record.Record{
		ID:             "inv-1",
		SchemaID:       "schema-inv",
		NetworkID:      "net-1",
		OrganizationID: "org-1",
		Data:           json.RawMessage(`{"name":"Ada","status":"active"}`),
	}
	fake := &fakeRecords{id: "rec", items: []*record.Record{existing}}
	h := NewBulk(fake, staticSystemUsers{id: "sys-1"})
	result, err := h.Execute(context.Background(), executor.ExecutionContext{
		NetworkID:      "net-1",
		OrganizationID: "org-1",
		IdempotencyKey: "pipe:4:0",
		Input: map[string]any{
			"0": map[string]any{
				"investors": []any{
					map[string]any{"id": "inv-1", "name": "Ada Lovelace"},
					map[string]any{"name": "Grace"},
				},
			},
		},
	}, pipeline.SnapshotNode{
		Type: node.TypeBulk,
		Definition: node.BulkContext{
			Operation: node.BulkOpUpsert,
			Records: []node.BulkRecord{{
				SchemaID: "schema-inv",
				From:     "{{ .Input.0.investors }}",
				As:       "investor",
				RecordID: "{{ .investor.id }}",
				Data: map[string]any{
					"name": "{{ .investor.name }}",
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var out struct {
		Total   int `json:"total"`
		Created []struct {
			Created int `json:"created"`
			Updated int `json:"updated"`
			Total   int `json:"total"`
			IDs     []string `json:"ids"`
		} `json:"created"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 || out.Created[0].Created != 1 || out.Created[0].Updated != 1 {
		t.Fatalf("got %#v", out)
	}
	if out.Created[0].IDs[0] != "inv-1" {
		t.Fatalf("updated id=%s", out.Created[0].IDs[0])
	}
	if len(fake.created) != 1 {
		t.Fatalf("created %d", len(fake.created))
	}
	var updated map[string]any
	if err := json.Unmarshal(existing.Data, &updated); err != nil {
		t.Fatal(err)
	}
	if updated["name"] != "Ada Lovelace" || updated["status"] != "active" {
		t.Fatalf("updated data %#v", updated)
	}
}

func TestBulkExecute_emptyList(t *testing.T) {
	fake := &fakeRecords{id: "rec"}
	h := NewBulk(fake, staticSystemUsers{id: "sys-1"})
	result, err := h.Execute(context.Background(), executor.ExecutionContext{
		NetworkID:      "net-1",
		OrganizationID: "org-1",
		IdempotencyKey: "pipe:3:0",
		Input: map[string]any{
			"0": map[string]any{"items": []any{}},
		},
	}, pipeline.SnapshotNode{
		Type: node.TypeBulk,
		Definition: node.BulkContext{
			Records: []node.BulkRecord{{
				SchemaID: "schema-inv",
				From:     "{{ .Input.0.items }}",
				As:       "item",
				Data:     map[string]any{"name": "{{ .item.name }}"},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 0 || len(fake.created) != 0 {
		t.Fatalf("got total=%d created=%d", out.Total, len(fake.created))
	}
}
