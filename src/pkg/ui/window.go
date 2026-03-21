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

// WindowState holds persisted window geometry including position.
type WindowState struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	HasPos bool    `json:"has_pos"`
}

// WindowPersist saves and restores window size and position.
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
			Height: 500,
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

// RestorePosition restores the window position after the window is created.
// Must be called from a goroutine (not the frame handler) with a short delay
// to ensure the window exists.
func (wp *WindowPersist) RestorePosition() {
	wp.mu.Lock()
	s := wp.state
	wp.mu.Unlock()

	if s.HasPos {
		SetWindowPosition(s.X, s.Y)
	}
}

// UpdateGeometry reads the current window frame from the OS and saves it.
// Call this on ConfigEvent (resize) or periodically.
func (wp *WindowPersist) UpdateGeometry(gioWidth, gioHeight int) {
	x, y, _, _, ok := GetWindowFrame()

	wp.mu.Lock()
	defer wp.mu.Unlock()

	changed := false
	if gioWidth > 0 && gioHeight > 0 && (wp.state.Width != gioWidth || wp.state.Height != gioHeight) {
		wp.state.Width = gioWidth
		wp.state.Height = gioHeight
		changed = true
	}
	if ok && (wp.state.X != x || wp.state.Y != y || !wp.state.HasPos) {
		wp.state.X = x
		wp.state.Y = y
		wp.state.HasPos = true
		changed = true
	}

	if !changed {
		return
	}

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
