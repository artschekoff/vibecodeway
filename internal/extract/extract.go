// Package extract provides text extraction from common document formats
// (PDF, DOCX, RTF, ODT, TXT) and images via OCR.
package extract

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gen2brain/go-fitz"
	"github.com/lu4p/cat"
)

// AllowedExtensions is the set of file extensions this package can handle.
var AllowedExtensions = map[string]bool{
	".pdf":      true,
	".docx":     true,
	".rtf":      true,
	".odt":      true,
	".txt":      true,
	".md":       true,
	".markdown": true,
	// Images — text extracted via OCR (macOS Vision framework)
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

// ErrUnsupportedFormat is returned when the file extension is not in AllowedExtensions.
type ErrUnsupportedFormat struct {
	Ext string
}

func (e ErrUnsupportedFormat) Error() string {
	exts := make([]string, 0, len(AllowedExtensions))
	for ext := range AllowedExtensions {
		exts = append(exts, strings.TrimPrefix(ext, "."))
	}
	sort.Strings(exts)
	return fmt.Sprintf("unsupported file format: %q (allowed: %s)", e.Ext, strings.Join(exts, ", "))
}

// MaxFileBytes is the upper bound on file size that FromFile will process.
// Files larger than this are rejected with ErrFileTooLarge to prevent OOM
// when loading large images into memory for OCR.
const MaxFileBytes int64 = 50 * 1024 * 1024 // 50 MB

// ErrFileTooLarge is returned when a file exceeds MaxFileBytes.
type ErrFileTooLarge struct {
	Path string
	Size int64
}

func (e *ErrFileTooLarge) Error() string {
	return fmt.Sprintf("%s is too large (%d MB, max %d MB)",
		filepath.Base(e.Path), e.Size/(1024*1024), MaxFileBytes/(1024*1024))
}

// ErrNoTextFound is returned when OCR succeeds but the image contains no readable text.
type ErrNoTextFound struct {
	Path string
}

func (e *ErrNoTextFound) Error() string {
	return fmt.Sprintf("no readable text found in %q — the image may be blank or contain non-text content", filepath.Base(e.Path))
}

// FromFile extracts plain text and returns the normalised file-type label.
// filePath must point to a readable file. The extension is validated against
// AllowedExtensions before any IO is performed.
func FromFile(filePath string) (text string, fileType string, err error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if !AllowedExtensions[ext] {
		return "", "", ErrUnsupportedFormat{Ext: ext}
	}

	fileType = strings.TrimPrefix(ext, ".")

	if fi, statErr := os.Stat(filePath); statErr != nil {
		return "", fileType, fmt.Errorf("stat: %w", statErr)
	} else if fi.Size() > MaxFileBytes {
		return "", fileType, &ErrFileTooLarge{Path: filePath, Size: fi.Size()}
	}

	switch ext {
	case ".pdf":
		text, err = extractPDF(filePath)
	case ".txt", ".md", ".markdown":
		text, err = extractTXT(filePath)
	case ".png", ".jpg", ".jpeg", ".webp":
		if err = checkImageMagic(filePath, ext); err != nil {
			break
		}
		text, err = extractImage(filePath)
		if err == nil && strings.TrimSpace(text) == "" {
			err = &ErrNoTextFound{Path: filePath}
		}
	default:
		// .docx, .rtf, .odt — handled by lu4p/cat
		text, err = extractViaCAT(filePath)
	}
	if err != nil {
		return "", fileType, fmt.Errorf("extracting %s: %w", ext, err)
	}

	return cleanText(text), fileType, nil
}

// extractPDF uses MuPDF (via go-fitz) to extract text page by page.
func extractPDF(path string) (string, error) {
	doc, err := fitz.New(path)
	if err != nil {
		return "", fmt.Errorf("opening pdf: %w", err)
	}
	defer doc.Close()

	var sb strings.Builder
	for n := 0; n < doc.NumPage(); n++ {
		text, err := doc.Text(n)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			sb.WriteByte('\n')
		}
	}
	return sb.String(), nil
}

// extractTXT reads a plain-text file, accepting UTF-8 or falling back to
// replacing invalid sequences so the DB always receives valid text.
func extractTXT(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading txt: %w", err)
	}
	if utf8.Valid(data) {
		return string(data), nil
	}
	// Replace invalid UTF-8 bytes with the replacement character.
	return strings.ToValidUTF8(string(data), "\uFFFD"), nil
}

// extractViaCAT delegates to lu4p/cat which supports .docx, .rtf, and .odt natively.
func extractViaCAT(path string) (string, error) {
	text, err := cat.File(path)
	if err != nil {
		return "", fmt.Errorf("cat extraction: %w", err)
	}
	return text, nil
}

// checkImageMagic reads the first 12 bytes of path and verifies they match the
// magic bytes for the given extension. This guards against files that have been
// renamed to a supported image extension but contain arbitrary content.
func checkImageMagic(path, ext string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening image: %w", err)
	}
	defer f.Close()

	header := make([]byte, 12)
	n, _ := f.Read(header)
	header = header[:n]

	var valid bool
	switch ext {
	case ".png":
		valid = len(header) >= 4 && bytes.Equal(header[:4], []byte{0x89, 0x50, 0x4E, 0x47})
	case ".jpg", ".jpeg":
		valid = len(header) >= 3 && bytes.Equal(header[:3], []byte{0xFF, 0xD8, 0xFF})
	case ".webp":
		// RIFF....WEBP
		valid = len(header) >= 12 &&
			bytes.Equal(header[0:4], []byte("RIFF")) &&
			bytes.Equal(header[8:12], []byte("WEBP"))
	default:
		valid = true
	}

	if !valid {
		return fmt.Errorf("file does not appear to be a valid %s image", strings.TrimPrefix(ext, "."))
	}
	return nil
}

// cleanText normalises extracted text: trims leading/trailing whitespace and
// collapses sequences of more than two consecutive blank lines into two.
func cleanText(s string) string {
	s = strings.TrimSpace(s)
	// Collapse 3+ consecutive newlines to 2.
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}
