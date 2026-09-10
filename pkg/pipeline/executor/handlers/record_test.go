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

type fakeRecords struct {
	items   []*record.Record
	created record.CreateParams
	id      string
}

func (f *fakeRecords) Create(_ context.Context, params record.CreateParams) (string, error) {
	f.created = params
	return f.id, nil
}

func (f *fakeRecords) Get(_ context.Context, recordID string) (*record.Record, bool, error) {
	for _, item := range f.items {
		if item.ID == recordID {
			return item, true, nil
		}
	}
	return nil, false, nil
}

func (f *fakeRecords) PatchData(_ context.Context, rec *record.Record, data json.RawMessage) error {
	rec.Data = data
	return nil
}

func (f *fakeRecords) ListForOrg(_ context.Context, params record.ListForOrgParams) ([]*record.Record, int, error) {
	out := make([]*record.Record, 0)
	for _, item := range f.items {
		if item.SchemaID != params.SchemaID {
			continue
		}
		if item.NetworkID != params.NetworkID || item.OrganizationID != params.OrganizationID {
			continue
		}
		out = append(out, item)
	}
	return out, len(out), nil
}

type staticSystemUsers struct {
	id string
}

func (s staticSystemUsers) SystemUserID(context.Context, string, string) (string, error) {
	return s.id, nil
}

func TestRecordList(t *testing.T) {
	h := NewRecord(&fakeRecords{items: []*record.Record{{
		ID:             "inv-1",
		SchemaID:       "schema-inv",
		NetworkID:      "net-1",
		OrganizationID: "org-1",
		Data:           json.RawMessage(`{"fundId":"fund-1"}`),
	}}}, nil)
	result, err := h.Execute(context.Background(), executor.ExecutionContext{
		NetworkID:      "net-1",
		OrganizationID: "org-1",
		Input:          map[string]any{"schemaId": "schema-inv", "fundId": "fund-1"},
	}, pipeline.SnapshotNode{
		Type: node.TypeRecord,
		Definition: node.RecordContext{
			Operation: node.RecordOpList,
			SchemaID:  "{{ .Input.schemaId }}",
			Filters:   []node.RecordFilter{{Field: "fundId", Op: "eq", Value: "{{ .Input.fundId }}"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Total   int `json:"total"`
		Records []struct {
			ID string `json:"id"`
		} `json:"records"`
	}
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || len(out.Records) != 1 || out.Records[0].ID != "inv-1" {
		t.Fatalf("got %#v", out)
	}
}

func TestRecordCreate(t *testing.T) {
	fake := &fakeRecords{id: "rec-1"}
	h := NewRecord(fake, staticSystemUsers{id: "sys-1"})
	result, err := h.Execute(context.Background(), executor.ExecutionContext{
		NetworkID:          "net-1",
		OrganizationID:     "org-1",
		OrganizationUserID: "ou-1",
		IdempotencyKey:     "pipe:1:0",
		Input: map[string]any{
			"schemaId":   "schema-dist",
			"propertyId": "prop-1",
			"2":          map[string]any{"fileId": "file-1"},
		},
	}, pipeline.SnapshotNode{
		Type: node.TypeRecord,
		Definition: node.RecordContext{
			Operation: node.RecordOpCreate,
			SchemaID:  "{{ .Input.schemaId }}",
			Data: map[string]any{
				"propertyId": "{{ .Input.propertyId }}",
				"fileId":     "{{ .Input.2.fileId }}",
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
	if out["id"] != "rec-1" {
		t.Fatalf("got %#v", out)
	}
	var data map[string]any
	if err := json.Unmarshal(fake.created.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data["propertyId"] != "prop-1" || data["fileId"] != "file-1" {
		t.Fatalf("created data %#v", data)
	}
	if fake.created.OrganizationUserID != "sys-1" {
		t.Fatalf("organization user id = %s, want sys-1", fake.created.OrganizationUserID)
	}
}
