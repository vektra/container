package commands

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/vektra/components/app"
	"github.com/vektra/container/env"
)

var forwardSignals = []os.Signal{
	syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM,
	syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGUSR2,
	syscall.SIGWINCH, syscall.SIGTTIN, syscall.SIGTTOU,
	os.Interrupt,
}

type runOptions struct {
	User       string   `short:"u" description:"Username or UID"`
	Memory     int64    `short:"m" description:"Memory limit (in bytes)"`
	CIDFile    string   `long:"cidfile" description:"Write the container ID to a file"`
	Network    bool     `short:"n" description:"Enable networking" default:"true"`
	CPU        int64    `short:"c" description:"CPU shares (relative weight)"`
	Ports      []string `short:"p" description:"Export a port"`
	Env        []string `short:"e" description:"Set environment variables"`
	EnvDir     string   `long:"envdir" description:"Load env vars from an envdir"`
	DNS        []string `long:"dns" description:"Set custom dns servers"`
	Volumes    []string `short:"v" description:"Bind mount volumes"`
	Save       bool     `long:"save" description:"Save the container when it exits"`
	EntryPoint string   `long:"entrypoint" description:"Set the default entrypoint"`
	Hook       string   `long:"hook" description:"Execute this command once the container is booted"`
	Tool       bool     `short:"t" description"Run a provided tool"`
}

func init() {
	app.AddCommand("run", "Run a command in a new container", "", &runOptions{})
}

func (ro *runOptions) Usage() string {
	return "[OPTIONS] <image> [<command>] [<args>...]"
}

func (ro *runOptions) Execute(args []string) error {
	capa := &env.Capabilities{}

	config, hostcfg, err := ParseRun(ro, args, capa)
	if err != nil {
		return err
	}

	if config == nil {
		return nil
	}

	if config.Image == "" {
		app.ShowHelp()
		return nil
	}

	tags, err := env.DefaultTagStore()

	if err != nil {
		return err
	}

	container, err := env.ContainerCreate(tags, config)

	if err != nil {
		return fmt.Errorf("Unable to create container: %s\n", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, forwardSignals...)

	go func() {
		for {
			sig := <-c
			fmt.Printf("Got signal!\n")
			container.Signal(sig)
			fmt.Printf("Done with signal!\n")
		}
	}()

	err = container.Start(hostcfg)

	if err != nil {
		container.Remove()
		return fmt.Errorf("Unable to start container: %s\n", err)
	}

	container.Wait(hostcfg)

	return nil
}

func ParseRun(ro *runOptions, args []string, capabilities *env.Capabilities) (*env.Config, *env.HostConfig, error) {
	if capabilities != nil && ro.Memory > 0 && !capabilities.MemoryLimit {
		//fmt.Fprintf(stdout, "WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.\n")
		ro.Memory = 0
	}

	var binds []string
	volumes := map[string]struct{}{}

	// add any bind targets to the list of container volumes
	for _, bind := range ro.Volumes {
		arr := strings.Split(bind, ":")
		if len(arr) > 1 && arr[1][0:1] != "@" {
			dstDir := arr[1]
			volumes[dstDir] = struct{}{}
			binds = append(binds, bind)
		} else {
			volumes[bind] = struct{}{}
		}
	}

	parsedArgs := args
	runCmd := []string{}
	entrypoint := []string{}
	image := ""

	if len(parsedArgs) >= 1 {
		image = parsedArgs[0]
	}

	if len(parsedArgs) > 1 {
		runCmd = parsedArgs[1:]
	}

	if ro.EntryPoint != "" {
		entrypoint = []string{ro.EntryPoint}
	}

	if ro.Tool {
		if len(runCmd) == 0 {
			return nil, nil, fmt.Errorf("Specify a tool to run")
		}

		tool := runCmd[0]

		entrypoint = []string{"/tool/" + tool}

		runCmd = runCmd[1:]
	}

	config := &env.Config{
		Hostname:        "",
		PortSpecs:       ro.Ports,
		User:            ro.User,
		Tty:             true,
		NetworkDisabled: !ro.Network,
		OpenStdin:       true,
		Memory:          ro.Memory,
		CpuShares:       ro.CPU,
		AttachStdin:     true,
		AttachStdout:    true,
		AttachStderr:    true,
		Env:             ro.Env,
		Cmd:             runCmd,
		Dns:             ro.DNS,
		Image:           image,
		Volumes:         volumes,
		VolumesFrom:     "",
		Entrypoint:      entrypoint,
	}

	hostConfig := &env.HostConfig{
		Binds:           binds,
		ContainerIDFile: ro.CIDFile,
		Save:            ro.Save,
		EnvDir:          ro.EnvDir,
		Hook:            ro.Hook,
	}

	if capabilities != nil && ro.Memory > 0 && !capabilities.SwapLimit {
		//fmt.Fprintf(stdout, "WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		config.MemorySwap = -1
	}

	// When allocating stdin in attached mode, close stdin at client disconnect
	if config.OpenStdin && config.AttachStdin {
		config.StdinOnce = true
	}

	return config, hostConfig, nil
}
