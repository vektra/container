package main

import (
	"flag"
	"fmt"
	"github.com/vektra/container/commands"
	"github.com/vektra/container/env"
	"os"
)

const VERSION = "1.13"

func main() {
	if selfPath := env.SelfPath(); selfPath == "/sbin/init" || selfPath == "/.dockerinit" {
		// Running in init mode
		env.SysInit()
		return
	}

	if err := env.Init(); err != nil {
		panic(err)
	}

  if len(os.Args) == 2 && os.Args[1] == "-v" {
    fmt.Printf("vk-container %s\n", VERSION)
    return
  }

	if len(os.Args) < 2 {
		commands.Usage()
		return
	}

	cmd := os.Args[1]

	for _, c := range commands.AllCmds() {
		if cmd == c.Name {
			err := c.Flags.Parse(os.Args[2:])

			if err == flag.ErrHelp {
				os.Exit(1)
			}

			if err != nil || len(c.Flags.Args()) < c.Extra {
				c.Flags.Usage()
				os.Exit(1)
			}

			c.Call(c.Flags)
			return
		}
	}

	commands.Usage()
}
