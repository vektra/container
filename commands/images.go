package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/container/env"
	"github.com/arch-reactor/container/utils"
	"os"
	"path"
	"sort"
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

	var repos []string

	for repo, _ := range ts.Repositories {
		repos = append(repos, repo)
	}

	sort.Strings(repos)

	for _, repo := range repos {
		tags := ts.Repositories[repo]

		var stags []string

		for tag, _ := range tags {
			stags = append(stags, tag)
		}

		sort.Strings(stags)

		for _, tag := range stags {
			id := tags[tag]

			fmt.Fprintf(w, "%s\t%s\t%s\n", repo, tag, utils.TruncateID(id))
		}
	}

	w.Flush()
}
