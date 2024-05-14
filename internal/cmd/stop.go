package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/vesta/internal/server/proto"
)

// DestroyCommand is the command to destroy an allocation
type StopCommand struct {
	*Meta

	watch bool
}

// Help implements the cli.Command interface
func (c *StopCommand) Help() string {
	return `Usage: vesta stop
	
  Stop a deployment`
}

// Synopsis implements the cli.Command interface
func (c *StopCommand) Synopsis() string {
	return "Stop a deployment"
}

// Run implements the cli.Command interface
func (c *StopCommand) Run(args []string) int {
	flags := c.FlagSet("stop")

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

	req := &proto.StopRequest{
		Id: args[0],
	}
	resp, err := clt.Stop(context.Background(), req)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	fmt.Println(resp)

	return 0
}
