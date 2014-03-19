package commands

import (
	"flag"
	"fmt"
	"github.com/vektra/container/env"
	"os"
)

var flDel *bool
var flResolve *bool

func init() {
	cmd := addCommand("tag", "[OPTIONS] <tag> [<id>]", "Add or remove a tag from an image", 1, tag)

	flDel = cmd.Bool("r", false, "Remove a tag from an image")
	flResolve = cmd.Bool("f", false, "Resolve a tag into an id")
}

func tag(cmd *flag.FlagSet) {
	ts, err := env.DefaultTagStore()

	if err != nil {
		panic(err)
	}

	if *flResolve {
		if len(cmd.Args()) < 1 {
			fmt.Printf("Specify a repo:tag to resolve\n")
			os.Exit(1)
		}

		id, err := ts.Lookup(cmd.Arg(0))

		if err != nil {
			fmt.Printf("Unable to resolve tag: %s\n", cmd.Arg(0))
			os.Exit(1)
		}

		fmt.Printf("%s\n", id)
		os.Exit(0)
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
