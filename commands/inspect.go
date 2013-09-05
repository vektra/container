package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"github.com/arch-reactor/container/env"
	"github.com/arch-reactor/container/utils"
	"os"
)

func init() {
	addCommand("inspect", "<id>", "Display details about a container", 1, inspect)
}

func inspect(cmd *flag.FlagSet) {
	id := utils.ExpandID(env.DIR, cmd.Arg(0))

	cont, err := env.LoadContainer(env.DIR, id)

	if err != nil {
		panic(err)
	}

	data, err := json.Marshal(cont)

	var out bytes.Buffer

	json.Indent(&out, data, "", "  ")

	out.WriteTo(os.Stdout)
	os.Stdout.Write([]byte("\n"))
}
