package pipeline

import (
	"encoding/json"
	"strings"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/slug"
)

// Node is one step in a pipeline definition. Config lives on the pipeline
// document, the same way workflow actions live on the workflow definition.
type Node struct {
	Name       string    `json:"name"`
	Type       node.Type `json:"type"`
	Definition any       `json:"definition"`
}

type definition struct {
	Nodes [][]Node `json:"nodes"`
}

// UnmarshalJSON decodes the type-specific definition so callers can type-assert
// Node.Definition instead of re-parsing raw JSON.
func (n *Node) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name       string          `json:"name"`
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

	n.Name = strings.TrimSpace(raw.Name)
	n.Type = raw.Type
	n.Definition = def
	return nil
}

func validateDefinition(def definition) error {
	if len(def.Nodes) == 0 {
		return apperror.NewBadRequestError("Pipeline definition requires at least one level", nil)
	}
	for _, level := range def.Nodes {
		if len(level) == 0 {
			return apperror.NewBadRequestError("Pipeline definition levels cannot be empty", nil)
		}
		for _, n := range level {
			if n.Name == "" {
				return apperror.NewBadRequestError("Each node needs a name", nil)
			}
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

func nodeSlug(n Node) string {
	s := slug.Slugify(n.Name)
	if s == "" {
		s = slug.Slugify(string(n.Type))
	}
	if s == "" {
		s = "node"
	}
	return s
}

func snapshotDefinition(def definition) SnapshotDefinition {
	out := SnapshotDefinition{Nodes: make([][]SnapshotNode, len(def.Nodes))}
	for i, level := range def.Nodes {
		out.Nodes[i] = make([]SnapshotNode, len(level))
		for j, n := range level {
			out.Nodes[i][j] = SnapshotNode{
				Name:       n.Name,
				Slug:       nodeSlug(n),
				Type:       n.Type,
				Definition: n.Definition,
			}
		}
	}
	return out
}
