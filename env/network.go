package env

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/vektra/container/utils"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
)

const (
	DefaultNetworkBridge = "golden0"
	DisableNetworkBridge = "none"
	portRangeStart       = 49153
	portRangeEnd         = 65535
)

type PortMapping map[string]string

type NetworkSettings struct {
	IPAddress   string
	IPPrefixLen int
	Gateway     string
	Gateway6    string
	Bridge      string
	PortMapping map[string]PortMapping
}

var NetworkBridgeIface string = DefaultNetworkBridge

// Calculates the first and last IP addresses in an IPNet
func networkRange(network *net.IPNet) (net.IP, net.IP) {
	netIP := network.IP.To4()
	firstIP := netIP.Mask(network.Mask)
	lastIP := net.IPv4(0, 0, 0, 0).To4()
	for i := 0; i < len(lastIP); i++ {
		lastIP[i] = netIP[i] | ^network.Mask[i]
	}
	return firstIP, lastIP
}

// Detects overlap between one IPNet and another
func networkOverlaps(netX *net.IPNet, netY *net.IPNet) bool {
	firstIP, _ := networkRange(netX)
	if netY.Contains(firstIP) {
		return true
	}
	firstIP, _ = networkRange(netY)
	if netX.Contains(firstIP) {
		return true
	}
	return false
}

// Converts a 4 bytes IP into a 32 bit integer
func ipToInt(ip net.IP) int32 {
	return int32(binary.BigEndian.Uint32(ip.To4()))
}

// Converts 32 bit integer into a 4 bytes IP address
func intToIP(n int32) net.IP {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(n))
	return net.IP(b)
}

// Given a netmask, calculates the number of available hosts
func networkSize(mask net.IPMask) int32 {
	m := net.IPv4Mask(0, 0, 0, 0)
	for i := 0; i < net.IPv4len; i++ {
		m[i] = ^mask[i]
	}

	return int32(binary.BigEndian.Uint32(m)) + 1
}

//Wrapper around the ip command
func ip(args ...string) (string, error) {
	path, err := exec.LookPath("ip")
	if err != nil {
		return "", fmt.Errorf("command not found: ip")
	}
	output, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ip failed: ip %v", strings.Join(args, " "))
	}
	return string(output), nil
}

// Wrapper around the iptables command
func iptables(args ...string) error {
	path, err := exec.LookPath("iptables")
	if err != nil {
		return fmt.Errorf("command not found: iptables")
	}
	if err := exec.Command(path, args...).Run(); err != nil {
		return fmt.Errorf("iptables failed: iptables %v", strings.Join(args, " "))
	}
	return nil
}

func checkRouteOverlaps(routes string, dockerNetwork *net.IPNet) error {
	utils.Debugf("Routes:\n\n%s", routes)
	for _, line := range strings.Split(routes, "\n") {
		if strings.Trim(line, "\r\n\t ") == "" || strings.Contains(line, "default") {
			continue
		}
		_, network, err := net.ParseCIDR(strings.Split(line, " ")[0])
		if err != nil {
			// is this a mask-less IP address?
			if ip := net.ParseIP(strings.Split(line, " ")[0]); ip == nil {
				// fail only if it's neither a network nor a mask-less IP address
				return fmt.Errorf("Unexpected ip route output: %s (%s)", err, line)
			} else {
				_, network, err = net.ParseCIDR(ip.String() + "/32")
				if err != nil {
					return err
				}
			}
		}
		if err == nil && network != nil {
			if networkOverlaps(dockerNetwork, network) {
				return fmt.Errorf("Network %s is already routed: '%s'", dockerNetwork, line)
			}
		}
	}
	return nil
}

// CreateBridgeIface creates a network bridge interface on the host system with the name `ifaceName`,
// and attempts to configure it with an address which doesn't conflict with any other interface on the host.
// If it can't find an address which doesn't conflict, it will return an error.
func CreateBridgeIface(ifaceName string) error {
	addrs := []string{
		// Here we don't follow the convention of using the 1st IP of the range for the gateway.
		// This is to use the same gateway IPs as the /24 ranges, which predate the /16 ranges.
		// In theory this shouldn't matter - in practice there's bound to be a few scripts relying
		// on the internal addressing or other stupid things like that.
		// The shouldn't, but hey, let's not break them unless we really have to.
		"172.17.42.1/16", // Don't use 172.16.0.0/16, it conflicts with EC2 DNS 172.16.0.23
		"10.0.42.1/16",   // Don't even try using the entire /8, that's too intrusive
		"10.1.42.1/16",
		"10.42.42.1/16",
		"172.16.42.1/24",
		"172.16.43.1/24",
		"172.16.44.1/24",
		"10.0.42.1/24",
		"10.0.43.1/24",
		"192.168.42.1/24",
		"192.168.43.1/24",
		"192.168.44.1/24",
	}

	var ifaceAddr string
	for _, addr := range addrs {
		_, dockerNetwork, err := net.ParseCIDR(addr)
		if err != nil {
			return err
		}
		routes, err := ip("route")
		if err != nil {
			return err
		}
		if err := checkRouteOverlaps(routes, dockerNetwork); err == nil {
			ifaceAddr = addr
			break
		} else {
			utils.Debugf("%s: %s", addr, err)
		}
	}
	if ifaceAddr == "" {
		return fmt.Errorf("Could not find a free IP address range for interface '%s'. Please configure its address manually and run 'docker -b %s'", ifaceName, ifaceName)
	}
	utils.Debugf("Creating bridge %s with network %s", ifaceName, ifaceAddr)

	if output, err := ip("link", "add", ifaceName, "type", "bridge"); err != nil {
		return fmt.Errorf("Error creating bridge: %s (output: %s)", err, output)
	}

	if output, err := ip("addr", "add", ifaceAddr, "dev", ifaceName); err != nil {
		return fmt.Errorf("Unable to add private network: %s (%s)", err, output)
	}
	if output, err := ip("link", "set", ifaceName, "up"); err != nil {
		return fmt.Errorf("Unable to start network bridge: %s (%s)", err, output)
	}
	if err := iptables("-t", "nat", "-A", "POSTROUTING", "-s", ifaceAddr,
		"!", "-d", ifaceAddr, "-j", "MASQUERADE"); err != nil {
		return fmt.Errorf("Unable to enable network bridge NAT: %s", err)
	}
	return nil
}

// Return the IPv4 address of a network interface
func getIfaceAddr(name string) (net.Addr, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	var addrs4 []net.Addr
	for _, addr := range addrs {
		ip := (addr.(*net.IPNet)).IP
		if ip4 := ip.To4(); len(ip4) == net.IPv4len {
			addrs4 = append(addrs4, addr)
		}
	}
	switch {
	case len(addrs4) == 0:
		return nil, fmt.Errorf("Interface %v has no IP addresses", name)
	case len(addrs4) > 1:
		fmt.Printf("Interface %v has more than 1 IPv4 address. Defaulting to using %v\n",
			name, (addrs4[0].(*net.IPNet)).IP)
	}
	return addrs4[0], nil
}

func getIfaceAddr6(name string) (net.Addr, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	var addrs6 []net.Addr
	for _, addr := range addrs {
		ip := (addr.(*net.IPNet)).IP
		if ip4 := ip.To4(); ip4 == nil {
			addrs6 = append(addrs6, addr)
		}
	}

	switch {
	case len(addrs6) == 0:
		return nil, fmt.Errorf("Interface %v has no IP addresses", name)
	case len(addrs6) > 1:
		fmt.Printf("Interface %v has more than 1 IPv6 address. Defaulting to using %v\n",
			name, (addrs6[0].(*net.IPNet)).IP)
	}
	return addrs6[0], nil
}

// Port mapper takes care of mapping external ports to containers by setting
// up iptables rules.
// It keeps track of all mappings and is able to unmap at will
type PortMapper struct {
	tcpMapping map[int]*net.TCPAddr
	tcpProxies map[int]Proxy
	udpMapping map[int]*net.UDPAddr
	udpProxies map[int]Proxy
}

func (mapper *PortMapper) cleanup() error {
	// Ignore errors - This could mean the chains were never set up
	iptables("-t", "nat", "-D", "PREROUTING", "-m", "addrtype", "--dst-type", "LOCAL", "-j", "AR")
	iptables("-t", "nat", "-D", "OUTPUT", "-m", "addrtype", "--dst-type", "LOCAL", "!", "--dst", "127.0.0.0/8", "-j", "AR")
	iptables("-t", "nat", "-D", "OUTPUT", "-m", "addrtype", "--dst-type", "LOCAL", "-j", "AR") // Created in versions <= 0.1.6
	// Also cleanup rules created by older versions, or -X might fail.
	mapper.tcpMapping = make(map[int]*net.TCPAddr)
	mapper.tcpProxies = make(map[int]Proxy)
	mapper.udpMapping = make(map[int]*net.UDPAddr)
	mapper.udpProxies = make(map[int]Proxy)
	return nil
}

func (mapper *PortMapper) setup() error {
	path, err := exec.LookPath("iptables")
	if err != nil {
		return fmt.Errorf("command not found: iptables")
	}

	if err := exec.Command(path, "-t", "nat", "-L", "AR").Run(); err != nil {
		if err := iptables("-t", "nat", "-N", "AR"); err != nil {
			return fmt.Errorf("Failed to create AR chain: %s", err)
		}
	}

	if err := iptables("-t", "nat", "-A", "PREROUTING", "-m", "addrtype", "--dst-type", "LOCAL", "-j", "AR"); err != nil {
		return fmt.Errorf("Failed to inject vk-container in PREROUTING chain: %s", err)
	}
	if err := iptables("-t", "nat", "-A", "OUTPUT", "-m", "addrtype", "--dst-type", "LOCAL", "!", "--dst", "127.0.0.0/8", "-j", "AR"); err != nil {
		return fmt.Errorf("Failed to inject vk-container in OUTPUT chain: %s", err)
	}
	return nil
}

func (mapper *PortMapper) iptablesForward(rule string, port int, proto string, dest_addr string, dest_port int) error {
	return iptables("-t", "nat", rule, "AR", "-p", proto, "--dport", strconv.Itoa(port),
		"!", "-i", NetworkBridgeIface,
		"-j", "DNAT", "--to-destination", net.JoinHostPort(dest_addr, strconv.Itoa(dest_port)))
}

func (mapper *PortMapper) Map(port int, backendAddr net.Addr) error {
	if _, isTCP := backendAddr.(*net.TCPAddr); isTCP {
		backendPort := backendAddr.(*net.TCPAddr).Port
		backendIP := backendAddr.(*net.TCPAddr).IP
		if err := mapper.iptablesForward("-A", port, "tcp", backendIP.String(), backendPort); err != nil {
			return err
		}
		mapper.tcpMapping[port] = backendAddr.(*net.TCPAddr)
		proxy, err := NewProxy(&net.TCPAddr{IP: net.IPv4(0, 0, 0, 0), Port: port}, backendAddr)
		if err != nil {
			mapper.Unmap(port, "tcp")
			return err
		}
		mapper.tcpProxies[port] = proxy
		go proxy.Run()
	} else {
		backendPort := backendAddr.(*net.UDPAddr).Port
		backendIP := backendAddr.(*net.UDPAddr).IP
		if err := mapper.iptablesForward("-A", port, "udp", backendIP.String(), backendPort); err != nil {
			return err
		}
		mapper.udpMapping[port] = backendAddr.(*net.UDPAddr)
		proxy, err := NewProxy(&net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: port}, backendAddr)
		if err != nil {
			mapper.Unmap(port, "udp")
			return err
		}
		mapper.udpProxies[port] = proxy
		go proxy.Run()
	}
	return nil
}

func (mapper *PortMapper) Unmap(port int, proto string) error {
	if proto == "tcp" {
		backendAddr, ok := mapper.tcpMapping[port]
		if !ok {
			return fmt.Errorf("Port tcp/%v is not mapped", port)
		}
		if proxy, exists := mapper.tcpProxies[port]; exists {
			proxy.Close()
			delete(mapper.tcpProxies, port)
		}
		if err := mapper.iptablesForward("-D", port, proto, backendAddr.IP.String(), backendAddr.Port); err != nil {
			return err
		}
		delete(mapper.tcpMapping, port)
	} else {
		backendAddr, ok := mapper.udpMapping[port]
		if !ok {
			return fmt.Errorf("Port udp/%v is not mapped", port)
		}
		if proxy, exists := mapper.udpProxies[port]; exists {
			proxy.Close()
			delete(mapper.udpProxies, port)
		}
		if err := mapper.iptablesForward("-D", port, proto, backendAddr.IP.String(), backendAddr.Port); err != nil {
			return err
		}
		delete(mapper.udpMapping, port)
	}
	return nil
}

func newPortMapper() (*PortMapper, error) {
	mapper := &PortMapper{}
	if err := mapper.cleanup(); err != nil {
		return nil, err
	}
	if err := mapper.setup(); err != nil {
		return nil, err
	}
	return mapper, nil
}

type IntSlice []int

func (pm *IntSlice) Remove(req int) bool {
	m := *pm

	for i, v := range m {
		if v == req {
			if i == 0 {
				*pm = m[i+1:]
			} else if i == len(m)-1 {
				*pm = m[:i]
			} else {
				*pm = append(m[:i], m[i+1:]...)
			}

			return true
		}
	}

	return false
}

func (pm *IntSlice) Append(req int) {
	*pm = append(*pm, req)
}

func (pm *IntSlice) Includes(req int) bool {
	for _, v := range *pm {
		if v == req {
			return true
		}
	}

	return false
}

type ipConfig struct {
	IPs      map[string]int `json:"ips"`
	TCPPorts IntSlice       `json:"tcp_ports"`
	UDPPorts IntSlice       `json:"udp_ports"`
}

// IP allocator: Atomatically allocate and release networking ports
type NetAllocator struct {
	network *net.IPNet
	cfg     *ipConfig
	lock    *os.File

	myTCP IntSlice
	myUDP IntSlice
}

func (alloc *NetAllocator) Load() {
	data, err := ioutil.ReadFile(path.Join(DIR, "ips"))

	if err != nil {
		return
	}

	json.Unmarshal(data, &alloc.cfg)
}

func (alloc *NetAllocator) Save() {
	data, err := json.Marshal(alloc.cfg)

	if err != nil {
		panic(err)
	}

	ioutil.WriteFile(path.Join(DIR, "ips"), data, 0644)
}

func (alloc *NetAllocator) Lock() {
	lock, err := os.OpenFile(path.Join(DIR, "ips"), os.O_WRONLY|os.O_CREATE, 0644)

	if err != nil {
		panic(err)
	}

	alloc.lock = lock

	err = syscall.Flock(int(alloc.lock.Fd()), syscall.LOCK_EX)

	if err != nil {
		panic(err)
	}
}

func (alloc *NetAllocator) Unlock() {
	alloc.lock.Close()
}

func (alloc *NetAllocator) LockAndLoad() {
	alloc.Lock()
	alloc.Load()
}

func (alloc *NetAllocator) SaveAndUnlock() {
	alloc.Save()
	alloc.Unlock()
}

func (alloc *NetAllocator) acquirePort(pm *IntSlice, req int) (int, error) {
	alloc.LockAndLoad()
	defer alloc.SaveAndUnlock()

	m := *pm

	if req == 0 {
		req = len(m) + portRangeStart

		if req > portRangeEnd {
			return -1, fmt.Errorf("Too many ports used!")
		}
	} else if pm.Includes(req) {
		return -1, fmt.Errorf("Port already in use: %d", req)
	}

	pm.Append(req)
	return req, nil
}

func (alloc *NetAllocator) AcquireTCPPort(req int) (int, error) {
	return alloc.acquirePort(&alloc.cfg.TCPPorts, req)
}

func (alloc *NetAllocator) AcquireUDPPort(req int) (int, error) {
	return alloc.acquirePort(&alloc.cfg.UDPPorts, req)
}

func (alloc *NetAllocator) releasePort(pm *IntSlice, req int) error {
	alloc.LockAndLoad()
	defer alloc.SaveAndUnlock()

	pm.Remove(req)

	return nil
}

func (alloc *NetAllocator) ReleaseTCPPort(port int) error {
	return alloc.releasePort(&alloc.cfg.TCPPorts, port)
}

func (alloc *NetAllocator) ReleaseUDPPort(port int) error {
	return alloc.releasePort(&alloc.cfg.UDPPorts, port)
}

func (alloc *NetAllocator) Acquire() (net.IP, error) {
	alloc.Lock()
	defer alloc.Unlock()

	firstIP, lastIP := networkRange(alloc.network)

	sz := ipToInt(lastIP) - ipToInt(firstIP)

	alloc.Load()

	var ip net.IP

	for {
		var buf [2]byte

		_, err := rand.Read(buf[:])

		if err != nil {
			return ip, err
		}

		num := int32(binary.BigEndian.Uint16(buf[:]))

		try := ipToInt(firstIP) + (num % sz)

		ip = intToIP(try)

		// Skip the router address, 0, and 255 octets
		if ip.Equal(alloc.network.IP) || ip[2] == 0 || ip[3] == 0 ||
			ip[2] == 255 || ip[3] == 255 {
			continue
		}

		if _, ok := alloc.cfg.IPs[ip.String()]; !ok {
			alloc.cfg.IPs[ip.String()] = 1
			break
		}
	}

	alloc.Save()

	return ip, nil
}

func (alloc *NetAllocator) Release(ip net.IP) {
	alloc.Lock()
	defer alloc.Unlock()

	alloc.Load()
	delete(alloc.cfg.IPs, ip.String())
	alloc.Save()
}

func newNetAllocator(network *net.IPNet) *NetAllocator {
	cfg := &ipConfig{make(map[string]int), nil, nil}

	alloc := &NetAllocator{
		network: network,
		cfg:     cfg,
	}

	alloc.Load()

	return alloc
}

// Network interface represents the networking stack of a container
type NetworkInterface struct {
	IPNet    net.IPNet
	Gateway  net.IP
	Gateway6 net.IP

	manager  *NetworkManager
	extPorts []*Nat
	disabled bool
}

// Allocate an external TCP port and map it to the interface
func (iface *NetworkInterface) AllocatePort(spec string) (*Nat, error) {

	if iface.disabled {
		return nil, fmt.Errorf("Trying to allocate port for interface %v, which is disabled", iface) // FIXME
	}

	nat, err := parseNat(spec)
	if err != nil {
		return nil, err
	}

	if nat.Proto == "tcp" {
		extPort, err := iface.manager.ipAllocator.AcquireTCPPort(nat.Frontend)
		if err != nil {
			return nil, err
		}
		backend := &net.TCPAddr{IP: iface.IPNet.IP, Port: nat.Backend}
		if err := iface.manager.portMapper.Map(extPort, backend); err != nil {
			iface.manager.ipAllocator.ReleaseTCPPort(extPort)
			return nil, err
		}
		nat.Frontend = extPort
	} else {
		extPort, err := iface.manager.ipAllocator.AcquireUDPPort(nat.Frontend)
		if err != nil {
			return nil, err
		}
		backend := &net.UDPAddr{IP: iface.IPNet.IP, Port: nat.Backend}
		if err := iface.manager.portMapper.Map(extPort, backend); err != nil {
			iface.manager.ipAllocator.ReleaseUDPPort(extPort)
			return nil, err
		}
		nat.Frontend = extPort
	}
	iface.extPorts = append(iface.extPorts, nat)

	return nat, nil
}

type Nat struct {
	Proto    string
	Frontend int
	Backend  int
}

func parseNat(spec string) (*Nat, error) {
	var nat Nat

	if strings.Contains(spec, "/") {
		specParts := strings.Split(spec, "/")
		if len(specParts) != 2 {
			return nil, fmt.Errorf("Invalid port format.")
		}
		proto := specParts[1]
		spec = specParts[0]
		if proto != "tcp" && proto != "udp" {
			return nil, fmt.Errorf("Invalid port format: unknown protocol %v.", proto)
		}
		nat.Proto = proto
	} else {
		nat.Proto = "tcp"
	}

	if strings.Contains(spec, ":") {
		specParts := strings.Split(spec, ":")
		if len(specParts) != 2 {
			return nil, fmt.Errorf("Invalid port format.")
		}
		// If spec starts with ':', external and internal ports must be the same.
		// This might fail if the requested external port is not available.
		var sameFrontend bool
		if len(specParts[0]) == 0 {
			sameFrontend = true
		} else {
			front, err := strconv.ParseUint(specParts[0], 10, 16)
			if err != nil {
				return nil, err
			}
			nat.Frontend = int(front)
		}
		back, err := strconv.ParseUint(specParts[1], 10, 16)
		if err != nil {
			return nil, err
		}
		nat.Backend = int(back)
		if sameFrontend {
			nat.Frontend = nat.Backend
		}
	} else {
		port, err := strconv.ParseUint(spec, 10, 16)
		if err != nil {
			return nil, err
		}
		nat.Backend = int(port)
	}

	return &nat, nil
}

// Release: Network cleanup - release all resources
func (iface *NetworkInterface) Release() {

	if iface.disabled {
		return
	}

	for _, nat := range iface.extPorts {
		utils.Debugf("Unmaping %v/%v", nat.Proto, nat.Frontend)
		if err := iface.manager.portMapper.Unmap(nat.Frontend, nat.Proto); err != nil {
			log.Printf("Unable to unmap port %v/%v: %v", nat.Proto, nat.Frontend, err)
		}
		if nat.Proto == "tcp" {
			if err := iface.manager.ipAllocator.ReleaseTCPPort(nat.Frontend); err != nil {
				log.Printf("Unable to release port tcp/%v: %v", nat.Frontend, err)
			}
		} else if err := iface.manager.ipAllocator.ReleaseUDPPort(nat.Frontend); err != nil {
			log.Printf("Unable to release port udp/%v: %v", nat.Frontend, err)
		}
	}

	iface.manager.ipAllocator.Release(iface.IPNet.IP)
}

// Network Manager manages a set of network interfaces
// Only *one* manager per host machine should be used
type NetworkManager struct {
	bridgeIface    string
	bridgeNetwork  *net.IPNet
	bridgeNetwork6 *net.IPNet

	ipAllocator *NetAllocator
	portMapper  *PortMapper

	disabled bool
}

// Allocate a network interface
func (manager *NetworkManager) Allocate() (*NetworkInterface, error) {

	if manager.disabled {
		return &NetworkInterface{disabled: true}, nil
	}

	ip, err := manager.ipAllocator.Acquire()
	if err != nil {
		return nil, err
	}
	iface := &NetworkInterface{
		IPNet:    net.IPNet{IP: ip, Mask: manager.bridgeNetwork.Mask},
		Gateway:  manager.bridgeNetwork.IP,
		Gateway6: manager.bridgeNetwork6.IP,
		manager:  manager,
	}
	return iface, nil
}

func newNetworkManager(bridgeIface string) (*NetworkManager, error) {

	if bridgeIface == DisableNetworkBridge {
		manager := &NetworkManager{
			disabled: true,
		}
		return manager, nil
	}

	addr, err := getIfaceAddr(bridgeIface)
	if err != nil {
		// If the iface is not found, try to create it
		if err := CreateBridgeIface(bridgeIface); err != nil {
			return nil, err
		}
		addr, err = getIfaceAddr(bridgeIface)
		if err != nil {
			return nil, err
		}
	}
	network := addr.(*net.IPNet)

	addr6, err := getIfaceAddr6(bridgeIface)

	if err != nil {
		addr6 = nil
	}

	ipAllocator := newNetAllocator(network)

	portMapper, err := newPortMapper()
	if err != nil {
		return nil, err
	}

	manager := &NetworkManager{
		bridgeIface:    bridgeIface,
		bridgeNetwork:  network,
		bridgeNetwork6: addr6.(*net.IPNet),
		ipAllocator:    ipAllocator,
		portMapper:     portMapper,
	}
	return manager, nil
}
