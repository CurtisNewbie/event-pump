package main

import (
	"os"

	"github.com/curtisnewbie/event-pump/internal/pump"
)

func main() {
	pump.BootstrapServer(os.Args)
}
