package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/components/container/env"
)

func init() {
	addCommand("rmi", "<id>", "Remove image with <id>", 1, rmi)
}

func rmi(cmd *flag.FlagSet) {
	if ts, err := env.DefaultTagStore(); err != nil {
		panic(err)
	} else {
		if !ts.RemoveByPrefix(cmd.Args()[0]) {
			fmt.Println("Unable to remove image.")
		}
		if err = ts.Flush(); err != nil {
			panic(err)
		}
	}
}
