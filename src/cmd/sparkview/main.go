// Command sparkview is a GPU inference dashboard for the Arbiter server.
package main

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"time"

	"sparkview/pkg/arbiter"
	"sparkview/pkg/ui"

	"gioui.org/app"
	"gioui.org/op"
	"github.com/darrenoakey/daz-golang-gio/macos"
	"github.com/darrenoakey/daz-golang-gio/persist"
)

//go:embed gui/icon.png
var dockIconBytes []byte

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	client := arbiter.NewClient(arbiter.DefaultURL)

	macos.SetDockIcon(dockIconBytes)

	go func() {
		win := persist.NewWindow("sparkview", app.Title("Spark View"))

		dashboard := ui.NewApp(win.Window, client)

		go func() {
			for {
				dashboard.Refresh()
				time.Sleep(3 * time.Second)
			}
		}()

		var ops op.Ops
		for {
			switch e := win.Event().(type) {
			case app.DestroyEvent:
				win.Close()
				if e.Err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", e.Err)
				}
				os.Exit(0)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				dashboard.Layout(gtx)
				e.Frame(gtx.Ops)
			}
		}
	}()

	app.Main()

	runtime.KeepAlive(client)
	return nil
}
