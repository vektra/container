package env

import (
	"text/template"
)

const LxcTemplate = `
# hostname
{{if .Config.Hostname}}
lxc.utsname = {{.Config.Hostname}}
{{else}}
lxc.utsname = {{.Id}}
{{end}}
#lxc.aa_profile = unconfined

{{if .Config.NetworkDisabled}}
# network is disabled (-n=false)
lxc.network.type = empty
{{else}}
# network configuration
lxc.network.type = veth
lxc.network.flags = up
lxc.network.link = {{.NetworkSettings.Bridge}}
lxc.network.name = eth0
lxc.network.mtu = 1500
lxc.network.ipv4 = {{.NetworkSettings.IPAddress}}/{{.NetworkSettings.IPPrefixLen}}
{{end}}

# root filesystem
{{$ROOTFS := .RootfsPath}}
lxc.rootfs = {{$ROOTFS}}

# use a dedicated pts for the container (and limit the number of pseudo terminal
# available)
lxc.pts = 1024

# disable the main console
lxc.console = none

# no controlling tty at all
lxc.tty = 1

# no implicit access to devices
lxc.cgroup.devices.deny = a

# /dev/null and zero
lxc.cgroup.devices.allow = c 1:3 rwm
lxc.cgroup.devices.allow = c 1:5 rwm

# consoles
lxc.cgroup.devices.allow = c 5:1 rwm
lxc.cgroup.devices.allow = c 5:0 rwm
lxc.cgroup.devices.allow = c 4:0 rwm
lxc.cgroup.devices.allow = c 4:1 rwm

# /dev/urandom,/dev/random
lxc.cgroup.devices.allow = c 1:9 rwm
lxc.cgroup.devices.allow = c 1:8 rwm

# /dev/pts/* - pts namespaces are "coming soon"
lxc.cgroup.devices.allow = c 136:* rwm
lxc.cgroup.devices.allow = c 5:2 rwm

# tuntap
lxc.cgroup.devices.allow = c 10:200 rwm

# fuse
#lxc.cgroup.devices.allow = c 10:229 rwm

# rtc
#lxc.cgroup.devices.allow = c 254:0 rwm


# standard mount point
#  WARNING: procfs is a known attack vector and should probably be disabled
#           if your userspace allows it. eg. see http://blog.zx2c4.com/749
lxc.mount.entry = proc {{$ROOTFS}}/proc proc nosuid,nodev,noexec 0 0
#  WARNING: sysfs is a known attack vector and should probably be disabled
#           if your userspace allows it. eg. see http://bit.ly/T9CkqJ
lxc.mount.entry = sysfs {{$ROOTFS}}/sys sysfs nosuid,nodev,noexec 0 0
lxc.mount.entry = devpts {{$ROOTFS}}/dev/pts devpts newinstance,ptmxmode=0666,nosuid,noexec 0 0
#lxc.mount.entry = varrun {{$ROOTFS}}/var/run tmpfs mode=755,size=4096k,nosuid,nodev,noexec 0 0
#lxc.mount.entry = varlock {{$ROOTFS}}/var/lock tmpfs size=1024k,nosuid,nodev,noexec 0 0
#lxc.mount.entry = shm {{$ROOTFS}}/dev/shm tmpfs size=65536k,nosuid,nodev,noexec 0 0

# Inject docker-init
lxc.mount.entry = {{.SysInitPath}} {{$ROOTFS}}/.dockerinit none bind,ro 0 0

# In order to get a working DNS environment, mount bind (ro) the host's /etc/resolv.conf into the container
lxc.mount.entry = {{.ResolvConfPath}} {{$ROOTFS}}/etc/resolv.conf none bind,ro 0 0
{{if .Volumes}}
{{ $rw := .VolumesRW }}
{{range $virtualPath, $realPath := .Volumes}}
lxc.mount.entry = {{$realPath}} {{$ROOTFS}}/{{$virtualPath}} none bind,{{ if index $rw $virtualPath }}rw{{else}}ro{{end}} 0 0
{{end}}
{{end}}

# drop linux capabilities (apply mainly to the user root in the container)
#  (Note: 'lxc.cap.keep' is coming soon and should replace this under the
#         security principle 'deny all unless explicitly permitted', see
#         http://sourceforge.net/mailarchive/message.php?msg_id=31054627 )
lxc.cap.drop = audit_control audit_write mac_admin mac_override mknod setfcap setpcap sys_admin sys_boot sys_module sys_nice sys_pacct sys_rawio sys_resource sys_time sys_tty_config

# limits
{{if .Config.Memory}}
lxc.cgroup.memory.limit_in_bytes = {{.Config.Memory}}
lxc.cgroup.memory.soft_limit_in_bytes = {{.Config.Memory}}
{{with $memSwap := getMemorySwap .Config}}
lxc.cgroup.memory.memsw.limit_in_bytes = {{$memSwap}}
{{end}}
{{end}}
{{if .Config.CpuShares}}
lxc.cgroup.cpu.shares = {{.Config.CpuShares}}
{{end}}
`

var LxcTemplateCompiled *template.Template

func getMemorySwap(config *Config) int64 {
	// By default, MemorySwap is set to twice the size of RAM.
	// If you want to omit MemorySwap, set it to `-1'.
	if config.MemorySwap < 0 {
		return 0
	}
	return config.Memory * 2
}

func init() {
	var err error
	funcMap := template.FuncMap{
		"getMemorySwap": getMemorySwap,
	}

	LxcTemplateCompiled, err = template.New("lxc").Funcs(funcMap).Parse(LxcTemplate)

	if err != nil {
		panic(err)
	}
}
