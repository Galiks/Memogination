// Package media processes uploaded images for Memomarium: it validates,
// decodes, strips metadata, and generates screen/thumbnail variants.
package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/nfnt/resize"
	"golang.org/x/image/webp"
)

// Sentinel errors returned by ProcessUpload.
var (
	// ErrFileTooLarge is returned when the upload exceeds MaxBytes.
	ErrFileTooLarge = errors.New("file too large")
	// ErrUnsupportedFile is returned when the file is not a supported image.
	ErrUnsupportedFile = errors.New("unsupported file type")
	// ErrInvalidImage is returned when the file cannot be decoded as an image.
	ErrInvalidImage = errors.New("invalid image")
)

// Service processes image uploads into cleaned originals plus screen and
// thumbnail variants.
type Service struct {
	// UploadsDir is the directory where processed files are written.
	UploadsDir string
	// MaxBytes is the maximum accepted upload size in bytes.
	MaxBytes int64
}

// Result describes the files produced by ProcessUpload. All paths are relative
// to UploadsDir.
type Result struct {
	OriginalPath  string
	ScreenPath    string
	ThumbnailPath string
	MimeType      string
	SHA256        string
	Width         int
	Height        int
}

// ProcessUpload validates, decodes, cleans, and stores an uploaded image.
//
// Pipeline: size check -> MIME sniffing -> decode -> orientation correction
// (best-effort) -> metadata stripping via re-encode -> SHA-256 -> screen and
// thumbnail variants -> atomic writes. Variants are never upscaled.
func (s *Service) ProcessUpload(ctx context.Context, filename string, data []byte) (*Result, error) {
	if int64(len(data)) > s.MaxBytes {
		return nil, ErrFileTooLarge
	}

	mime := detectMIME(data)
	if mime == "" {
		return nil, ErrUnsupportedFile
	}

	img, err := decodeImage(data, mime)
	if err != nil {
		return nil, ErrInvalidImage
	}
	img = applyOrientation(img, exifOrientation(data))
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	if width == 0 || height == 0 {
		return nil, ErrInvalidImage
	}

	// Strip metadata by re-encoding the original. WebP has no pure-Go encoder,
	// so WebP originals are re-encoded losslessly as PNG.
	origBytes, origMime, err := encodeOriginal(img, mime)
	if err != nil {
		return nil, ErrInvalidImage
	}

	sum := sha256.Sum256(origBytes)
	sha := hex.EncodeToString(sum[:])

	screenBytes, err := encodeJPEG(resizeToMax(img, 1920))
	if err != nil {
		return nil, ErrInvalidImage
	}
	thumbBytes, err := encodeJPEG(resizeToMax(img, 480))
	if err != nil {
		return nil, ErrInvalidImage
	}

	id := uuid.NewString()
	origName := id + extForMime(origMime)
	screenName := id + ".screen.jpg"
	thumbName := id + ".thumb.jpg"

	if err := s.writeAtomic(origName, origBytes); err != nil {
		return nil, err
	}
	if err := s.writeAtomic(screenName, screenBytes); err != nil {
		return nil, err
	}
	if err := s.writeAtomic(thumbName, thumbBytes); err != nil {
		return nil, err
	}

	return &Result{
		OriginalPath:  origName,
		ScreenPath:    screenName,
		ThumbnailPath: thumbName,
		MimeType:      origMime,
		SHA256:        sha,
		Width:         width,
		Height:        height,
	}, nil
}

// RemoveResult deletes the files produced by ProcessUpload. It is used to
// clean up orphaned files when an upload turns out to be a duplicate and is
// not persisted. It is best-effort and ignores errors.
func (s *Service) RemoveResult(res *Result) {
	for _, name := range []string{res.OriginalPath, res.ScreenPath, res.ThumbnailPath} {
		if name == "" {
			continue
		}
		_ = os.Remove(filepath.Join(s.UploadsDir, name))
	}
}

// writeAtomic writes data to a temp file in UploadsDir and renames it to the
// final name, so readers never observe a partially-written file.
func (s *Service) writeAtomic(name string, data []byte) error {
	final := filepath.Join(s.UploadsDir, name)
	tmp, err := os.CreateTemp(s.UploadsDir, ".upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, final); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// detectMIME sniffs the magic bytes of data and returns the MIME type for
// supported formats (jpeg/png/webp), or "" if unsupported.
func detectMIME(data []byte) string {
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
		return "image/png"
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return ""
}

func decodeImage(data []byte, mime string) (image.Image, error) {
	switch mime {
	case "image/jpeg":
		return jpeg.Decode(bytes.NewReader(data))
	case "image/png":
		return png.Decode(bytes.NewReader(data))
	case "image/webp":
		return webp.Decode(bytes.NewReader(data))
	}
	return nil, ErrUnsupportedFile
}

// encodeOriginal re-encodes img to strip metadata, preserving the format when
// possible. WebP is re-encoded as PNG (no pure-Go WebP encoder).
func encodeOriginal(img image.Image, mime string) ([]byte, string, error) {
	var buf bytes.Buffer
	switch mime {
	case "image/jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/jpeg", nil
	case "image/png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	case "image/webp":
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	}
	return nil, "", ErrUnsupportedFile
}

func encodeJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// resizeToMax scales img down so its longest side is at most max pixels. It
// never upscales.
func resizeToMax(img image.Image, max int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		return img
	}
	ratio := float64(max) / float64(maxInt(w, h))
	nw := int(float64(w) * ratio)
	nh := int(float64(h) * ratio)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	return resize.Resize(uint(nw), uint(nh), img, resize.Lanczos3)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func extForMime(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	}
	return ".bin"
}

// exifOrientation reads the EXIF orientation tag (0x0112) from a JPEG APP1
// segment. It returns 1 (normal) when no orientation is present or parsing
// fails, so callers can treat it as best-effort.
func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		marker := data[i+1]
		if marker == 0xD8 || marker == 0xD9 || (marker >= 0xD0 && marker <= 0xD7) {
			i += 2
			continue
		}
		if i+4 > len(data) {
			break
		}
		segLen := int(data[i+2])<<8 | int(data[i+3])
		if marker == 0xE1 && segLen >= 8 && i+10 <= len(data) {
			if string(data[i+4:i+10]) == "Exif\x00\x00" {
				return parseTIFFOrientation(data[i+10:])
			}
		}
		i += 2 + segLen
	}
	return 1
}

// parseTIFFOrientation parses the orientation tag from a TIFF/EXIF block.
func parseTIFFOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(tiff[0:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	if bo.Uint16(tiff[2:4]) != 0x002A {
		return 1
	}
	ifdOffset := int(bo.Uint32(tiff[4:8]))
	if ifdOffset+2 > len(tiff) {
		return 1
	}
	count := int(bo.Uint16(tiff[ifdOffset : ifdOffset+2]))
	pos := ifdOffset + 2
	for i := 0; i < count; i++ {
		if pos+12 > len(tiff) {
			return 1
		}
		tag := bo.Uint16(tiff[pos : pos+2])
		typ := bo.Uint16(tiff[pos+2 : pos+4])
		if tag == 0x0112 && typ == 3 {
			val := bo.Uint16(tiff[pos+8 : pos+10])
			if val >= 1 && val <= 8 {
				return int(val)
			}
			return 1
		}
		pos += 12
	}
	return 1
}

// applyOrientation transforms img according to an EXIF orientation value
// (1-8). Orientation 1 (normal) returns img unchanged.
func applyOrientation(img image.Image, o int) image.Image {
	if o == 1 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var outW, outH int
	switch o {
	case 5, 6, 7, 8:
		outW, outH = h, w
	default:
		outW, outH = w, h
	}
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.At(b.Min.X+x, b.Min.Y+y)
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			dst.Set(nx, ny, c)
		}
	}
	return dst
}
