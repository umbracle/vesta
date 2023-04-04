package cmd

import (
	"github.com/mitchellh/cli"
)

// CatalogCommand is the command to show the version of the agent
type CatalogCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *CatalogCommand) Help() string {
	return `Usage: vesta catalog

  This command groups subcommands for interacting with the catalog.`
}

// Synopsis implements the cli.Command interface
func (c *CatalogCommand) Synopsis() string {
	return "Interact with the catalog plugins"
}

// Run implements the cli.Command interface
func (c *CatalogCommand) Run(args []string) int {
	return cli.RunResultHelp
}
