package action

import (
	"encoding/json"
	"fmt"
)

type Type string

const (
	TypeCreateRecord    Type = "CREATE_RECORD"
	TypeUpdateRecord    Type = "UPDATE_RECORD"
	TypeUpsertRecord    Type = "UPSERT_RECORD"
	TypeTriggerPipeline Type = "TRIGGER_PIPELINE"
)

type CreateRecordContext struct {
	Schema string         `json:"schema"`
	Data   map[string]any `json:"data"`
}

type UpdateRecordContext struct {
	Schema string         `json:"schema"`
	Data   map[string]any `json:"data"`
}

type UpsertRecordContext struct {
	Schema string         `json:"schema"`
	Data   map[string]any `json:"data"`
}

type TriggerPipelineContext struct {
	Pipeline string         `json:"pipeline"`
	Input    map[string]any `json:"input"`
}

type Action struct {
	Type    Type `json:"type"`
	Context any  `json:"context"`
}

func (a *Action) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type    Type            `json:"type"`
		Context json.RawMessage `json:"context"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	a.Type = raw.Type

	switch raw.Type {
	case TypeCreateRecord:
		var ctx CreateRecordContext
		if err := json.Unmarshal(raw.Context, &ctx); err != nil {
			return fmt.Errorf("invalid CREATE_RECORD context: %w", err)
		}
		a.Context = ctx
	case TypeUpdateRecord:
		var ctx UpdateRecordContext
		if err := json.Unmarshal(raw.Context, &ctx); err != nil {
			return fmt.Errorf("invalid UPDATE_RECORD context: %w", err)
		}
		a.Context = ctx
	case TypeUpsertRecord:
		var ctx UpsertRecordContext
		if err := json.Unmarshal(raw.Context, &ctx); err != nil {
			return fmt.Errorf("invalid UPSERT_RECORD context: %w", err)
		}
		a.Context = ctx
	case TypeTriggerPipeline:
		var ctx TriggerPipelineContext
		if err := json.Unmarshal(raw.Context, &ctx); err != nil {
			return fmt.Errorf("invalid TRIGGER_PIPELINE context: %w", err)
		}
		a.Context = ctx
	default:
		return fmt.Errorf("unknown action type %q", raw.Type)
	}

	return nil
}
