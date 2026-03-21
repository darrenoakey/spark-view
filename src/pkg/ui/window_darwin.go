package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// enableFrameAutosave sets the NSWindow frame autosave name so macOS
// persists and restores the window position automatically across launches.
void enableFrameAutosave(const char *name) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!name) return;
        NSArray *windows = [NSApp windows];
        if (windows.count > 0) {
            NSWindow *win = windows[0];
            NSString *saveName = [NSString stringWithUTF8String:name];
            if (saveName) {
                [win setFrameAutosaveName:saveName];
            }
        }
    });
}
*/
import "C"
import "unsafe"

// EnableFrameAutosave tells macOS to persist the window position under the given name.
// Must be called after the window is created.
func EnableFrameAutosave(name string) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	C.enableFrameAutosave(cname)
}
