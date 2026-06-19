//go:build !darwin || ios

package main

// disableAppNap is a no-op on platforms without macOS App Nap.
func disableAppNap() {}
