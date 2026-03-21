//go:build !darwin

package ui

// GetWindowFrame is a no-op on non-macOS platforms.
func GetWindowFrame() (x, y, w, h float64, ok bool) {
	return 0, 0, 0, 0, false
}

// SetWindowPosition is a no-op on non-macOS platforms.
func SetWindowPosition(_, _ float64) {}
