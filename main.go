package main

import (
	"os"

	"github.com/curtisnewbie/event-pump/pump"
	"github.com/curtisnewbie/gocommon/server"
)

func main() {
	server.PreServerBootstrap(pump.PreServerBootstrap)
	server.PostServerBootstrapped(pump.PostServerBootstrap)
	server.BootstrapServer(os.Args)
}
