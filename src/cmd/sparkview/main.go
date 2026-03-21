// Command sparkview is a GPU inference dashboard for the Arbiter server.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"sparkview/pkg/arbiter"
	"sparkview/pkg/ui"

	"gioui.org/app"
	"gioui.org/op"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Resolve project root: executable lives in output/bin/ under project root
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(resolved)))
	localDir := filepath.Join(projectRoot, "local")
	windowPath := filepath.Join(localDir, "window.json")

	windowState, err := ui.NewWindowPersist(windowPath)
	if err != nil {
		return fmt.Errorf("loading window state: %w", err)
	}

	client := arbiter.NewClient(arbiter.DefaultURL)

	setDockIcon()

	// Run the event loop in a goroutine — app.Main() must own the main goroutine on macOS
	go func() {
		win := new(app.Window)
		win.Option(app.Title("Spark View"))
		windowState.Apply(win)

		// Enable macOS native frame autosave for position persistence
		go func() {
			time.Sleep(500 * time.Millisecond)
			ui.EnableFrameAutosave("SparkView")
		}()

		dashboard := ui.NewApp(win, client)

		// Initial fetch
		go dashboard.Refresh()

		// Periodic refresh every 60 seconds
		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				dashboard.Refresh()
			}
		}()

		var ops op.Ops
		for {
			switch e := win.Event().(type) {
			case app.DestroyEvent:
				if e.Err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", e.Err)
				}
				os.Exit(0)
			case app.ConfigEvent:
				c := e.Config
				windowState.UpdateSize(c.Size.X, c.Size.Y)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				dashboard.Layout(gtx)
				e.Frame(gtx.Ops)
			}
		}
	}()

	// app.Main() runs the platform event loop on the main goroutine (required on macOS)
	app.Main()

	runtime.KeepAlive(client)
	return nil
}
