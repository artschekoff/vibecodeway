// Package perm reads macOS privacy permission state for the setup checklist.
package perm

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AVFoundation -framework Foundation
#import <AVFoundation/AVFoundation.h>

static int perm_mic_status(void) {
	if (@available(macOS 10.14, *)) {
		return (int)[AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
	}
	return 3;
}
*/
import "C"

// MicrophoneStatus returns the TCC state for audio capture:
// not_determined | restricted | denied | authorized.
func MicrophoneStatus() string {
	switch C.perm_mic_status() {
	case 0:
		return "not_determined"
	case 1:
		return "restricted"
	case 2:
		return "denied"
	default:
		return "authorized"
	}
}
