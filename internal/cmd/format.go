package cmd

import (
	"fmt"
	"io/ioutil"

	"cuelang.org/go/cue/format"
	"github.com/mitchellh/cli"
)

// FormatCommand is the command to show the version of the agent
type FormatCommand struct {
	UI cli.Ui
}

// Help implements the cli.Command interface
func (c *FormatCommand) Help() string {
	return `Usage: vesta format
	
  Format a cue document`
}

// Synopsis implements the cli.Command interface
func (c *FormatCommand) Synopsis() string {
	return "Format a cue document"
}

// Run implements the cli.Command interface
func (c *FormatCommand) Run(args []string) int {
	if len(args) < 1 {
		c.UI.Error("one argument expected to format")
		return 1
	}

	path := args[0]

	data, err := ioutil.ReadFile(path)
	if err != nil {
		c.UI.Error(fmt.Sprintf("failed to read file %s: %v", path, err))
		return 1
	}

	res, err := format.Source(data)
	if err != nil {
		c.UI.Error(fmt.Sprintf("failed to format %s: %v", path, err))
		return 1
	}

	if err := ioutil.WriteFile(path, res, 0755); err != nil {
		c.UI.Error(fmt.Sprintf("failed to write file %s: %v", path, err))
		return 1
	}
	return 0
}
