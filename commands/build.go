package commands

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/vektra/container/env"
	"github.com/vektra/container/utils"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/signal"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

type BuildFile interface {
	Build(io.Reader) (string, error)
	CmdFrom(string) error
	CmdRun(string) error
}

type buildFile struct {
	tags       *env.TagStore
	image      string
	maintainer string
	config     *env.Config
	context    string
	verbose    bool
	hostcfg    *env.HostConfig
	container  *env.Container
	outImage   string
	out        io.Writer
	abort      chan os.Signal

	// Use to be sure to cleanup when errors happen
	saveContainer bool
	squash        bool
	experiment    bool
}

func (b *buildFile) CmdFrom(name string) error {
	b.image = name
	b.config = &env.Config{}

	b.hostcfg = &env.HostConfig{Save: true, Quiet: true}

	if b.config.Env == nil || len(b.config.Env) == 0 {
		b.config.Env = append(b.config.Env, "HOME=/", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}

	return b.Start()
}

func (b *buildFile) CmdMaintainer(name string) error {
	b.maintainer = name
	return nil
}

func (b *buildFile) Start() error {
	b.config.Cmd = []string{"/bin/sh", "-c", "#(nop) START"}

	b.config.Image = b.image

	// Create the container and start it
	container, err := env.ContainerCreate(b.tags, b.config)
	if err != nil {
		return err
	}

	b.container = container

	return nil
}

func (b *buildFile) CmdRun(args string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
	}

	b.container.Path = "/bin/sh"
	b.container.Args = []string{"-c", args}

	if err := b.container.Start(b.hostcfg); err != nil {
		return err
	}

	defer b.container.Wait(b.hostcfg)

	return nil
}

func (b *buildFile) FindEnvKey(key string) int {
	for k, envVar := range b.config.Env {
		envParts := strings.SplitN(envVar, "=", 2)
		if key == envParts[0] {
			return k
		}
	}
	return -1
}

func (b *buildFile) ReplaceEnvMatches(value string) (string, error) {
	exp, err := regexp.Compile("(\\\\\\\\+|[^\\\\]|\\b|\\A)\\$({?)([[:alnum:]_]+)(}?)")
	if err != nil {
		return value, err
	}
	matches := exp.FindAllString(value, -1)
	for _, match := range matches {
		match = match[strings.Index(match, "$"):]
		matchKey := strings.Trim(match, "${}")

		for _, envVar := range b.config.Env {
			envParts := strings.SplitN(envVar, "=", 2)
			envKey := envParts[0]
			envValue := envParts[1]

			if envKey == matchKey {
				value = strings.Replace(value, match, envValue, -1)
				break
			}
		}
	}
	return value, nil
}

func (b *buildFile) CmdEnv(args string) error {
	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid ENV format")
	}
	key := strings.Trim(tmp[0], " \t")
	value := strings.Trim(tmp[1], " \t")

	envKey := b.FindEnvKey(key)
	replacedValue, err := b.ReplaceEnvMatches(value)
	if err != nil {
		return err
	}
	replacedVar := fmt.Sprintf("%s=%s", key, replacedValue)

	if envKey >= 0 {
		b.config.Env[envKey] = replacedVar
	} else {
		b.config.Env = append(b.config.Env, replacedVar)
	}

	return nil
}

func (b *buildFile) CmdCmd(args string) error {
	var cmd []string

	if err := json.Unmarshal([]byte(args), &cmd); err != nil {
		utils.Debugf("Error unmarshalling: %s, setting cmd to /bin/sh -c", err)
		cmd = []string{"/bin/sh", "-c", args}
	}

	b.config.Cmd = cmd
	return nil
}

func (b *buildFile) CmdExpose(args string) error {
	ports := strings.Split(args, " ")
	b.config.PortSpecs = append(ports, b.config.PortSpecs...)
	return nil
}

// SERVICE NAME PORT [VERSION]
func (b *buildFile) CmdService(args string) error {
	parts := strings.Split(args, " ")
	name := parts[0]
	p, _ := strconv.ParseUint(parts[1], 10, 16)
	port := uint16(p)
	var version string
	if len(parts) > 2 {
		version = parts[2]
	} else {
		version = "unknown"
	}

	service := env.ServiceSpec{Name: name, Port: port, Version: version}
	b.config.ServiceSpecs = append([]env.ServiceSpec{service}, b.config.ServiceSpecs...)
	b.config.PortSpecs = append([]string{parts[1]}, b.config.PortSpecs...)
	return nil
}

func (b *buildFile) CmdInsert(args string) error {
	return fmt.Errorf("INSERT has been deprecated. Please use ADD instead")
}

func (b *buildFile) CmdCopy(args string) error {
	return fmt.Errorf("COPY has been deprecated. Please use ADD instead")
}

func (b *buildFile) CmdEntrypoint(args string) error {
	if args == "" {
		return fmt.Errorf("Entrypoint cannot be empty")
	}

	var entrypoint []string
	if err := json.Unmarshal([]byte(args), &entrypoint); err != nil {
		b.config.Entrypoint = []string{"/bin/sh", "-c", args}
	} else {
		b.config.Entrypoint = entrypoint
	}

	return nil
}

func (b *buildFile) CmdVolume(args string) error {
	if args == "" {
		return fmt.Errorf("Volume cannot be empty")
	}

	var volume []string
	if err := json.Unmarshal([]byte(args), &volume); err != nil {
		volume = []string{args}
	}

	if b.config.Volumes == nil {
		b.config.Volumes = utils.NewPathOpts()
	}

	for _, v := range volume {
		b.config.Volumes[v] = struct{}{}
	}

	return nil
}

func (b *buildFile) addRemote(container *env.Container, orig, dest string) error {
	file, err := utils.Download(orig, ioutil.Discard)
	if err != nil {
		return err
	}
	defer file.Body.Close()

	// If the destination is a directory, figure out the filename.
	if strings.HasSuffix(dest, "/") {
		u, err := url.Parse(orig)
		if err != nil {
			return err
		}
		path := u.Path
		if strings.HasSuffix(path, "/") {
			path = path[:len(path)-1]
		}
		parts := strings.Split(path, "/")
		filename := parts[len(parts)-1]
		if filename == "" {
			return fmt.Errorf("cannot determine filename from url: %s", u)
		}
		dest = dest + filename
	}

	return container.Inject(file.Body, dest)
}

func (b *buildFile) addContext(container *env.Container, orig, dest string) error {
	origPath := path.Join(b.context, orig)
	destPath := path.Join(container.RootfsPath(), dest)

	// Preserve the trailing '/'
	if strings.HasSuffix(dest, "/") {
		destPath = destPath + "/"
	}

	fi, err := os.Stat(origPath)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		if err := utils.CopyWithTar(origPath, destPath); err != nil {
			return err
		}
		// First try to unpack the source as an archive
	} else if err := utils.UntarPath(origPath, destPath); err != nil {
		utils.Debugf("Couldn't untar %s to %s: %s", origPath, destPath, err)
		// If that fails, just copy it as a regular file
		if err := os.MkdirAll(path.Dir(destPath), 0755); err != nil {
			return err
		}

		if err := utils.CopyWithTar(origPath, destPath); err != nil {
			return err
		}
	}

	return nil
}

func (b *buildFile) CmdAdd(args string) error {
	if b.context == "" {
		return fmt.Errorf("No context given. Impossible to use ADD")
	}

	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid ADD format")
	}

	orig, err := b.ReplaceEnvMatches(strings.Trim(tmp[0], " \t"))
	if err != nil {
		return err
	}

	dest, err := b.ReplaceEnvMatches(strings.Trim(tmp[1], " \t"))
	if err != nil {
		return err
	}

	cmd := b.config.Cmd
	b.config.Cmd = []string{"/bin/sh", "-c", fmt.Sprintf("#(nop) ADD %s in %s", orig, dest)}

	b.config.Image = b.image

	if err := b.container.EnsureMounted(); err != nil {
		return err
	}
	defer b.container.Unmount()

	if isURL(orig) {
		if err := b.addRemote(b.container, orig, dest); err != nil {
			return err
		}
	} else {
		if err := b.addContext(b.container, orig, dest); err != nil {
			return err
		}
	}

	b.config.Cmd = cmd
	return nil
}

func isURL(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
}

var ErrAbort = fmt.Errorf("Aborted build")

func (b *buildFile) cleanup() {
	if !b.saveContainer && b.container != nil {
		b.container.Remove()
	}
}

func (b *buildFile) Build(context string) error {
	defer b.cleanup()

	b.context = context

	if _, err := os.Stat(path.Join(context, "build.sh")); err == nil {
		fmt.Printf("Step 0: Execute build.sh on host\n")
		utils.Shell("cd " + context + "; bash ./build.sh")
	}

	dockerfile, err := os.Open(path.Join(context, "Dockerfile"))
	if err != nil {
		return fmt.Errorf("Can't build a directory with no Dockerfile")
	}

	file := bufio.NewReader(dockerfile)
	stepN := 0
	for {
		select {
		case <-b.abort:
			fmt.Printf("Aborting...\n")
			if b.container != nil {
				b.container.Remove()
			}
			return ErrAbort
		default:
			// continue
		}

		line, err := file.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" {
				break
			} else if err != io.EOF {
				return err
			}
		}

		line = strings.Trim(strings.Replace(line, "\t", " ", -1), " \t\r\n")
		// Skip comments and empty line
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		tmp := strings.SplitN(line, " ", 2)
		if len(tmp) != 2 {
			return fmt.Errorf("Invalid Dockerfile format")
		}

		instruction := strings.ToLower(strings.Trim(tmp[0], " "))
		arguments := strings.Trim(tmp[1], " ")

		method, exists := reflect.TypeOf(b).MethodByName("Cmd" + strings.ToUpper(instruction[:1]) + strings.ToLower(instruction[1:]))
		if !exists {
			fmt.Fprintf(b.out, "# Skipping unknown instruction %s\n", strings.ToUpper(instruction))
			continue
		}

		stepN += 1
		fmt.Fprintf(b.out, "Step %d : %s %s\n", stepN, strings.ToUpper(instruction), arguments)

		ret := method.Func.Call([]reflect.Value{reflect.ValueOf(b), reflect.ValueOf(arguments)})[0].Interface()
		if ret != nil {
			return ret.(error)
		}
	}

	b.config.Cmd = nil

	err = b.container.ToDisk()

	if err != nil {
		return err
	}

	if b.experiment {
		b.CmdRun("/bin/bash")
		return nil
	}

	if b.image != "" && b.outImage != "" {
		img, err := b.container.Commit("", "", nil, b.squash, true)

		if err != nil {
			return err
		}

		ts, err := env.DefaultTagStore()

		if err != nil {
			return err
		}

		repo, tag := env.ParseRepositoryTag(b.outImage)

		ts.Add(repo, tag, img.ID)
		ts.Flush()

		fmt.Fprintf(b.out, "Built %s successfully\n", b.outImage)
		return nil
	}

	if b.image != "" {
		b.saveContainer = true
		fmt.Fprintf(b.out, "Successfully built %s\n", utils.TruncateID(b.container.ID))
		return nil
	}

	return fmt.Errorf("An error occured during the build\n")
}

func (b *buildFile) BuildTar(tar string) error {
	defer b.cleanup()

	b.context = "/"

	if err := b.CmdFrom(""); err != nil {
		return err
	}

	if err := b.CmdAdd(tar + " /"); err != nil {
		return err
	}

	if err := b.container.ToDisk(); err != nil {
		return err
	}

	if b.outImage != "" {
		img, err := b.container.Commit("", "", nil, b.squash, true)

		if err != nil {
			return err
		}

		ts, err := env.DefaultTagStore()

		if err != nil {
			return err
		}

		repo, tag := env.ParseRepositoryTag(b.outImage)

		ts.Add(repo, tag, img.ID)
		ts.Flush()

		fmt.Fprintf(b.out, "Built %s successfully\n", b.outImage)
		return nil
	}

	b.saveContainer = true

	fmt.Fprintf(b.out, "Successfully built %s\n", utils.TruncateID(b.container.ID))
	return nil
}

var flImage *string
var flTar *bool
var flSquash *bool
var flExperiment *bool

func init() {
	cmd := addCommand("build", "[OPTIONS] <dir|tar>", "Build a container or image from a Dockerfile", 1, build)

	flTar = cmd.Bool("t", false, "Create an image from a tar.gz or directory")

	flImage = cmd.String("i", "", "Image repo[:tag] to save the output as")

	flSquash = cmd.Bool("s", false, "Make a squashfs image")

	flExperiment = cmd.Bool("x", false, "Start a shell to experiment with the built image")
}

func build(cmd *flag.FlagSet) {
	ts, err := env.DefaultTagStore()

	if err != nil {
		panic(err)
	}

	abort := make(chan os.Signal, 1)

	signal.Notify(abort, syscall.SIGINT)

	b := &buildFile{
		tags:       ts,
		config:     &env.Config{},
		out:        os.Stdout,
		verbose:    true,
		outImage:   *flImage,
		abort:      abort,
		squash:     *flSquash,
		experiment: *flExperiment,
	}

	if *flTar {
		err = b.BuildTar(cmd.Arg(0))
	} else {
		err = b.Build(cmd.Arg(0))
	}

	if err != nil && err != ErrAbort {
		panic(err)
	}
}
