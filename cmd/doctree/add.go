package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/hexops/cmder"
	"github.com/pkg/errors"
	"github.com/sourcegraph/doctree/doctree/indexer"
)

func init() {
	const usage = `
Examples:

  Register current directory for auto-indexing:

    $ doctree add .
`
	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("add", flag.ExitOnError)
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

		projectPath, err := filepath.Abs(dir)
		if err != nil {
			return errors.Wrap(err, "projectPath")
		}
		autoIndexPath := filepath.Join(*dataDirFlag, "autoindex")

		// Read JSON from ~/.doctree/autoindex
		autoIndexedProjects, err := indexer.ReadAutoIndex(autoIndexPath)
		if err != nil {
			return err
		}

		// Update the autoIndexProjects array
		autoIndexedProjects[projectPath] = indexer.AutoIndexedProject{
			Name: *projectFlag,
		}

		err = indexer.WriteAutoIndex(autoIndexPath, autoIndexedProjects)
		if err != nil {
			return err
		}

		// Run indexers on the newly registered dir
		ctx := context.Background()
		return indexer.RunIndexers(ctx, projectPath, *dataDirFlag, *projectFlag)
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
