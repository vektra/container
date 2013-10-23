package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/container/env"
	"github.com/arch-reactor/container/utils"
)

var flAuthor *string
var flComment *string
var flComSquash *bool

func init() {
	cmd := addCommand("commit", "[OPTIONS] <id> <image>[:<tag>]",
		"Convert a container to an image", 2, commit)
	flAuthor = cmd.String("author", "", "Who is creating this image")
	flComment = cmd.String("comment", "", "Any comment?")
	flComSquash = cmd.Bool("s", false, "Make a squashfs based image")
}

func commit(cmd *flag.FlagSet) {
	id := utils.ExpandID(env.DIR, cmd.Arg(0))

	cont, err := env.LoadContainer(env.DIR, id)

	if err != nil {
		fmt.Printf("Unable to load %s: %s\n", id, err)
		return
	}

	ts, err := env.DefaultTagStore()

	if err != nil {
		panic(err)
	}

	img, err := cont.Commit(*flComment, *flAuthor, nil, *flComSquash, false)

	if err != nil {
		fmt.Printf("Unable to create image: %s\n", err)
		return
	}

	repo, tag := env.ParseRepositoryTag(cmd.Arg(1))

	ts.Add(repo, tag, img.ID)
	ts.Flush()
}
