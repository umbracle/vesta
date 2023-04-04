package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/umbracle/vesta/internal/server/proto"
)

// CatalogListCommand is the command to show the version of the agent
type CatalogListCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *CatalogListCommand) Help() string {
	return `Usage: vesta catalog list
	
  List the plugins available in the catalog`
}

// Synopsis implements the cli.Command interface
func (c *CatalogListCommand) Synopsis() string {
	return "List the plugins available in the catalog"
}

// Run implements the cli.Command interface
func (c *CatalogListCommand) Run(args []string) int {
	flags := c.FlagSet("catalog list")
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	client, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	resp, err := client.CatalogList(context.Background(), &proto.CatalogListRequest{})
	if err != nil {
		c.UI.Error(fmt.Sprintf("failed to get catalog list: %v", err.Error()))
		return 1
	}

	items := resp.Plugins

	rows := make([]string, len(items)+1)
	rows[0] = "Name"
	for i, d := range items {
		rows[i+1] = fmt.Sprintf("%s",
			strings.Title(d),
		)
	}

	c.UI.Output(c.Colorize().Color(formatList(rows)))
	return 0
}
