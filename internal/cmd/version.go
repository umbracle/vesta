package cmd

import (
	"github.com/mitchellh/cli"
	"github.com/umbracle/vesta/internal/version"
)

// VersionCommand is the command to show the version of the agent
type VersionCommand struct {
	UI cli.Ui
}

// Help implements the cli.Command interface
func (c *VersionCommand) Help() string {
	return `Usage: vesta version
	
  Display the Vesta version`
}

// Synopsis implements the cli.Command interface
func (c *VersionCommand) Synopsis() string {
	return "Display the Vesta version"
}

// Run implements the cli.Command interface
func (c *VersionCommand) Run(args []string) int {
	c.UI.Output(version.GetVersion())
	return 0
}
