package commands

import (
	"flag"
	"fmt"
	"github.com/vektra/container/env"
	"github.com/vektra/container/utils"
	"io/ioutil"
	"os"
	"path"
)

func nukeImage(id string) {
	ts, err := env.DefaultTagStore()

	if err != nil {
		panic(err)
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
			fmt.Printf("Unable to find repo '%s'\n", id)
			return
		} else {
			err = nil
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
		fmt.Printf("Error locating image: %s\n", err)
		return
	}

	ts.Flush()
}

func init() {
	addCommand("nuke", "<image> | <id>", "Delete an image or container", 1, nuke)
}

func nuke(cmd *flag.FlagSet) {
	id := utils.ExpandID(env.DIR, cmd.Arg(0))

	root := path.Join(env.DIR, "containers", id)

	_, err := os.Stat(root)

	if err != nil {
		// Look for an image instead
		nukeImage(id)
		return
	}

	_, err = ioutil.ReadFile(path.Join(root, "running"))

	if err == nil {
		fmt.Printf("Cowardly refusing to nuke running container\n")
		os.Exit(1)
	}

	err = os.RemoveAll(root)

	fmt.Printf("Removed %s\n", id)

	if err != nil {
		panic(err)
	}
}
