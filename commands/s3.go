package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/arch-reactor/container/env"
	"github.com/arch-reactor/container/utils"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"os"
	"os/exec"
	"path"
)

const accessKey = "AKIAJ23SZJOF6SSCAYFQ"
const secretKey = "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"
const defBucket = "priv.nextphase.io"

var awsAuth = aws.Auth{accessKey, secretKey}
var awsRegion = aws.USEast

func (i *Importer) download(buk *s3.Bucket, id string) {
	tmpPath := path.Join(env.DIR, "graph", ":artmp:"+id)

	outPath := path.Join(env.DIR, "graph", id)

	os.MkdirAll(tmpPath, 0755)

	os.MkdirAll(path.Join(outPath, "layer"), 0755)

	key := fmt.Sprintf("/binary/repos/%s.layer", id)

	rc, err := buk.GetReader(key)

	if err != nil {
		panic(err)
	}

	cmd := exec.Command("tar", "-f", "-", "-C", tmpPath, "-x")
	cmd.Stdin = rc
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Run()

	rc.Close()

	img := i.extract(id, tmpPath)

	if img.Parent != "" {
		if i.alreadyExists(img.Parent) {
			fmt.Printf("Parent layer %s already installed, not overwriting\n", img.Parent)
			return
		}

		fmt.Printf("Moving to download parent %s...\n", img.Parent)
		i.download(buk, img.Parent)
	}
}

/*

	fmt.Printf("Extracting data...\n")

	jsonData, err := ioutil.ReadFile(path.Join(tmpPath, "metadata.js"))

	if err != nil {
		panic(err)
	}

	img := &Image{}

	err = json.Unmarshal(jsonData, &img)

	if err != nil {
		panic(err)
	}

	run("cp", path.Join(tmpPath, "metadata.js"), path.Join(outPath, "json"))

	run("tar", "--numeric-owner", "-f", path.Join(tmpPath, "data.tar.bz2"),
		"-C", path.Join(outPath, "layer"), "-xj")

	run("cp", path.Join(tmpPath, "data.tar.bz2"), path.Join(outPath, "layer.tar.bz2"))
	os.RemoveAll(tmpPath)

	fmt.Printf("Importing tags...\n")

  dts, _ := defaultTagStore()

	ts.CopyTo(dts, id, false)

  dts.Flush()

}
*/

var flForce *bool

func init() {
	cmd := addCommand("s3", "[-f] <repo>[:<tag>]", "Pull down a repo from S3", 1, s3pull)

	flForce = cmd.Bool("f", false, "Download repos even if we already have them")
}

func s3pull(cmd *flag.FlagSet) {
	repo := cmd.Arg(0)

	s3 := s3.New(awsAuth, awsRegion)
	buk := s3.Bucket(defBucket)

	data, err := buk.Get("/binary/repos/repositories")

	if err != nil {
		panic(err)
	}

	ts := &env.TagStore{}

	err = json.Unmarshal(data, &ts)

	id, err := ts.Lookup(repo)

	if err != nil {
		panic(err)
	}

	dts, err := env.DefaultTagStore()

	if err != nil {
		panic(err)
	}

	i := &Importer{tags: ts, sysTags: dts}

	if !*flForce {
		if i.alreadyExists(id) {
			fmt.Printf("Already have %s, skipping download\n", utils.TruncateID(id))
			return
		}
	}

	fmt.Printf("Downloading %s (%s)\n", repo, utils.TruncateID(id))

	i.download(buk, id)

	dts.Flush()
}
