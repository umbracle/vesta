package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/vesta/internal/server/proto"
)

// CatalogInfo is the command to show the list of the catalog
type CatalogInfo struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *CatalogInfo) Help() string {
	return `Usage: vesta catalog info
	
  Returns info of a given catalog entry`
}

// Synopsis implements the cli.Command interface
func (c *CatalogInfo) Synopsis() string {
	return "Returns info of a given catalog entry"
}

// Run implements the cli.Command interface
func (c *CatalogInfo) Run(args []string) int {
	flags := c.FlagSet("catalog info")
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	args = flags.Args()
	if len(args) != 1 {
		c.UI.Error("one arg expected")
		return 1
	}

	client, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	resp, err := client.CatalogEntry(context.Background(), &proto.CatalogEntryRequest{Action: args[0]})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	fmt.Println(resp)
	return 0
}
