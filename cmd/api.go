package cmd

import (
	"flag"

	"github.com/lutia-io/huma/internal/api"
)

var flags = flag.NewFlagSet("api", flag.ExitOnError)

func NewApi(args []string) {
	flags.Parse(args)
	api.NewApi()
}
