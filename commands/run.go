package commands

import (
	"flag"
	"fmt"
	"github.com/arch-reactor/container/env"
	"github.com/arch-reactor/container/utils"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var forwardSignals = []os.Signal{
	syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM,
	syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGUSR2,
	syscall.SIGWINCH, syscall.SIGTTIN, syscall.SIGTTOU,
}

var flUser *string
var flMemory *int64
var flContainerIDFile *string
var flNetwork *bool
var flCpuShares *int64
var flPorts utils.ListOpts
var flEnv utils.ListOpts
var flDns utils.ListOpts
var flVolumes utils.PathOpts
var flSave *bool
var flEntrypoint *string
var flEnvDir *string
var flTool *bool

func init() {
	cmd := addCommand("run", "[OPTIONS] <image> [<command>] [<args>...]", "Run a command in a new container", 1, runContainer)

	flUser = cmd.String("u", "", "Username or UID")
	flMemory = cmd.Int64("m", 0, "Memory limit (in bytes)")
	flContainerIDFile = cmd.String("cidfile", "", "Write the container ID to the file")
	flNetwork = cmd.Bool("n", true, "Enable networking for this container")

	flCpuShares = cmd.Int64("c", 0, "CPU shares (relative weight)")

	cmd.Var(&flPorts, "p", "Expose a container's port to the host (use 'docker port' to see the actual mapping)")

	cmd.Var(&flEnv, "e", "Set environment variables")

	flEnvDir = cmd.String("envdir", "", "Load environment variables from an envdir")

	cmd.Var(&flDns, "dns", "Set custom dns servers")

	flVolumes = utils.NewPathOpts()
	cmd.Var(flVolumes, "v", "Bind mount a volume (e.g. from the host: -v /host:/container, from docker: -v /container)")

	flEntrypoint = cmd.String("entrypoint", "", "Overwrite the default entrypoint of the image")

	flSave = cmd.Bool("save", false, "Save the container when it exits")

	flTool = cmd.Bool("t", false, "Run a provided tool")
}

func runContainer(cmd *flag.FlagSet) {
	capa := &env.Capabilities{}

	config, hostcfg, err := ParseRun(cmd, capa)

	if config == nil {
		return
	}

	if config.Image == "" {
		cmd.Usage()
		return
	}

	if err != nil {
		panic(err)
	}

	tags, err := env.DefaultTagStore()

	if err != nil {
		panic(err)
	}

	container, err := env.ContainerCreate(tags, config)

	if err != nil {
		fmt.Printf("Unable to create container: %s\n", err)
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, forwardSignals...)

	go func() {
		sig := <-c
		container.Signal(sig)
	}()

	err = container.Start(hostcfg)

	if err != nil {
		container.Remove()
		fmt.Printf("Unable to start container: %s\n", err)
		os.Exit(1)
	}

	container.Wait(hostcfg)
}

func ParseRun(cmd *flag.FlagSet, capabilities *env.Capabilities) (*env.Config, *env.HostConfig, error) {
	if capabilities != nil && *flMemory > 0 && !capabilities.MemoryLimit {
		//fmt.Fprintf(stdout, "WARNING: Your kernel does not support memory limit capabilities. Limitation discarded.\n")
		*flMemory = 0
	}

	var binds []string

	// add any bind targets to the list of container volumes
	for bind := range flVolumes {
		arr := strings.Split(bind, ":")
		if len(arr) > 1 && arr[1][0:1] != "@" {
			dstDir := arr[1]
			flVolumes[dstDir] = struct{}{}
			binds = append(binds, bind)
			delete(flVolumes, bind)
		}
	}

	parsedArgs := cmd.Args()
	runCmd := []string{}
	entrypoint := []string{}
	image := ""

	if len(parsedArgs) >= 1 {
		image = cmd.Arg(0)
	}

	if len(parsedArgs) > 1 {
		runCmd = parsedArgs[1:]
	}

	if *flEntrypoint != "" {
		entrypoint = []string{*flEntrypoint}
	}

	if *flTool {
		if len(runCmd) == 0 {
			return nil, nil, fmt.Errorf("Specify a tool to run")
		}

		tool := runCmd[0]

		entrypoint = []string{"/tool/" + tool}

		runCmd = runCmd[1:]
	}

	config := &env.Config{
		Hostname:        "",
		PortSpecs:       flPorts,
		User:            *flUser,
		Tty:             true,
		NetworkDisabled: !*flNetwork,
		OpenStdin:       true,
		Memory:          *flMemory,
		CpuShares:       *flCpuShares,
		AttachStdin:     true,
		AttachStdout:    true,
		AttachStderr:    true,
		Env:             flEnv,
		Cmd:             runCmd,
		Dns:             flDns,
		Image:           image,
		Volumes:         flVolumes,
		VolumesFrom:     "",
		Entrypoint:      entrypoint,
	}

	hostConfig := &env.HostConfig{
		Binds:           binds,
		ContainerIDFile: *flContainerIDFile,
		Save:            *flSave,
		EnvDir:          *flEnvDir,
	}

	if capabilities != nil && *flMemory > 0 && !capabilities.SwapLimit {
		//fmt.Fprintf(stdout, "WARNING: Your kernel does not support swap limit capabilities. Limitation discarded.\n")
		config.MemorySwap = -1
	}

	// When allocating stdin in attached mode, close stdin at client disconnect
	if config.OpenStdin && config.AttachStdin {
		config.StdinOnce = true
	}

	return config, hostConfig, nil
}
