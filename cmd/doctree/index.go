package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/hexops/cmder"

	"github.com/sourcegraph/doctree/doctree/indexer"
)

func init() {
	const usage = `
Examples:

  Index all code in the current directory:

    $ doctree index .

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("index", flag.ExitOnError)
	dataDirFlag := flagSet.String("data-dir", defaultDataDir(), "where doctree stores its data")
	projectFlag := flagSet.String("project", defaultProjectName("."), "name of the project")

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		if flagSet.NArg() != 1 {
			return &cmder.UsageError{}
		}
		dir := flagSet.Arg(0)
		if dir != "." {
			*projectFlag = defaultProjectName(dir)
		}

		ctx := context.Background()
		return indexer.RunIndexers(ctx, dir, *dataDirFlag, *projectFlag)
	}

	// Register the command.
	commands = append(commands, &cmder.Command{
		FlagSet: flagSet,
		Aliases: []string{},
		Handler: handler,
		UsageFunc: func() {
			fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'doctree %s':\n", flagSet.Name())
			flagSet.PrintDefaults()
			fmt.Fprintf(flag.CommandLine.Output(), "%s", usage)
		},
	})
}
