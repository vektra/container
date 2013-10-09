package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/container/env"
	"github.com/arch-reactor/container/utils"
	"os"
	"sort"
	"text/tabwriter"
)

var flVerbose *bool

func init() {
	cmd := addCommand("images", "[OPTIONS] [directory]", "List images installed", 0, images)

	flVerbose = cmd.Bool("v", false, "Show more details about images")
}

func images(cmd *flag.FlagSet) {
	var repoDir string
	if len(cmd.Args()) > 0 {
		repoDir = cmd.Arg(0)
		fmt.Printf("Loading tag store from: %s\n", repoDir)
	} else {
		repoDir = env.DIR
	}

	// TODO(kev): Don't hardcode file name
	ts, err := env.LoadTagStore(repoDir)

	if err != nil {
		fmt.Printf("No images: %s\n", err)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	if *flVerbose {
		fmt.Fprintf(w, "REPO\tTAG\tID\tPARENT\tCREATED\n")
	} else {
		fmt.Fprintf(w, "REPO\tTAG\tID\n")
	}

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

			if *flVerbose {
				img := ts.Entries[id]
				if img == nil {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", repo, tag, utils.TruncateID(id), "?", "?")
				} else {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", repo, tag, utils.TruncateID(id),
						utils.TruncateID(img.Parent), img.Created)
				}
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\n", repo, tag, utils.TruncateID(id))
			}
		}
	}

	w.Flush()
}
