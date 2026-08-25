package cmd

import (
	"fmt"
	"os"
)

func Execute() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "api":
		NewApi(args)
	case "workflow-evaluator":
		NewWorkflowEvaluator(args)
	case "workflow-executor":
		NewWorkflowExecutor(args)
	case "pipeline-executor":
		NewPipelineExecutor(args)
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	prog := os.Args[0]
	fmt.Fprintf(os.Stdout, "Usage: %s <command> [options]\n\n", prog)
	fmt.Fprintln(os.Stdout, "Commands:")
	fmt.Fprintln(os.Stdout, "  api                	The api service serves the main public api.")
	fmt.Fprintln(os.Stdout, "  workflow-evaluator 	Consumes record events, evaluates workflow criteria, and inserts pending workflows.")
	fmt.Fprintln(os.Stdout, "  workflow-executor  	Claims pending workflows and executes their actions.")
	fmt.Fprintln(os.Stdout, "  pipeline-executor  	Claims pending pipelines and executes their nodes.")
}
