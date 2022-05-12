package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hexops/cmder"
	"github.com/pkg/errors"
	"github.com/sourcegraph/doctree/doctree/schema"
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
	projectFlag := flagSet.String("project", defaultProjectName(), "name of the project")

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		if flagSet.NArg() != 1 {
			return &cmder.UsageError{}
		}
		dir := flagSet.Arg(0)

		projectPath, err := filepath.Abs(dir)
		if err != nil {
			return errors.Wrap(err, "projectPath")
		}
		autoIndexPath := filepath.Join(*dataDirFlag, "autoindex")

		// Read JSON from ~/.doctree/autoindex
		autoIndexedProjects, err := ReadAutoIndex(autoIndexPath)
		if err != nil {
			return err
		}

		// Update the autoIndexProjects array
		autoIndexedProjects = append(autoIndexedProjects, schema.AutoIndexedProject{
			Name: *projectFlag,
			Path: projectPath,
			Hash: GetDirHash(projectPath),
		})

		return WriteAutoIndex(autoIndexPath, autoIndexedProjects)
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

// Write autoindexedProjects as JSON in the provided filepath.
func WriteAutoIndex(path string, autoindexedProjects []schema.AutoIndexedProject) error {
	f, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "Create")
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(autoindexedProjects); err != nil {
		return errors.Wrap(err, "Encode")
	}

	return nil
}

// Read autoindexedProjects array from the provided filepath.
func ReadAutoIndex(path string) ([]schema.AutoIndexedProject, error) {
	var autoIndexedProjects []schema.AutoIndexedProject
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "ReadAutoIndexFile")
	}
	err = json.Unmarshal(data, &autoIndexedProjects)
	if err != nil {
		return nil, errors.Wrap(err, "ParseAutoIndexFile")
	}

	return autoIndexedProjects, nil
}
