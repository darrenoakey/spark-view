package main

import (
	"fmt"
	"log"
	"time"

	"sparkview/pkg/ui"
)

// Watchdog tuning. The poll loop refreshes every ~3s (plus up to a 10s HTTP
// timeout), so a healthy lastRefresh is never older than ~13s. A 60s threshold
// means several missed cycles before we act, avoiding false positives.
const (
	watchdogInterval = 15 * time.Second // how often liveness is checked
	watchdogStall    = 60 * time.Second // poll silence that counts as wedged
)

// pollStalled reports whether the polling loop has gone silent for too long.
//
// It deliberately keys off POLL liveness (lastRefresh, updated every cycle
// regardless of success/failure), NOT render/frame liveness. An occluded or
// unfocused window legitimately stops drawing, so a frame-based watchdog would
// restart-loop whenever Spark View is in the background. The poll loop, by
// contrast, must keep running whatever the window state — if it stops, the
// process is genuinely wedged.
//
// A zero lastRefresh means no poll has completed yet: during startup that is
// fine, but if it persists past the stall threshold the poll goroutine never
// came up and we should relaunch.
func pollStalled(lastRefresh, now, started time.Time, threshold time.Duration) bool {
	if lastRefresh.IsZero() {
		return now.Sub(started) > threshold
	}
	return now.Sub(lastRefresh) > threshold
}

// runWatchdog monitors poll liveness forever and calls exit (typically os.Exit)
// when the poll loop has wedged, so the supervising launcher respawns a fresh
// GUI. App Nap must be disabled first, or background throttling could delay the
// poll loop enough to trip this. Runs on its own goroutine; never returns until
// exit is called.
func runWatchdog(app *ui.App, started time.Time, sleep func(time.Duration), exit func(int)) {
	for {
		sleep(watchdogInterval)
		last := app.LastRefresh()
		if pollStalled(last, time.Now(), started, watchdogStall) {
			log.Printf("watchdog: poll loop stalled (last refresh %s) — exiting for relaunch",
				sinceDesc(last, started))
			exit(70)
			return
		}
	}
}

// sinceDesc renders a human-readable staleness for the log line.
func sinceDesc(lastRefresh, started time.Time) string {
	if lastRefresh.IsZero() {
		return fmt.Sprintf("never, up %s", time.Since(started).Round(time.Second))
	}
	return time.Since(lastRefresh).Round(time.Second).String() + " ago"
}
