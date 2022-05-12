package main

import (
	"flag"
	"os"

	"github.com/hexops/cmder"
)

// commands contains all registered subcommands.
var commands cmder.Commander

var usageText = `doctree is a tool for library documentation.

Usage:
	doctree <command> [arguments]

The commands are:
	serve    runs a doctree server
	index    index a directory
	add		 auto-index a directory on changes whenever the server is running

Use "doctree <command> -h" for more information about a command.
`

func main() {
	commands.Run(flag.CommandLine, "doctree", usageText, os.Args[1:])
}
