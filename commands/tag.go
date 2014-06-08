package commands

import (
	"fmt"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
)

type tagOptions struct {
	Del     bool `short:"r" description:"Remove a tag from an image"`
	Resolve bool `short:"f" description:"Resolve a tag into an id"`
}

func init() {
	app.AddCommand("tag", "Add or remove a tag from an image", "", &tagOptions{})
}

func (to *tagOptions) Usage() string {
	return "[OPTIONS] <tag> [id]"
}

func (to *tagOptions) Execute(args []string) error {
	ts, err := env.DefaultTagStore()

	if err != nil {
		panic(err)
	}

	if to.Resolve {
		if len(args) < 1 {
			return fmt.Errorf("Specify a repo:tag to resolve\n")
		}

		id, err := ts.Lookup(args[0])

		if err != nil {
			return fmt.Errorf("Unable to resolve tag: %s\n", args[0])
		}

		fmt.Printf("%s\n", id)
		return nil
	}

	if to.Del {
		if len(args) < 1 {
			return fmt.Errorf("Specify a repo:tag to delete\n")
		}

		repo, tag := env.ParseRepositoryTag(args[0])

		ts.RemoveTag(repo, tag)
	} else {
		if len(args) < 2 {
			return fmt.Errorf("Specify a repo:tag and id to add\n")
		}
		repo, tag := env.ParseRepositoryTag(args[0])

		id, ok := env.SafelyExpandImageID(args[1])

		if !ok {
			return fmt.Errorf("Unable to find image matching '%s'\n", args[1])
		}

		ts.Add(repo, tag, id)
	}

	ts.Flush()

	return nil
}
