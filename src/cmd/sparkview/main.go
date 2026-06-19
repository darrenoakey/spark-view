// Command sparkview is a GPU inference dashboard for the Arbiter server.
package main

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"sync"
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

	// Keep the polling loop alive when the window is backgrounded. Without this,
	// macOS App Nap throttles the bare binary and the dashboard freezes on a
	// stale frame. Must happen before polling starts.
	disableAppNap()

	started := time.Now()

	go func() {
		win := persist.NewWindow("sparkview", app.Title("Spark View"))

		dashboard := ui.NewApp(win.Window, client)

		go func() {
			for {
				dashboard.Refresh()
				time.Sleep(3 * time.Second)
			}
		}()

		// Self-heal: if the poll loop ever wedges, exit so the supervising
		// launcher respawns a fresh GUI. Keys off poll liveness, not rendering,
		// so a merely occluded/idle window is never restarted.
		go runWatchdog(dashboard, started, time.Sleep, os.Exit)

		var ops op.Ops
		var iconOnce sync.Once
		for {
			switch e := win.Event().(type) {
			case app.DestroyEvent:
				win.Close()
				if e.Err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", e.Err)
				}
				os.Exit(0)
			case app.FrameEvent:
				iconOnce.Do(func() { macos.SetDockIcon(dockIconBytes) })
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
