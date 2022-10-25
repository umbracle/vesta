package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"github.com/umbracle/vesta/internal/client"
	"github.com/umbracle/vesta/internal/server"
)

// ServerCommand is the command to show the version of the agent
type ServerCommand struct {
	UI     cli.Ui
	server *server.Server
	client *client.Client
}

// Help implements the cli.Command interface
func (c *ServerCommand) Help() string {
	return `Usage: vesta server
	
  Start the Vesta server`
}

// Synopsis implements the cli.Command interface
func (c *ServerCommand) Synopsis() string {
	return "Start the Vesta server"
}

// Run implements the cli.Command interface
func (c *ServerCommand) Run(args []string) int {

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "vesta",
		Level: hclog.LevelFromString("info"),
	})

	srv, err := server.NewServer(logger, server.DefaultConfig())
	if err != nil {
		c.UI.Output(fmt.Sprintf("failed to start validator: %v", err))
		return 1
	}
	c.server = srv

	cfg := &client.Config{
		ControlPlane: srv,
		NodeID:       "local",
	}
	client, err := client.NewClient(logger, cfg)
	if err != nil {
		c.UI.Output(fmt.Sprintf("failed to start agent: %v", err))
		return 1
	}
	c.client = client

	return c.handleSignals()
}

func (c *ServerCommand) handleSignals() int {
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	sig := <-signalCh

	c.UI.Output(fmt.Sprintf("Caught signal: %v", sig))
	c.UI.Output("Gracefully shutting down agent...")

	gracefulCh := make(chan struct{})
	go func() {
		c.server.Stop()
		c.client.Stop()
		close(gracefulCh)
	}()

	select {
	case <-signalCh:
		return 1
	case <-time.After(10 * time.Second):
		return 1
	case <-gracefulCh:
		return 0
	}
}
