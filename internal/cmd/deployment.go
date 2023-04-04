package cmd

import (
	"github.com/mitchellh/cli"
)

// DeploymentCommand is the command to show the version of the agent
type DeploymentCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *DeploymentCommand) Help() string {
	return `Usage: vesta deployment

  This command groups subcommands for interacting with deployments.`
}

// Synopsis implements the cli.Command interface
func (c *DeploymentCommand) Synopsis() string {
	return "Interact with deployments"
}

// Run implements the cli.Command interface
func (c *DeploymentCommand) Run(args []string) int {
	return cli.RunResultHelp
}
