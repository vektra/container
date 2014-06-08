package commands

import (
	"os"
	"os/exec"
	"path"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
)

type lsOptions struct{}

func init() {
	app.AddCommand("ls", "List files in a container", "", &lsOptions{})
}

func (lo *lsOptions) Usage() string {
	return "<repo:tag> [dir]"
}

func (lo *lsOptions) Execute(args []string) error {
	if err := app.CheckArity(1, 2, args); err != nil {
		return err
	}

	repo := args[0]

	dir := "."

	if len(args) > 1 {
		dir = args[1]
	}

	tags, err := env.DefaultTagStore()

	if err != nil {
		return err
	}

	img, err := tags.LookupImage(repo)

	if err != nil {
		return err
	}

	e := exec.Command("ls", path.Join(env.DIR, "graph", img.ID, "layer", dir))
	e.Stdout = os.Stdout

	return e.Run()
}
