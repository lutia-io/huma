package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/record"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

type fakeRecords struct {
	got     *record.Record
	found   bool
	patched json.RawMessage
}

func (f *fakeRecords) Create(context.Context, record.CreateParams) (string, error) {
	return "", nil
}

func (f *fakeRecords) Get(context.Context, string) (*record.Record, bool, error) {
	return f.got, f.found, nil
}

func (f *fakeRecords) PatchData(_ context.Context, _ *record.Record, data json.RawMessage) error {
	f.patched = data
	return nil
}

func TestUpdateRecord_resolvesContextFromTargetRow(t *testing.T) {
	existing := &record.Record{
		ID:   "fund-1",
		Data: json.RawMessage(`{"contributedAmountCents":100,"name":"General"}`),
	}
	records := &fakeRecords{got: existing, found: true}
	h := NewUpdateRecord(records)

	_, err := h.Execute(context.Background(), executor.ExecutionContext{
		TriggerRecordID: "contrib-1",
		TriggerData:     map[string]any{"amountCents": float64(50), "fundId": "fund-1"},
	}, action.Action{
		Type: action.TypeUpdateRecord,
		Context: action.UpdateRecordContext{
			RecordID: "{{ .Record.data.fundId }}",
			Data: map[string]any{
				"contributedAmountCents": "{{ add .Context.data.contributedAmountCents .Record.data.amountCents }}",
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(records.patched, &got); err != nil {
		t.Fatalf("unmarshal patched data: %v", err)
	}
	if got["name"] != "General" {
		t.Errorf("name = %#v, want General", got["name"])
	}
	// add of two whole numbers returns int64; JSON numbers unmarshal as float64.
	if got["contributedAmountCents"] != float64(150) {
		t.Errorf("contributedAmountCents = %#v, want 150", got["contributedAmountCents"])
	}
}
