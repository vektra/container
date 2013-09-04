package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/components/container/env"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"text/tabwriter"
)

type Containers []*env.Container

func (c Containers) Len() int      { return len(c) }
func (c Containers) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

func (c Containers) Less(i, j int) bool {
	return c[i].Created.After(c[j].Created)
}

func init() {
	addCommand("ps", "", "List containers", 0, ps)
}

func ps(cmd *flag.FlagSet) {
	dir, err := ioutil.ReadDir(path.Join(env.DIR, "containers"))

	if err != nil {
		panic(err)
	}

	ts, err := env.DefaultTagStore()

	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "  ID\tREPO\tCREATED\n")

	var cs Containers

	for _, f := range dir {
		id := f.Name()

		cont, err := env.LoadContainer(env.DIR, id)
		if err != nil {
			continue
		}

		cs = append(cs, cont)
	}

	sort.Sort(cs)

	for _, cont := range cs {
		repo, tag := ts.Find(cont.Image)

		state := "  "

		_, err := os.Stat(cont.PathTo("running"))

		if err == nil {
			state = "* "
		}

		if repo == "" {
			fmt.Fprintf(w, "%s%s\t \t%s\n", state, cont.ID[0:12], cont.Created.String())
		} else {
			fmt.Fprintf(w, "%s%s\t%s:%s\t%s\n", state, cont.ID[0:12], repo, tag, cont.Created.String())
		}

	}

	w.Flush()

}
