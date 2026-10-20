// Package imaging validates, resizes and stores the pictures a user uploads.
package imaging

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// MaxUploadBytes is the hard limit for a single uploaded picture.
const MaxUploadBytes int64 = 5 << 20 // 5 MiB

// MinDimension is the smallest accepted width/height of an upload.
const MinDimension = 128

// Result describes what was written to disk.
type Result struct {
	File  string // file name inside the upload directory
	Thumb string // file name of the square thumbnail
}

// ErrUnsupported is returned when the uploaded file is not a supported image.
var ErrUnsupported = errors.New("unsupported image type")

// ErrTooSmall is returned when the uploaded image is smaller than MinDimension.
var ErrTooSmall = errors.New("image is too small")

// ErrTooLarge is returned when the uploaded file exceeds the size limit.
var ErrTooLarge = errors.New("image is too large")

// Detect inspects the first bytes of an upload and returns the sniffed content
// type. The browser supplied type is never trusted.
func Detect(head []byte) (string, error) {
	switch http.DetectContentType(head) {
	case "image/jpeg":
		return "image/jpeg", nil
	case "image/png":
		return "image/png", nil
	case "image/gif":
		return "image/gif", nil
	default:
		return "", ErrUnsupported
	}
}

// Save validates the uploaded picture and writes a master image plus a square
// thumbnail into dir. Both files get a random name so a user can never choose
// the path of a served file.
func Save(dir string, src io.Reader) (Result, error) {
	buf, err := io.ReadAll(io.LimitReader(src, MaxUploadBytes+1))
	if err != nil {
		return Result{}, fmt.Errorf("read upload: %w", err)
	}
	if int64(len(buf)) > MaxUploadBytes {
		return Result{}, ErrTooLarge
	}
	if len(buf) == 0 {
		return Result{}, ErrUnsupported
	}
	if _, err := Detect(buf[:min(len(buf), 512)]); err != nil {
		return Result{}, err
	}

	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return Result{}, ErrUnsupported
	}
	bounds := img.Bounds()
	if bounds.Dx() < MinDimension || bounds.Dy() < MinDimension {
		return Result{}, ErrTooSmall
	}

	master := resizeToFit(img, 1024)
	thumb := centerCrop(master, 256)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create upload directory: %w", err)
	}
	name, err := randomName(".jpg")
	if err != nil {
		return Result{}, err
	}
	thumbName, err := randomName(".jpg")
	if err != nil {
		return Result{}, err
	}

	if err := writeJPEG(filepath.Join(dir, name), master, 85); err != nil {
		return Result{}, err
	}
	if err := writeJPEG(filepath.Join(dir, thumbName), thumb, 82); err != nil {
		os.Remove(filepath.Join(dir, name)) //nolint:errcheck
		return Result{}, err
	}
	return Result{File: name, Thumb: thumbName}, nil
}

// Remove deletes the files of a picture, ignoring missing files.
func Remove(dir string, names ...string) {
	for _, name := range names {
		if name == "" || strings.ContainsAny(name, `/\`) {
			continue
		}
		os.Remove(filepath.Join(dir, name)) //nolint:errcheck
	}
}

func writeJPEG(path string, img image.Image, quality int) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create image file: %w", err)
	}
	defer file.Close()
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: quality}); err != nil {
		return fmt.Errorf("encode image: %w", err)
	}
	return nil
}

// EncodePNG writes an image as PNG (used for tests and fixtures).
func EncodePNG(w io.Writer, img image.Image) error { return png.Encode(w, img) }

func randomName(ext string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate file name: %w", err)
	}
	return hex.EncodeToString(buf) + ext, nil
}

// resizeToFit scales the image so that neither side exceeds max, keeping the
// aspect ratio. A simple bilinear filter is used, which is more than good
// enough for avatars and keeps the project dependency free.
func resizeToFit(src image.Image, max int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		return clone(src)
	}
	scale := float64(max) / float64(maxInt(w, h))
	nw, nh := int(float64(w)*scale+0.5), int(float64(h)*scale+0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := b.Min.Y + int(float64(y)/scale)
		if sy > b.Max.Y-1 {
			sy = b.Max.Y - 1
		}
		for x := 0; x < nw; x++ {
			sx := b.Min.X + int(float64(x)/scale)
			if sx > b.Max.X-1 {
				sx = b.Max.X - 1
			}
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

// centerCrop cuts a centered square of size out of the image.
func centerCrop(src image.Image, size int) image.Image {
	b := src.Bounds()
	side := minInt(b.Dx(), b.Dy())
	left := b.Min.X + (b.Dx()-side)/2
	top := b.Min.Y + (b.Dy()-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			draw.Draw(dst, image.Rect(x, y, x+1, y+1), src, image.Pt(left+x*side/size, top+y*side/size), draw.Src)
		}
	}
	return dst
}

func clone(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int { return minInt(a, b) }
