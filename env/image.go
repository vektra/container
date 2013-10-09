package env

import (
	"fmt"
	"github.com/arch-reactor/container/utils"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"time"
)

type Image struct {
	ID              string    `json:"id"`
	Parent          string    `json:"parent,omitempty"`
	Comment         string    `json:"comment,omitempty"`
	Created         time.Time `json:"created"`
	Container       string    `json:"container,omitempty"`
	ContainerConfig Config    `json:"container_config,omitempty"`
	DockerVersion   string    `json:"docker_version,omitempty"`
	Author          string    `json:"author,omitempty"`
	Config          *Config   `json:"config,omitempty"`
	Architecture    string    `json:"architecture,omitempty"`
	Size            int64
	Ids             []string
	parentImage     *Image
}

func (image *Image) WithPrimaryId(fn func(string)) {
	if len(image.Ids) > 0 {
		fn(image.Ids[0])
	}
}

func (image *Image) layers() ([]string, error) {
	os.MkdirAll(path.Join(DIR, "graph", "_init"), 0755)
	os.MkdirAll(path.Join(DIR, "graph", "_init", "proc"), 0755)
	os.MkdirAll(path.Join(DIR, "graph", "_init", "dev"), 0755)
	os.MkdirAll(path.Join(DIR, "graph", "_init", "dev", "pts"), 0755)
	os.MkdirAll(path.Join(DIR, "graph", "_init", "sys"), 0755)
	os.MkdirAll(path.Join(DIR, "graph", "_init", "etc"), 0755)

	f, err := os.Create(path.Join(DIR, "graph", "_init", ".dockerinit"))

	if err != nil {
		return nil, err
	}

	f.Close()

	f, err = os.Create(path.Join(DIR, "graph", "_init", "etc", "resolv.conf"))

	if err != nil {
		return nil, err
	}

	f.Close()

	var layers []string

	if image.ID != "" {
		cur := image

		for cur != nil {
			lp := path.Join(DIR, "graph", cur.ID, "layer")

			os.MkdirAll(lp, 0755)

			lst, _ := ioutil.ReadDir(lp)

			if len(lst) == 0 {
				lpfs := path.Join(DIR, "graph", cur.ID, "layer.fs")

				if _, err := os.Stat(lpfs); err == nil {
					utils.Run("mount", lpfs, lp)
				} else {
					return nil, fmt.Errorf("No layer.fs file to mount")
				}
			}

			layers = append(layers, lp)
			cur = cur.parentImage
		}
	}

	layers = append(layers, path.Join(DIR, "graph", "_init"))
	return layers, nil
}

func (image *Image) Remove() error {
	lpfs := path.Join(DIR, "graph", image.ID, "layer.fs")

	if _, err := os.Stat(lpfs); err == nil {
		lp := path.Join(DIR, "graph", image.ID, "layer")
		utils.RunUnchecked("umount", lp)
	}

	return os.RemoveAll(path.Join(DIR, "graph", image.ID))
}

func (image *Image) Mount(root, rw string) error {
	if mounted, err := Mounted(root); err != nil {
		return err
	} else if mounted {
		return fmt.Errorf("%s is already mounted", root)
	}
	layers, err := image.layers()
	if err != nil {
		return err
	}
	// Create the target directories if they don't exist
	if err := os.Mkdir(root, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := os.Mkdir(rw, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := MountAUFS(layers, rw, root); err != nil {
		return err
	}
	return nil
}

func MountAUFS(ro []string, rw string, target string) error {
	// FIXME: Now mount the layers
	rwBranch := fmt.Sprintf("%v=rw", rw)
	roBranches := ""
	for _, layer := range ro {
		roBranches += fmt.Sprintf("%v=ro+wh:", layer)
	}
	var branches string

	if roBranches == "" {
		branches = "br:" + rwBranch
	} else {
		branches = fmt.Sprintf("br:%v:%v", rwBranch, roBranches)
	}

	branches += ",xino=/dev/shm/aufs.xino"

	//if error, try to load aufs kernel module
	if err := mount("none", target, "aufs", 0, branches); err != nil {
		log.Printf("Kernel does not support AUFS, trying to load the AUFS module with modprobe...")
		if err := exec.Command("modprobe", "aufs").Run(); err != nil {
			return fmt.Errorf("Unable to load the AUFS module")
		}
		log.Printf("...module loaded.")
		if err := mount("none", target, "aufs", 0, branches); err != nil {
			return fmt.Errorf("Unable to mount using aufs")
		}
	}
	return nil
}

func ExpandImageID(id string) string {
	id, _ = SafelyExpandImageID(id)
	return id
}

func SafelyExpandImageID(id string) (string, bool) {
	dirs, err := ioutil.ReadDir(path.Join(DIR, "graph"))

	if err != nil {
		return id, false
	}

	found := false
	res := id

	for _, f := range dirs {
		dir := f.Name()

		if len(dir) >= len(id) && dir[0:len(id)] == id {
			// This is for when multiple containers match the prefix
			if found {
				return id, false
			}

			res = dir
			found = true
		}
	}

	return res, found
}
