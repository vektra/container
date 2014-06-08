package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
	"github.com/vektra/container/utils"
)

type Importer struct {
	dir     string
	tags    *env.TagStore
	sysTags *env.TagStore
}

type importOptions struct{}

func (io *importOptions) Usage() string {
	return "<dir> <repo:tag>"
}

func (io *importOptions) Execute(args []string) error {
	if err := app.CheckArity(2, 2, args); err != nil {
		return err
	}

	dir := args[0]

	i := &Importer{dir, nil, nil}

	name, tag := env.ParseRepositoryTag(args[1])

	fmt.Printf("Importing %s:%s...\n", name, tag)

	repoPath := path.Join(i.dir, "repositories")

	data, err := ioutil.ReadFile(repoPath)

	if err != nil {
		return err
	}

	i.tags = &env.TagStore{}

	err = json.Unmarshal(data, &i.tags)

	if err != nil {
		return err
	}

	sub := i.tags.Repositories[name]

	if sub == nil {
		return fmt.Errorf("No repo named %s found\n", name)
	}

	hash, ok := sub[tag]

	if !ok {
		return fmt.Errorf("No tag named %s found\n", tag)
	}

	i.sysTags = &env.TagStore{}
	sysPath := path.Join(env.DIR, "repositories")

	sysData, err := ioutil.ReadFile(sysPath)

	if err != nil {
		i.sysTags.Repositories = make(map[string]env.Repository)
	} else {
		err = json.Unmarshal(sysData, &i.sysTags)

		if err != nil {
			return err
		}
	}

	i.importLayer(hash)

	sysData, err = json.Marshal(i.sysTags)

	if err != nil {
		return err
	}

	ioutil.WriteFile(sysPath, sysData, 0644)

	return nil
}

func init() {
	app.AddCommand("import", "Import an image from disk", "", &importOptions{})
}

func (i *Importer) alreadyExists(hash string) bool {
	outPath := path.Join(env.DIR, "graph", hash)

	_, err := os.Stat(outPath)

	return err == nil
}

func (i *Importer) importLayer(hash string) {
	layerPath := path.Join(i.dir, hash+".layer")

	if i.alreadyExists(hash) {
		fmt.Printf("Layer %s already installed, not overwriting\n", hash)
		return
	}

	tmpPath := path.Join(env.DIR, "graph", ":artmp:"+hash)

	outPath := path.Join(env.DIR, "graph", hash)

	os.MkdirAll(tmpPath, 0755)

	os.MkdirAll(path.Join(outPath, "layer"), 0755)

	c := exec.Command("tar", "--numeric-owner", "-f", layerPath,
		"-C", tmpPath, "-x")

	sout, err := c.CombinedOutput()

	if err != nil {
		fmt.Printf("Error: %s\n", string(sout))
		panic(err)
	}

	img := i.extract(hash, tmpPath)

	if img.Parent != "" {
		fmt.Printf("Moving to import parent %s...\n", img.Parent)
		i.importLayer(img.Parent)
	}
}

func (i *Importer) extract(hash, tmpPath string) *env.Image {
	outPath := path.Join(env.DIR, "graph", hash)

	fmt.Printf("Extracting data...\n")

	jsonData, err := ioutil.ReadFile(path.Join(tmpPath, "metadata.js"))

	if err != nil {
		panic(err)
	}

	img := &env.Image{}

	err = json.Unmarshal(jsonData, &img)

	if err != nil {
		panic(err)
	}

	utils.Run("cp", path.Join(tmpPath, "metadata.js"), path.Join(outPath, "json"))

	utils.Run("tar", "--numeric-owner", "-f", path.Join(tmpPath, "data.tar.bz2"),
		"-C", path.Join(outPath, "layer"), "-xj")

	utils.Run("cp", path.Join(tmpPath, "data.tar.bz2"), path.Join(outPath, "layer.tar.bz2"))

	os.RemoveAll(tmpPath)

	fmt.Printf("Importing tags...\n")

	i.tags.CopyTo(i.sysTags, hash, false)

	return img
}
