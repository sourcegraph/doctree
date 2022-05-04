package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hexops/cmder"

	// Register language indexers.
	"github.com/sourcegraph/doctree/doctree/indexer"
	_ "github.com/sourcegraph/doctree/doctree/indexer/golang"
	_ "github.com/sourcegraph/doctree/doctree/indexer/python"
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
	projectFlag := flagSet.String("project", defaultProjectName(), "name of the project")

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		if flagSet.NArg() != 1 {
			return &cmder.UsageError{}
		}
		dir := flagSet.Arg(0)

		ctx := context.Background()
		indexes, indexErr := indexer.IndexDir(ctx, dir)
		for _, index := range indexes {
			fmt.Printf("%v: indexed %v files (%v bytes) in %v\n", index.Language.ID, index.NumFiles, index.NumBytes, time.Duration(index.DurationSeconds*float64(time.Second)).Round(time.Millisecond))
		}

		indexDataDir := filepath.Join(*dataDirFlag, "index")
		writeErr := indexer.WriteIndexes(*projectFlag, indexDataDir, indexes)
		if indexErr != nil && writeErr != nil {
			return multierror.Append(indexErr, writeErr)
		}
		if indexErr != nil {
			return indexErr
		}
		return writeErr
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
