package cmd

import (
	"os"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-hclog"
	flag "github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/mitchellh/cli"
	"github.com/mitchellh/colorstring"
	"github.com/ryanuber/columnize"
	"github.com/umbracle/vesta/internal/server"
)

// Commands returns the cli commands
func Commands() map[string]cli.CommandFactory {
	ui := &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	meta := &Meta{
		UI: ui,
	}

	return map[string]cli.CommandFactory{
		"catalog": func() (cli.Command, error) {
			return &CatalogCommand{
				Meta: meta,
			}, nil
		},
		"catalog list": func() (cli.Command, error) {
			return &CatalogListCommand{
				Meta: meta,
			}, nil
		},
		"catalog inspect": func() (cli.Command, error) {
			return &CatalogInspectCommand{
				Meta: meta,
			}, nil
		},
		"deployment ": func() (cli.Command, error) {
			return &DeploymentCommand{
				Meta: meta,
			}, nil
		},
		"deployment list": func() (cli.Command, error) {
			return &DeploymentListCommand{
				Meta: meta,
			}, nil
		},
		"destroy": func() (cli.Command, error) {
			return &DestroyCommand{
				Meta: meta,
			}, nil
		},
		"stop": func() (cli.Command, error) {
			return &StopCommand{
				Meta: meta,
			}, nil
		},
		"deploy": func() (cli.Command, error) {
			return &DeployCommand{
				Meta: meta,
			}, nil
		},
		"version": func() (cli.Command, error) {
			return &VersionCommand{
				UI: ui,
			}, nil
		},
		"volume": func() (cli.Command, error) {
			return &VolumeCommand{
				Meta: meta,
			}, nil
		},
		"volume list": func() (cli.Command, error) {
			return &VolumeListCommand{
				Meta: meta,
			}, nil
		},
	}
}

type Meta struct {
	UI   cli.Ui
	addr string
}

func (m *Meta) FlagSet(n string) *flag.FlagSet {
	f := flag.NewFlagSet(n, flag.ContinueOnError)
	f.StringVar(&m.addr, "address", "localhost:4003", "Address of the http api")
	return f
}

// Conn returns a grpc connection
func (m *Meta) Conn() (*server.Server, error) {
	srv, err := newTempServer()
	if err != nil {
		return nil, err
	}
	return srv, nil
}

func newTempServer() (*server.Server, error) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "vesta",
		Level: hclog.LevelFromString("info"),
	})

	db, err := bolt.Open("vesta.db", 0600, nil)
	if err != nil {
		return nil, err
	}

	sCfg := server.DefaultConfig()
	sCfg.PersistentDB = db

	srv, err := server.NewServer(logger, sCfg)
	if err != nil {
		return nil, err
	}
	return srv, nil
}

func formatList(in []string) string {
	columnConf := columnize.DefaultConfig()
	columnConf.Empty = "<none>"
	return columnize.Format(in, columnConf)
}

func formatKV(in []string) string {
	columnConf := columnize.DefaultConfig()
	columnConf.Empty = "<none>"
	columnConf.Glue = " = "
	return columnize.Format(in, columnConf)
}

func (m *Meta) Colorize() *colorstring.Colorize {
	return &colorstring.Colorize{
		Colors:  colorstring.DefaultColors,
		Disable: !terminal.IsTerminal(int(os.Stdout.Fd())),
		Reset:   true,
	}
}
