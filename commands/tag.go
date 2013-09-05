package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/container/env"
	"os"
)

var flDel *bool

func init() {
	cmd := addCommand("tag", "[OPTIONS] <tag> [<id>]", "Add or remove a tag from an image", 1, tag)

	flDel = cmd.Bool("r", false, "Remove a tag from an image")
}

func tag(cmd *flag.FlagSet) {
	ts, err := env.DefaultTagStore()

	if err != nil {
		panic(err)
	}

	if *flDel {
		if len(cmd.Args()) < 1 {
			fmt.Printf("Specify a repo:tag to delete\n")
			os.Exit(1)
		}

		repo, tag := env.ParseRepositoryTag(cmd.Arg(0))

		ts.RemoveTag(repo, tag)
	} else {
		if len(cmd.Args()) < 2 {
			fmt.Printf("Specify a repo:tag and id to add\n")
			os.Exit(1)
		}
		repo, tag := env.ParseRepositoryTag(cmd.Arg(0))

		id, ok := env.SafelyExpandImageID(cmd.Arg(1))

		if !ok {
			fmt.Printf("Unable to find image matching '%s'\n", cmd.Arg(1))
			return
		}

		ts.Add(repo, tag, id)
	}

	ts.Flush()
}
