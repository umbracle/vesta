package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/vesta/internal/server/proto"
)

// DeploymentListCommand is the command to show the version of the agent
type DeploymentListCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *DeploymentListCommand) Help() string {
	return `Usage: vesta deployment list
	
  List the active deployments`
}

// Synopsis implements the cli.Command interface
func (c *DeploymentListCommand) Synopsis() string {
	return "List the active deployments"
}

// Run implements the cli.Command interface
func (c *DeploymentListCommand) Run(args []string) int {
	flags := c.FlagSet("deployment list")
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	client, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	resp, err := client.DeploymentList(context.Background(), &proto.ListDeploymentRequest{})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	c.UI.Output(formatNodes(resp.Allocations))
	return 0
}

func formatNodes(allocs []*proto.Allocation) string {
	if len(allocs) == 0 {
		return "No allocations found"
	}

	rows := make([]string, len(allocs)+1)
	rows[0] = "ID|Status|Alias"
	for i, d := range allocs {
		rows[i+1] = fmt.Sprintf("%s|%s|%s",
			d.Id,
			d.Status.String(),
			d.Alias,
		)
	}
	return formatList(rows)
}
