package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/container/utils"
)

type command struct {
	Name  string
	usage string
	desc  string
	Flags *flag.FlagSet
	Extra int
	Call  runner
}

func (cmd *command) Usage() string {
	return fmt.Sprintf("%s %s", cmd.Name, cmd.usage)
}

var commands []command = nil

func Usage() {
	fmt.Println(`
Usage: ar-container COMMAND

Where COMMAND is one of the following:
`)
	for _, cmd := range commands {
		fmt.Printf("\t%s\n", cmd.Usage())
	}
}

type runner func(*flag.FlagSet)

func AllCmds() []command {
	return commands
}

func addCommand(name, usage, desc string, x int, r runner) *flag.FlagSet {
	fs := utils.Subcmd(name, usage, desc)

	commands = append(commands, command{name, usage, desc, fs, x, r})

	return fs
}
