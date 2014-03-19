package env

import (
	"flag"
	"fmt"
	"github.com/vektra/container/utils"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

// Setup networking
func setupNetworking(gw string) {
	if gw == "" {
		return
	}
	if _, err := ip("route", "add", "default", "via", gw); err != nil {
		log.Fatalf("Unable to set up networking: %v", err)
	}
}

func setupNetworking6(gw string) {
	if gw == "" {
		return
	}
	if _, err := ip("route", "add", "default", "via", gw, "dev", "eth0"); err != nil {
		log.Fatalf("Unable to set up networking: %v", err)
	}
}

// Takes care of dropping privileges to the desired user
func changeUser(u string) {
	if u == "" {
		return
	}
	userent, err := UserLookup(u)
	if err != nil {
		log.Fatalf("Unable to find user %v: %v", u, err)
	}

	uid, err := strconv.Atoi(userent.Uid)
	if err != nil {
		log.Fatalf("Invalid uid: %v", userent.Uid)
	}
	gid, err := strconv.Atoi(userent.Gid)
	if err != nil {
		log.Fatalf("Invalid gid: %v", userent.Gid)
	}

	if err := syscall.Setgid(gid); err != nil {
		log.Fatalf("setgid failed: %v", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		log.Fatalf("setuid failed: %v", err)
	}
}

// UserLookup check if the given username or uid is present in /etc/passwd
// and returns the user struct.
// If the username is not found, an error is returned.
func UserLookup(uid string) (*user.User, error) {
	file, err := ioutil.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(file), "\n") {
		data := strings.Split(line, ":")
		if len(data) > 5 && (data[0] == uid || data[2] == uid) {
			return &user.User{
				Uid:      data[2],
				Gid:      data[3],
				Username: data[0],
				Name:     data[4],
				HomeDir:  data[5],
			}, nil
		}
	}
	return nil, fmt.Errorf("User not found in /etc/passwd")
}

// Clear environment pollution introduced by lxc-start
func cleanupEnv(env utils.ListOpts) {
	os.Clearenv()
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 1 {
			parts = append(parts, "")
		}
		os.Setenv(parts[0], parts[1])
	}
}

func executeProgram(name string, args []string) {
	path, err := exec.LookPath(name)
	if err != nil {
		log.Printf("Unable to locate %v", name)
		os.Exit(127)
	}

	if err := syscall.Exec(path, args, os.Environ()); err != nil {
		panic(err)
	}
}

// Sys Init code
// This code is run INSIDE the container and is responsible for setting
// up the environment before running the actual process
func SysInit() {
	if len(os.Args) <= 1 {
		fmt.Println("You should not invoke docker-init manually")
		os.Exit(1)
	}
	var u = flag.String("u", "", "username or uid")
	var gw = flag.String("g", "", "gateway address")
	var gw6 = flag.String("g6", "", "ipv6 gateway address")

	var flEnv utils.ListOpts
	flag.Var(&flEnv, "e", "Set environment variables")

	flag.Parse()

	cleanupEnv(flEnv)
	setupNetworking(*gw)
	setupNetworking6(*gw6)
	changeUser(*u)
	executeProgram(flag.Arg(0), flag.Args())
}
