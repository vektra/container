package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/crowdmob/goamz/aws"
	"github.com/crowdmob/goamz/s3"
	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
	"github.com/vektra/container/utils"
)

const accessKey = "AKIAJ23SZJOF6SSCAYFQ"
const secretKey = "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"
const defBucket = "priv.nextphase.io"

var awsAuth = aws.Auth{AccessKey: accessKey, SecretKey: secretKey}
var awsRegion = aws.USEast

func (i *Importer) download(buk *s3.Bucket, id string) error {
	tmpPath := path.Join(env.DIR, "graph", ":artmp:"+id)

	outPath := path.Join(env.DIR, "graph", id)

	os.MkdirAll(tmpPath, 0755)

	os.MkdirAll(path.Join(outPath, "layer"), 0755)

	key := fmt.Sprintf("/binary/repos/%s.layer", id)

	rc, err := buk.GetReader(key)

	if err != nil {
		return err
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
			return fmt.Errorf("Parent layer %s already installed, not overwriting\n", img.Parent)
		}

		fmt.Printf("Moving to download parent %s...\n", img.Parent)
		return i.download(buk, img.Parent)
	}

	return nil
}

/*

	fmt.Printf("Extracting data...\n")

	jsonData, err := ioutil.ReadFile(path.Join(tmpPath, "metadata.js"))

	if err != nil {
		return err
	}

	img := &Image{}

	err = json.Unmarshal(jsonData, &img)

	if err != nil {
		return err
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

type s3Options struct {
	Force bool `short:"f" description:"Download repos even if we already have them"`
}

func init() {
	app.AddCommand("s3", "Pull down a repo from S3", "", &s3Options{})
}

func (so *s3Options) Usage() string {
	return "[OPTIONS] <repo:tag>"
}

func (so *s3Options) Execute(args []string) error {
	if err := app.CheckArity(1, 1, args); err != nil {
		return err
	}

	repo := args[0]

	s3 := s3.New(awsAuth, awsRegion)
	buk := s3.Bucket(defBucket)

	data, err := buk.Get("/binary/repos/repositories")

	if err != nil {
		return err
	}

	ts := &env.TagStore{}

	err = json.Unmarshal(data, &ts)

	id, err := ts.Lookup(repo)

	if err != nil {
		return err
	}

	dts, err := env.DefaultTagStore()

	if err != nil {
		return err
	}

	i := &Importer{tags: ts, sysTags: dts}

	if !so.Force {
		if i.alreadyExists(id) {
			return fmt.Errorf("Already have %s, skipping download\n", utils.TruncateID(id))
		}
	}

	fmt.Printf("Downloading %s (%s)\n", repo, utils.TruncateID(id))

	err = i.download(buk, id)

	dts.Flush()

	return err
}
