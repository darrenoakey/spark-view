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

	go func() {
		win := new(app.Window)
		win.Option(app.Title("Spark View"))
		windowState.Apply(win)

		// Restore position after the first frame renders.
		// Must be delayed: Gio/macOS places the window during the first
		// FrameEvent, so our position must come after that.
		posRestored := false

		dashboard := ui.NewApp(win, client)

		go dashboard.Refresh()

		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				dashboard.Refresh()
			}
		}()

		// Poll for position changes every 2 seconds (Gio has no move event)
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				windowState.UpdateGeometry(0, 0)
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
				windowState.UpdateGeometry(c.Size.X, c.Size.Y)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				dashboard.Layout(gtx)
				e.Frame(gtx.Ops)
				if !posRestored {
					posRestored = true
					go func() {
						time.Sleep(100 * time.Millisecond)
						windowState.RestorePosition()
					}()
				}
			}
		}
	}()

	app.Main()

	runtime.KeepAlive(client)
	return nil
}
