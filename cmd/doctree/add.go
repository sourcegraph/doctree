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

  Register current directory for auto-indexing on changes whenever the server is running:

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
		// ctx := context.Background()
		projectDir, err := filepath.Abs(dir)
		if err != nil {
			return errors.Wrap(err, "AbsoluteProjectDir")
		}

		monitoredJsonPath := filepath.Join(*dataDirFlag, "monitored")

		// Read JSON from ~/doctree/monitored
		var monitored []schema.MonitoredDirectory
		data, err := os.ReadFile(monitoredJsonPath)
		if err != nil {
			return errors.Wrap(err, "ReadMonitoredDirectory")
		}
		json.Unmarshal(data, &monitored)

		// Update the monitored list
		monitored = append(monitored, schema.MonitoredDirectory{
			ProjectName: *projectFlag,
			Path:        projectDir,
		})

		// Store JSON in ~/.doctree/monitored
		// [{"projectName": "..", "path": "..."}, {...}, ...]
		f, err := os.Create(monitoredJsonPath)
		if err != nil {
			return errors.Wrap(err, "Create")
		}
		defer f.Close()

		if err := json.NewEncoder(f).Encode(monitored); err != nil {
			return errors.Wrap(err, "Encode")
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
