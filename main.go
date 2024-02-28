package main

import (
	"os"

	"github.com/curtisnewbie/event-pump/pump"
)

func main() {
	pump.BootstrapServer(os.Args)
}
