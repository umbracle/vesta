package cmd

import (
	"github.com/mitchellh/cli"
)

// DeploymentCommand is the command to show the version of the agent
type VolumeCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *VolumeCommand) Help() string {
	return `Usage: vesta volume

  This command groups subcommands for interacting with volumes.`
}

// Synopsis implements the cli.Command interface
func (c *VolumeCommand) Synopsis() string {
	return "Interact with volumes"
}

// Run implements the cli.Command interface
func (c *VolumeCommand) Run(args []string) int {
	return cli.RunResultHelp
}
