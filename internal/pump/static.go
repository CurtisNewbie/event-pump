package pump

import (
	"embed"

	"github.com/curtisnewbie/miso/miso"
)

//go:embed static
var tmplFs embed.FS

func ServeTempl(inb *miso.Inbound, s string, dat any) {
	miso.ServeTempl(inb, tmplFs, s, dat)
}

func ServeStatic(inb *miso.Inbound, s string) {
	miso.ServeStatic(inb, tmplFs, s)
}

func PrepareStatic(rail miso.Rail) error {
	miso.PrepareWebStaticFs(tmplFs, "/static")
	return nil
}
