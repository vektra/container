package commands

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
	"github.com/vektra/container/utils"
)

type imagesOptions struct {
	Verbose bool `short:"v" description:"Show more details"`
}

func (io *imagesOptions) Usage() string {
	return "[OPTIONS] [repoDir]"
}

func (io *imagesOptions) Execute(args []string) error {
	if err := app.CheckArity(0, 1, args); err != nil {
		return err
	}

	var repoDir string
	if len(args) > 0 {
		repoDir = args[0]
		fmt.Printf("Loading tag store from: %s\n", repoDir)
	} else {
		repoDir = env.DIR
	}

	// TODO(kev): Don't hardcode file name
	ts, err := env.LoadTagStore(repoDir)

	if err != nil {
		return fmt.Errorf("No images: %s\n", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	if io.Verbose {
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

			if io.Verbose {
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

	return nil
}

func init() {
	app.AddCommand("images", "List images installed", "", &imagesOptions{})
}
