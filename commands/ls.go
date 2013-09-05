package commands

import (
	"flag"
	"github.com/arch-reactor/container/env"
	"os"
	"os/exec"
	"path"
)

func init() {
	addCommand("ls", "<id> [<path...>]", "List files in a container", 1, ls)
}

func ls(cmd *flag.FlagSet) {
	repo := cmd.Arg(0)

	dir := "."

	if len(cmd.Args()) > 1 {
		dir = cmd.Arg(1)
	}

	tags, err := env.DefaultTagStore()

	if err != nil {
		panic(err)
	}

	img, err := tags.LookupImage(repo)

	if err != nil {
		panic(err)
	}

	e := exec.Command("ls", path.Join(env.DIR, "graph", img.ID, "layer", dir))
	e.Stdout = os.Stdout

	err = e.Run()

	if err != nil {
		panic(err)
	}
}
