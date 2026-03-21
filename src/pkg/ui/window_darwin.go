package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// getWindowFrame reads the NSWindow frame origin (bottom-left in screen coords).
// Returns x, y, width, height via out pointers. Returns 0 if no window found.
int getWindowFrame(double *x, double *y, double *w, double *h) {
    NSArray *windows = [NSApp windows];
    if (windows.count == 0) return 0;
    NSWindow *win = windows[0];
    NSRect frame = [win frame];
    *x = frame.origin.x;
    *y = frame.origin.y;
    *w = frame.size.width;
    *h = frame.size.height;
    return 1;
}

// setWindowPosition moves the NSWindow to the given origin (bottom-left in screen coords).
void setWindowPosition(double x, double y) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSArray *windows = [NSApp windows];
        if (windows.count > 0) {
            NSWindow *win = windows[0];
            [win setFrameOrigin:NSMakePoint(x, y)];
        }
    });
}
*/
import "C"

// GetWindowFrame reads the current NSWindow frame. Returns x, y, w, h and ok.
// Coordinates are macOS screen coords (origin at bottom-left of screen).
func GetWindowFrame() (x, y, w, h float64, ok bool) {
	var cx, cy, cw, ch C.double
	if C.getWindowFrame(&cx, &cy, &cw, &ch) == 0 {
		return 0, 0, 0, 0, false
	}
	return float64(cx), float64(cy), float64(cw), float64(ch), true
}

// SetWindowPosition moves the NSWindow to the given screen coordinates.
func SetWindowPosition(x, y float64) {
	C.setWindowPosition(C.double(x), C.double(y))
}
