package commands

import (
	"fmt"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
)

type rmiOptions struct{}

func init() {
	app.AddCommand("rmi", "Remove an image", "", &rmiOptions{})
}

func (ro *rmiOptions) Usage() string {
	return "<repo:tag>"
}

func (ro *rmiOptions) Execute(args []string) error {
	if err := app.CheckArity(1, 1, args); err != nil {
		return err
	}

	ts, err := env.DefaultTagStore()
	if err != nil {
		return err
	}

	if !ts.RemoveByPrefix(args[0]) {
		fmt.Println("Unable to remove image.")
	}

	return ts.Flush()
}
