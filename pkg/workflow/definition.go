package workflow

import (
	"github.com/lutia-io/huma/pkg/action"
	"github.com/lutia-io/huma/pkg/criteria"
)

type definition struct {
	Criteria criteria.Criteria `json:"criteria"`
	Actions  []action.Action   `json:"actions"`
}
