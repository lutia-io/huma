package cmd

import (
	"flag"

	"github.com/lutia-io/huma/internal/api"
)

func NewApi(args []string) {
	flags := flag.NewFlagSet("api", flag.ExitOnError)
	flags.Parse(args)
	api.New()
}
