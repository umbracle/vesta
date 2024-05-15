package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/vesta/internal/server/proto"
)

// VolumeListCommand is the command to show the version of the agent
type VolumeListCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *VolumeListCommand) Help() string {
	return `Usage: vesta volume list
	
  List the active deployments`
}

// Synopsis implements the cli.Command interface
func (c *VolumeListCommand) Synopsis() string {
	return "List the active deployments"
}

// Run implements the cli.Command interface
func (c *VolumeListCommand) Run(args []string) int {
	flags := c.FlagSet("volume list")
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	client, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	resp, err := client.VolumeList(context.Background())
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	c.UI.Output(formatVolumes(resp))
	return 0
}

func formatVolumes(volumes []*proto.Volume) string {
	if len(volumes) == 0 {
		return "No volumes found"
	}

	rows := make([]string, len(volumes)+1)
	rows[0] = "ID|Chain"
	for i, d := range volumes {
		labels := map[string]string{}
		for k, v := range d.Labels {
			labels[k] = v
		}

		rows[i+1] = fmt.Sprintf("%s|%s",
			d.Id,
			labels["chain"],
		)
	}
	return formatList(rows)
}
