package commands

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
	"github.com/vektra/container/utils"
)

type inspectOptions struct{}

func init() {
	app.AddCommand("inspect", "Display details about a container", "", &inspectOptions{})
}

func (io *inspectOptions) Usage() string {
	return "<id>"
}

func (io *inspectOptions) Execute(args []string) error {
	if err := app.CheckArity(1, 1, args); err != nil {
		return err
	}

	id := utils.ExpandID(env.DIR, args[0])

	cont, err := env.LoadContainer(env.DIR, id)

	if err != nil {
		return err
	}

	data, err := json.Marshal(cont)

	var out bytes.Buffer

	json.Indent(&out, data, "", "  ")

	out.WriteTo(os.Stdout)
	os.Stdout.Write([]byte("\n"))

	return nil
}
