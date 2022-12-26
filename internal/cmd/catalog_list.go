package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/vesta/internal/server/proto"
)

// CatalogList is the command to show the list of the catalog
type CatalogList struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *CatalogList) Help() string {
	return `Usage: vesta catalog list
	
  List the available deployments in the catalog`
}

// Synopsis implements the cli.Command interface
func (c *CatalogList) Synopsis() string {
	return "List the available deployments in the catalog"
}

// Run implements the cli.Command interface
func (c *CatalogList) Run(args []string) int {
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
		c.UI.Error(err.Error())
		return 1
	}

	fmt.Println(resp)
	return 0
}
