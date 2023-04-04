package cmd

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/mitchellh/cli"
	"github.com/mitchellh/colorstring"
	"github.com/ryanuber/columnize"
	"github.com/umbracle/vesta/internal/server/proto"
	"google.golang.org/grpc"
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
		"server": func() (cli.Command, error) {
			return &ServerCommand{
				UI: ui,
			}, nil
		},
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
		"deployment status": func() (cli.Command, error) {
			return &DeploymentStatusCommand{
				Meta: meta,
			}, nil
		},
		"destroy": func() (cli.Command, error) {
			return &DestroyCommand{
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
func (m *Meta) Conn() (proto.VestaServiceClient, error) {
	conn, err := grpc.Dial(m.addr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %v", err)
	}
	clt := proto.NewVestaServiceClient(conn)
	return clt, nil
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
