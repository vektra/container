package env

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const DIR = "/var/lib/ar-container"

// ar-container stores details about each container in /var/run/ar-container/$PID/
const RUN_DIR = "/var/run/ar-container"

// When the container's data is initialized, it advertises by placing a file at INIT_DIR/$PID
const INIT_DIR = "/var/run/ar-container/running"

func Init() error { // Not auto-run on purpose.
	paths := []string{RUN_DIR, INIT_DIR, path.Join(DIR, "graph"), path.Join(DIR, "containers")}
	for _, dir := range paths {
		if err := os.MkdirAll(dir, 0755); err != nil {
			errors.New(fmt.Sprintf("Unable to create dir: %s.\nError: %s", dir, err))
		}
	}

	_, err := os.Stat(path.Join(DIR, "repositories"))

	if err != nil {
		if err = ioutil.WriteFile(path.Join(DIR, "repositories"), []byte("{}"), 0644); err != nil {
			return err
		}
	}

	return nil
}

var Verbose bool

func init() {
	Verbose = os.Getenv("VERBOSE") != ""
}

func logv(s string, a ...interface{}) {
	if !Verbose {
		return
	}

	fmt.Printf("[*] "+s+"\n", a...)
}
