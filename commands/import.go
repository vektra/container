package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/arch-reactor/container/env"
	"github.com/arch-reactor/container/utils"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
)

type Importer struct {
	dir     string
	tags    *env.TagStore
	sysTags *env.TagStore
}

func init() {
	addCommand("import", "<directory> <image>", "Import an image from disk", 2, importLayer)
}

func importLayer(cmd *flag.FlagSet) {
	dir := cmd.Arg(0)

	i := &Importer{dir, nil, nil}

	name, tag := env.ParseRepositoryTag(cmd.Arg(1))

	fmt.Printf("Importing %s:%s...\n", name, tag)

	repoPath := path.Join(i.dir, "repositories")

	data, err := ioutil.ReadFile(repoPath)

	if err != nil {
		panic(err)
	}

	i.tags = &env.TagStore{}

	err = json.Unmarshal(data, &i.tags)

	if err != nil {
		panic(err)
	}

	sub := i.tags.Repositories[name]

	if sub == nil {
		fmt.Printf("No repo named %s found\n", name)
		os.Exit(1)
	}

	hash, ok := sub[tag]

	if !ok {
		fmt.Printf("No tag named %s found\n", tag)
		os.Exit(1)
	}

	i.sysTags = &env.TagStore{}
	sysPath := path.Join(env.DIR, "repositories")

	sysData, err := ioutil.ReadFile(sysPath)

	if err != nil {
		i.sysTags.Repositories = make(map[string]env.Repository)
	} else {
		err = json.Unmarshal(sysData, &i.sysTags)

		if err != nil {
			panic(err)
		}
	}

	i.importLayer(hash)

	sysData, err = json.Marshal(i.sysTags)

	if err != nil {
		panic(err)
	}

	ioutil.WriteFile(sysPath, sysData, 0644)
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
