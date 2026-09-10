package handlers

import (
	"context"
	"fmt"

	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/pipeline/executor"
	"github.com/lutia-io/huma/pkg/resolver"
)

type Mapper struct{}

func NewMapper() *Mapper {
	return &Mapper{}
}

func (h *Mapper) Type() node.Type {
	return node.TypeMapper
}

func (h *Mapper) Execute(_ context.Context, execCtx executor.ExecutionContext, n pipeline.SnapshotNode) (executor.Result, error) {
	def, err := parseDefinition[node.MapperContext](n, node.TypeMapper)
	if err != nil {
		return executor.Result{}, err
	}
	resolved, err := resolver.ResolveInput(def.Mapping, execCtx.Input)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving MAPPER mapping: %w", err)
	}
	return executor.MarshalOutput(resolved)
}
