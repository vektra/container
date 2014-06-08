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

type Exporter struct {
	out  string
	ents env.Entries
	tags *env.TagStore
	tout *env.TagStore
}

func (e *Exporter) pkg(img *env.Image, hash string) {
	tarbz2 := path.Join(e.out, "data.tar.bz2")
	layerPath := path.Join(env.DIR, "graph", hash, "layer")
	jsonPath := path.Join(env.DIR, "graph", hash, "json")

	fmt.Printf("Creating archive of layer %s...\n", hash)

	c := exec.Command("tar", "--numeric-owner", "-f", tarbz2,
		"-C", layerPath, "-cj", ".")

	sout, err := c.CombinedOutput()

	if err != nil {
		fmt.Printf("Error: %s\n", string(sout))
		panic(err)
	}

	fmt.Printf("Packaging layer and json...\n")

	c = exec.Command("cp", jsonPath, path.Join(e.out, "metadata.js"))

	sout, err = c.CombinedOutput()

	if err != nil {
		fmt.Printf("Error: %s\n", string(sout))
		panic(err)
	}

	final := path.Join(e.out, hash+".layer")

	args := []string{"--numeric-owner", "-c", "-f", final,
		"-C", e.out, "data.tar.bz2", "metadata.js"}

	img.WithPrimaryId(func(id string) {
		ioutil.WriteFile(path.Join(e.out, "id"), []byte(id), 0644)
		args = append(args, "id")
	})

	utils.Run("tar", args...)

	os.Remove(path.Join(e.out, "id"))
	os.Remove(tarbz2)
	os.Remove(path.Join(e.out, "metadata.js"))

	fmt.Printf("Packaged!\n")

	e.tags.CopyTo(e.tout, hash, true)

	if len(img.Parent) > 0 {
		nxt := e.ents[img.Parent]

		_, err := os.Stat(path.Join(e.out, img.Parent+".layer"))

		if err == nil {
			fmt.Printf("Skipping %s, already archived\n", img.Parent)
		} else {
			e.pkg(nxt, img.Parent)
		}
	}
}

type exportOptions struct{}

func (eo *exportOptions) Usage() string {
	return "<dir> <repo:tag>"
}

func (eo *exportOptions) Execute(args []string) error {
	if err := app.CheckArity(2, 2, args); err != nil {
		return err
	}

	tags, err := env.DefaultTagStore()

	if err != nil {
		return err
	}

	dir := args[0]

	e := &Exporter{dir, tags.Entries, tags, nil}

	imageName, tagName := env.ParseRepositoryTag(args[1])

	curTagData, err := ioutil.ReadFile(path.Join(dir, "repositories"))

	if err != nil {
		e.tout = &env.TagStore{"", e.ents, make(map[string]env.Repository)}
	} else {
		err = json.Unmarshal(curTagData, &e.tout)

		if err != nil {
			return err
		}
	}

	if top, ok := e.tags.Repositories[imageName]; ok {
		if hash, ok := top[tagName]; ok {
			if img, ok := e.ents[hash]; ok {
				img.Ids = []string{imageName + ":" + tagName}
				fmt.Printf("Found %s:%s (parent: %s)\n", imageName, tagName, img.Parent)
				e.pkg(img, hash)

				jsonData, err := json.Marshal(e.tout)

				if err != nil {
					return err
				}

				ioutil.WriteFile(path.Join(dir, "repositories"), jsonData, 0644)
			} else {
				return fmt.Errorf("Tag doesn't reference an image!\n")
			}
		} else {
			return fmt.Errorf("Can't find tag %s in %s\n", imageName, tagName)
		}
	} else {
		return fmt.Errorf("Can't find %s\n", imageName)
	}

	return nil
}

func init() {
	app.AddCommand("export", "Export an image to disk", "", &exportOptions{})
}
