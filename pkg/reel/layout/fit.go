package layout

import (
	"image"
	"math"
)

// FitOptions controls how an image is fitted onto the cell grid.
type FitOptions struct {
	MaxWidth  int // max width in cells; zero means the grid width
	MaxHeight int // max height in cells; zero means the grid height
}

// Fit computes the cell rectangle an image should occupy, preserving aspect
// ratio and rounding up to whole cells so following text stays aligned.
func (g *Grid) Fit(img image.Image, opts FitOptions) CellRect {
	b := img.Bounds()
	pxW, pxH := b.Dx(), b.Dy()

	cw, ch := g.CellWidth, g.CellHeight
	if cw <= 0 {
		cw = 1
	}
	if ch <= 0 {
		ch = 1
	}

	cellW := float64(pxW) / float64(cw)
	cellH := float64(pxH) / float64(ch)

	maxW := opts.MaxWidth
	if maxW <= 0 {
		maxW = g.Cols
	}
	if maxW > 0 && cellW > float64(maxW) {
		scale := float64(maxW) / cellW
		cellW = float64(maxW)
		cellH *= scale
	}

	maxH := opts.MaxHeight
	if maxH <= 0 {
		maxH = g.Rows
	}
	if maxH > 0 && cellH > float64(maxH) {
		scale := float64(maxH) / cellH
		cellH = float64(maxH)
		cellW *= scale
	}

	w := int(math.Ceil(cellW))
	h := int(math.Ceil(cellH))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return CellRect{Width: w, Height: h}
}
