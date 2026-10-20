package imaging

import (
	"bytes"
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
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 120, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestSaveWritesMasterAndThumbnail(t *testing.T) {
	dir := t.TempDir()
	result, err := Save(dir, bytes.NewReader(makePNG(t, 800, 600)))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if result.File == "" || result.Thumb == "" {
		t.Fatal("expected a file and a thumbnail")
	}
	for _, name := range []string{result.File, result.Thumb} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
}

func TestSaveRejectsNonImages(t *testing.T) {
	dir := t.TempDir()
	if _, err := Save(dir, bytes.NewReader([]byte("not an image at all"))); err != ErrUnsupported {
		t.Fatalf("err = %v, want %v", err, ErrUnsupported)
	}
}

func TestSaveRejectsTinyImages(t *testing.T) {
	dir := t.TempDir()
	if _, err := Save(dir, bytes.NewReader(makePNG(t, 32, 32))); err != ErrTooSmall {
		t.Fatalf("err = %v, want %v", err, ErrTooSmall)
	}
}

func TestSaveRejectsOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	big := bytes.Repeat([]byte("a"), int(MaxUploadBytes)+16)
	if _, err := Save(dir, bytes.NewReader(big)); err != ErrTooLarge {
		t.Fatalf("err = %v, want %v", err, ErrTooLarge)
	}
}

func TestDetectContentTypes(t *testing.T) {
	if _, err := Detect(makePNG(t, 10, 10)[:16]); err != nil {
		t.Fatalf("detect png: %v", err)
	}
	if _, err := Detect([]byte("plain text file")); err != ErrUnsupported {
		t.Fatalf("err = %v, want %v", err, ErrUnsupported)
	}
}

func TestRemoveIgnoresPaths(t *testing.T) {
	dir := t.TempDir()
	// A name with a path separator must never delete anything.
	Remove(dir, "../escape.jpg", "")
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.jpg")); err == nil {
		t.Fatal("Remove deleted a file outside of the upload directory")
	}
}
