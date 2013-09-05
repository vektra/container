package env

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"syscall"
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

	f, err := os.Create(path.Join(DIR, "graph", "_init", ".dockerinit"))

	if err != nil {
		panic(err)
	}

	f.Close()

	var layers []string

	cur := image

	for cur != nil {
		layers = append(layers, path.Join(DIR, "graph", cur.ID, "layer"))
		cur = cur.parentImage
	}

	layers = append(layers, path.Join(DIR, "graph", "_init"))
	return layers, nil
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
	branches := fmt.Sprintf("br:%v:%v", rwBranch, roBranches)

	branches += ",xino=/dev/shm/aufs.xino"

	//if error, try to load aufs kernel module
	if err := syscall.Mount("none", target, "aufs", 0, branches); err != nil {
		log.Printf("Kernel does not support AUFS, trying to load the AUFS module with modprobe...")
		if err := exec.Command("modprobe", "aufs").Run(); err != nil {
			return fmt.Errorf("Unable to load the AUFS module")
		}
		log.Printf("...module loaded.")
		if err := syscall.Mount("none", target, "aufs", 0, branches); err != nil {
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
