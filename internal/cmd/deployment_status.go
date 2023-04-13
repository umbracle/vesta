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

	args = flags.Args()
	if len(args) != 1 {
		c.UI.Error("incorrect input, provide one argument")
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
		fmt.Sprintf("Status|%s", node.Status),
		fmt.Sprintf("Sequence|%d", node.Sequence),
	})

	taskRows := make([]string, len(node.Tasks)+1)
	taskRows[0] = "ID|Name|Image|State"

	i := 1
	for name, d := range node.Tasks {
		var state, id string
		if taskState, ok := node.TaskStates[name]; ok {
			state = taskState.State.String()
			id = taskState.Id
		}

		taskRows[i] = fmt.Sprintf("%s|%s|%s|%s",
			id,
			name,
			d.Image,
			state,
		)
		i += 1
	}

	base += "\n\n[bold]Tasks[reset]\n"
	base += formatList(taskRows)

	if len(node.SyncStatus) == 1 {
		for _, syncStatus := range node.SyncStatus {
			base += "\n\n[bold]Sync status[reset]\n"
			base += formatKV([]string{
				fmt.Sprintf("Peers|%d", syncStatus.NumPeers),
				fmt.Sprintf("Highest block|%d", syncStatus.HighestBlock),
				fmt.Sprintf("Current block|%d", syncStatus.CurrentBlock),
				fmt.Sprintf("Synced|%v", syncStatus.IsSynced),
			})
		}
	}

	return base
}
