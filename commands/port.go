package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/container/env"
	"github.com/arch-reactor/container/utils"
	"os"
)

var flTCP *string
var flUDP *string

func init() {
	cmd := addCommand("port", "[OPTIONS] <id>", "Disable ports for a container", 1, port)

	flTCP = cmd.String("t", "", "Print the tcp host port for a containers port")
	flUDP = cmd.String("u", "", "Print the udp host port for a containers port")
}

func port(cmd *flag.FlagSet) {
	id := utils.ExpandID(env.DIR, cmd.Arg(0))

	cont, err := env.LoadContainer(env.DIR, id)

	if err != nil {
		fmt.Printf("Error loading conatiner %s: %s\n", id, err)
		return
	}

	if *flTCP != "" {
		h, ok := cont.NetworkSettings.PortMapping["Tcp"][*flTCP]

		if ok {
			fmt.Printf("%s\n", h)
		} else {
			fmt.Printf("Unknown tcp port %s\n", *flTCP)
			os.Exit(1)
		}

		return
	}

	if *flUDP != "" {
		h, ok := cont.NetworkSettings.PortMapping["Udp"][*flUDP]

		if ok {
			fmt.Printf("%s\n", h)
		} else {
			fmt.Printf("Unknown udp port %s\n", *flUDP)
			os.Exit(1)
		}

		return
	}

	for c, h := range cont.NetworkSettings.PortMapping["Tcp"] {
		fmt.Printf("tcp %s -> tcp %s\n", c, h)
	}

	for c, h := range cont.NetworkSettings.PortMapping["Udp"] {
		fmt.Printf("udp %s -> udp %s\n", c, h)
	}
}
