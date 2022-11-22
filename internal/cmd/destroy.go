package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/vesta/internal/server/proto"
)

// DestroyCommand is the command to destroy an allocation
type DestroyCommand struct {
	*Meta

	allocId string
}

// Help implements the cli.Command interface
func (c *DestroyCommand) Help() string {
	return `Usage: vesta destroy
	
  Destroy a deployment`
}

// Synopsis implements the cli.Command interface
func (c *DestroyCommand) Synopsis() string {
	return "Destroy a deployment"
}

// Run implements the cli.Command interface
func (c *DestroyCommand) Run(args []string) int {
	flags := c.FlagSet("destroy")
	flags.StringVar(&c.allocId, "alloc", "", "")

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	clt, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	req := &proto.DestroyRequest{
		Id: c.allocId,
	}
	resp, err := clt.Destroy(context.Background(), req)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	fmt.Println(resp)

	return 0
}
