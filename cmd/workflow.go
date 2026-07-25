package cmd

import (
	"flag"

	"github.com/lutia-io/huma/internal/workflow"
)

func NewWorkflow(args []string) {
	flags := flag.NewFlagSet("workflow", flag.ExitOnError)
	flags.Parse(args)
	workflow.New()
}
