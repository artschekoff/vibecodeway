package hotkey

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices -framework CoreGraphics -framework CoreFoundation
#import <Cocoa/Cocoa.h>
#include <ApplicationServices/ApplicationServices.h>
#include <CoreGraphics/CoreGraphics.h>
#include <stdlib.h>
#include <string.h>

// hk_read_clipboard reads the current plain-text content of the system clipboard
// directly via NSPasteboard — no subprocess, no Wails main-thread dispatch.
char* hk_read_clipboard(void) {
	@autoreleasepool {
		NSPasteboard *pb = [NSPasteboard generalPasteboard];
		NSString *str = [pb stringForType:NSPasteboardTypeString];
		if (!str || str.length == 0) return NULL;
		return strdup(str.UTF8String);
	}
}

// hk_deactivate_app hides the current application so the previously focused app
// regains focus, allowing the user to immediately invoke the hotkey again.
void hk_deactivate_app(void) {
	@autoreleasepool {
		[NSApp hide:nil];
	}
}

// hk_get_selected_text reads the selected text from the frontmost app's focused
// UI element via the Accessibility API — no clipboard is touched.
char* hk_get_selected_text(void) {
	AXUIElementRef sysWide = AXUIElementCreateSystemWide();

	AXUIElementRef focusedApp = NULL;
	AXError err = AXUIElementCopyAttributeValue(
		sysWide, kAXFocusedApplicationAttribute, (CFTypeRef*)&focusedApp);
	CFRelease(sysWide);
	if (err != kAXErrorSuccess || focusedApp == NULL) return NULL;

	AXUIElementRef focusedEl = NULL;
	err = AXUIElementCopyAttributeValue(
		focusedApp, kAXFocusedUIElementAttribute, (CFTypeRef*)&focusedEl);
	CFRelease(focusedApp);
	if (err != kAXErrorSuccess || focusedEl == NULL) return NULL;

	CFStringRef selected = NULL;
	err = AXUIElementCopyAttributeValue(
		focusedEl, kAXSelectedTextAttribute, (CFTypeRef*)&selected);
	CFRelease(focusedEl);
	if (err != kAXErrorSuccess || selected == NULL) return NULL;

	if (CFStringGetLength(selected) == 0) {
		CFRelease(selected);
		return NULL;
	}

	CFIndex maxLen = CFStringGetMaximumSizeForEncoding(
		CFStringGetLength(selected), kCFStringEncodingUTF8) + 1;
	char* buf = (char*)malloc(maxLen);
	if (!CFStringGetCString(selected, buf, maxLen, kCFStringEncodingUTF8)) {
		free(buf);
		CFRelease(selected);
		return NULL;
	}
	CFRelease(selected);
	return buf;
}

// hk_simulate_cmd_c posts a synthetic Cmd+C key-down/up via the HID event tap.
// Requires Accessibility permission.
// kVK_ANSI_C (0x08) is the physical key position of C on all layouts.
void hk_simulate_cmd_c(void) {
	CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);

	CGEventRef down = CGEventCreateKeyboardEvent(src, (CGKeyCode)0x08, true);
	if (down) {
		CGEventSetFlags(down, kCGEventFlagMaskCommand);
		CGEventPost(kCGHIDEventTap, down);
		CFRelease(down);
	}

	CGEventRef up = CGEventCreateKeyboardEvent(src, (CGKeyCode)0x08, false);
	if (up) {
		CGEventSetFlags(up, kCGEventFlagMaskCommand);
		CGEventPost(kCGHIDEventTap, up);
		CFRelease(up);
	}

	if (src) CFRelease(src);
}

// hk_check_accessibility returns non-zero if this process has the Accessibility
// permission required to read UI elements and post synthetic events.
// Uses AXIsProcessTrustedWithOptions — no subprocess, no shell interpreter.
int hk_check_accessibility(void) {
	return AXIsProcessTrustedWithOptions(NULL) ? 1 : 0;
}
*/
import "C"
import "unsafe"

// ReadClipboard reads the current plain-text clipboard content directly from
// NSPasteboard. Runs entirely in the calling goroutine — no subprocess fork,
// no Wails main-thread dispatch — so it cannot steal focus from the target app.
func ReadClipboard() string {
	ptr := C.hk_read_clipboard()
	if ptr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(ptr))
	return C.GoString(ptr)
}

// GetSelectedText returns the selected text in the frontmost app via the
// Accessibility API. Returns empty string if nothing is selected or permission
// is not granted.
func GetSelectedText() string {
	ptr := C.hk_get_selected_text()
	if ptr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(ptr))
	return C.GoString(ptr)
}

// SimulateCmdC posts a synthetic Cmd+C via CoreGraphics HID event tap.
func SimulateCmdC() {
	C.hk_simulate_cmd_c()
}

// CheckAccessibility returns true if this process has Accessibility permission,
// using AXIsProcessTrustedWithOptions — no subprocess, no shell interpreter.
// Passing NULL suppresses the system permission prompt; the caller is
// responsible for directing the user to System Settings manually.
func CheckAccessibility() bool {
	return C.hk_check_accessibility() != 0
}

// DeactivateApp hides the current application so the previously focused app
// regains focus immediately, without requiring the user to switch apps manually.
func DeactivateApp() {
	C.hk_deactivate_app()
}
