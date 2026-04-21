// Package extract provides text extraction from common document formats
// (PDF, DOCX, RTF, ODT, TXT).
package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gen2brain/go-fitz"
	"github.com/lu4p/cat"
)

// AllowedExtensions is the set of file extensions this package can handle.
var AllowedExtensions = map[string]bool{
	".pdf":  true,
	".docx": true,
	".rtf":  true,
	".odt":  true,
	".txt":  true,
}

// ErrUnsupportedFormat is returned when the file extension is not in AllowedExtensions.
type ErrUnsupportedFormat struct {
	Ext string
}

func (e ErrUnsupportedFormat) Error() string {
	return fmt.Sprintf("unsupported file format: %q (allowed: pdf, docx, rtf, odt, txt)", e.Ext)
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

	switch ext {
	case ".pdf":
		text, err = extractPDF(filePath)
	case ".txt":
		text, err = extractTXT(filePath)
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
