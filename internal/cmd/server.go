package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
	"github.com/ferranbt/composer"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	flag "github.com/spf13/pflag"
	"github.com/umbracle/vesta/internal/server"
)

// ServerCommand is the command to show the version of the agent
type ServerCommand struct {
	UI     cli.Ui
	server *server.Server
	client *composer.Server

	logLevel string
	volume   string
	catalog  []string
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
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.StringVar(&c.logLevel, "log-level", "info", "")
	flags.StringVar(&c.volume, "volume", "", "")
	flags.StringSliceVar(&c.catalog, "catalog", []string{}, "")

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "vesta",
		Level: hclog.LevelFromString(c.logLevel),
	})

	db, err := bolt.Open("vesta.db", 0600, nil)
	if err != nil {
		c.UI.Output(fmt.Sprintf("failed to open persistence layer: %v", err))
		return 1
	}

	sCfg := server.DefaultConfig()
	sCfg.Catalog = c.catalog
	sCfg.PersistentDB = db

	srv, err := server.NewServer(logger, sCfg)
	if err != nil {
		c.UI.Output(fmt.Sprintf("failed to start validator: %v", err))
		return 1
	}
	c.server = srv

	clt, err := composer.NewServer()
	if err != nil {
		c.UI.Output(fmt.Sprintf("failed to start local scheduler: %v", err))
		return 1
	}
	c.client = clt

	/*
		cfg := &client.Config{
			ControlPlane: srv,
			NodeID:       "local",
			PersistentDB: db,
		}
		if c.volume == "" {
			logger.Warn("no volume is set")
		} else {
			cfg.Volume = &client.HostVolume{
				Path: c.volume,
			}
		}

		client, err := client.NewClient(logger, cfg)
		if err != nil {
			c.UI.Output(fmt.Sprintf("failed to start agent: %v", err))
			return 1
		}
		c.client = client
	*/

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
