//go:build darwin && !ios

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation

#import <Foundation/Foundation.h>

// Hold a single process-wide activity assertion for the lifetime of the app.
static id sparkviewActivityToken = nil;

// beginDashboardActivity opts the process out of App Nap.
//
// Without a .app bundle / Info.plist, Spark View has no LSAppNapIsDisabled key,
// so when its window sits in the background macOS throttles the process: timers
// (including the Go poll loop), CPU and network I/O are deferred. The dashboard
// then stops fetching, and because an occluded window is never redrawn, the last
// rendered frame ("server down") lingers — it looks frozen.
//
// NSActivityUserInitiatedAllowingIdleSystemSleep marks the work as user-relevant
// and ongoing (prevents App Nap deferral) while still allowing normal idle system
// sleep. The token is retained for the process lifetime; we never end it.
static void beginDashboardActivity(void) {
	@autoreleasepool {
		if (sparkviewActivityToken != nil) {
			return;
		}
		NSActivityOptions opts = NSActivityUserInitiatedAllowingIdleSystemSleep;
		sparkviewActivityToken = [[[NSProcessInfo processInfo]
			beginActivityWithOptions:opts
			                  reason:@"Spark View live dashboard"] retain];
	}
}
*/
import "C"

// disableAppNap prevents macOS from throttling the polling loop while Spark View
// is in the background. Safe to call from any goroutine; idempotent.
func disableAppNap() {
	C.beginDashboardActivity()
}
