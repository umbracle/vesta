package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/vesta/internal/server/proto"
)

// DeploymentStatusCommand is the command to show the version of the agent
type DeploymentStatusCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *DeploymentStatusCommand) Help() string {
	return `Usage: vesta deployment status
	
  Output the status of a deployment`
}

// Synopsis implements the cli.Command interface
func (c *DeploymentStatusCommand) Synopsis() string {
	return "Output the status of a deployment"
}

// Run implements the cli.Command interface
func (c *DeploymentStatusCommand) Run(args []string) int {
	flags := c.FlagSet("deployment status")
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	id := args[0]

	client, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	resp, err := client.DeploymentStatus(context.Background(), &proto.DeploymentStatusRequest{Id: id})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	c.UI.Output(c.Colorize().Color(formatNodeStatus(resp)))
	return 0
}

func formatNodeStatus(r *proto.DeploymentStatusResponse) string {
	node := r.Allocation

	base := formatKV([]string{
		fmt.Sprintf("ID|%s", node.Id),
	})

	rows := make([]string, len(node.Deployment.Tasks)+1)
	rows[0] = "ID|Name"

	i := 1
	for _, d := range node.Deployment.Tasks {
		rows[i] = fmt.Sprintf("%s|%s",
			d.Id,
			d.Name,
		)
		i += 1
	}

	return base + "\n\n" + formatList(rows)
}
