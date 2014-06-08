package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
	"github.com/vektra/container/utils"
)

func nukeImage(no *nukeOptions, id string) error {
	ts, err := env.DefaultTagStore()

	if err != nil {
		return err
	}

	img, err := ts.LookupImage(id)

	if err == nil {
		repo, tag := env.ParseRepositoryTag(id)
		ts.RemoveTag(repo, tag)
	} else {
		long := env.ExpandImageID(id)

		var ok bool
		img, ok = ts.Entries[long]

		if !ok {
			return fmt.Errorf("Unable to find repo '%s'\n", id)
		} else {
			err = nil
		}

		if !no.Force && ts.UsedAsParent(long) {
			return fmt.Errorf("%s is a parent image, not removing (use -force to force)\n", id)
		}

	}

	if img != nil {
		otherRepo, _ := ts.Find(img.ID)

		if otherRepo != "" {
			fmt.Printf("Removing %s tag on %s only\n", id, utils.TruncateID(img.ID))
		} else {
			fmt.Printf("Nuking image %s..\n", id)
			img.Remove()
		}
	} else {
		return fmt.Errorf("Error locating image: %s\n", err)
	}

	return ts.Flush()
}

type nukeOptions struct {
	Force bool `long:"force" description:"Forcefully remove the image"`
}

func (no *nukeOptions) Usage() string {
	return "[OPTIONS] <repo:tag> | <id>"
}

func init() {
	app.AddCommand("nuke", "Delete an image or container", "", &nukeOptions{})
}

func (no *nukeOptions) Execute(args []string) error {
	if err := app.CheckArity(1, 1, args); err != nil {
		return err
	}

	id := utils.ExpandID(env.DIR, args[0])

	root := path.Join(env.DIR, "containers", id)

	_, err := os.Stat(root)

	if err != nil {
		// Look for an image instead
		return nukeImage(no, id)
	}

	_, err = ioutil.ReadFile(path.Join(root, "running"))

	if err == nil {
		fmt.Printf("Cowardly refusing to nuke running container\n")
		os.Exit(1)
	}

	err = os.RemoveAll(root)

	if err != nil {
		return err
	}

	fmt.Printf("Removed %s\n", id)

	return nil
}
