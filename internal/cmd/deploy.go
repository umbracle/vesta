package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/umbracle/vesta/internal/server/proto"
)

// DeployCommand is the command to show the version of the agent
type DeployCommand struct {
	*Meta
	chain    string
	typ      string
	allocId  string
	params   string
	metrics  bool
	alias    string
	logLevel string
	watch    bool
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
	flags.StringVar(&c.params, "params", "", "")
	flags.BoolVar(&c.metrics, "metrics", true, "")
	flags.StringVar(&c.alias, "alias", "", "")
	flags.StringVar(&c.logLevel, "log-level", "info", "")
	flags.BoolVar(&c.watch, "watch", false, "")
	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	args = flags.Args()

	spec := map[string]interface{}{}
	if c.params != "" {
		data, err := ioutil.ReadFile(c.params)
		if err != nil {
			c.UI.Error(fmt.Sprintf("failed to read params file: %v", err))
			return 1
		}
		if err := json.Unmarshal(data, &spec); err != nil {
			c.UI.Error(fmt.Sprintf("failed to decode params '%s': %v", c.params, err))
			return 1
		}
	}

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

	req := &proto.ApplyRequest{
		Action:       c.typ,
		Input:        spec,
		AllocationId: c.allocId,
		Chain:        c.chain,
		Metrics:      c.metrics,
		Alias:        c.alias,
		LogLevel:     c.logLevel,
	}
	resp, err := clt.Apply(context.Background(), req)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	/*
		if c.watch {
			stream, err := clt.SubscribeEvents(context.TODO(), &proto.SubscribeEventsRequest{Service: resp.Id})
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
	*/

	c.UI.Output(resp)
	return 0
}
