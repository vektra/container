package main

import (
	"github.com/vektra/components/app"
	_ "github.com/vektra/container/commands"
	"github.com/vektra/container/env"
)

const VERSION = "1.13"

func main() {
	if selfPath := env.SelfPath(); selfPath == "/sbin/init" || selfPath == "/.dockerinit" {
		// Running in init mode
		env.SysInit()
		return
	}

	if err := env.Init(); err != nil {
		panic(err)
	}

	app.InitTool()
}
