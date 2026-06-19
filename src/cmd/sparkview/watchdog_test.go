package main

import (
	"testing"
	"time"

	"sparkview/pkg/ui"
)

// TestPollStalled verifies the watchdog only fires on a genuine poll wedge, not
// during normal startup or recent polling.
func TestPollStalled(t *testing.T) {
	base := time.Now()
	started := base.Add(-5 * time.Minute) // process has been up a while

	tests := []struct {
		name        string
		lastRefresh time.Time
		now         time.Time
		started     time.Time
		want        bool
	}{
		{"recent poll", base, base.Add(10 * time.Second), started, false},
		{"poll just under threshold", base, base.Add(watchdogStall - time.Second), started, false},
		{"poll exactly at threshold", base, base.Add(watchdogStall), started, false},
		{"poll past threshold is stalled", base, base.Add(2 * time.Minute), started, true},
		{"startup grace, no poll yet", time.Time{}, base.Add(10 * time.Second), base, false},
		{"startup exceeded, no poll ever", time.Time{}, base.Add(2 * time.Minute), base, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pollStalled(tt.lastRefresh, tt.now, tt.started, watchdogStall)
			if got != tt.want {
				t.Errorf("pollStalled(last=%v, now=%v) = %v, want %v",
					tt.lastRefresh, tt.now, got, tt.want)
			}
		})
	}
}

// TestRouteWedged verifies the no-route restart fires only after a sustained
// routing-failure streak that followed a prior successful connection.
func TestRouteWedged(t *testing.T) {
	base := time.Now()
	prior := base.Add(-time.Hour) // a prior success exists
	tests := []struct {
		name         string
		noRouteSince time.Time
		lastSuccess  time.Time
		now          time.Time
		want         bool
	}{
		{"no streak", time.Time{}, prior, base, false},
		{"streak but never connected", base, time.Time{}, base.Add(2 * noRouteRestart), false},
		{"brief streak under threshold", base, prior, base.Add(noRouteRestart - time.Second), false},
		{"streak exactly at threshold", base, prior, base.Add(noRouteRestart), false},
		{"streak past threshold after success", base, prior, base.Add(noRouteRestart + time.Second), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := routeWedged(tt.noRouteSince, tt.lastSuccess, tt.now, noRouteRestart); got != tt.want {
				t.Errorf("routeWedged(%v, success=%v, %v) = %v, want %v",
					tt.noRouteSince, tt.lastSuccess, tt.now, got, tt.want)
			}
		})
	}
}

// TestRunWatchdogExitsOnNoRoute verifies a sustained no-route streak (after a
// prior success) triggers a TREE restart (exit 75), even while the poll loop
// itself is healthy (lastRefresh fresh).
func TestRunWatchdogExitsOnNoRoute(t *testing.T) {
	app := &ui.App{}
	app.SetLastRefreshForTest(time.Now())                           // poll loop alive
	app.SetLastSuccessForTest(time.Now().Add(-time.Hour))           // had connected before
	app.SetNoRouteSinceForTest(time.Now().Add(-2 * noRouteRestart)) // long no-route streak
	started := time.Now().Add(-10 * time.Minute)

	gotCode := -1
	runWatchdog(app, started, func(time.Duration) {
		app.SetLastRefreshForTest(time.Now()) // keep poll loop "alive" each tick
	}, func(code int) { gotCode = code })

	if gotCode != exitRestartTree {
		t.Fatalf("watchdog exit code = %d, want %d", gotCode, exitRestartTree)
	}
}

// TestRunWatchdogNoRestartWhenNeverConnected verifies that a no-route streak with
// NO prior success does not trigger a restart (avoids restart-looping on a real
// outage or a zero-grace denial). The loop must instead trip the poll-stall path.
func TestRunWatchdogNeverConnectedUsesStallNotTree(t *testing.T) {
	app := &ui.App{}
	app.SetNoRouteSinceForTest(time.Now().Add(-2 * noRouteRestart)) // long no-route streak
	// lastSuccess stays zero (never connected); lastRefresh stays zero too, so the
	// poll-stall path is what should fire — as a GUI restart, not a tree restart.
	started := time.Now().Add(-10 * time.Minute)

	gotCode := -1
	runWatchdog(app, started, func(time.Duration) {}, func(code int) { gotCode = code })

	if gotCode != exitRestartGUI {
		t.Fatalf("never-connected no-route should fall through to poll-stall GUI restart (%d), got %d",
			exitRestartGUI, gotCode)
	}
}

// TestRunWatchdogExitsOnStall verifies runWatchdog calls exit when the poll loop
// is wedged, and supplies the right code.
func TestRunWatchdogExitsOnStall(t *testing.T) {
	app := &ui.App{} // never polls — LastRefresh stays zero
	started := time.Now().Add(-10 * time.Minute)

	gotCode := -1
	exit := func(code int) { gotCode = code }
	// Instant sleeps so the loop reaches the stall check immediately.
	runWatchdog(app, started, func(time.Duration) {}, exit)

	if gotCode != 70 {
		t.Fatalf("watchdog exit code = %d, want 70", gotCode)
	}
}

// TestRunWatchdogStaysAliveWhilePolling verifies a healthy poll loop never
// triggers a restart.
func TestRunWatchdogStaysAliveWhilePolling(t *testing.T) {
	app := &ui.App{}
	app.SetLastRefreshForTest(time.Now())
	started := time.Now().Add(-10 * time.Minute)

	calls := 0
	exited := false
	sleep := func(time.Duration) {
		calls++
		// Keep the poll fresh, then stop the loop after a few iterations.
		app.SetLastRefreshForTest(time.Now())
		if calls >= 5 {
			// Force a stall to break out of the otherwise-infinite loop.
			app.SetLastRefreshForTest(time.Now().Add(-2 * watchdogStall))
		}
	}
	exit := func(int) { exited = true }
	runWatchdog(app, started, sleep, exit)

	if !exited {
		t.Fatal("watchdog never exited")
	}
	if calls < 5 {
		t.Fatalf("watchdog exited too early after %d checks while polling was healthy", calls)
	}
}
