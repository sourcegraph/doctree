package main

import (
	"flag"
	"os"

	"github.com/hexops/cmder"

	// Register language indexers.
	_ "github.com/sourcegraph/doctree/doctree/indexer/golang"
	_ "github.com/sourcegraph/doctree/doctree/indexer/javascript"
	_ "github.com/sourcegraph/doctree/doctree/indexer/markdown"
	_ "github.com/sourcegraph/doctree/doctree/indexer/python"
	_ "github.com/sourcegraph/doctree/doctree/indexer/zig"
)

// commands contains all registered subcommands.
var commands cmder.Commander

var usageText = `doctree is a tool for library documentation.

Usage:
	doctree <command> [arguments]

The commands are:
	serve    runs a doctree server
	index    index a directory
	add      (EXPERIMENTAL) register a directory for auto-indexing

Use "doctree <command> -h" for more information about a command.
`

func main() {
	commands.Run(flag.CommandLine, "doctree", usageText, os.Args[1:])
}
