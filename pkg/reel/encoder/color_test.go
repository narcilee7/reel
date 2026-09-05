package encoder

import (
	"image/color"
	"testing"
)

func TestQuantize256ExactColors(t *testing.T) {
	cases := []struct {
		c    color.RGBA
		want uint8
	}{
		{color.RGBA{0, 0, 0, 255}, 16},        // cube(0,0,0)
		{color.RGBA{255, 255, 255, 255}, 231}, // cube(5,5,5)
		{color.RGBA{255, 0, 0, 255}, 196},     // cube(5,0,0)
		{color.RGBA{95, 135, 175, 255}, 67},   // cube(1,2,3) = 16+36+12+3
		{color.RGBA{128, 128, 128, 255}, 244}, // gray ramp 8+10*12=128
		{color.RGBA{238, 238, 238, 255}, 255}, // last gray entry
		{color.RGBA{0, 205, 0, 255}, 2},       // standard green
	}
	for _, tc := range cases {
		if got := Quantize256(tc.c); got != tc.want {
			t.Errorf("Quantize256(%v) = %d, want %d", tc.c, got, tc.want)
		}
	}
}

func TestQuantize256Ranges(t *testing.T) {
	for r := 0; r < 256; r += 7 {
		for g := 0; g < 256; g += 11 {
			for b := 0; b < 256; b += 13 {
				idx := Quantize256(color.RGBA{uint8(r), uint8(g), uint8(b), 255})
				if idx > 255 {
					t.Fatalf("index out of range: %d", idx)
				}
			}
		}
	}
}

func TestQuantize256GrayPrefersRamp(t *testing.T) {
	// A mid gray must land on the gray ramp, not a desaturated cube color.
	idx := Quantize256(color.RGBA{100, 100, 100, 255})
	if idx < 232 || idx > 255 {
		t.Errorf("gray 100 quantized to %d, want a gray-ramp index", idx)
	}
}
