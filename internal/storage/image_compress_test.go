package storage

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"testing"
)

func noise(w, h int) *image.RGBA {
	r := rand.New(rand.NewSource(1))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r.Read(img.Pix)
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 0xff
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func dims(t *testing.T, data []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func TestCompressPhotoBecomesCappedJPEG(t *testing.T) {
	out, ext := compressImage(encodePNG(t, noise(3200, 1600)), ".png")
	if ext != ".jpg" {
		t.Fatalf("ext %q", ext)
	}
	if w, h := dims(t, out); w != maxImageEdge || h != 1280 {
		t.Errorf("size %dx%d", w, h)
	}
}

// A screenshot or diagram keeps PNG, so text edges stay sharp.
func TestCompressGraphicStaysPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3000, 1000))
	for y := 0; y < 1000; y++ {
		for x := 0; x < 3000; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x / 300 * 20), uint8(y / 100 * 20), 40, 0xff})
		}
	}
	out, ext := compressImage(encodePNG(t, img), ".png")
	if ext != ".png" {
		t.Fatalf("ext %q", ext)
	}
	if w, _ := dims(t, out); w != maxImageEdge {
		t.Errorf("width %d", w)
	}

	clear := image.NewRGBA(image.Rect(0, 0, 400, 400))
	if _, ext := compressImage(encodePNG(t, clear), ".png"); ext != ".png" {
		t.Errorf("transparent image became %q", ext)
	}
}

// Re-encoding a small image that wouldn't get much smaller only loses quality.
func TestCompressKeepsWhatItCannotImprove(t *testing.T) {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, noise(600, 400), &jpeg.Options{Quality: 70}); err != nil {
		t.Fatal(err)
	}
	in := b.Bytes()
	if out, ext := compressImage(in, ".jpg"); ext != ".jpg" || !bytes.Equal(out, in) {
		t.Error("small JPEG was re-encoded")
	}
	for _, ext := range []string{".gif", ".webp", ".svg", ".avif"} {
		if out, got := compressImage([]byte("x"), ext); got != ext || string(out) != "x" {
			t.Errorf("%s was touched", ext)
		}
	}
}

// A phone photo stored sideways with an EXIF turn comes out upright, with the EXIF gone.
func TestCompressAppliesEXIFOrientation(t *testing.T) {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, noise(400, 200), &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x01\x01\x12\x00\x03\x00\x00\x00\x01\x00\x06\x00\x00\x00\x00\x00\x00")
	seg := append([]byte("Exif\x00\x00"), tiff...)
	app1 := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(seg)+2))
	in := append(append(append([]byte{}, b.Bytes()[:2]...), append(app1, seg...)...), b.Bytes()[2:]...)
	if jpegOrientation(in) != 6 {
		t.Fatal("orientation not read")
	}

	out, _ := compressImage(in, ".jpg")
	if w, h := dims(t, out); w != 200 || h != 400 {
		t.Errorf("size %dx%d, want 200x400", w, h)
	}
	if jpegOrientation(out) != 1 {
		t.Error("EXIF kept")
	}
}
