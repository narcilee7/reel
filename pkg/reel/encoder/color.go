// Package encoder implements the pure pixel-processing helpers shared by the
// image protocols: RGB to 256-color quantization, box downsampling and sixel
// encoding.
package encoder

import "image/color"

// ansi16 holds the RGB values of the 16 standard xterm colors.
var ansi16 = [16][3]int{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
	{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

// cubeLevel returns the xterm color-cube level (0-5) nearest to v, where
// level values map to 0, 95, 135, 175, 215, 255.
func cubeLevel(v uint8) int {
	if v < 48 {
		return 0
	}
	if v < 115 {
		return 1
	}
	return (int(v) - 35) / 40
}

// cubeComponent maps a cube level back to its channel value.
func cubeComponent(level int) int {
	if level == 0 {
		return 0
	}
	return 55 + 40*level
}

func distRGB(c color.RGBA, r, g, b int) int {
	dr := int(c.R) - r
	dg := int(c.G) - g
	db := int(c.B) - b
	return dr*dr + dg*dg + db*db
}

// Quantize256 maps an RGBA color to the xterm 256-color palette, returning
// the palette index. Alpha is ignored; fully transparent pixels should be
// composited by the caller first.
func Quantize256(c color.RGBA) uint8 {
	// The cube is separable, so the nearest cube entry is the per-channel
	// nearest level. Compare it against the analytic gray-ramp best and the
	// 16 standard colors.
	lr, lg, lb := cubeLevel(c.R), cubeLevel(c.G), cubeLevel(c.B)
	best := 16 + 36*lr + 6*lg + lb
	bestDist := distRGB(c, cubeComponent(lr), cubeComponent(lg), cubeComponent(lb))

	avg := (int(c.R) + int(c.G) + int(c.B)) / 3
	// Gray ramp: 8 + 10*i, closest entry to avg clamped to [0, 23].
	gi := (avg - 8 + 5) / 10
	if gi < 0 {
		gi = 0
	}
	if gi > 23 {
		gi = 23
	}
	gy := 8 + 10*gi
	if d := distRGB(c, gy, gy, gy); d < bestDist {
		best, bestDist = 232+gi, d
	}

	for i, rgb := range ansi16 {
		if d := distRGB(c, rgb[0], rgb[1], rgb[2]); d < bestDist {
			best, bestDist = i, d
		}
	}
	return uint8(best)
}
