package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/vesta/internal/server/proto"
)

// DestroyCommand is the command to destroy an allocation
type DestroyCommand struct {
	*Meta

	watch bool
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

	flags.BoolVar(&c.watch, "watch", false, "")
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	args = flags.Args()
	if len(args) != 1 {
		c.UI.Error(fmt.Sprintf("one argument expected"))
		return 1
	}

	clt, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	req := &proto.DestroyRequest{
		Id: args[0],
	}
	resp, err := clt.Destroy(context.Background(), req)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	fmt.Println(resp)

	if c.watch {
		stream, err := clt.SubscribeEvents(context.TODO(), &proto.SubscribeEventsRequest{Service: args[0]})
		if err != nil {
			c.UI.Error(err.Error())
			return 1
		}

		for {
			ev, err := stream.Recv()
			if err != nil {
				c.UI.Error(err.Error())
				return 1
			}
			c.UI.Output(fmt.Sprintf("New event (%s)", ev))
		}
	}

	return 0
}
