package handlers

import (
	"context"
	"fmt"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/workflow"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

type Mapper struct{}

func NewMapper() *Mapper {
	return &Mapper{}
}

func (h *Mapper) Type() node.Type {
	return node.TypeMapper
}

func (h *Mapper) Execute(_ context.Context, execCtx executor.ExecutionContext, n workflow.SnapshotNode) (executor.Result, error) {
	def, err := parseDefinition[node.MapperContext](n, node.TypeMapper)
	if err != nil {
		return executor.Result{}, err
	}
	resolved, err := resolveInput(def.Mapping, execCtx)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving MAPPER mapping: %w", err)
	}
	return executor.MarshalOutput(resolved)
}
