package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/vesta/internal/server/proto"
)

// CatalogInspectCommand is the command to show the version of the agent
type CatalogInspectCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *CatalogInspectCommand) Help() string {
	return `Usage: vesta catalog inspect <name>
	
  Output the status and information of a plugin`
}

// Synopsis implements the cli.Command interface
func (c *CatalogInspectCommand) Synopsis() string {
	return "Output the status and information of a plugin"
}

// Run implements the cli.Command interface
func (c *CatalogInspectCommand) Run(args []string) int {
	flags := c.FlagSet("catalog inspect")
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	args = flags.Args()
	if len(args) != 1 {
		c.UI.Error("incorrect input, provide one argument")
		return 1
	}

	name := args[0]

	client, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	resp, err := client.CatalogInspect(context.Background(), &proto.CatalogInspectRequest{Name: name})
	if err != nil {
		c.UI.Error(fmt.Sprintf("failed to get status: %v", err.Error()))
		return 1
	}

	c.UI.Output(c.Colorize().Color(formatItem(resp.Item)))
	return 0
}

func formatItem(item *proto.Item) string {
	base := formatKV([]string{
		fmt.Sprintf("Name|%s", item.Name),
	})

	taskRows := make([]string, len(item.Input)+1)
	taskRows[0] = "Name"

	i := 1
	for _, name := range item.Input {
		taskRows[i] = fmt.Sprintf("%s",
			name,
		)
		i += 1
	}

	base += "\n\n[bold]Input fields[reset]\n"
	base += formatList(taskRows)

	chainRows := make([]string, len(item.Chains)+1)
	chainRows[0] = "Name"

	i = 1
	for _, name := range item.Chains {
		chainRows[i] = fmt.Sprintf("%s",
			name,
		)
		i += 1
	}

	base += "\n\n[bold]Available chains[reset]\n"
	base += formatList(chainRows)

	return base
}
