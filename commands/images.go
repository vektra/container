package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/components/container/env"
	"github.com/arch-reactor/components/container/utils"
	"os"
	"path"
	"text/tabwriter"
)

func init() {
	addCommand("images", "[directory]", "List images installed", 0, images)
}

func images(cmd *flag.FlagSet) {
	var repoDir string
	if len(cmd.Args()) > 0 {
		repoDir = cmd.Arg(0)
	} else {
		repoDir = env.DIR
	}

	fmt.Printf("Loading tag store from: %s.\n", repoDir)
	// TODO(kev): Don't hardcode file name
	ts, err := env.ReadRepoFile(path.Join(repoDir, "repositories"))

	if err != nil {
		fmt.Printf("No images: %s\n", err)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "REPO\tTAG\tID\n")

	for repo, tags := range ts.Repositories {
		for tag, id := range tags {
			fmt.Fprintf(w, "%s\t%s\t%s\n", repo, tag, utils.TruncateID(id))
		}
	}

	w.Flush()
}
