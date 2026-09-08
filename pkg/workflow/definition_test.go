package workflow

import (
	"testing"

	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/criteria"
	"github.com/lutia-io/huma/pkg/uuid"
)

func validActions() []action.Action {
	return []action.Action{{
		Type: action.TypeCreateRecord,
		Context: action.CreateRecordContext{
			SchemaID: uuid.MustNew(),
			Data:     map[string]any{"ok": true},
		},
	}}
}

func TestValidateDefinition_defaultCreated(t *testing.T) {
	err := validateDefinition(Definition{
		Criteria: criteria.Criteria{Field: "status", Operator: criteria.OpEq, Value: "pending"},
		Actions:  validActions(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateDefinition_emptyCriteria(t *testing.T) {
	err := validateDefinition(Definition{
		Trigger: Trigger{On: []string{TriggerOnUpdated}, Changed: []string{"status"}},
		Actions: validActions(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateDefinition_scheduleRequiresCronAndTimezone(t *testing.T) {
	if err := validateDefinition(Definition{
		Trigger: Trigger{On: []string{TriggerOnSchedule}},
		Actions: validActions(),
	}); err == nil {
		t.Fatal("expected error")
	}
	if err := validateDefinition(Definition{
		Trigger: Trigger{On: []string{TriggerOnSchedule}, Cron: "0 9 * * *"},
		Actions: validActions(),
	}); err == nil {
		t.Fatal("expected error")
	}
	err := validateDefinition(Definition{
		Trigger: Trigger{
			On:       []string{TriggerOnSchedule},
			Cron:     "0 9 * * *",
			Timezone: "America/Los_Angeles",
		},
		Actions: validActions(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateDefinition_scheduleCannotMix(t *testing.T) {
	err := validateDefinition(Definition{
		Trigger: Trigger{
			On:       []string{TriggerOnCreated, TriggerOnSchedule},
			Cron:     "0 9 * * *",
			Timezone: "UTC",
		},
		Actions: validActions(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateDefinition_changedOnlyWithUpdated(t *testing.T) {
	if err := validateDefinition(Definition{
		Trigger: Trigger{On: []string{TriggerOnCreated}, Changed: []string{"status"}},
		Actions: validActions(),
	}); err == nil {
		t.Fatal("expected error")
	}
	err := validateDefinition(Definition{
		Trigger: Trigger{On: []string{TriggerOnCreated, TriggerOnUpdated}, Changed: []string{"status"}},
		Actions: validActions(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateDefinition_unknownEvent(t *testing.T) {
	if err := validateDefinition(Definition{
		Trigger: Trigger{On: []string{"deleted"}},
		Actions: validActions(),
	}); err == nil {
		t.Fatal("expected error")
	}
}

func TestTriggerEventsDefaultCreated(t *testing.T) {
	var tgr Trigger
	if !tgr.Includes(TriggerOnCreated) || tgr.Includes(TriggerOnUpdated) {
		t.Fatalf("Events() = %v", tgr.Events())
	}
}
