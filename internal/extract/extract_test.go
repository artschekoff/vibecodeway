package extract_test

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/extract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- AllowedExtensions ---

func TestAllowedExtensions_IncludesExistingFormats(t *testing.T) {
	for _, ext := range []string{".pdf", ".docx", ".rtf", ".odt", ".txt"} {
		assert.True(t, extract.AllowedExtensions[ext], "expected %s to be allowed", ext)
	}
}

func TestAllowedExtensions_IncludesImageFormats(t *testing.T) {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
		assert.True(t, extract.AllowedExtensions[ext], "expected image extension %s to be allowed", ext)
	}
}

// --- ErrUnsupportedFormat ---

func TestFromFile_UnsupportedFormat_ReturnsTypedError(t *testing.T) {
	_, _, err := extract.FromFile("foo.bmp")
	require.Error(t, err)

	var unsupported extract.ErrUnsupportedFormat
	require.ErrorAs(t, err, &unsupported)
	assert.Equal(t, ".bmp", unsupported.Ext)
}

func TestFromFile_GIF_IsUnsupported(t *testing.T) {
	_, _, err := extract.FromFile("foo.gif")
	require.Error(t, err)
	var unsupported extract.ErrUnsupportedFormat
	assert.ErrorAs(t, err, &unsupported)
}

func TestErrUnsupportedFormat_MessageIsDerivedFromAllowedExtensions(t *testing.T) {
	// The error message must list all currently allowed extensions, sorted, with
	// no hardcoded string — so adding a new extension automatically updates it.
	expected := make([]string, 0, len(extract.AllowedExtensions))
	for ext := range extract.AllowedExtensions {
		expected = append(expected, strings.TrimPrefix(ext, "."))
	}
	sort.Strings(expected)

	err := extract.ErrUnsupportedFormat{Ext: ".xyz"}
	assert.Contains(t, err.Error(), strings.Join(expected, ", "))
}

// --- checkImageMagic (magic-byte validation) ---

func TestFromFile_PNG_WithWrongMagicBytes_ReturnsError(t *testing.T) {
	// A file with a .png extension containing JPEG bytes must be rejected.
	tmp := filepath.Join(t.TempDir(), "fake.png")
	require.NoError(t, os.WriteFile(tmp, []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, 0o600))

	_, _, err := extract.FromFile(tmp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valid png")
}

func TestFromFile_JPEG_WithWrongMagicBytes_ReturnsError(t *testing.T) {
	// A file with a .jpg extension containing PNG bytes must be rejected.
	tmp := filepath.Join(t.TempDir(), "fake.jpg")
	require.NoError(t, os.WriteFile(tmp, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A}, 0o600))

	_, _, err := extract.FromFile(tmp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valid jpg")
}

func TestFromFile_WEBP_WithWrongMagicBytes_ReturnsError(t *testing.T) {
	// A file with a .webp extension that lacks the RIFF…WEBP signature must be rejected.
	tmp := filepath.Join(t.TempDir(), "fake.webp")
	require.NoError(t, os.WriteFile(tmp, []byte("not a webp file at all!!"), 0o600))

	_, _, err := extract.FromFile(tmp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "valid webp")
}

func TestFromFile_PNG_ValidMagicBytes_PassesMagicCheck(t *testing.T) {
	// A real PNG file must not be rejected by the magic check.
	_, _, err := extract.FromFile("testdata/hello_world.png")
	// Any error must not be a magic-byte rejection.
	if err != nil {
		assert.NotContains(t, err.Error(), "valid png")
	}
}

// --- File size limit ---

func TestFromFile_FileTooLarge_ReturnsErrFileTooLarge(t *testing.T) {
	// Create a sparse file larger than MaxFileBytes with a valid PNG header.
	tmp := filepath.Join(t.TempDir(), "huge.png")
	f, err := os.Create(tmp)
	require.NoError(t, err)
	_, err = f.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	require.NoError(t, err)
	// Seek past the limit and write one byte to produce a sparse file that exceeds MaxFileBytes.
	_, err = f.Seek(extract.MaxFileBytes, 0)
	require.NoError(t, err)
	_, err = f.Write([]byte{0x00})
	require.NoError(t, err)
	require.NoError(t, f.Close())

	_, _, err = extract.FromFile(tmp)
	require.Error(t, err)
	var tooLarge *extract.ErrFileTooLarge
	require.ErrorAs(t, err, &tooLarge)
}

func TestFromFile_SmallFile_NotRejectedForSize(t *testing.T) {
	// A small .txt file must never be rejected for exceeding the size limit.
	tmp := filepath.Join(t.TempDir(), "small.txt")
	require.NoError(t, os.WriteFile(tmp, []byte("hello world"), 0o600))

	_, _, err := extract.FromFile(tmp)
	require.NoError(t, err)
	// Belt-and-suspenders: also assert the error type even if NoError already covers it.
	var tooLarge *extract.ErrFileTooLarge
	assert.False(t, errors.As(err, &tooLarge))
}

// --- Image extraction (OCR via Vision framework) ---

func TestFromFile_PNG_ReturnsCorrectFileType(t *testing.T) {
	_, fileType, err := extract.FromFile("testdata/hello_world.png")
	require.NoError(t, err)
	assert.Equal(t, "png", fileType)
}

func TestFromFile_PNG_ExtractsText(t *testing.T) {
	text, _, err := extract.FromFile("testdata/hello_world.png")
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(text), "hello")
}

func TestFromFile_PNG_BlankImage_ReturnsErrNoTextFound(t *testing.T) {
	// blank.png has no text — must be rejected with ErrNoTextFound so the
	// upload is visible to the user rather than silently stored as empty.
	_, _, err := extract.FromFile("testdata/blank.png")
	require.Error(t, err)
	var noText *extract.ErrNoTextFound
	require.ErrorAs(t, err, &noText)
}
