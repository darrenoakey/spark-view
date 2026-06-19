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
	// noRouteRestart is how long an unbroken "no route to host" streak may last
	// before we restart. Two causes produce EHOSTUNREACH on a same-subnet host:
	// (a) macOS multi-homing flapping the route, and (b) macOS Local Network
	// privacy revoking this background app's LAN access after its grace window —
	// both wedge a long-lived process while curl and fresh processes succeed.
	// Only routing failures count (not refused/timeout), so a genuine spark
	// outage never triggers this. Generous enough to ride out a brief
	// unplug→Wi-Fi failover.
	noRouteRestart = 45 * time.Second
)

// Watchdog exit codes. The launcher distinguishes them (see sparkview-launcher).
const (
	// exitRestartGUI: the poll loop wedged. A fresh GUI under the SAME launcher
	// fixes it, so the launcher just relaunches the binary.
	exitRestartGUI = 70
	// exitRestartTree: sustained "no route to host". Recovery needs a fresh
	// RESPONSIBLE PROCESS — restarting only the GUI keeps the same (denied)
	// launcher, so the launcher must exit too and let auto respawn the whole
	// tree, which gets a fresh macOS Local Network grant. See CLAUDE.md.
	exitRestartTree = 75
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
		now := time.Now()

		last := app.LastRefresh()
		if pollStalled(last, now, started, watchdogStall) {
			log.Printf("watchdog: poll loop stalled (last refresh %s) — restarting GUI",
				sinceDesc(last, started))
			exit(exitRestartGUI)
			return
		}

		since := app.NoRouteSince()
		if routeWedged(since, app.LastSuccess(), now, noRouteRestart) {
			log.Printf("watchdog: %s of \"no route to host\" after a working connection — restarting process tree for a fresh network grant",
				now.Sub(since).Round(time.Second))
			exit(exitRestartTree)
			return
		}
	}
}

// routeWedged reports whether the process is stuck on routing-level dial failures
// long enough that only a fresh process tree will recover it.
//
// It requires a PRIOR successful poll (lastSuccess set): the failure mode we heal
// is "was connected, then macOS revoked LAN access / the route flapped". If the
// process never connected (genuine spark outage, or a denial with no grace), a
// restart would not help, so we stay put and just show "down" rather than
// restart-loop.
func routeWedged(noRouteSince, lastSuccess, now time.Time, threshold time.Duration) bool {
	if noRouteSince.IsZero() || lastSuccess.IsZero() {
		return false
	}
	return now.Sub(noRouteSince) > threshold
}

// sinceDesc renders a human-readable staleness for the log line.
func sinceDesc(lastRefresh, started time.Time) string {
	if lastRefresh.IsZero() {
		return fmt.Sprintf("never, up %s", time.Since(started).Round(time.Second))
	}
	return time.Since(lastRefresh).Round(time.Second).String() + " ago"
}
