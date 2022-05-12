package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
		projectDir, err := filepath.Abs(dir)
		if err != nil {
			return errors.Wrap(err, "AbsProjectDir")
		}
		autoIndexPath := filepath.Join(*dataDirFlag, "autoindex")

		// Read JSON from ~/.doctree/autoindex
		autoIndexProjects, err := ReadAutoIndex(autoIndexPath)
		if err != nil {
			return err
		}

		// Update the autoIndexProjects array
		autoIndexProjects = append(autoIndexProjects, schema.AutoIndexedProject{
			Name: *projectFlag,
			Path: projectDir,
			Hash: GetDirHash(projectDir),
		})

		return WriteAutoIndex(autoIndexPath, autoIndexProjects)
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

func WriteAutoIndex(autoIndexPath string, autoindexProjects []schema.AutoIndexedProject) error {
	// Store JSON in ~/.doctree/monitored
	// [{"projectName": "..", "path": "..."}, {...}, ...]
	f, err := os.Create(autoIndexPath)
	if err != nil {
		return errors.Wrap(err, "Create")
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(autoindexProjects); err != nil {
		return errors.Wrap(err, "Encode")
	}

	return nil
}

func ReadAutoIndex(autoIndexPath string) ([]schema.AutoIndexedProject, error) {
	var autoIndexList []schema.AutoIndexedProject
	data, err := os.ReadFile(autoIndexPath)
	if err != nil {
		return nil, errors.Wrap(err, "ReadAutoIndexFile")
	}
	json.Unmarshal(data, &autoIndexList)

	return autoIndexList, nil
}

func GetDirHash(dir string) string {
	// Reference: https://unix.stackexchange.com/questions/35832/how-do-i-get-the-md5-sum-of-a-directorys-contents-as-one-sum
	tarCmd := exec.Command("tar", "-cf", "-", dir)
	md5sumCmd := exec.Command("md5sum")

	pipe, _ := tarCmd.StdoutPipe()
	defer pipe.Close()

	md5sumCmd.Stdin = pipe

	// Run the tarCmd
	err := tarCmd.Start()
	if err != nil {
		fmt.Printf("tar command failed with '%s'\n", err)
		return "0"
	}
	// Run and get the output of md5sum
	res, _ := md5sumCmd.Output()

	return string(res)
}
