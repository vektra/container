package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/components/container/env"
	"github.com/arch-reactor/components/container/utils"
	"io/ioutil"
	"os"
	"path"
)

func nukeImage(id string) {
	ts, err := env.DefaultTagStore()

	if err != nil {
		panic(err)
	}

	repo, tag := env.ParseRepositoryTag(id)

	if !ts.RemoveTag(repo, tag) {
		fmt.Printf("Unable to find repo '%s'\n", id)
		return
	}

	img, err := ts.LookupImage(id)

	if err == nil {
		otherRepo, _ := ts.Find(img.ID)

		if otherRepo != "" {
			fmt.Printf("Removing %s tag on %s only\n", id, utils.TruncateID(img.ID))
		} else {
			fmt.Printf("Nuking image %s..\n", id)
			os.RemoveAll(path.Join(env.DIR, "graph", img.ID))
		}
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