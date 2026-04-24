//go:build !darwin

package extract

import "fmt"

// extractImage is a stub for non-macOS platforms. Image OCR via the Vision
// framework is only available on macOS. On Windows, use extract_image_windows.go
// with Windows.Media.Ocr; on Linux, Tesseract can be wired in similarly.
func extractImage(path string) (string, error) {
	return "", fmt.Errorf("image OCR is not supported on this platform (macOS only)")
}
