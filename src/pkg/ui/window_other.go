//go:build !darwin

package ui

// EnableFrameAutosave is a no-op on non-macOS platforms.
func EnableFrameAutosave(_ string) {}
