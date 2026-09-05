package encoder

import (
	"image"
	"image/color"
)

// BoxResize returns img resampled to w×h pixels by box (area-average)
// filtering. It targets terminal cell resolutions, where quality interpolation
// is wasted work; regions smaller than one source pixel fall back to nearest
// sampling. The result is always an *image.RGBA.
func BoxResize(img image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	b := img.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	for y := 0; y < h; y++ {
		// Source row range (exclusive upper bound) covered by this dst row.
		y0 := float64(y) * sh / float64(h)
		y1 := float64(y+1) * sh / float64(h)
		for x := 0; x < w; x++ {
			x0 := float64(x) * sw / float64(w)
			x1 := float64(x+1) * sw / float64(w)
			dst.SetRGBA(x, y, boxAverage(img, b, x0, y0, x1, y1))
		}
	}
	return dst
}

// boxAverage averages the source pixels overlapping [x0,x1)×[y0,y1).
func boxAverage(img image.Image, b image.Rectangle, x0, y0, x1, y1 float64) color.RGBA {
	dx, dy := b.Dx(), b.Dy()
	ix0 := clampInt(int(x0), dx-1)
	iy0 := clampInt(int(y0), dy-1)
	ix1 := clampInt(int(x1+0.5), dx)
	iy1 := clampInt(int(y1+0.5), dy)
	if ix1 <= ix0 {
		ix1 = ix0 + 1
	}
	if iy1 <= iy0 {
		iy1 = iy0 + 1
	}
	var r, g, bl, a, n uint64
	for y := iy0; y < iy1; y++ {
		for x := ix0; x < ix1; x++ {
			c := color.RGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.RGBA)
			r += uint64(c.R)
			g += uint64(c.G)
			bl += uint64(c.B)
			a += uint64(c.A)
			n++
		}
	}
	if n == 0 {
		return color.RGBAModel.Convert(img.At(b.Min.X+ix0, b.Min.Y+iy0)).(color.RGBA)
	}
	return color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), uint8(a / n)}
}

func clampInt(v, max int) int {
	if v < 0 {
		return 0
	}
	if v > max {
		return max
	}
	return v
}
