package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/umbracle/vesta/internal/server/proto"
)

// DeployCommand is the command to show the version of the agent
type DeployCommand struct {
	*Meta

	chain string

	typ string

	allocId string
}

// Help implements the cli.Command interface
func (c *DeployCommand) Help() string {
	return `Usage: vesta deploy
	
  Create a deployment`
}

// Synopsis implements the cli.Command interface
func (c *DeployCommand) Synopsis() string {
	return "Create a deployment"
}

// Run implements the cli.Command interface
func (c *DeployCommand) Run(args []string) int {
	flags := c.FlagSet("deploy")
	flags.StringVar(&c.chain, "chain", "", "")
	flags.StringVar(&c.typ, "type", "", "")
	flags.StringVar(&c.allocId, "alloc", "", "")

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	args = flags.Args()

	spec := map[string]interface{}{}
	if c.chain != "" {
		spec["chain"] = c.chain
	}

	fmt.Println(args)

	for _, raw := range args {
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			c.UI.Error("format must be key=value")
			return 1
		}
		spec[parts[0]] = parts[1]
	}

	clt, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	raw, err := json.Marshal(spec)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	req := &proto.ApplyRequest{
		Action:       c.typ,
		Input:        raw,
		AllocationId: c.allocId,
	}
	resp, err := clt.Apply(context.Background(), req)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Output(resp.Id)
	return 0
}
