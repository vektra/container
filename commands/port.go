package commands

import (
	"fmt"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
	"github.com/vektra/container/utils"
)

type portOptions struct {
	TCP string `short:"t" description:"Print the tcp port for a containers port"`
	UDP string `short:"u" description:"Print the udp port for a containers port"`
}

func init() {
	app.AddCommand("port", "Display ports for a container", "", &portOptions{})
}

func (po *portOptions) Usage() string {
	return "[OPTIONS] <id>"
}

func (po *portOptions) Execute(args []string) error {
	if err := app.CheckArity(1, 1, args); err != nil {
		return err
	}

	id := utils.ExpandID(env.DIR, args[0])

	cont, err := env.LoadContainer(env.DIR, id)

	if err != nil {
		return fmt.Errorf("Error loading conatiner %s: %s\n", id, err)
	}

	if po.TCP != "" {
		h, ok := cont.NetworkSettings.PortMapping["Tcp"][po.TCP]

		if ok {
			fmt.Printf("%s\n", h)
		} else {
			return fmt.Errorf("Unknown tcp port %s\n", po.TCP)
		}

		return nil
	}

	if po.UDP != "" {
		h, ok := cont.NetworkSettings.PortMapping["Udp"][po.UDP]

		if ok {
			fmt.Printf("%s\n", h)
		} else {
			return fmt.Errorf("Unknown udp port %s\n", po.UDP)
		}

		return nil
	}

	for c, h := range cont.NetworkSettings.PortMapping["Tcp"] {
		fmt.Printf("tcp %s -> tcp %s\n", c, h)
	}

	for c, h := range cont.NetworkSettings.PortMapping["Udp"] {
		fmt.Printf("udp %s -> udp %s\n", c, h)
	}

	return nil
}
