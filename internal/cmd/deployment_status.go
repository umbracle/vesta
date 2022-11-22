package cmd

import (
	"context"
	"fmt"
	"sort"

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
		fmt.Sprintf("Status|%s", node.Status),
		fmt.Sprintf("Sequence|%d", node.Sequence),
	})

	taskRows := make([]string, len(node.Deployment.Tasks)+1)
	taskRows[0] = "ID|Name|Image|Tag|State"

	fmt.Println(node.TaskStates)

	i := 1
	for _, d := range node.Deployment.Tasks {
		var state string
		if taskState, ok := node.TaskStates[d.Name]; ok {
			state = taskState.State.String()
		}

		taskRows[i] = fmt.Sprintf("%s|%s|%s|%s|%s",
			d.Id,
			d.Name,
			d.Image,
			d.Tag,
			state,
		)
		i += 1
	}

	base += "\n\n[bold]Tasks[reset]\n"
	base += formatList(taskRows)

	// sort the last 10 events by timestamp
	events := []*proto.TaskState_Event{}
	for _, taskState := range node.TaskStates {
		events = append(events, taskState.Events...)
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Time.AsTime().Before(events[j].Time.AsTime())
	})

	if len(events) > 10 {
		events = events[:10]
	}
	eventsRows := make([]string, len(events)+1)
	eventsRows[0] = "Time|Type"

	for indx, event := range events {
		eventsRows[indx+1] = fmt.Sprintf("%s|%s",
			event.Time.AsTime().String(),
			event.Type,
		)
	}

	base += "\n\n[bold]Events[reset]\n"
	base += formatList(eventsRows)
	return base
}
