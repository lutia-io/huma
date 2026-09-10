package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/pipeline/executor"
)

func TestMapperExecute(t *testing.T) {
	h := NewMapper()
	result, err := h.Execute(context.Background(), executor.ExecutionContext{
		Input: map[string]any{"salePrice": 100.0},
	}, pipeline.SnapshotNode{
		Type: node.TypeMapper,
		Definition: node.MapperContext{
			Mapping: map[string]any{
				"total": "{{ mul .Input.salePrice 2 }}",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out["total"] != float64(200) {
		t.Fatalf("got %#v", out)
	}
}

func TestListMapperExecute(t *testing.T) {
	h := NewListMapper()
	result, err := h.Execute(context.Background(), executor.ExecutionContext{
		Input: map[string]any{
			"salePrice": 100.0,
			"0": map[string]any{
				"records": []any{
					map[string]any{"id": "inv-1", "data": map[string]any{"ownershipPercent": 25.0}},
					map[string]any{"id": "inv-2", "data": map[string]any{"ownershipPercent": 75.0}},
				},
			},
		},
	}, pipeline.SnapshotNode{
		Type: node.TypeListMapper,
		Definition: node.ListMapperContext{
			From: "{{ .Input.0.records }}",
			As:   "investor",
			Mapping: map[string]any{
				"investorId": "{{ .investor.id }}",
				"amount":     "{{ mul .Input.salePrice (div .investor.data.ownershipPercent 100) }}",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("got %d items", len(out.Items))
	}
	if out.Items[0]["investorId"] != "inv-1" || out.Items[0]["amount"] != float64(25) {
		t.Fatalf("got first %#v", out.Items[0])
	}
	if out.Items[1]["investorId"] != "inv-2" || out.Items[1]["amount"] != float64(75) {
		t.Fatalf("got second %#v", out.Items[1])
	}
}
