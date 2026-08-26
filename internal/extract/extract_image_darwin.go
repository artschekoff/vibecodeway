package extract

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Vision -framework AppKit -framework Foundation

#import <Vision/Vision.h>
#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>
#import <stdlib.h>

// recognizeTextInImage performs OCR on the image at imagePath using the macOS
// Vision framework (VNRecognizeTextRequest). Returns a heap-allocated C string
// that the caller must free(). Returns an empty string when the image has no
// text; returns NULL only on an unrecoverable internal error (OOM in strdup).
//
// Requires macOS 10.15+ for VNRecognizeTextRequest.
// WEBP is only decoded by NSImage on macOS 11+; older versions return nil for
// nsImage and the function returns an empty string.
char* recognizeTextInImage(const char* imagePath) {
    @autoreleasepool {
        NSString *path = [NSString stringWithUTF8String:imagePath];
        NSURL *url = [NSURL fileURLWithPath:path];

        NSImage *nsImage = [[NSImage alloc] initWithContentsOfURL:url];
        if (!nsImage) {
            return strdup("");
        }

        CGImageRef cgImage = [nsImage CGImageForProposedRect:NULL context:nil hints:nil];
        if (!cgImage) {
            return strdup("");
        }

        // Retain explicitly so the CGImageRef stays valid for the duration of the
        // Vision request, independent of ARC pool drain timing on nsImage.
        CGImageRetain(cgImage);

        VNRecognizeTextRequest *request = [[VNRecognizeTextRequest alloc] init];
        request.recognitionLevel = VNRequestTextRecognitionLevelAccurate;
        request.usesLanguageCorrection = YES;

        NSError *error = nil;
        VNImageRequestHandler *handler = [[VNImageRequestHandler alloc]
            initWithCGImage:cgImage
            options:@{}];
        [handler performRequests:@[request] error:&error];

        CGImageRelease(cgImage);

        if (error) {
            return strdup("");
        }

        NSMutableString *result = [NSMutableString string];
        for (VNRecognizedTextObservation *obs in request.results) {
            VNRecognizedText *top = [obs topCandidates:1].firstObject;
            if (top && top.string.length > 0) {
                [result appendString:top.string];
                [result appendString:@"\n"];
            }
        }

        // -UTF8String can return NULL under extreme memory pressure; guard before
        // passing to strdup to avoid undefined behaviour.
        const char *utf8 = [result UTF8String];
        return strdup(utf8 ? utf8 : "");
    }
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// extractImage uses the macOS Vision framework to perform OCR on the image file
// at path. Supports PNG, JPEG, and WEBP formats understood by NSImage.
func extractImage(path string) (string, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	cResult := C.recognizeTextInImage(cPath)
	if cResult == nil {
		return "", fmt.Errorf("Vision OCR returned nil for %q", path)
	}
	defer C.free(unsafe.Pointer(cResult))

	return C.GoString(cResult), nil
}
