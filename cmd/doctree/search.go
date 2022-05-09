package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/hexops/cmder"
	"github.com/pkg/errors"

	// Register language indexers.
	"github.com/sourcegraph/doctree/doctree/indexer"
	_ "github.com/sourcegraph/doctree/doctree/indexer/golang"
	_ "github.com/sourcegraph/doctree/doctree/indexer/markdown"
	_ "github.com/sourcegraph/doctree/doctree/indexer/python"
)

func init() {
	const usage = `
Examples:

  Search :

    $ doctree search 'myquery'

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("search", flag.ExitOnError)
	dataDirFlag := flagSet.String("data-dir", defaultDataDir(), "where doctree stores its data")

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		if flagSet.NArg() != 1 {
			return &cmder.UsageError{}
		}
		query := flagSet.Arg(0)

		ctx := context.Background()
		_ = dataDirFlag
		_ = ctx
		_, err := indexer.Search(query)
		if err != nil {
			return errors.Wrap(err, "Search")
		}
		return nil
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
