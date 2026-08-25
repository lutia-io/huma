package cmd

import (
	"flag"

	"github.com/lutia-io/huma/internal/pipeline"
)

func NewPipelineExecutor(args []string) {
	flags := flag.NewFlagSet("pipeline-executor", flag.ExitOnError)
	flags.Parse(args)
	pipeline.NewExecutor()
}
