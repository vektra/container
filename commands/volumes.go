package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/container/env"
	"io/ioutil"
	"os"
	"path"
)

var flRemove *string
var flDir *string

func init() {
	cmd := addCommand("volumes", "[OPTIONS]", "Disable and manipulate named volumes", 0, volumes)

	flRemove = cmd.String("r", "", "Remove a named volume")
	flDir = cmd.String("d", "", "Print the directory a volume is at")
}

func volumes(cmd *flag.FlagSet) {
	if *flRemove != "" {
		pth := path.Join(env.DIR, "volumes", *flRemove)

		_, err := os.Stat(pth)

		if err != nil {
			fmt.Printf("No volume to remove: %s\n", *flRemove)
			return
		}

		os.RemoveAll(pth)
		return
	}

	if *flDir != "" {
		pth := path.Join(env.DIR, "volumes", *flRemove)

		_, err := os.Stat(pth)

		if err != nil {
			fmt.Printf("No volume: %s\n", *flDir)
			os.Exit(1)
			return
		}

		fmt.Printf("%s\n", pth)
		return
	}

	dirs, err := ioutil.ReadDir(path.Join(env.DIR, "volumes"))

	if err != nil {
		fmt.Printf("Error reading volumes: %s\n", err)
		return
	}

	for _, d := range dirs {
		fmt.Printf("%s\n", d.Name())
	}
}
