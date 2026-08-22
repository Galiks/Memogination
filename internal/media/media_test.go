package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func newService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	return &Service{UploadsDir: dir, MaxBytes: 1 << 20}
}

func TestProcessUploadValidPNG(t *testing.T) {
	s := newService(t)
	data := makePNG(t, 100, 80)

	res, err := s.ProcessUpload(context.Background(), "meme.png", data)
	if err != nil {
		t.Fatalf("ProcessUpload: %v", err)
	}

	// Three files must exist.
	for _, p := range []string{res.OriginalPath, res.ScreenPath, res.ThumbnailPath} {
		if p == "" {
			t.Fatalf("empty path in result: %+v", res)
		}
		if _, err := os.Stat(filepath.Join(s.UploadsDir, p)); err != nil {
			t.Fatalf("expected file %q: %v", p, err)
		}
	}

	if res.MimeType != "image/png" {
		t.Errorf("MimeType = %q, want image/png", res.MimeType)
	}
	if res.Width != 100 || res.Height != 80 {
		t.Errorf("dims = %dx%d, want 100x80", res.Width, res.Height)
	}
	if len(res.SHA256) != 64 {
		t.Errorf("SHA256 length = %d, want 64", len(res.SHA256))
	}

	// Screen and thumbnail must be JPEG (magic bytes FF D8 FF).
	for _, p := range []string{res.ScreenPath, res.ThumbnailPath} {
		b, err := os.ReadFile(filepath.Join(s.UploadsDir, p))
		if err != nil {
			t.Fatalf("read %q: %v", p, err)
		}
		if len(b) < 3 || b[0] != 0xFF || b[1] != 0xD8 || b[2] != 0xFF {
			t.Errorf("%q is not a JPEG", p)
		}
	}
}

func TestProcessUploadUnsupportedType(t *testing.T) {
	s := newService(t)
	_, err := s.ProcessUpload(context.Background(), "note.txt", []byte("hello world, definitely not an image"))
	if !errors.Is(err, ErrUnsupportedFile) {
		t.Fatalf("err = %v, want ErrUnsupportedFile", err)
	}
}

func TestProcessUploadOversized(t *testing.T) {
	s := newService(t)
	s.MaxBytes = 10
	_, err := s.ProcessUpload(context.Background(), "big.png", makePNG(t, 50, 50))
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("err = %v, want ErrFileTooLarge", err)
	}
}

func TestProcessUploadCorruptImage(t *testing.T) {
	s := newService(t)
	// Valid PNG magic bytes but truncated/corrupt payload.
	corrupt := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, []byte("garbage")...)
	_, err := s.ProcessUpload(context.Background(), "corrupt.png", corrupt)
	if !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("err = %v, want ErrInvalidImage", err)
	}
}
