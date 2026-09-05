package encoder

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func solidImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestEncodeSixelEmpty(t *testing.T) {
	if _, err := EncodeSixel(image.NewRGBA(image.Rectangle{}), 256); err == nil {
		t.Fatal("expected error for empty image")
	}
}

func TestEncodeSixelSolid(t *testing.T) {
	payload, err := EncodeSixel(solidImage(8, 8, color.RGBA{255, 0, 0, 255}), 256)
	if err != nil {
		t.Fatal(err)
	}
	// One color definition: #1;2;r;g;b with sixel 0-100 components.
	if !bytes.HasPrefix(payload, []byte("#1;2;")) {
		t.Errorf("payload missing color definition: %q", payload[:min(16, len(payload))])
	}
	// Red at full intensity: components 100;0;0.
	if !bytes.Contains(payload, []byte(";100;0;0")) {
		t.Errorf("payload missing red definition: %q", payload)
	}
	// A solid 8x8 image is one band of solid sixel data terminated by '$'.
	if !bytes.HasSuffix(payload, []byte("$")) {
		t.Errorf("payload missing band terminator: %q", payload[len(payload)-8:])
	}
	// Solid color over 8 columns: RLE must kick in (!8~ or similar).
	if !bytes.Contains(payload, []byte("!")) {
		t.Errorf("expected RLE in solid payload: %q", payload)
	}
}

func TestEncodeSixelBands(t *testing.T) {
	// 13 pixels tall spans three bands (6+6+1).
	payload, err := EncodeSixel(solidImage(4, 13, color.RGBA{0, 0, 255, 255}), 256)
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(payload, []byte("-")); got != 2 {
		t.Errorf("band separators = %d, want 2", got)
	}
}

func TestEncodeSixelMaxColors(t *testing.T) {
	// Two distinct colors, palette capped at one: must still encode.
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if x < 2 {
				img.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{0, 0, 255, 255})
			}
		}
	}
	if _, err := EncodeSixel(img, 1); err != nil {
		t.Fatal(err)
	}
}
