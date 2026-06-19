// Command sparkview is a GPU inference dashboard for the Arbiter server.
package main

import (
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	_ "embed"

	"sparkview/pkg/arbiter"
	"sparkview/pkg/ui"

	"gioui.org/app"
	"gioui.org/op"
	"github.com/darrenoakey/daz-golang-gio/macos"
	"github.com/darrenoakey/daz-golang-gio/persist"
)

//go:embed gui/icon.png
var dockIconBytes []byte

// pollInterval is the gap BETWEEN polls. The poller sleeps this long after each
// fetch completes (see runPoller), so the interval is measured from the end of
// the previous poll — a slow or timed-out fetch never overlaps the next one.
const pollInterval = 3 * time.Second

func main() {
	// Timestamped lines to stderr; `auto` captures this into
	// ~/local/auto/output/logs/sparkview/. Microseconds help correlate the
	// poll loop with the watchdog. (`./run logs` tails the live file.)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("sparkview ")

	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	log.Printf("starting (pid %d)", os.Getpid())

	client := arbiter.NewClient(arbiter.DefaultURL)

	// Keep the polling loop alive when the window is backgrounded. Without this,
	// macOS App Nap throttles the bare binary and the dashboard freezes on a
	// stale frame. Must happen before polling starts.
	disableAppNap()
	log.Printf("App Nap disabled; polling %s every %s (interval from end of previous poll)",
		arbiter.DefaultURL, pollInterval)

	go func() {
		win := persist.NewWindow("sparkview", app.Title("Spark View"))

		dashboard := ui.NewApp(win.Window, client)

		// Poll OFF the GUI thread: the GUI goroutine below only ever runs Layout.
		// All network I/O (polling here, and the right-click write actions in the
		// ui package) happens on background goroutines so the UI never blocks.
		go runPoller(dashboard, pollInterval)

		var ops op.Ops
		var iconOnce sync.Once
		for {
			switch e := win.Event().(type) {
			case app.DestroyEvent:
				win.Close()
				if e.Err != nil {
					log.Printf("window destroyed with error: %v", e.Err)
				} else {
					log.Printf("window closed; exiting (launcher will relaunch)")
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

// runPoller refreshes the dashboard forever. It runs on its OWN goroutine, never
// the GUI thread, and sleeps `interval` AFTER each poll returns — so the cadence
// is measured from the end of the previous poll and a slow/timed-out fetch can
// never overlap the next request or hammer the server.
func runPoller(dashboard *ui.App, interval time.Duration) {
	for {
		dashboard.Refresh()
		time.Sleep(interval)
	}
}
