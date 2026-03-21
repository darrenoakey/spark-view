//go:build !darwin

package main

// setDockIcon is a no-op on non-macOS platforms.
func setDockIcon() {}
