package main

import (
	"os"

	"github.com/curtisnewbie/event-pump/pump"
	"github.com/curtisnewbie/miso/miso"
)

func main() {
	miso.PreServerBootstrap(pump.PreServerBootstrap)
	miso.PostServerBootstrapped(pump.PostServerBootstrap)
	miso.BootstrapServer(os.Args)
}
