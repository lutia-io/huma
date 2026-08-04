package cmd

import (
	"flag"

	"github.com/lutia-io/huma/internal/workflow"
)

func NewWorkflowEvaluator(args []string) {
	flags := flag.NewFlagSet("workflow-evaluator", flag.ExitOnError)
	flags.Parse(args)
	workflow.NewEvaluator()
}

func NewWorkflowExecutor(args []string) {
	flags := flag.NewFlagSet("workflow-executor", flag.ExitOnError)
	flags.Parse(args)
	workflow.NewExecutor()
}
