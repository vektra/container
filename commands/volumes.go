package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
)

type volumeOptions struct {
	Remove string `short:"r" description:"Remove a named volume"`
	Dir    string `short:"d" description:"Print the directory a volume is at"`
}

var flRemove *string
var flDir *string

func init() {
	app.AddCommand("volumes", "Disable and manipulate named volumes", "", &volumeOptions{})
}

func (vo *volumeOptions) Execute(args []string) error {
	if err := app.CheckArity(0, 0, args); err != nil {
		return err
	}

	if vo.Remove != "" {
		pth := path.Join(env.DIR, "volumes", vo.Remove)

		_, err := os.Stat(pth)

		if err != nil {
			return fmt.Errorf("No volume to remove: %s\n", vo.Remove)
		}

		os.RemoveAll(pth)
		return nil
	}

	if vo.Dir != "" {
		pth := path.Join(env.DIR, "volumes", vo.Dir)

		_, err := os.Stat(pth)

		if err != nil {
			return fmt.Errorf("No volume: %s\n", vo.Dir)
		}

		fmt.Printf("%s\n", pth)
		return nil
	}

	dirs, err := ioutil.ReadDir(path.Join(env.DIR, "volumes"))

	if err != nil {
		return fmt.Errorf("Error reading volumes: %s\n", err)
	}

	for _, d := range dirs {
		fmt.Printf("%s\n", d.Name())
	}

	return nil
}
