package env

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/arch-reactor/container/utils"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var sysInitPath string

func init() {
	sysInitPath = SelfPath()
}

// Figure out the absolute path of our own binary
func SelfPath() string {
	path, err := exec.LookPath(os.Args[0])
	if err != nil {
		panic(err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return path
}

// Represents
type Container struct {
	root string

	ID string

	Created time.Time

	Path string
	Args []string

	Config *Config
	State  State
	Image  string
	imageO *Image

	network         *NetworkInterface
	NetworkSettings *NetworkSettings

	SysInitPath    string
	ResolvConfPath string

	cmd       *exec.Cmd
	stdout    *WriteBroadcaster
	stderr    *WriteBroadcaster
	stdin     io.ReadCloser
	stdinPipe io.WriteCloser
	ptyMaster io.Closer

	waitLock chan struct{}
	Volumes  map[string]string
	// Store rw/ro in a separate structure to preserve reverse-compatibility on-disk.
	// Easier than migrating older container configs :)
	VolumesRW map[string]bool

	networkManager *NetworkManager
}

type HostConfig struct {
	Binds           []string
	ContainerIDFile string
	Save            bool
	Quiet           bool
}

type BindMap struct {
	SrcPath string
	DstPath string
	Mode    string
}

type Capabilities struct {
	MemoryLimit    bool
	SwapLimit      bool
	IPv4Forwarding bool
}

func ContainerCreate(r *TagStore, config *Config) (*Container, error) {
	if config.Memory != 0 && config.Memory < 524288 {
		return nil, fmt.Errorf("Memory limit must be given in bytes (minimum 524288 bytes)")
	}

	var err error
	var img *Image

	if config.Image == "" {
		img = &Image{}
	} else {
		// Lookup image
		img, err = r.LookupImage(config.Image)
		if err != nil {
			return nil, err
		}

		if img.Config != nil {
			MergeConfig(config, img.Config)
		}
	}

	if len(config.Entrypoint) != 0 && config.Cmd == nil {
		config.Cmd = []string{}
	} else if config.Cmd == nil || len(config.Cmd) == 0 {
		return nil, fmt.Errorf("No command specified")
	}

	// Generate id
	id := utils.GenerateID()

	// Generate default hostname
	// FIXME: the lxc template no longer needs to set a default hostname
	if config.Hostname == "" {
		config.Hostname = id[:12]
	}

	var args []string
	var entrypoint string

	if len(config.Entrypoint) != 0 {
		entrypoint = config.Entrypoint[0]
		args = append(config.Entrypoint[1:], config.Cmd...)
	} else {
		entrypoint = config.Cmd[0]
		args = config.Cmd[1:]
	}

	container := &Container{
		ID:              id,
		Created:         time.Now(),
		Path:            entrypoint,
		Args:            args, //FIXME: de-duplicate from config
		Config:          config,
		Image:           img.ID, // Always use the resolved image id
		imageO:          img,
		NetworkSettings: &NetworkSettings{},
		// FIXME: do we need to store this in the container?
		SysInitPath: sysInitPath,
	}

	container.root = path.Join(DIR, "containers", container.ID)

	// Step 1: create the container directory.
	// This doubles as a barrier to avoid race conditions.
	if err := os.Mkdir(container.root, 0700); err != nil {
		return nil, err
	}

	// resolvConf, err := GetResolvConf()
	// if err != nil {
	// return nil, err
	// }

	// If custom dns exists, then create a resolv.conf for the container
	if len(config.Dns) > 0 {
		var dns []string
		if len(config.Dns) > 0 {
			dns = config.Dns
		}

		container.ResolvConfPath = path.Join(container.root, "resolv.conf")

		f, err := os.Create(container.ResolvConfPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		for _, dns := range dns {
			if _, err := f.Write([]byte("nameserver " + dns + "\n")); err != nil {
				return nil, err
			}
		}
	} else {
		container.ResolvConfPath = "/etc/resolv.conf"
	}

	// Step 2: save the container json
	if err := container.ToDisk(); err != nil {
		return nil, err
	}

	return container, nil
}

func (container *Container) PathTo(fileName string) string {
	return path.Join(container.root, fileName)
}

func (container *Container) Commit(comment, author string, config *Config, squash bool) (*Image, error) {

	if config == nil {
		config = container.Config
	} else {
		MergeConfig(config, container.Config)
	}

	img := &Image{
		ID:              utils.GenerateID(),
		Parent:          container.Image,
		Comment:         comment,
		Created:         time.Now(),
		ContainerConfig: *container.Config,
		DockerVersion:   "0.5.2",
		Author:          author,
		Config:          config,
		Architecture:    "x86_64",
	}

	root := path.Join(DIR, "graph", "_armktmp")

	os.MkdirAll(root, 0755)

	layerPath := path.Join(root, "layer")

	os.MkdirAll(layerPath, 0700)

	if squash {
		layerFs := path.Join(root, "layer.fs")

		utils.Run("mksquashfs", container.rwPath(), layerFs, "-comp", "xz")
	} else {
		tarbz2 := path.Join(root, "layer.tar.bz2")

		utils.Run("tar", "--numeric-owner", "-cjf", tarbz2, "-C", container.rwPath(), ".")

		utils.Run("tar", "-xjvf", tarbz2, "-C", layerPath)
	}

	jsonData, err := json.Marshal(img)

	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(path.Join(root, "json"), jsonData, 0644)

	if err != nil {
		panic(err)
	}

	err = os.Rename(root, path.Join(DIR, "graph", img.ID))

	if err != nil {
		panic(err)
	}

	return img, nil
}

func (container *Container) logPath(name string) string {
	return path.Join(container.root, fmt.Sprintf("%s-%s.log", container.ID, name))
}

func (container *Container) hostConfigPath() string {
	return path.Join(container.root, "hostconfig.json")
}

func (container *Container) jsonPath() string {
	return path.Join(container.root, "config.json")
}

func (container *Container) lxcConfigPath() string {
	return path.Join(container.root, "config.lxc")
}

// This method must be exported to be used from the lxc template
func (container *Container) RootfsPath() string {
	return path.Join(container.root, "rootfs")
}

func (container *Container) rwPath() string {
	return path.Join(container.root, "rw")
}

func (container *Container) ToDisk() (err error) {
	data, err := json.Marshal(container)
	if err != nil {
		return
	}
	return ioutil.WriteFile(container.jsonPath(), data, 0666)
}

func LoadContainer(dir, id string) (*Container, error) {
	root := path.Join(dir, "containers", id)

	cont := &Container{root: root}

	data, err := ioutil.ReadFile(path.Join(root, "config.json"))

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &cont)

	if err != nil {
		return nil, err
	}

	return cont, nil
}

func (container *Container) generateLXCConfig() error {
	fo, err := os.Create(container.lxcConfigPath())
	if err != nil {
		return err
	}
	defer fo.Close()
	if err := LxcTemplateCompiled.Execute(fo, container); err != nil {
		return err
	}
	return nil
}

func (container *Container) ReadHostConfig() (*HostConfig, error) {
	data, err := ioutil.ReadFile(container.hostConfigPath())
	if err != nil {
		return &HostConfig{}, err
	}

	hostConfig := &HostConfig{}
	if err := json.Unmarshal(data, hostConfig); err != nil {
		return &HostConfig{}, err
	}

	return hostConfig, nil
}

func (container *Container) EnsureMounted() error {
	if mounted, err := container.Mounted(); err != nil {
		return err
	} else if mounted {
		return nil
	}
	return container.Mount()
}

func (container *Container) Mount() error {
	image, err := container.GetImage()
	if err != nil {
		return err
	}
	return image.Mount(container.RootfsPath(), container.rwPath())
}

func Unmount(target string) error {
	_, err := os.Stat(target)

	if err != nil {
		return err
	}

	if err := exec.Command("auplink", target, "flush").Run(); err != nil {
		utils.Debugf("[warning]: couldn't run auplink before unmount: %s", err)
	}

	if err := syscall.Unmount(target, 0); err != nil {
		return err
	}
	// Even though we just unmounted the filesystem, AUFS will prevent deleting the mntpoint
	// for some time. We'll just keep retrying until it succeeds.
	for retries := 0; retries < 1000; retries++ {
		err := os.Remove(target)
		if err == nil {
			// rm mntpoint succeeded
			return nil
		}
		if os.IsNotExist(err) {
			// mntpoint doesn't exist anymore. Success.
			return nil
		}
		// fmt.Printf("(%v) Remove %v returned: %v\n", retries, target, err)
		time.Sleep(10 * time.Millisecond)
	}

	return fmt.Errorf("Umount: Failed to umount %v", target)
}

func Mounted(mountpoint string) (bool, error) {
	mntpoint, err := os.Stat(mountpoint)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	parent, err := os.Stat(filepath.Join(mountpoint, ".."))
	if err != nil {
		return false, err
	}
	mntpointSt := mntpoint.Sys().(*syscall.Stat_t)
	parentSt := parent.Sys().(*syscall.Stat_t)
	return mntpointSt.Dev != parentSt.Dev, nil
}

func (container *Container) Mounted() (bool, error) {
	return Mounted(container.RootfsPath())
}

func (container *Container) GetImage() (*Image, error) {
	return container.imageO, nil
}

const defaultHosts = `127.0.0.1	localhost
::1		localhost ip6-localhost ip6-loopback
fe00::0		ip6-localnet
ff00::0		ip6-mcastprefix
ff02::1		ip6-allnodes
ff02::2		ip6-allrouters`

func (container *Container) allocateNetwork() error {
	if container.Config.NetworkDisabled {
		return nil
	}

	nm, err := newNetworkManager(DefaultNetworkBridge)

	if err != nil {
		return err
	}

	container.networkManager = nm

	iface, err := container.networkManager.Allocate()
	if err != nil {
		return err
	}
	container.NetworkSettings.PortMapping = make(map[string]PortMapping)
	container.NetworkSettings.PortMapping["Tcp"] = make(PortMapping)
	container.NetworkSettings.PortMapping["Udp"] = make(PortMapping)
	for _, spec := range container.Config.PortSpecs {
		nat, err := iface.AllocatePort(spec)
		if err != nil {
			iface.Release()
			return err
		}
		proto := strings.Title(nat.Proto)
		backend, frontend := strconv.Itoa(nat.Backend), strconv.Itoa(nat.Frontend)
		container.NetworkSettings.PortMapping[proto][backend] = frontend
	}
	container.network = iface
	container.NetworkSettings.Bridge = container.networkManager.bridgeIface
	container.NetworkSettings.IPAddress = iface.IPNet.IP.String()
	container.NetworkSettings.IPPrefixLen, _ = iface.IPNet.Mask.Size()
	container.NetworkSettings.Gateway = iface.Gateway.String()
	return nil
}

func (container *Container) cleanup() {
	if !container.State.Running {
		container.Unmount()
		container.network.Release()
		container.setStopped(0)
	}
}

func (container *Container) Start(hostConfig *HostConfig) error {
	defer container.cleanup()

	if len(hostConfig.Binds) == 0 {
		hostConfig, _ = container.ReadHostConfig()
	}

	if container.State.Running {
		return fmt.Errorf("The container %s is already running.", container.ID)
	}

	if err := container.EnsureMounted(); err != nil {
		return err
	}

	if err := container.allocateNetwork(); err != nil {
		return err
	}

	// Create the requested bind mounts
	binds := make(map[string]BindMap)
	// Define illegal container destinations
	illegalDsts := []string{"/", "."}

	for _, bind := range hostConfig.Binds {
		// FIXME: factorize bind parsing in parseBind
		var src, dst, mode string
		arr := strings.Split(bind, ":")
		if len(arr) == 2 {
			src = arr[0]
			dst = arr[1]
			mode = "rw"
		} else if len(arr) == 3 {
			src = arr[0]
			dst = arr[1]
			mode = arr[2]
		} else {
			return fmt.Errorf("Invalid bind specification: %s", bind)
		}

		// Bail if trying to mount to an illegal destination
		for _, illegal := range illegalDsts {
			if dst == illegal {
				return fmt.Errorf("Illegal bind destination: %s", dst)
			}
		}

		if dst[0:1] == "@" {
			continue
		}

		bindMap := BindMap{
			SrcPath: src,
			DstPath: dst,
			Mode:    mode,
		}
		binds[path.Clean(dst)] = bindMap
	}

	// FIXME: evaluate volumes-from before individual volumes, so that the latter can override the former.
	// Create the requested volumes volumes
	if container.Volumes == nil || len(container.Volumes) == 0 {
		container.Volumes = make(map[string]string)
		container.VolumesRW = make(map[string]bool)

		for volPath := range container.Config.Volumes {
			volPath = path.Clean(volPath)
			// If an external bind is defined for this volume, use that as a source
			if bindMap, exists := binds[volPath]; exists {
				container.Volumes[volPath] = bindMap.SrcPath
				if strings.ToLower(bindMap.Mode) == "rw" {
					container.VolumesRW[volPath] = true
				}
				// Otherwise create an directory in $ROOT/volumes/ and use that
			} else {
				n := strings.LastIndex(volPath, ":@")
				if n < 0 {
					return fmt.Errorf("Invalid volume configuration: %s", volPath)
				} else {
					volName := path.Join(DIR, "volumes", volPath[n+2:])
					volPath = volPath[0:n]

					os.MkdirAll(volName, 0755)

					container.Volumes[volPath] = volName
					container.VolumesRW[volPath] = true
				}
			}

			// Create the mountpoint
			if err := os.MkdirAll(path.Join(container.RootfsPath(), volPath), 0755); err != nil {
				return nil
			}
		}
	}

	if err := container.generateLXCConfig(); err != nil {
		return err
	}

	// Update /etc/hosts in the container to have an etc/hosts entry
	// for itself.
	hosts := defaultHosts + "\n127.0.0.1\t" + container.Config.Hostname + "\n"
	os.MkdirAll(path.Join(container.rwPath(), "etc"), 0755)
	err := ioutil.WriteFile(path.Join(container.rwPath(), "etc/hosts"), []byte(hosts), 0644)

	if err != nil {
		fmt.Printf("error writing hosts file: %s\n", err)
	}

	params := []string{
		"-n", container.ID,
		"-f", container.lxcConfigPath(),
		"--",
		"/.dockerinit",
	}

	// Networking
	if !container.Config.NetworkDisabled {
		params = append(params, "-g", container.network.Gateway.String())
	}

	// User
	if container.Config.User != "" {
		params = append(params, "-u", container.Config.User)
	}

	if container.Config.Tty {
		params = append(params, "-e", "TERM=xterm")
	}

	// Setup environment
	params = append(params,
		"-e", "HOME=/",
		"-e", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"-e", "container=lxc",
		"-e", "HOSTNAME="+container.Config.Hostname,
	)

	for _, elem := range container.Config.Env {
		params = append(params, "-e", elem)
	}

	// Program
	params = append(params, "--", container.Path)
	params = append(params, container.Args...)

	container.cmd = exec.Command("lxc-start", params...)

	container.cmd.Stdout = os.Stdout
	container.cmd.Stderr = os.Stderr
	container.cmd.Stdin = os.Stdin
	container.cmd.Start()

	// FIXME: save state on disk *first*, then converge
	// this way disk state is used as a journal, eg. we can restore after crash etc.
	container.setRunning()

	// Init the lock
	container.waitLock = make(chan struct{})

	container.ToDisk()
	container.SaveHostConfig(hostConfig)

	return nil
}

func (container *Container) Signal(sig os.Signal) {
	container.cmd.Process.Signal(sig)
}

// Returns a pointer to the first net.Addr on eth0, if it exists. Otherwise nil.
func hostAddr() (string, error) {
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if "eth0" == iface.Name {
				if addrs, err := iface.Addrs(); err == nil {
					if len(addrs) > 0 {
						addr := addrs[0].String() // looks like 10.0.0.1/24
						parts := strings.Split(addr, "/")
						return parts[0], nil
					}
				} else {
					return "", err
				}
			}
		}
	} else {
		return "", err
	}
	return "", errors.New("Unable to find address for eth0")
}

func writeAsJson(obj interface{}, fileName string) {
	if data, err := json.Marshal(obj); err == nil {
		ioutil.WriteFile(fileName, data, 0644)
	} else {
		panic(err)
	}
}

func (c *Container) setRunning() {
	c.State.setRunning(c.cmd.Process.Pid)
	// Advertise presence only after we've set running locally
	pidStr := strconv.Itoa(c.cmd.Process.Pid)

	processDir := path.Join(RUN_DIR, pidStr)
	os.Mkdir(processDir, 0755)

	// Write port descriptions
	writeAsJson(c.NetworkSettings.PortMapping, path.Join(processDir, "ports"))
	if addr, err := hostAddr(); err == nil {
		ioutil.WriteFile(path.Join(processDir, "ip"), []byte(addr), 0644)
	} else {
		panic(err)
	}
	writeAsJson(c.Config.ServiceSpecs, path.Join(processDir, "services"))

	// There's a race condition between creating the containing directory and it being initialized
	// so we have a separate dir where we mark containers as ready to go. Watches should be applied there.
	os.Create(path.Join(INIT_DIR, pidStr))
}

func (c *Container) Kill() {
	c.cmd.Process.Kill()
}

func (c *Container) Wait(cfg *HostConfig) {
	pid := fmt.Sprintf("%d\n", c.cmd.Process.Pid)
	ioutil.WriteFile(path.Join(c.root, "running"), []byte(pid), 0644)

	c.cmd.Wait()
	c.Unmount()

	c.network.Release()

	c.setStopped(0)

	if cfg.Save {
		if !cfg.Quiet {
			fmt.Printf("== Saved: %s\n", c.ID)
		}
		os.RemoveAll(path.Join(c.root, "running"))
	} else {
		os.RemoveAll(path.Join(DIR, "containers", c.ID))
	}
}

func (c *Container) setStopped(exitCode int) {
	pid := strconv.Itoa(c.State.Pid)
	// Nuke fs-level presence information
	os.RemoveAll(path.Join(RUN_DIR, pid))
	os.RemoveAll(path.Join(INIT_DIR, pid))
	c.State.setStopped(exitCode)
}

func (c *Container) Remove() {
	os.RemoveAll(c.root)
}

// Inject the io.Reader at the given path. Note: do not close the reader
func (container *Container) Inject(file io.Reader, pth string) error {
	// Make sure the directory exists
	if err := os.MkdirAll(path.Join(container.rwPath(), path.Dir(pth)), 0755); err != nil {
		return err
	}
	// FIXME: Handle permissions/already existing dest
	dest, err := os.Create(path.Join(container.rwPath(), pth))
	if err != nil {
		return err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		return err
	}

	return nil
}

func (container *Container) Unmount() error {
	return Unmount(container.RootfsPath())
}

func (container *Container) SaveHostConfig(hostConfig *HostConfig) (err error) {
	data, err := json.Marshal(hostConfig)
	if err != nil {
		return
	}
	return ioutil.WriteFile(container.hostConfigPath(), data, 0666)
}
