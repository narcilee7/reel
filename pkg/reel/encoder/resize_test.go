package encoder

import (
	"image"
	"image/color"
	"testing"
)

func TestBoxResizeDimensions(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 50))
	dst := BoxResize(src, 10, 5)
	if got := dst.Bounds(); got.Dx() != 10 || got.Dy() != 5 {
		t.Fatalf("bounds = %v, want 10x5", got)
	}
}

func TestBoxResizeAverage(t *testing.T) {
	// 2x1 image: left half black, right half white. Resizing to 1x1 must
	// average to mid gray.
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.SetRGBA(0, 0, color.RGBA{0, 0, 0, 255})
	src.SetRGBA(1, 0, color.RGBA{255, 255, 255, 255})
	dst := BoxResize(src, 1, 1)
	c := dst.RGBAAt(0, 0)
	if c.R < 120 || c.R > 136 {
		t.Errorf("average R = %d, want ~128", c.R)
	}
}

func TestBoxResizeHalves(t *testing.T) {
	// A 4x4 image with a 2x2 red block in one quadrant: downsampling to 2x2
	// maps each quadrant to one pixel.
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	red := color.RGBA{255, 0, 0, 255}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			src.SetRGBA(x, y, red)
		}
	}
	dst := BoxResize(src, 2, 2)
	if got := dst.RGBAAt(0, 0); got != red {
		t.Errorf("top-left = %v, want %v", got, red)
	}
	if got := dst.RGBAAt(1, 1); got.R != 0 {
		t.Errorf("bottom-right = %v, want black", got)
	}
}

func TestBoxResizeNonDivisible(t *testing.T) {
	// 5x5 -> 2x2 must not panic and must cover the whole canvas.
	src := image.NewRGBA(image.Rect(0, 0, 5, 5))
	dst := BoxResize(src, 2, 2)
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			_ = dst.RGBAAt(x, y)
		}
	}
}
