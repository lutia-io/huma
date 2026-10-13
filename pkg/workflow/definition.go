package workflow

import (
	"encoding/json"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/criteria"
	"github.com/lutia-io/huma/pkg/node"
)

// Node is one step in a workflow definition level.
type Node struct {
	Type       node.Type `json:"type"`
	Definition any       `json:"definition"`
}

// UnmarshalJSON decodes the type-specific definition so callers can type-assert
// Node.Definition instead of re-parsing raw JSON.
func (n *Node) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type       node.Type       `json:"type"`
		Definition json.RawMessage `json:"definition"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	def, err := node.ParseDefinition(raw.Type, raw.Definition)
	if err != nil {
		return err
	}

	n.Type = raw.Type
	n.Definition = def
	return nil
}

func validateNodes(nodes [][]Node) error {
	if len(nodes) == 0 {
		return apperror.NewBadRequestError("Workflow definition requires at least one level", nil)
	}
	for _, level := range nodes {
		if len(level) == 0 {
			return apperror.NewBadRequestError("Workflow definition levels cannot be empty", nil)
		}
		for _, n := range level {
			if n.Type == "" {
				return apperror.NewBadRequestError("Each node needs a type", nil)
			}
			if n.Definition == nil {
				return apperror.NewBadRequestError("Each node needs a definition", nil)
			}
		}
	}
	return nil
}

// SnapshotNode is a node frozen onto a workflow run.
type SnapshotNode struct {
	Type       node.Type `json:"type"`
	Definition any       `json:"definition"`
}

// RunDefinition is the definition snapshotted on a workflow run.
type RunDefinition struct {
	Trigger  Trigger           `json:"trigger"`
	Criteria criteria.Criteria `json:"criteria"`
	Nodes    [][]SnapshotNode  `json:"nodes"`
}

// SnapshotRunDefinition freezes a definition for persistence on a run.
func SnapshotRunDefinition(def Definition) RunDefinition {
	out := RunDefinition{
		Trigger:  def.Trigger,
		Criteria: def.Criteria,
		Nodes:    make([][]SnapshotNode, len(def.Nodes)),
	}
	for i, level := range def.Nodes {
		out.Nodes[i] = make([]SnapshotNode, len(level))
		for j, n := range level {
			out.Nodes[i][j] = SnapshotNode{
				Type:       n.Type,
				Definition: n.Definition,
			}
		}
	}
	return out
}
