package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/unit"
)

// WindowState holds persisted window geometry.
type WindowState struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// WindowPersist saves and restores window size.
// Position is handled by macOS NSWindow frame autosave (window_darwin.go).
type WindowPersist struct {
	mu       sync.Mutex
	state    WindowState
	path     string
	dirty    bool
	debounce *time.Timer
}

// NewWindowPersist creates a WindowPersist backed by the given JSON file.
func NewWindowPersist(path string) (*WindowPersist, error) {
	wp := &WindowPersist{
		path: path,
		state: WindowState{
			Width:  680,
			Height: 580,
		},
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return wp, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading window state: %w", err)
	}

	if err := json.Unmarshal(data, &wp.state); err != nil {
		return nil, fmt.Errorf("parsing window state: %w", err)
	}

	return wp, nil
}

// Apply sets the window size from persisted state.
func (wp *WindowPersist) Apply(win *app.Window) {
	wp.mu.Lock()
	s := wp.state
	wp.mu.Unlock()

	win.Option(app.Size(unit.Dp(s.Width), unit.Dp(s.Height)))
}

// UpdateSize records a new window size, saving after a debounce.
func (wp *WindowPersist) UpdateSize(width, height int) {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.state.Width == width && wp.state.Height == height {
		return
	}

	wp.state.Width = width
	wp.state.Height = height
	wp.dirty = true

	if wp.debounce != nil {
		wp.debounce.Stop()
	}
	wp.debounce = time.AfterFunc(500*time.Millisecond, func() {
		wp.mu.Lock()
		defer wp.mu.Unlock()
		if !wp.dirty {
			return
		}
		wp.dirty = false
		if err := wp.save(); err != nil {
			fmt.Printf("window state save error: %v\n", err)
		}
	})
}

// save writes the current state to disk. Caller must hold wp.mu.
func (wp *WindowPersist) save() error {
	dir := filepath.Dir(wp.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(wp.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling: %w", err)
	}

	return os.WriteFile(wp.path, data, 0o644)
}
