package commands

import (
	"fmt"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
	"github.com/vektra/container/utils"
)

type commitOptions struct {
	Author  string `long:"author" description:"Who is creating this image?"`
	Comment string `long:"comment" description:"Any comment?"`
	Squash  bool   `short:"s" description:"Make a squashfs based image"`
}

func (co *commitOptions) Usage() string {
	return "[OPTIONS] <id>"
}

func (co *commitOptions) Execute(args []string) error {
	if err := app.CheckArity(1, 1, args); err != nil {
		return err
	}

	id := utils.ExpandID(env.DIR, args[0])

	cont, err := env.LoadContainer(env.DIR, id)

	if err != nil {
		return fmt.Errorf("Unable to load %s: %s\n", id, err)
	}

	ts, err := env.DefaultTagStore()

	if err != nil {
		return err
	}

	img, err := cont.Commit(co.Comment, co.Author, nil, co.Squash, false)

	if err != nil {
		return fmt.Errorf("Unable to create image: %s\n", err)
	}

	repo, tag := env.ParseRepositoryTag(args[1])

	ts.Add(repo, tag, img.ID)
	ts.Flush()

	return nil
}

func init() {
	app.AddCommand("commit", "Convert a container to an image", "", &commitOptions{})
}
